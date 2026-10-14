package internalhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/app"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/logger"
	memorystorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/memory"
)

const validEventBody = `{
 "id":"one","title":"Meeting","startsAt":"2026-10-13T10:00:00Z",
 "duration":"1h","userId":"alice","notifyBefore":"0s"
 }`

func apiServer() *Server {
	return NewServer(logger.New("error"), "127.0.0.1:0", app.New(memorystorage.New()))
}

func apiRequest(t *testing.T, s *Server, method, path, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(response, req)
	if response.Code != want {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.Code, want, response.Body.String())
	}
	return response
}

func TestHTTPCalendarCRUD(t *testing.T) {
	s := apiServer()
	apiRequest(t, s, http.MethodPost, "/events", validEventBody, http.StatusCreated)
	apiRequest(t, s, http.MethodPost, "/events", validEventBody, http.StatusConflict)
	for _, period := range []string{"day", "week", "month"} {
		response := apiRequest(t, s, http.MethodGet, "/events/"+period+"?date=2026-10-13", "", http.StatusOK)
		var result struct {
			Events []eventDTO `json:"events"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Events) != 1 || result.Events[0].ID != "one" || result.Events[0].NotifyBefore == nil ||
			*result.Events[0].NotifyBefore != "0s" {
			t.Fatalf("list: %+v", result)
		}
	}
	updated := `{"id":"one","title":"Updated","startsAt":"2026-10-13T10:00:00Z","duration":"30m","userId":"alice"}`
	apiRequest(t, s, http.MethodPut, "/events/one", updated, http.StatusOK)
	response := apiRequest(t, s, http.MethodGet, "/events/day?date=2026-10-13", "", http.StatusOK)
	var result struct {
		Events []eventDTO `json:"events"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Events[0].Title != "Updated" || result.Events[0].NotifyBefore != nil {
		t.Fatalf("update: %+v", result)
	}
	apiRequest(t, s, http.MethodDelete, "/events/one", "", http.StatusNoContent)
	apiRequest(t, s, http.MethodDelete, "/events/one", "", http.StatusNotFound)
	response = apiRequest(t, s, http.MethodGet, "/events/day?date=2026-10-13", "", http.StatusOK)
	if response.Body.String() != `{"events":[]}`+"\n" {
		t.Fatalf("empty list: %s", response.Body.String())
	}
	apiRequest(t, s, http.MethodPut, "/events/one", updated, http.StatusNotFound)
}

func TestHTTPInvalidRequests(t *testing.T) {
	s := apiServer()
	for _, body := range []string{
		"{", "null", validEventBody + validEventBody, `{"unknown":true}`,
		`{"id":"x","title":"x","startsAt":"bad","duration":"1h","userId":"a"}`,
		`{"id":"x","title":"x","startsAt":"2026-10-13T10:00:00Z","duration":"bad","userId":"a"}`,
	} {
		apiRequest(t, s, http.MethodPost, "/events", body, http.StatusBadRequest)
	}
	apiRequest(t, s, http.MethodPut, "/events/different", validEventBody, http.StatusBadRequest)
	for _, query := range []string{"", "?date=bad", "?date=2026-02-30", "?date=2026-10-13&timezone=bad"} {
		apiRequest(t, s, http.MethodGet, "/events/day"+query, "", http.StatusBadRequest)
	}
	apiRequest(t, s, http.MethodPatch, "/events/one", validEventBody, http.StatusMethodNotAllowed)
}

func TestHTTPOverlap(t *testing.T) {
	s := apiServer()
	apiRequest(t, s, http.MethodPost, "/events", validEventBody, http.StatusCreated)
	overlap := `{"id":"two","title":"Overlap","startsAt":"2026-10-13T10:30:00Z","duration":"1h","userId":"alice"}`
	apiRequest(t, s, http.MethodPost, "/events", overlap, http.StatusConflict)
}

func TestHTTPDateWindows(t *testing.T) {
	s := apiServer()
	for _, e := range []struct{ id, start string }{
		{"before", "2026-03-07T23:00:00-05:00"},
		{"at-start", "2026-03-08T00:00:00-05:00"},
		{"next", "2026-03-09T00:00:00-04:00"},
		{"week-end", "2026-03-15T00:00:00-04:00"},
		{"month-end", "2026-04-01T00:00:00-04:00"},
	} {
		body, err := json.Marshal(eventDTO{ID: e.id, Title: e.id, UserID: e.id, StartsAt: e.start, Duration: "1h"})
		if err != nil {
			t.Fatal(err)
		}
		apiRequest(t, s, http.MethodPost, "/events", string(body), http.StatusCreated)
	}
	for _, tc := range []struct {
		period string
		want   int
	}{{"day", 1}, {"week", 2}, {"month", 4}} {
		response := apiRequest(t, s, http.MethodGet, "/events/"+tc.period+"?date=2026-03-08&timezone=America%2FNew_York",
			"", http.StatusOK)
		var result listEnvelope
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Events) != tc.want {
			t.Fatalf("%s: got %d, want %d", tc.period, len(result.Events), tc.want)
		}
	}
}

func TestHTTPErrorMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{context.Canceled, http.StatusRequestTimeout},
		{context.DeadlineExceeded, http.StatusGatewayTimeout},
		{errors.New("database-secret"), http.StatusInternalServerError},
	} {
		response := httptest.NewRecorder()
		writeAPIError(response, tc.err)
		if response.Code != tc.want || strings.Contains(response.Body.String(), "database-secret") {
			t.Fatalf("error response: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestHTTPBodyLimitsAndMedia(t *testing.T) {
	s := apiServer()
	body := strings.Repeat(" ", 1<<20) + validEventBody
	apiRequest(t, s, http.MethodPost, "/events", body, http.StatusRequestEntityTooLarge)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/events",
		strings.NewReader(validEventBody))
	req.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(response, req)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("content type: %d", response.Code)
	}
}

func TestHTTPProtobufTimestampBounds(t *testing.T) {
	s := apiServer()
	for _, start := range []string{"9999-12-31T23:00:00-01:00", "0001-01-01T00:00:00+01:00"} {
		event := eventDTO{ID: "boundary", Title: "boundary", UserID: "alice", StartsAt: start, Duration: "1m"}
		body, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		apiRequest(t, s, http.MethodPost, "/events", string(body), http.StatusBadRequest)
	}
}
