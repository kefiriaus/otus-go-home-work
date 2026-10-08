package memorystorage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

func fixture(id string) storage.Event {
	reminder := 30 * time.Minute
	return storage.Event{
		ID: id, Title: "Meeting", UserID: "alice",
		StartsAt: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC), Duration: time.Hour,
		Description: "Project", NotifyBefore: &reminder,
	}
}

func TestCRUD(t *testing.T) {
	ctx := context.Background()
	s := New()
	e := fixture("one")
	if err := s.CreateEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateEvent(ctx, e); !errors.Is(err, storage.ErrAlreadyExists) {
		t.Fatalf("duplicate: %v", err)
	}
	*e.NotifyBefore = 0
	got, err := s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil || len(got) != 1 {
		t.Fatalf("list: %v, %v", got, err)
	}
	if *got[0].NotifyBefore != 30*time.Minute {
		t.Fatal("input aliases stored event")
	}
	*got[0].NotifyBefore = 0
	got[0].Title = "mutated"
	got, err = s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil || got[0].Title != "Meeting" || *got[0].NotifyBefore != 30*time.Minute {
		t.Fatal("output aliases stored event")
	}
	e.Title = "Updated"
	if err := s.UpdateEvent(ctx, e.ID, e); err != nil {
		t.Fatal(err)
	}
	got, err = s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil || got[0].Title != "Updated" {
		t.Fatalf("update: %v, %v", got, err)
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

func TestOverlapAndOrdering(t *testing.T) {
	s := New()
	ctx := context.Background()
	one := fixture("one")
	if err := s.CreateEvent(ctx, one); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []time.Duration{-30 * time.Minute, 0, 30 * time.Minute} {
		e := fixture("overlap")
		e.StartsAt = e.StartsAt.Add(offset)
		if err := s.CreateEvent(ctx, e); !errors.Is(err, storage.ErrDateBusy) {
			t.Fatalf("overlap: %v", err)
		}
	}
	other := fixture("other")
	other.UserID = "bob"
	if err := s.CreateEvent(ctx, other); err != nil {
		t.Fatal(err)
	}
	adjacent := fixture("adjacent")
	adjacent.StartsAt = adjacent.StartsAt.Add(time.Hour)
	if err := s.CreateEvent(ctx, adjacent); err != nil {
		t.Fatal(err)
	}
	adjacent.StartsAt = one.StartsAt
	if err := s.UpdateEvent(ctx, adjacent.ID, adjacent); !errors.Is(err, storage.ErrDateBusy) {
		t.Fatalf("update overlap: %v", err)
	}
	got, err := s.ListEvents(ctx, one.StartsAt, one.StartsAt.Add(time.Hour))
	if err != nil || len(got) != 2 || got[0].ID != "one" || got[1].ID != "other" {
		t.Fatalf("range/order: %v, %v", got, err)
	}
	got, err = s.ListEvents(ctx, one.StartsAt.Add(time.Hour), one.StartsAt.Add(2*time.Hour))
	if err != nil || len(got) != 1 || got[0].ID != "adjacent" {
		t.Fatalf("failed update changed data: %v, %v", got, err)
	}
	if err := s.UpdateEvent(ctx, one.ID, one); err != nil {
		t.Fatalf("self update: %v", err)
	}
}

func TestInvalidAndCanceled(t *testing.T) {
	s := New()
	ctx := context.Background()
	for _, mutate := range []func(*storage.Event){
		func(e *storage.Event) { e.ID = "" },
		func(e *storage.Event) { e.Title = " " },
		func(e *storage.Event) { e.UserID = "" },
		func(e *storage.Event) { e.StartsAt = time.Time{} },
		func(e *storage.Event) { e.Duration = 0 },
		func(e *storage.Event) { *e.NotifyBefore = -1 },
	} {
		e := fixture("invalid")
		mutate(&e)
		if err := s.CreateEvent(ctx, e); !errors.Is(err, storage.ErrInvalidEvent) {
			t.Fatalf("invalid event: %v", err)
		}
	}
	if _, err := s.ListEvents(ctx, time.Now(), time.Time{}); !errors.Is(err, storage.ErrInvalidRange) {
		t.Fatalf("range: %v", err)
	}
	e := fixture("one")
	if err := s.UpdateEvent(ctx, "different", e); !errors.Is(err, storage.ErrInvalidEvent) {
		t.Fatalf("ID mismatch: %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, err := range []error{
		s.CreateEvent(canceled, e),
		s.UpdateEvent(canceled, e.ID, e),
		s.DeleteEvent(canceled, e.ID),
	} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	}
	if _, err := s.ListEvents(canceled, e.StartsAt, e.StartsAt.Add(time.Hour)); !errors.Is(err, context.Canceled) {
		t.Fatalf("list cancellation: %v", err)
	}
}

func TestConcurrentOverlap(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	var winners atomic.Int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := fixture(fmt.Sprint(i))
			err := s.CreateEvent(context.Background(), e)
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, storage.ErrDateBusy) {
				t.Errorf("create: %v", err)
			}
			if _, err := s.ListEvents(context.Background(), e.StartsAt, e.StartsAt.Add(time.Hour)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("overlapping winners: %d", winners.Load())
	}
}

func TestConcurrentCRUD(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			e := fixture(fmt.Sprint(i))
			e.UserID = e.ID
			if err := s.CreateEvent(ctx, e); err != nil {
				t.Error(err)
				return
			}
			e.Title = "updated"
			if err := s.UpdateEvent(ctx, e.ID, e); err != nil {
				t.Error(err)
			}
			if _, err := s.ListEvents(ctx, e.StartsAt, e.StartsAt.Add(time.Hour)); err != nil {
				t.Error(err)
			}
			if err := s.DeleteEvent(ctx, e.ID); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	e := fixture("range")
	got, err := s.ListEvents(context.Background(), e.StartsAt, e.StartsAt.Add(time.Hour))
	if err != nil || len(got) != 0 {
		t.Fatalf("remaining: %v, %v", got, err)
	}
}

func TestRangePrecision(t *testing.T) {
	s := New()
	ctx := context.Background()
	e := fixture("one")
	if err := s.CreateEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListEvents(ctx, e.StartsAt.Add(time.Nanosecond), e.StartsAt.Add(time.Hour))
	if err != nil || len(events) != 0 {
		t.Fatalf("lower bound: %v, %v", events, err)
	}
	events, err = s.ListEvents(ctx, e.StartsAt.Add(-time.Hour), e.StartsAt.Add(time.Nanosecond))
	if err != nil || len(events) != 1 {
		t.Fatalf("upper bound: %v, %v", events, err)
	}
}
