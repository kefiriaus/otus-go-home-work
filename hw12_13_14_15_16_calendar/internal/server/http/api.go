package internalhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
)

const jsonContentType = "application/json"

type eventEnvelope struct {
	Event eventDTO `json:"event"`
}
type listEnvelope struct {
	Events []eventDTO `json:"events"`
}
type errorEnvelope struct {
	Error string `json:"error"`
}

func registerAPI(mux *http.ServeMux, calendar server.Application) {
	mux.HandleFunc("POST /events", func(w http.ResponseWriter, r *http.Request) {
		event, ok := decodeEvent(w, r)
		if !ok {
			return
		}
		if err := calendar.CreateEvent(r.Context(), event); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, eventEnvelope{Event: eventResponse(event)})
	})
	mux.HandleFunc("PUT /events/{id}", func(w http.ResponseWriter, r *http.Request) {
		event, ok := decodeEvent(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" || id != event.ID {
			writeAPIError(w, storage.ErrInvalidEvent)
			return
		}
		if err := calendar.UpdateEvent(r.Context(), id, event); err != nil {
			writeAPIError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, eventEnvelope{Event: eventResponse(event)})
	})
	mux.HandleFunc("DELETE /events/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if strings.TrimSpace(id) == "" {
			writeAPIError(w, storage.ErrInvalidEvent)
			return
		}
		if err := calendar.DeleteEvent(r.Context(), id); err != nil {
			writeAPIError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /events/day", listHandler(calendar.ListEventsOnDay))
	mux.HandleFunc("GET /events/week", listHandler(calendar.ListEventsOnWeek))
	mux.HandleFunc("GET /events/month", listHandler(calendar.ListEventsOnMonth))
}

func listHandler(list func(context.Context, time.Time) ([]storage.Event, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date, err := server.ParseDate(r.URL.Query().Get("date"), r.URL.Query().Get("timezone"))
		if err != nil {
			writeAPIError(w, err)
			return
		}
		events, err := list(r.Context(), date)
		if err != nil {
			writeAPIError(w, err)
			return
		}
		response := listEnvelope{Events: make([]eventDTO, 0, len(events))}
		for _, event := range events {
			response.Events = append(response.Events, eventResponse(event))
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func decodeEvent(w http.ResponseWriter, r *http.Request) (storage.Event, bool) {
	if header := r.Header.Get("Content-Type"); header != "" {
		media, _, err := mime.ParseMediaType(header)
		if err != nil || media != jsonContentType {
			writeJSON(w, http.StatusUnsupportedMediaType, errorEnvelope{Error: "Content-Type must be application/json"})
			return storage.Event{}, false
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input *eventDTO
	if err := decoder.Decode(&input); err != nil {
		writeDecodeError(w, err)
		return storage.Event{}, false
	}
	if input == nil {
		writeAPIError(w, storage.ErrInvalidEvent)
		return storage.Event{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeDecodeError(w, err)
		return storage.Event{}, false
	}
	event, err := input.domain()
	if err != nil {
		writeAPIError(w, err)
		return storage.Event{}, false
	}
	return event, true
}

func writeDecodeError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	var limit *http.MaxBytesError
	if errors.As(err, &limit) {
		code = http.StatusRequestEntityTooLarge
	}
	writeJSON(w, code, errorEnvelope{Error: "invalid JSON event body"})
}

func writeAPIError(w http.ResponseWriter, err error) {
	code, message := http.StatusInternalServerError, "internal server error"
	switch {
	case errors.Is(err, storage.ErrInvalidEvent):
		code, message = http.StatusBadRequest, "invalid event"
	case errors.Is(err, storage.ErrInvalidRange):
		code, message = http.StatusBadRequest, "invalid date or timezone"
	case errors.Is(err, storage.ErrNotFound):
		code, message = http.StatusNotFound, "event not found"
	case errors.Is(err, storage.ErrAlreadyExists):
		code, message = http.StatusConflict, "event already exists"
	case errors.Is(err, storage.ErrDateBusy):
		code, message = http.StatusConflict, "event time is already occupied"
	case errors.Is(err, context.Canceled):
		code, message = http.StatusRequestTimeout, "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = http.StatusGatewayTimeout, "request deadline exceeded"
	}
	writeJSON(w, code, errorEnvelope{Error: message})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
