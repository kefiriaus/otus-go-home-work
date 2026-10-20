package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	StorageMemory = "memory"
	StorageSQL    = "sql"
)

type Config struct {
	Logger    LoggerConf    `yaml:"logger"`
	HTTP      HTTPConf      `yaml:"http"`
	GRPC      HTTPConf      `yaml:"grpc"`
	Storage   StorageConf   `yaml:"storage"`
	Database  DatabaseConf  `yaml:"database"`
	RabbitMQ  RabbitConf    `yaml:"rabbitmq"`
	Scheduler SchedulerConf `yaml:"scheduler"`
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

func New() Config {
	return Config{
		Logger:  LoggerConf{Level: "info"},
		HTTP:    HTTPConf{Host: "0.0.0.0", Port: 8080},
		GRPC:    HTTPConf{Host: "0.0.0.0", Port: 50051},
		Storage: StorageConf{Type: StorageMemory},
		RabbitMQ: RabbitConf{
			URL: "amqp://localhost:5672/", Queue: "calendar.notifications",
			Prefetch: 10, Timeout: 5 * time.Second,
		},
		Scheduler: SchedulerConf{Interval: time.Minute, BatchSize: 100, Timeout: 10 * time.Second},
	}
}

type Role string

const (
	API       Role = "api"
	Scheduler Role = "scheduler"
	Sender    Role = "sender"
)

type RabbitConf struct {
	URL      string        `yaml:"url"`
	Queue    string        `yaml:"queue"`
	Prefetch int           `yaml:"prefetch"`
	Timeout  time.Duration `yaml:"timeout"`
}
type SchedulerConf struct {
	Interval  time.Duration `yaml:"interval"`
	BatchSize int           `yaml:"batchSize"`
	Timeout   time.Duration `yaml:"timeout"`
}

func Load(path string) (Config, error) { return LoadFor(path, API) }

func LoadFor(path string, role Role) (Config, error) {
	c := New()
	if role == Scheduler {
		c.Storage.Type = StorageSQL
	}
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
	return c, c.validate(role)
}

func (c *Config) applyEnvironment() error {
	for key, target := range map[string]*string{
		"CALENDAR_LOG_LEVEL":      &c.Logger.Level,
		"CALENDAR_LOG_FILE":       &c.Logger.File,
		"CALENDAR_HTTP_HOST":      &c.HTTP.Host,
		"CALENDAR_GRPC_HOST":      &c.GRPC.Host,
		"CALENDAR_STORAGE_TYPE":   &c.Storage.Type,
		"CALENDAR_DATABASE_DSN":   &c.Database.DSN,
		"CALENDAR_RABBITMQ_URL":   &c.RabbitMQ.URL,
		"CALENDAR_RABBITMQ_QUEUE": &c.RabbitMQ.Queue,
	} {
		if value, ok := os.LookupEnv(key); ok {
			*target = value
		}
	}
	for key, target := range map[string]*int{
		"CALENDAR_HTTP_PORT": &c.HTTP.Port, "CALENDAR_GRPC_PORT": &c.GRPC.Port,
		"CALENDAR_RABBITMQ_PREFETCH": &c.RabbitMQ.Prefetch, "CALENDAR_SCHEDULER_BATCH_SIZE": &c.Scheduler.BatchSize,
	} {
		if value, ok := os.LookupEnv(key); ok {
			port, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%s must be an integer", key)
			}
			*target = port
		}
	}
	for key, target := range map[string]*time.Duration{
		"CALENDAR_RABBITMQ_TIMEOUT": &c.RabbitMQ.Timeout, "CALENDAR_SCHEDULER_INTERVAL": &c.Scheduler.Interval,
		"CALENDAR_SCHEDULER_TIMEOUT": &c.Scheduler.Timeout,
	} {
		if value, ok := os.LookupEnv(key); ok {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("%s must be a duration", key)
			}
			*target = duration
		}
	}
	return nil
}

func (c Config) validate(role Role) error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToUpper(c.Logger.Level))); err != nil ||
		(level != slog.LevelDebug && level != slog.LevelInfo && level != slog.LevelWarn && level != slog.LevelError) {
		return fmt.Errorf("logger.level must be error, warn, info or debug")
	}
	if role == API && (strings.TrimSpace(c.HTTP.Host) == "" || c.HTTP.Port < 1 || c.HTTP.Port > 65535) {
		return fmt.Errorf("http requires a host and port between 1 and 65535")
	}
	if role == API && (strings.TrimSpace(c.GRPC.Host) == "" || c.GRPC.Port < 1 || c.GRPC.Port > 65535) {
		return fmt.Errorf("grpc requires a host and port between 1 and 65535")
	}
	if c.Storage.Type != StorageMemory && c.Storage.Type != StorageSQL {
		return fmt.Errorf("storage.type must be memory or sql")
	}
	if ((role == API && c.Storage.Type == StorageSQL) || role == Scheduler) && strings.TrimSpace(c.Database.DSN) == "" {
		return fmt.Errorf("database.dsn is required for sql storage")
	}
	if role != API {
		if strings.TrimSpace(c.RabbitMQ.URL) == "" || strings.TrimSpace(c.RabbitMQ.Queue) == "" ||
			c.RabbitMQ.Prefetch < 1 || c.RabbitMQ.Timeout <= 0 {
			return fmt.Errorf("rabbitmq requires url, queue, positive prefetch and timeout")
		}
	}
	if role == Scheduler && (c.Scheduler.Interval <= 0 || c.Scheduler.BatchSize < 1 || c.Scheduler.Timeout <= 0) {
		return fmt.Errorf("scheduler interval, batchSize and timeout must be positive")
	}

	return nil
}
