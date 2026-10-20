package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func workerConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWorkerConfig(t *testing.T) {
	body := "database:\n  dsn: postgres://localhost/calendar\nscheduler:\n  interval: 2s"
	cfg, err := LoadFor(workerConfig(t, body), Scheduler)
	if err != nil || cfg.Scheduler.Interval != 2*time.Second || cfg.Storage.Type != StorageSQL {
		t.Fatalf("scheduler: %+v %v", cfg, err)
	}
	t.Setenv("CALENDAR_RABBITMQ_QUEUE", "test-queue")
	t.Setenv("CALENDAR_SCHEDULER_INTERVAL", "3s")
	cfg, err = LoadFor(workerConfig(t, "database:\n  dsn: postgres://localhost/calendar"), Sender)
	if err != nil || cfg.RabbitMQ.Queue != "test-queue" || cfg.Scheduler.Interval != 3*time.Second {
		t.Fatalf("override: %+v %v", cfg, err)
	}
}

func TestWorkerConfigRejectsInvalid(t *testing.T) {
	for _, body := range []string{
		"database:\n  dsn: ''", "scheduler:\n  interval: 0s", "rabbitmq:\n  timeout: 0s", "rabbitmq:\n  prefetch: 0",
	} {
		if _, err := LoadFor(workerConfig(t, body), Scheduler); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	t.Setenv("CALENDAR_SCHEDULER_INTERVAL", "bad")
	if _, err := LoadFor(workerConfig(t, "{}"), Sender); err == nil {
		t.Fatal("invalid interval")
	}
}

func TestSenderRequiresDatabase(t *testing.T) {
	if _, err := LoadFor(workerConfig(t, "{}"), Sender); err == nil {
		t.Fatal("sender accepted missing database DSN")
	}
}
