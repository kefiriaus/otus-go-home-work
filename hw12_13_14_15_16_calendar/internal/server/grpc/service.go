package internalgrpc

import (
	"context"
	"errors"
	"strings"
	"time"

	calendarpb "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/pb"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type service struct {
	calendarpb.UnimplementedEventServiceServer
	app server.Application
}

func (s *service) CreateEvent(
	ctx context.Context,
	request *calendarpb.CreateEventRequest,
) (*calendarpb.EventResponse, error) {
	event, err := domainEvent(request.GetEvent())
	if err != nil {
		return nil, rpcError(err)
	}
	if err := s.app.CreateEvent(ctx, event); err != nil {
		return nil, rpcError(err)
	}
	return &calendarpb.EventResponse{Event: protobufEvent(event)}, nil
}

func (s *service) UpdateEvent(
	ctx context.Context,
	request *calendarpb.UpdateEventRequest,
) (*calendarpb.EventResponse, error) {
	event, err := domainEvent(request.GetEvent())
	if err != nil {
		return nil, rpcError(err)
	}
	if strings.TrimSpace(request.GetId()) == "" || request.GetId() != event.ID {
		return nil, rpcError(storage.ErrInvalidEvent)
	}
	if err := s.app.UpdateEvent(ctx, request.GetId(), event); err != nil {
		return nil, rpcError(err)
	}
	return &calendarpb.EventResponse{Event: protobufEvent(event)}, nil
}

func (s *service) DeleteEvent(ctx context.Context, request *calendarpb.DeleteEventRequest) (*emptypb.Empty, error) {
	if strings.TrimSpace(request.GetId()) == "" {
		return nil, rpcError(storage.ErrInvalidEvent)
	}
	if err := s.app.DeleteEvent(ctx, request.GetId()); err != nil {
		return nil, rpcError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *service) ListEventsOnDay(
	ctx context.Context,
	r *calendarpb.ListEventsRequest,
) (*calendarpb.ListEventsResponse, error) {
	return listEvents(ctx, r, s.app.ListEventsOnDay)
}

func (s *service) ListEventsOnWeek(
	ctx context.Context,
	r *calendarpb.ListEventsRequest,
) (*calendarpb.ListEventsResponse, error) {
	return listEvents(ctx, r, s.app.ListEventsOnWeek)
}

func (s *service) ListEventsOnMonth(
	ctx context.Context,
	r *calendarpb.ListEventsRequest,
) (*calendarpb.ListEventsResponse, error) {
	return listEvents(ctx, r, s.app.ListEventsOnMonth)
}

func listEvents(ctx context.Context, r *calendarpb.ListEventsRequest,
	list func(context.Context, time.Time) ([]storage.Event, error),
) (*calendarpb.ListEventsResponse, error) {
	date, err := server.ParseDate(r.GetDate(), r.GetTimezone())
	if err != nil {
		return nil, rpcError(err)
	}
	events, err := list(ctx, date)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &calendarpb.ListEventsResponse{Events: make([]*calendarpb.Event, 0, len(events))}
	for _, event := range events {
		response.Events = append(response.Events, protobufEvent(event))
	}
	return response, nil
}

func rpcError(err error) error {
	switch {
	case errors.Is(err, storage.ErrInvalidEvent):
		return status.Error(codes.InvalidArgument, "invalid event")
	case errors.Is(err, storage.ErrInvalidRange):
		return status.Error(codes.InvalidArgument, "invalid date or timezone")
	case errors.Is(err, storage.ErrNotFound):
		return status.Error(codes.NotFound, "event not found")
	case errors.Is(err, storage.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "event already exists")
	case errors.Is(err, storage.ErrDateBusy):
		return status.Error(codes.AlreadyExists, "event time is already occupied")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
