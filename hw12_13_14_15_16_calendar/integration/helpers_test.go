//go:build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

type event struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	StartsAt     string  `json:"startsAt"`
	Duration     string  `json:"duration"`
	UserID       string  `json:"userId"`
	NotifyBefore *string `json:"notifyBefore,omitempty"`
}

type environment struct {
	url    string
	client *http.Client
	db     *sql.DB
}

func setup(t *testing.T) *environment {
	t.Helper()
	address := os.Getenv("CALENDAR_API_URL")
	dsn := os.Getenv("CALENDAR_TEST_DSN")
	if address == "" || dsn == "" {
		t.Fatal("CALENDAR_API_URL and CALENDAR_TEST_DSN are required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	env := &environment{url: strings.TrimRight(address, "/"), client: &http.Client{Timeout: 2 * time.Second}, db: db}
	await(t, 30*time.Second, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if db.PingContext(ctx) != nil {
			return false
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, env.url+"/hello", nil)
		if err != nil {
			return false
		}
		res, err := env.client.Do(req)
		if err != nil {
			return false
		}
		defer func() { _ = res.Body.Close() }()
		return res.StatusCode == http.StatusOK
	})
	return env
}

func await(t *testing.T, timeout time.Duration, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("condition not met before deadline")
		case <-ticker.C:
		}
	}
}

func (e *environment) request(t *testing.T, method, path string, input any, status int) []byte {
	t.Helper()
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, e.url+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, res.StatusCode, status, data)
	}
	return data
}

func fixture(t *testing.T, start time.Time) event {
	t.Helper()
	id := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
	return event{
		ID: id, Title: "Integration meeting", StartsAt: start.Format(time.RFC3339Nano),
		Duration: "1m", UserID: id,
	}
}

func (e *environment) create(t *testing.T, v event) {
	t.Helper()
	data := e.request(t, http.MethodPost, "/events", v, http.StatusCreated)
	var response struct {
		Event event `json:"event"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if response.Event.ID != v.ID || response.Event.Title != v.Title || response.Event.StartsAt != v.StartsAt {
		t.Fatalf("created payload: %s", data)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, err := e.db.ExecContext(ctx, "DELETE FROM events WHERE id=$1", v.ID); err != nil {
			t.Error(err)
		}
		if _, err := e.db.ExecContext(ctx, "DELETE FROM notification_deliveries WHERE event_id=$1", v.ID); err != nil {
			t.Error(err)
		}
	})
}

func (e *environment) list(t *testing.T, period, date string) []event {
	t.Helper()
	data := e.request(t, http.MethodGet, "/events/"+period+"?date="+date, nil, http.StatusOK)
	var response struct {
		Events []event `json:"events"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	return response.Events
}

func (e *environment) receiptCount(t *testing.T, id string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var count int
	query := "SELECT count(*) FROM notification_deliveries WHERE event_id=$1"
	if err := e.db.QueryRowContext(ctx, query, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
