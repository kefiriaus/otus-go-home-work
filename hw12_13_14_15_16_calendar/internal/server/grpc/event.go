package internalgrpc

import (
	"math"
	"time"

	calendarpb "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/pb"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func domainEvent(input *calendarpb.Event) (storage.Event, error) {
	if input == nil || input.StartsAt == nil || input.StartsAt.CheckValid() != nil {
		return storage.Event{}, storage.ErrInvalidEvent
	}
	duration, err := domainDuration(input.Duration)
	if err != nil {
		return storage.Event{}, err
	}
	event := storage.Event{
		ID: input.Id, Title: input.Title, StartsAt: input.StartsAt.AsTime(), Duration: duration,
		Description: input.Description, UserID: input.UserId,
	}
	if input.NotifyBefore != nil {
		notify, err := domainDuration(input.NotifyBefore)
		if err != nil {
			return storage.Event{}, err
		}
		event.NotifyBefore = &notify
	}
	return event, event.Validate()
}

func domainDuration(input *durationpb.Duration) (time.Duration, error) {
	if input == nil || input.CheckValid() != nil || input.Seconds < 0 || input.Nanos < 0 {
		return 0, storage.ErrInvalidEvent
	}
	const billion = int64(time.Second)
	if input.Seconds > math.MaxInt64/billion ||
		(input.Seconds == math.MaxInt64/billion && int64(input.Nanos) > math.MaxInt64%billion) {
		return 0, storage.ErrInvalidEvent
	}
	return time.Duration(input.Seconds)*time.Second + time.Duration(input.Nanos), nil
}

func protobufEvent(event storage.Event) *calendarpb.Event {
	result := &calendarpb.Event{
		Id: event.ID, Title: event.Title, StartsAt: timestamppb.New(event.StartsAt),
		Duration: durationpb.New(event.Duration), Description: event.Description, UserId: event.UserID,
	}
	if event.NotifyBefore != nil {
		result.NotifyBefore = durationpb.New(*event.NotifyBefore)
	}
	return result
}
