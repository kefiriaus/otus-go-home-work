package internalgrpc

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/app"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/logger"
	calendarpb "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/pb"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	memorystorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/memory"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func grpcClient(t *testing.T) calendarpb.EventServiceClient {
	t.Helper()
	var logs bytes.Buffer
	s := NewServer(logger.New("info", &logs), "127.0.0.1:0", app.New(memorystorage.New()))
	listener := bufconn.Listen(1 << 20)
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///calendar", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(); s.grpcServer.Stop(); _ = listener.Close() })
	return calendarpb.NewEventServiceClient(conn)
}

func grpcEvent(id string) *calendarpb.Event {
	return &calendarpb.Event{
		Id: id, Title: "Meeting", UserId: "alice",
		StartsAt: timestamppb.New(time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)), Duration: durationpb.New(time.Hour),
		NotifyBefore: durationpb.New(0),
	}
}

func TestGRPCCRUD(t *testing.T) {
	client := grpcClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	event := grpcEvent("one")
	if _, err := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: event}); err != nil {
		t.Fatal(err)
	}
	_, duplicateErr := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: event})
	if err := duplicateErr; status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate: %v", err)
	}
	request := &calendarpb.ListEventsRequest{Date: "2026-10-13"}
	type listRPC func(context.Context, *calendarpb.ListEventsRequest, ...grpc.CallOption) (
		*calendarpb.ListEventsResponse, error)
	for _, list := range []listRPC{
		client.ListEventsOnDay, client.ListEventsOnWeek, client.ListEventsOnMonth,
	} {
		result, err := list(ctx, request)
		if err != nil || len(result.GetEvents()) != 1 || result.Events[0].NotifyBefore == nil {
			t.Fatalf("list: %v, %v", result, err)
		}
	}
	event.Title = "Updated"
	event.NotifyBefore = nil
	if _, err := client.UpdateEvent(ctx, &calendarpb.UpdateEventRequest{Id: event.Id, Event: event}); err != nil {
		t.Fatal(err)
	}
	result, err := client.ListEventsOnDay(ctx, request)
	if err != nil || result.Events[0].Title != "Updated" || result.Events[0].NotifyBefore != nil {
		t.Fatalf("update: %v, %v", result, err)
	}
	if _, err := client.DeleteEvent(ctx, &calendarpb.DeleteEventRequest{Id: event.Id}); err != nil {
		t.Fatal(err)
	}
	_, missingErr := client.DeleteEvent(ctx, &calendarpb.DeleteEventRequest{Id: event.Id})
	if err := missingErr; status.Code(err) != codes.NotFound {
		t.Fatalf("missing: %v", err)
	}
	result, err = client.ListEventsOnDay(ctx, request)
	if err != nil || len(result.GetEvents()) != 0 {
		t.Fatalf("empty: %v, %v", result, err)
	}
}

func TestGRPCValidation(t *testing.T) {
	client := grpcClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, mutate := range []func(*calendarpb.Event){
		func(e *calendarpb.Event) { e.StartsAt = nil }, func(e *calendarpb.Event) { e.StartsAt.Nanos = -1 },
		func(e *calendarpb.Event) { e.Duration = nil }, func(e *calendarpb.Event) { e.Duration.Seconds = 9223372037 },
		func(e *calendarpb.Event) { e.NotifyBefore.Nanos = -1 }, func(e *calendarpb.Event) { e.Title = "" },
	} {
		event := grpcEvent("invalid")
		mutate(event)
		_, err := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: event})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("validation: %v", err)
		}
	}
	if _, err := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("missing event: %v", err)
	}
	update := &calendarpb.UpdateEventRequest{Id: "other", Event: grpcEvent("one")}
	_, updateErr := client.UpdateEvent(ctx, update)
	if err := updateErr; status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ID mismatch: %v", err)
	}
	for _, request := range []*calendarpb.ListEventsRequest{
		{}, {Date: "2026-02-30"}, {Date: "2026-10-13", Timezone: "bad"},
	} {
		if _, err := client.ListEventsOnDay(ctx, request); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("date: %v", err)
		}
	}
}

func TestGRPCOverlapAndLogging(t *testing.T) {
	client := grpcClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: grpcEvent("one")}); err != nil {
		t.Fatal(err)
	}
	_, overlapErr := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: grpcEvent("two")})
	if err := overlapErr; status.Code(err) != codes.AlreadyExists {
		t.Fatalf("overlap: %v", err)
	}
}

func TestGRPCLoggingInterceptor(t *testing.T) {
	var logs bytes.Buffer
	interceptor := loggingInterceptor(logger.New("info", &logs))
	info := &grpc.UnaryServerInfo{FullMethod: "/calendar.v1.EventService/CreateEvent"}
	_, err := interceptor(context.Background(), nil, info,
		func(context.Context, any) (any, error) {
			return nil, status.Error(codes.InvalidArgument, "invalid event")
		})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	for _, field := range []string{"CreateEvent", "InvalidArgument", "latency=", "client_ip=", "request_time="} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("missing %s: %s", field, logs.String())
		}
	}
}

func TestGRPCErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want codes.Code
	}{
		{context.Canceled, codes.Canceled},
		{context.DeadlineExceeded, codes.DeadlineExceeded},
		{errors.New("database-secret"), codes.Internal},
	} {
		err := rpcError(tc.err)
		if status.Code(err) != tc.want || strings.Contains(err.Error(), "database-secret") {
			t.Fatalf("mapped error: %v", err)
		}
	}
}

func TestGRPCDateWindows(t *testing.T) {
	client := grpcClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, e := range []struct{ id, start string }{
		{"before", "2026-03-07T23:00:00-05:00"},
		{"day", "2026-03-08T00:00:00-05:00"},
		{"next", "2026-03-09T00:00:00-04:00"},
		{"week-end", "2026-03-15T00:00:00-04:00"},
		{"month-end", "2026-04-01T00:00:00-04:00"},
	} {
		event := grpcEvent(e.id)
		event.UserId = e.id
		start, err := time.Parse(time.RFC3339, e.start)
		if err != nil {
			t.Fatal(err)
		}
		event.StartsAt = timestamppb.New(start)
		if _, err := client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: event}); err != nil {
			t.Fatal(err)
		}
	}
	type listRPC func(context.Context, *calendarpb.ListEventsRequest, ...grpc.CallOption) (
		*calendarpb.ListEventsResponse, error)
	for _, tc := range []struct {
		list listRPC
		want int
	}{
		{client.ListEventsOnDay, 1}, {client.ListEventsOnWeek, 2}, {client.ListEventsOnMonth, 4},
	} {
		result, err := tc.list(ctx, &calendarpb.ListEventsRequest{Date: "2026-03-08", Timezone: "America/New_York"})
		if err != nil || len(result.GetEvents()) != tc.want {
			t.Fatalf("calendar window: %v, %v", result, err)
		}
	}
}

type blockedApplication struct {
	server.Application
	entered chan struct{}
	release chan struct{}
}

func (a *blockedApplication) CreateEvent(context.Context, storage.Event) error {
	close(a.entered)
	<-a.release
	return nil
}

func TestForcedShutdownDoesNotWaitForBlockedRPC(t *testing.T) {
	calendar := &blockedApplication{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewServer(logger.New("error"), "127.0.0.1:0", calendar)
	listener := bufconn.Listen(1 << 20)
	go func() { _ = s.grpcServer.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///calendar", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { close(calendar.release); _ = conn.Close(); s.grpcServer.Stop(); _ = listener.Close() })
	client := calendarpb.NewEventServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _, _ = client.CreateEvent(ctx, &calendarpb.CreateEventRequest{Event: grpcEvent("blocked")}) }()
	select {
	case <-calendar.entered:
	case <-ctx.Done():
		t.Fatal("RPC did not start")
	}
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelShutdown()
	result := make(chan error, 1)
	go func() { result <- s.Stop(shutdown) }()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("forced shutdown waits for a blocked handler")
	}
}

func TestRunningServerDeadlineWithBlockedRPC(t *testing.T) {
	calendar := &blockedApplication{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewServer(logger.New("error"), "127.0.0.1:0", calendar)
	listener := bufconn.Listen(1 << 20)
	serverContext, cancelServer := context.WithCancel(context.Background())
	defer cancelServer()
	result := make(chan error, 1)
	go func() { result <- s.serve(serverContext, listener) }()
	conn, err := grpc.NewClient("passthrough:///calendar", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { close(calendar.release); _ = conn.Close(); s.grpcServer.Stop(); _ = listener.Close() })
	client := calendarpb.NewEventServiceClient(conn)
	requestContext, cancelRequest := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelRequest()
	go func() {
		_, _ = client.CreateEvent(requestContext, &calendarpb.CreateEventRequest{Event: grpcEvent("blocked")})
	}()
	select {
	case <-calendar.entered:
	case <-requestContext.Done():
		t.Fatal("RPC did not start")
	}
	cancelServer()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("server shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("server waits indefinitely for a blocked RPC")
	}
}
