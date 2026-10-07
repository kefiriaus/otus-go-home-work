package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger writes structured records safely from concurrent goroutines.
type Logger struct{ *slog.Logger }

// New defaults to stdout. Config validation rejects unknown log levels.
func New(level string, outputs ...io.Writer) *Logger {
	var minimum slog.Level
	if err := minimum.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		minimum = slog.LevelInfo
	}
	var output io.Writer = os.Stdout
	if len(outputs) > 0 {
		output = io.MultiWriter(outputs...)
	}
	return &Logger{Logger: slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: minimum}))}
}
