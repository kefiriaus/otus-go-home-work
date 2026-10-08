package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	storageMemory = "memory"
	storageSQL    = "sql"
)

type Config struct {
	Logger   LoggerConf   `yaml:"logger"`
	HTTP     HTTPConf     `yaml:"http"`
	Storage  StorageConf  `yaml:"storage"`
	Database DatabaseConf `yaml:"database"`
}

type LoggerConf struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type HTTPConf struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type StorageConf struct {
	Type string `yaml:"type"`
}
type DatabaseConf struct {
	DSN string `yaml:"dsn"`
}

func NewConfig() Config {
	return Config{
		Logger:  LoggerConf{Level: "info"},
		HTTP:    HTTPConf{Host: "0.0.0.0", Port: 8080},
		Storage: StorageConf{Type: storageMemory},
	}
}

func LoadConfig(path string) (Config, error) {
	c := NewConfig()
	f, err := os.Open(path)
	if err != nil {
		return c, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = f.Close() }()
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	input := &c
	if err := decoder.Decode(&input); err != nil {
		return c, fmt.Errorf("decode config: %w", err)
	}
	if input == nil {
		return c, fmt.Errorf("config must be a YAML mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("config must contain exactly one YAML document")
	}
	if err := c.applyEnvironment(); err != nil {
		return c, err
	}
	return c, c.validate()
}

func (c *Config) applyEnvironment() error {
	for key, target := range map[string]*string{
		"CALENDAR_LOG_LEVEL":    &c.Logger.Level,
		"CALENDAR_LOG_FILE":     &c.Logger.File,
		"CALENDAR_HTTP_HOST":    &c.HTTP.Host,
		"CALENDAR_STORAGE_TYPE": &c.Storage.Type,
		"CALENDAR_DATABASE_DSN": &c.Database.DSN,
	} {
		if value, ok := os.LookupEnv(key); ok {
			*target = value
		}
	}
	if value, ok := os.LookupEnv("CALENDAR_HTTP_PORT"); ok {
		port, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("CALENDAR_HTTP_PORT must be an integer")
		}
		c.HTTP.Port = port
	}
	return nil
}

func (c Config) validate() error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(c.Logger.Level))); err != nil ||
		(level != slog.LevelDebug && level != slog.LevelInfo && level != slog.LevelWarn && level != slog.LevelError) {
		return fmt.Errorf("logger.level must be error, warn, info or debug")
	}
	if strings.TrimSpace(c.HTTP.Host) == "" || c.HTTP.Port < 1 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http requires a host and port between 1 and 65535")
	}
	if c.Storage.Type != storageMemory && c.Storage.Type != storageSQL {
		return fmt.Errorf("storage.type must be memory or sql")
	}
	if c.Storage.Type == storageSQL && strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("database.dsn is required for sql storage")
	}
	return nil
}
