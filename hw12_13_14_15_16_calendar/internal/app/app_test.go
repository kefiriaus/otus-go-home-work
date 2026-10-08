package app

import (
	"context"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	memorystorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/memory"
)

func TestCalendarWindows(t *testing.T) {
	const dayEventID = "day"
	// US spring DST transition is a 23-hour day; windows must use calendar arithmetic.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s := memorystorage.New()
	a := New(s)
	for _, tc := range []struct {
		id    string
		year  int
		month time.Month
		day   int
	}{
		{"before", 2026, 3, 7},
		{dayEventID, 2026, 3, 8},
		{"next", 2026, 3, 9},
		{"week-end", 2026, 3, 15},
		{"month-end", 2026, 4, 1},
	} {
		e := storage.Event{
			ID: tc.id, Title: tc.id, UserID: "alice",
			StartsAt: time.Date(tc.year, tc.month, tc.day, 0, 0, 0, 0, loc), Duration: time.Hour,
		}
		if err := a.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	date := time.Date(2026, 3, 8, 15, 0, 0, 0, loc)
	day, err := a.ListEventsOnDay(ctx, date)
	if err != nil || len(day) != 1 || day[0].ID != dayEventID {
		t.Fatalf("day: %v, %v", day, err)
	}
	week, err := a.ListEventsOnWeek(ctx, date)
	if err != nil || len(week) != 2 || week[0].ID != dayEventID || week[1].ID != "next" {
		t.Fatalf("week: %v, %v", week, err)
	}
	month, err := a.ListEventsOnMonth(ctx, date)
	if err != nil || len(month) != 4 {
		t.Fatalf("month: %v, %v", month, err)
	}
	if _, err := a.ListEventsOnDay(ctx, time.Time{}); err == nil {
		t.Fatal("zero date accepted")
	}
}
