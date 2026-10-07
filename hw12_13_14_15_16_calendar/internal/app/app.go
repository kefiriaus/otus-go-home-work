package app

import (
	"context"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

// App implements calendar operations independently of the storage backend.
type App struct{ storage storage.Storage }

func New(s storage.Storage) *App { return &App{storage: s} }

func (a *App) CreateEvent(ctx context.Context, e storage.Event) error {
	return a.storage.CreateEvent(ctx, e)
}

func (a *App) UpdateEvent(ctx context.Context, id string, e storage.Event) error {
	return a.storage.UpdateEvent(ctx, id, e)
}

func (a *App) DeleteEvent(ctx context.Context, id string) error {
	return a.storage.DeleteEvent(ctx, id)
}

func (a *App) ListEventsOnDay(ctx context.Context, date time.Time) ([]storage.Event, error) {
	if date.IsZero() {
		return nil, storage.ErrInvalidRange
	}
	start := midnight(date)
	return a.storage.ListEvents(ctx, start, start.AddDate(0, 0, 1))
}

// ListEventsOnWeek treats date as the supplied first day of the week.
func (a *App) ListEventsOnWeek(ctx context.Context, date time.Time) ([]storage.Event, error) {
	if date.IsZero() {
		return nil, storage.ErrInvalidRange
	}
	start := midnight(date)
	return a.storage.ListEvents(ctx, start, start.AddDate(0, 0, 7))
}

func (a *App) ListEventsOnMonth(ctx context.Context, date time.Time) ([]storage.Event, error) {
	if date.IsZero() {
		return nil, storage.ErrInvalidRange
	}
	start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	return a.storage.ListEvents(ctx, start, start.AddDate(0, 1, 0))
}

func midnight(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}
