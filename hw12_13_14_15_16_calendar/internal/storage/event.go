package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("event not found")
	ErrAlreadyExists = errors.New("event already exists")
	ErrDateBusy      = errors.New("event time is already occupied")
	ErrInvalidEvent  = errors.New("invalid event")
	ErrInvalidRange  = errors.New("invalid date range")
)

// Event describes an interval owned by one user. NotifyBefore is optional.
type Event struct {
	ID           string
	Title        string
	StartsAt     time.Time
	Duration     time.Duration
	Description  string
	UserID       string
	NotifyBefore *time.Duration
}

// Storage lists events whose start times lie in [from, to), ordered by start and ID.
type Storage interface {
	CreateEvent(context.Context, Event) error
	UpdateEvent(context.Context, string, Event) error
	DeleteEvent(context.Context, string) error
	ListEvents(context.Context, time.Time, time.Time) ([]Event, error)
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.Title) == "" || strings.TrimSpace(e.UserID) == "" {
		return fmt.Errorf("%w: ID, title and user ID are required", ErrInvalidEvent)
	}
	// Match PostgreSQL timestamps and its microsecond resolution in both backends.
	if e.StartsAt.IsZero() || e.StartsAt.Year() < 1 || e.StartsAt.Year() > 9999 ||
		e.Duration <= 0 || e.EndsAt().Year() > 9999 ||
		e.StartsAt.Nanosecond()%1000 != 0 || e.Duration%time.Microsecond != 0 {
		return fmt.Errorf("%w: require a valid start and positive duration with microsecond precision", ErrInvalidEvent)
	}
	if e.NotifyBefore != nil && *e.NotifyBefore < 0 {
		return fmt.Errorf("%w: reminder must be nonnegative", ErrInvalidEvent)
	}
	return nil
}

func (e Event) EndsAt() time.Time { return e.StartsAt.Add(e.Duration) }

// Clone prevents callers from mutating an optional reminder stored in memory.
func (e Event) Clone() Event {
	if e.NotifyBefore != nil {
		d := *e.NotifyBefore
		e.NotifyBefore = &d
	}
	return e
}

func ValidateRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return ErrInvalidRange
	}
	return nil
}
