package main

import (
	"os"
	"path/filepath"
	"testing"
)

func configFileForTest(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig(t *testing.T) {
	p := configFileForTest(t, `# Calendar configuration
logger:
  level: debug
http:
  host: 127.0.0.1
  port: 8123
storage:
  type: memory
`)
	c, err := LoadConfig(p)
	if err != nil || c.Logger.Level != "debug" || c.HTTP.Port != 8123 || c.Storage.Type != "memory" {
		t.Fatalf("config: %+v, %v", c, err)
	}
	t.Setenv("CALENDAR_HTTP_PORT", "9001")
	t.Setenv("CALENDAR_STORAGE_TYPE", "sql")
	t.Setenv("CALENDAR_DATABASE_DSN", "postgres://example/db")
	c, err = LoadConfig(p)
	if err != nil || c.HTTP.Port != 9001 || c.Storage.Type != "sql" || c.Database.DSN != "postgres://example/db" {
		t.Fatalf("override: %+v, %v", c, err)
	}
}

func TestConfigValidation(t *testing.T) {
	for _, body := range []string{
		"", "# empty", "null", "[]", "text", "http: [", "unknown: true",
		"http:\n  unknown: true", "{}\n---\n{}", "logger:\n  level: trace",
		"http:\n  port: 0", "http:\n  port: 65536", "http:\n  port: abc",
		"storage:\n  type: invalid", "storage:\n  type: sql",
		"http:\n  port: 8080\n  port: 9000",
	} {
		if _, err := LoadConfig(configFileForTest(t, body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
	t.Setenv("CALENDAR_HTTP_PORT", "abc")
	if _, err := LoadConfig(configFileForTest(t, `{}`)); err == nil {
		t.Fatal("invalid env accepted")
	}
}

func TestYAMLDefaults(t *testing.T) {
	c, err := LoadConfig(configFileForTest(t, "{}"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Logger.Level != "info" || c.HTTP.Host != "0.0.0.0" || c.HTTP.Port != 8080 || c.Storage.Type != storageMemory {
		t.Fatalf("defaults: %+v", c)
	}
}
