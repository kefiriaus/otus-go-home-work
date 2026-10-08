package sqlstorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

// Use a dedicated test database: the migration creates the events table.
func integrationStorage(t *testing.T) *Storage {
	t.Helper()
	dsn := os.Getenv("CALENDAR_TEST_DSN")
	if dsn == "" {
		t.Skip("set CALENDAR_TEST_DSN to run real PostgreSQL storage tests")
	}
	s := New(dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "001_events.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	return s
}

func sqlFixture(prefix, id string) storage.Event {
	reminder := 30 * time.Minute
	return storage.Event{
		ID: prefix + id, Title: "Meeting", UserID: prefix + "alice", Description: "Project",
		StartsAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Duration: time.Hour, NotifyBefore: &reminder,
	}
}

func TestPostgresCRUD(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("crud-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		_, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE id LIKE $1", prefix+"%")
		if err != nil {
			t.Error(err)
		}
	})
	e := sqlFixture(prefix, "one")
	if err := s.CreateEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEvent(ctx, e); !errors.Is(err, storage.ErrAlreadyExists) {
		t.Fatalf("duplicate: %v", err)
	}
	got, err := s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	event := eventByID(t, got, e.ID)
	if event.Title != e.Title || event.Description != e.Description || event.UserID != e.UserID ||
		event.Duration != time.Hour || event.NotifyBefore == nil || *event.NotifyBefore != 30*time.Minute {
		t.Fatalf("roundtrip: %+v", event)
	}
	other := sqlFixture(prefix, "two")
	other.StartsAt = other.StartsAt.Add(time.Hour)
	other.NotifyBefore = nil
	if err := s.CreateEvent(ctx, other); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	other.StartsAt = e.StartsAt
	if err := s.UpdateEvent(ctx, other.ID, other); !errors.Is(err, storage.ErrDateBusy) {
		t.Fatalf("overlap: %v", err)
	}
	preserved, err := s.ListEvents(ctx, e.StartsAt.Add(time.Hour), e.StartsAt.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	eventByID(t, preserved, other.ID)
	if slices.ContainsFunc(got, func(event storage.Event) bool { return event.ID == other.ID }) {
		t.Fatal("exclusive upper boundary included adjacent event")
	}
	other.StartsAt = e.StartsAt.Add(2 * time.Hour)
	other.Title = "Updated"
	if err := s.UpdateEvent(ctx, other.ID, other); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListEvents(ctx, other.StartsAt, other.StartsAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	event = eventByID(t, got, other.ID)
	if event.Title != "Updated" || event.NotifyBefore != nil {
		t.Fatalf("update: %+v", event)
	}
	if err := s.DeleteEvent(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteEvent(ctx, e.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
	if err := s.UpdateEvent(ctx, e.ID, e); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
}

func TestPostgresConcurrentOverlap(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("race-%d-", time.Now().UnixNano())
	t.Cleanup(func() {
		_, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE id LIKE $1", prefix+"%")
		if err != nil {
			t.Error(err)
		}
	})
	var wg sync.WaitGroup
	var winners atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := s.CreateEvent(ctx, sqlFixture(prefix, fmt.Sprint(i)))
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, storage.ErrDateBusy) {
				t.Errorf("insert: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("overlapping winners: %d", winners.Load())
	}
}

func TestConnectionFailure(t *testing.T) {
	s := New("postgres://user:secret@127.0.0.1:1/database?connect_timeout=1&sslmode=disable")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Connect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("connect cancellation: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRangePrecision(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	e := sqlFixture(fmt.Sprintf("precision-%d-", time.Now().UnixNano()), "one")
	if err := s.CreateEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.DeleteEvent(ctx, e.ID); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		name     string
		from, to time.Time
		want     bool
	}{
		{"lower", e.StartsAt.Add(time.Nanosecond), e.StartsAt.Add(time.Hour), false},
		{"upper", e.StartsAt.Add(-time.Hour), e.StartsAt.Add(time.Nanosecond), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, err := s.ListEvents(ctx, tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, event := range events {
				if event.ID == e.ID {
					found = true
				}
			}
			if found != tc.want {
				t.Fatalf("event present: %v, want %v", found, tc.want)
			}
		})
	}
}

func TestMalformedDSNRedaction(t *testing.T) {
	s := New("postgres://user:private-test-secret@localhost:not-a-port/database")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := s.Connect(ctx)
	if err == nil {
		t.Fatal("invalid DSN accepted")
	}
	if strings.Contains(err.Error(), "private-test-secret") {
		t.Fatalf("credentials leaked: %v", err)
	}
}

func eventByID(t *testing.T, events []storage.Event, id string) storage.Event {
	t.Helper()
	for _, event := range events {
		if event.ID == id {
			return event
		}
	}
	t.Fatalf("event %q missing from listing", id)
	return storage.Event{}
}
