//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreateAndBusinessErrors(t *testing.T) {
	env := setup(t)
	v := fixture(t, time.Date(2035, 1, 10, 12, 0, 0, 0, time.UTC))
	env.create(t, v)
	missing := v
	missing.ID = "missing"
	conflict := v
	conflict.ID += "-conflict"
	const eventsPath = "/events"
	cases := []struct {
		name, method, path string
		input              any
		status             int
		message            string
	}{
		{"duplicate", http.MethodPost, eventsPath, v, http.StatusConflict, "event already exists"},
		{"invalid", http.MethodPost, eventsPath, event{}, http.StatusBadRequest, "invalid event"},
		{"missing update", http.MethodPut, eventsPath + "/missing", missing, http.StatusNotFound, "event not found"},
		{"missing delete", http.MethodDelete, eventsPath + "/missing", nil, http.StatusNotFound, "event not found"},
		{"time conflict", http.MethodPost, eventsPath, conflict, http.StatusConflict, "event time is already occupied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := env.request(t, tc.method, tc.path, tc.input, tc.status)
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(data, &body); err != nil || body.Error != tc.message {
				t.Fatalf("error body %s: %v", data, err)
			}
		})
	}
}

func TestUpdateAndDelete(t *testing.T) {
	env := setup(t)
	v := fixture(t, time.Date(2036, 2, 10, 12, 0, 0, 0, time.UTC))
	env.create(t, v)
	v.Title = "Updated title"
	env.request(t, http.MethodPut, "/events/"+v.ID, v, http.StatusOK)
	events := env.list(t, "day", "2036-02-10")
	found := false
	for _, got := range events {
		if got.ID == v.ID {
			found = true
			if got.Title != v.Title {
				t.Fatalf("unchanged title: %+v", got)
			}
		}
	}
	if !found {
		t.Fatal("updated event missing")
	}
	env.request(t, http.MethodDelete, "/events/"+v.ID, nil, http.StatusNoContent)
	for _, got := range env.list(t, "day", "2036-02-10") {
		if got.ID == v.ID {
			t.Fatal("deleted event listed")
		}
	}
}

func TestPeriodListings(t *testing.T) {
	env := setup(t)
	for _, tc := range []struct {
		period, date string
		start, end   time.Time
	}{
		{"day", "2037-03-10", time.Date(2037, 3, 10, 0, 0, 0, 0, time.UTC), time.Date(2037, 3, 11, 0, 0, 0, 0, time.UTC)},
		{"week", "2038-04-12", time.Date(2038, 4, 12, 0, 0, 0, 0, time.UTC), time.Date(2038, 4, 19, 0, 0, 0, 0, time.UTC)},
		{"month", "2039-05-01", time.Date(2039, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2039, 6, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.period, func(t *testing.T) {
			want := map[string]bool{}
			for i, start := range []time.Time{tc.start.Add(-time.Hour), tc.start, tc.start.Add(time.Hour), tc.end} {
				v := fixture(t, start)
				env.create(t, v)
				if i == 1 || i == 2 {
					want[v.ID] = true
				}
			}
			got := env.list(t, tc.period, tc.date)
			if len(got) != len(want) {
				t.Fatalf("got %d events want %d: %+v", len(got), len(want), got)
			}
			for _, v := range got {
				if !want[v.ID] {
					t.Fatalf("unexpected event: %+v", v)
				}
				delete(want, v.ID)
			}
			if len(want) != 0 {
				t.Fatalf("missing events: %v", want)
			}
		})
	}
}

func TestNotificationDelivery(t *testing.T) {
	env := setup(t)
	now := time.Now().UTC().Truncate(time.Second)
	reminder := "1h"
	due := fixture(t, now.Add(10*time.Minute))
	due.NotifyBefore = &reminder
	future := fixture(t, now.Add(24*time.Hour))
	future.NotifyBefore = &reminder
	env.create(t, due)
	env.create(t, future)
	await(t, 20*time.Second, func() bool { return env.receiptCount(t, due.ID) == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var start time.Time
	query := "SELECT starts_at FROM notification_deliveries WHERE event_id=$1"
	if err := env.db.QueryRowContext(ctx, query, due.ID).Scan(&start); err != nil {
		t.Fatal(err)
	}
	if !start.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("wrong occurrence: %v", start)
	}
	// Observe several scheduler intervals to exclude premature delivery.
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			return
		case <-ticker.C:
			if env.receiptCount(t, future.ID) != 0 {
				t.Fatal("future reminder delivered early")
			}
		}
	}
}
