package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelFiltering(t *testing.T) {
	for _, tc := range []struct {
		level string
		count int
	}{{"debug", 4}, {"info", 3}, {"warn", 2}, {"error", 1}} {
		t.Run(tc.level, func(t *testing.T) {
			var out bytes.Buffer
			l := New(tc.level, &out)
			l.Debug("debug message")
			l.Info("info message")
			l.Warn("warn message")
			l.Error("error message")
			if n := strings.Count(out.String(), "message"); n != tc.count {
				t.Fatalf("got %d logs: %s", n, out.String())
			}
			if !strings.Contains(out.String(), "level=ERROR") {
				t.Fatal("error not logged")
			}
		})
	}
}
