package internalhttp

import (
	"fmt"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

type eventDTO struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	StartsAt     string  `json:"startsAt"`
	Duration     string  `json:"duration"`
	Description  string  `json:"description,omitempty"`
	UserID       string  `json:"userId"`
	NotifyBefore *string `json:"notifyBefore,omitempty"`
}

func (e eventDTO) domain() (storage.Event, error) {
	start, err := time.Parse(time.RFC3339Nano, e.StartsAt)
	if err != nil {
		return storage.Event{}, fmt.Errorf("%w: startsAt must be RFC3339", storage.ErrInvalidEvent)
	}
	duration, err := time.ParseDuration(e.Duration)
	if err != nil {
		return storage.Event{}, fmt.Errorf("%w: invalid duration", storage.ErrInvalidEvent)
	}
	event := storage.Event{
		ID: e.ID, Title: e.Title, StartsAt: start.UTC(), Duration: duration,
		Description: e.Description, UserID: e.UserID,
	}
	if e.NotifyBefore != nil {
		notify, err := time.ParseDuration(*e.NotifyBefore)
		if err != nil {
			return storage.Event{}, fmt.Errorf("%w: invalid notifyBefore", storage.ErrInvalidEvent)
		}
		event.NotifyBefore = &notify
	}
	return event, event.Validate()
}

func eventResponse(e storage.Event) eventDTO {
	result := eventDTO{
		ID: e.ID, Title: e.Title, StartsAt: e.StartsAt.Format(time.RFC3339Nano), Duration: e.Duration.String(),
		Description: e.Description, UserID: e.UserID,
	}
	if e.NotifyBefore != nil {
		notify := e.NotifyBefore.String()
		result.NotifyBefore = &notify
	}
	return result
}
