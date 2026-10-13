package server

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	date, err := ParseDate("2026-03-08", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if date.Hour() != 0 || date.Location().String() != "America/New_York" ||
		date.AddDate(0, 0, 1).Sub(date) != 23*time.Hour {
		t.Fatalf("calendar/DST boundary: %v", date)
	}
	utc, err := ParseDate("2026-10-13", "")
	if err != nil || utc.Location() != time.UTC {
		t.Fatalf("default timezone: %v, %v", utc, err)
	}
	for _, tc := range []struct{ date, zone string }{
		{"", ""}, {"2026-02-30", ""}, {"2026-10-13", "invalid"}, {"0000-01-01", ""},
	} {
		if _, err := ParseDate(tc.date, tc.zone); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}
