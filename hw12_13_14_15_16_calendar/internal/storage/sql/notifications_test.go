package sqlstorage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

func TestPostgresReminders(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("reminder-%d-", time.Now().UnixNano())
	t.Cleanup(func() { _, _ = s.db.ExecContext(ctx, "DELETE FROM events WHERE id LIKE $1", prefix+"%") })
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, kind := range []string{"due", "future", "none", "ended"} {
		e := sqlFixture(prefix, kind)
		e.UserID = e.ID
		e.StartsAt = now.Add(10 * time.Minute)
		if kind == "future" {
			e.StartsAt = now.Add(time.Hour)
		}
		if kind == "none" {
			e.NotifyBefore = nil
		}
		if kind == "ended" {
			e.StartsAt = now.Add(-2 * time.Hour)
		}
		if err := s.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	var sent atomic.Int32
	publish := func(_ context.Context, n notification.Notification) error {
		if n.EventID != prefix+"due" {
			t.Errorf("unexpected: %+v", n)
		}
		sent.Add(1)
		return n.Validate()
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.DispatchDue(ctx, now, 10, publish); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if sent.Load() != 1 {
		t.Fatalf("concurrent delivery count: %d", sent.Load())
	}
	if count, err := s.DispatchDue(ctx, now, 10, publish); err != nil || count != 0 {
		t.Fatalf("repeat: %d, %v", count, err)
	}
	e := sqlFixture(prefix, "due")
	e.UserID = e.ID
	e.StartsAt = now.Add(5 * time.Minute)
	if err := s.UpdateEvent(ctx, e.ID, e); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("broker failed")
	failedPublish := func(context.Context, notification.Notification) error { return failure }
	_, err := s.DispatchDue(ctx, now, 10, failedPublish)
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if count, err := s.DispatchDue(ctx, now, 10, publish); err != nil || count != 1 {
		t.Fatalf("retry after failed publish: %d, %v", count, err)
	}
}

func TestPostgresCleanup(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	prefix := fmt.Sprintf("cleanup-%d-", time.Now().UnixNano())
	t.Cleanup(func() { _, _ = s.db.ExecContext(ctx, "DELETE FROM events WHERE id LIKE $1", prefix+"%") })
	cutoff := time.Date(1900, 10, 14, 10, 0, 0, 0, time.UTC)
	old := sqlFixture(prefix, "old")
	old.StartsAt = cutoff.Add(-2 * time.Hour)
	old.UserID = old.ID
	boundary := sqlFixture(prefix, "boundary")
	boundary.StartsAt = cutoff.Add(-time.Hour)
	boundary.UserID = boundary.ID
	for _, e := range []storage.Event{old, boundary} {
		if err := s.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	if count, err := s.DeleteBefore(ctx, cutoff); err != nil || count != 1 {
		t.Fatalf("cleanup: %d, %v", count, err)
	}
	if err := s.DeleteEvent(ctx, boundary.ID); err != nil {
		t.Fatalf("boundary removed: %v", err)
	}
}
