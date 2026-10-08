package internalhttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type Server struct{ httpServer *http.Server }

type Logger interface{ Info(string, ...any) }

// NewServer deliberately exposes only a hello-world route in homework 12.
func NewServer(log Logger, address string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Hello, calendar!\n"))
	})
	return &Server{httpServer: &http.Server{
		Addr:              address,
		Handler:           loggingMiddleware(log, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       time.Minute,
	}}
}

func (s *Server) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	default:
	}
	done := make(chan struct{})
	stopped := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			err := s.Stop(shutdown)
			if err != nil {
				_ = s.httpServer.Close()
			}
			stopped <- err
		case <-done:
			stopped <- nil
		}
	}()
	err := s.httpServer.ListenAndServe()
	close(done)
	shutdownErr := <-stopped
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return shutdownErr
}

func (s *Server) Stop(ctx context.Context) error { return s.httpServer.Shutdown(ctx) }
