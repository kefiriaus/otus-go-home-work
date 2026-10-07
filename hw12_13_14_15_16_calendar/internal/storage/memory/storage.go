package memorystorage

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

type Storage struct {
	mu     sync.RWMutex
	events map[string]storage.Event
}

var _ storage.Storage = (*Storage)(nil)

func New() *Storage { return &Storage{events: make(map[string]storage.Event)} }

func (s *Storage) CreateEvent(ctx context.Context, e storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := s.events[e.ID]; ok {
		return storage.ErrAlreadyExists
	}
	if s.overlaps(e, "") {
		return storage.ErrDateBusy
	}
	s.events[e.ID] = e.Clone()
	return nil
}

func (s *Storage) UpdateEvent(ctx context.Context, id string, e storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id != e.ID {
		return storage.ErrInvalidEvent
	}
	if err := e.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := s.events[id]; !ok {
		return storage.ErrNotFound
	}
	if s.overlaps(e, id) {
		return storage.ErrDateBusy
	}
	s.events[id] = e.Clone()
	return nil
}

func (s *Storage) DeleteEvent(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := s.events[id]; !ok {
		return storage.ErrNotFound
	}
	delete(s.events, id)
	return nil
}

func (s *Storage) ListEvents(ctx context.Context, from, to time.Time) ([]storage.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := storage.ValidateRange(from, to); err != nil {
		return nil, err
	}
	s.mu.RLock()
	if err := ctx.Err(); err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	events := make([]storage.Event, 0)
	for _, e := range s.events {
		if !e.StartsAt.Before(from) && e.StartsAt.Before(to) {
			events = append(events, e.Clone())
		}
	}
	s.mu.RUnlock()
	sort.Slice(events, func(i, j int) bool {
		if events[i].StartsAt.Equal(events[j].StartsAt) {
			return events[i].ID < events[j].ID
		}
		return events[i].StartsAt.Before(events[j].StartsAt)
	})
	return events, nil
}

// overlaps must be called with the write lock held.
func (s *Storage) overlaps(e storage.Event, except string) bool {
	for id, existing := range s.events {
		if id != except && existing.UserID == e.UserID &&
			e.StartsAt.Before(existing.EndsAt()) && existing.StartsAt.Before(e.EndsAt()) {
			return true
		}
	}
	return false
}
