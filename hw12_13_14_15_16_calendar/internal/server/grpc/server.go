package internalgrpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	calendarpb "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/pb"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server"
	"google.golang.org/grpc"
)

type Server struct {
	address    string
	grpcServer *grpc.Server
}

func NewServer(log Logger, address string, calendar server.Application) *Server {
	transport := grpc.NewServer(grpc.UnaryInterceptor(loggingInterceptor(log)))
	calendarpb.RegisterEventServiceServer(transport, &service{app: calendar})
	return &Server{address: address, grpcServer: transport}
}

func (s *Server) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}
	var config net.ListenConfig
	listener, err := config.Listen(ctx, "tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	return s.serve(ctx, listener)
}

func (s *Server) serve(ctx context.Context, listener net.Listener) error {
	served := make(chan error, 1)
	go func() { served <- s.grpcServer.Serve(listener) }()
	select {
	case err := <-served:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve gRPC: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		err := s.Stop(shutdown)
		if err != nil {
			_ = listener.Close()
		}
		return err
	}
}

func (s *Server) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() { s.grpcServer.GracefulStop(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// grpc-go may hold its mutex while waiting for graceful handlers.
		// Both forced Stop and Serve can block behind that wait.
		go s.grpcServer.Stop()
		return ctx.Err()
	}
}
