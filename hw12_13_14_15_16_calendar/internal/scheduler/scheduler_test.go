package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

type testStore struct {
	notice notification.Notification
	cutoff time.Time
	failed error
}

func (s *testStore) DispatchDue(ctx context.Context, _ time.Time, _ int,
	publish func(context.Context, notification.Notification) error,
) (int, error) {
	if s.failed != nil {
		return 0, s.failed
	}
	if err := publish(ctx, s.notice); err != nil {
		return 0, err
	}
	return 1, nil
}

func (s *testStore) DeleteBefore(_ context.Context, cutoff time.Time) (int64, error) {
	s.cutoff = cutoff
	return 1, nil
}

type testPublisher struct {
	got notification.Notification
	err error
}

func (p *testPublisher) Publish(_ context.Context, n notification.Notification) error {
	p.got = n
	return p.err
}

type quietLogger struct{}

func (quietLogger) Info(string, ...any)  {}
func (quietLogger) Error(string, ...any) {}

func TestTick(t *testing.T) {
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	notice := notification.Notification{EventID: "one", Title: "Meeting", UserID: "alice", StartsAt: now}
	store := &testStore{notice: notice}
	publisher := &testPublisher{}
	worker := New(store, publisher, quietLogger{}, time.Minute, 10, time.Second)
	if err := worker.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if publisher.got.EventID != "one" || !store.cutoff.Equal(time.Date(2025, 10, 14, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("tick: %+v, %v", publisher.got, store.cutoff)
	}
}

func TestTickPublishFailureStillCleans(t *testing.T) {
	store := &testStore{}
	publisher := &testPublisher{err: errors.New("broker unavailable")}
	worker := New(store, publisher, quietLogger{}, time.Minute, 10, time.Second)
	if err := worker.Tick(context.Background(), time.Now()); !errors.Is(err, publisher.err) {
		t.Fatalf("error: %v", err)
	}
	if store.cutoff.IsZero() {
		t.Fatal("cleanup skipped")
	}
}
