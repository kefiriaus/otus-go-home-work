// Package server defines the application boundary shared by transport adapters.
package server

import (
	"context"
	"fmt"
	"time"
	_ "time/tzdata" // Embed IANA data for minimal container images.

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

type Application interface {
	CreateEvent(context.Context, storage.Event) error
	UpdateEvent(context.Context, string, storage.Event) error
	DeleteEvent(context.Context, string) error
	ListEventsOnDay(context.Context, time.Time) ([]storage.Event, error)
	ListEventsOnWeek(context.Context, time.Time) ([]storage.Event, error)
	ListEventsOnMonth(context.Context, time.Time) ([]storage.Event, error)
}

func ParseDate(value, zone string) (time.Time, error) {
	if zone == "" {
		zone = "UTC"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid timezone", storage.ErrInvalidRange)
	}
	date, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil || date.IsZero() || date.Year() < 1 {
		return time.Time{}, fmt.Errorf("%w: date must be YYYY-MM-DD", storage.ErrInvalidRange)
	}
	return date, nil
}
