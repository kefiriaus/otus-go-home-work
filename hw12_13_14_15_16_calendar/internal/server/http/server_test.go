package internalhttp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/logger"
)

func TestHelloAndRequestLogging(t *testing.T) {
	var out bytes.Buffer
	s := NewServer(logger.New("info", &out), "127.0.0.1:0")
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/hello?q=1", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("User-Agent", "calendar-test")
	rr := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "Hello, calendar!\n" {
		t.Fatalf("response: %d %q", rr.Code, rr.Body.String())
	}
	for _, field := range []string{
		"192.0.2.1", "GET", "/hello?q=1", "HTTP/1.1", "200", "latency", "calendar-test", "time=",
	} {
		if !strings.Contains(out.String(), field) {
			t.Fatalf("missing %s in %s", field, out.String())
		}
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost/missing", nil)
	s.httpServer.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown route: %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://localhost/hello", nil)
	s.httpServer.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method: %d", rr.Code)
	}
}

func TestLoggedStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"implicit", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }, "status=200"},
		{"first", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201); w.WriteHeader(500) }, "status=201"},
		{"informational", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(103)
			w.WriteHeader(201)
		}, "status=201"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			h := loggingMiddleware(logger.New("info", &out), tc.handler)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/hello", nil)
			h.ServeHTTP(httptest.NewRecorder(), req)
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("status log: %s", out.String())
			}
		})
	}
}

func TestServerLifecycle(t *testing.T) {
	s := NewServer(logger.New("error"), "127.0.0.1:0")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	stop, cancelStop := context.WithTimeout(context.Background(), time.Second)
	defer cancelStop()
	if err := s.Stop(stop); err != nil {
		t.Fatal(err)
	}
}
