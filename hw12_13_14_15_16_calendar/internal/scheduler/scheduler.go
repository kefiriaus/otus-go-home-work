// Package scheduler dispatches due reminders and cleans old events.
package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

type Storage interface {
	DispatchDue(context.Context, time.Time, int, func(context.Context, notification.Notification) error) (int, error)
	DeleteBefore(context.Context, time.Time) (int64, error)
}
type Logger interface {
	Info(string, ...any)
	Error(string, ...any)
}
type Scheduler struct {
	store     Storage
	publisher notification.Publisher
	log       Logger
	interval  time.Duration
	batchSize int
	timeout   time.Duration
}

func New(store Storage, publisher notification.Publisher, log Logger,
	interval time.Duration, batch int, timeout time.Duration,
) *Scheduler {
	return &Scheduler{store: store, publisher: publisher, log: log, interval: interval, batchSize: batch, timeout: timeout}
}

func (s *Scheduler) Tick(ctx context.Context, now time.Time) error {
	operation, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	deleted, cleanupErr := s.store.DeleteBefore(operation, now.AddDate(-1, 0, 0))
	sent, sendErr := s.store.DispatchDue(operation, now, s.batchSize, s.publisher.Publish)
	s.log.Info("scheduler scan", "published", sent, "deleted", deleted)
	return errors.Join(cleanupErr, sendErr)
}

func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := s.Tick(ctx, time.Now().UTC()); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
