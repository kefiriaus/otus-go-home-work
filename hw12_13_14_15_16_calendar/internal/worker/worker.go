// Package worker shares process startup for scheduler and sender.
package worker

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/config"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/logger"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/rabbitmq"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/scheduler"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/sender"
	sqlstorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/sql"
)

func Main(role config.Role, defaultConfig string) {
	var path string
	flag.StringVar(&path, "config", defaultConfig, "Path to YAML configuration file")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	err := Run(ctx, path, role)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Run(ctx context.Context, path string, role config.Role) (result error) {
	cfg, err := config.LoadFor(path, role)
	if err != nil {
		return err
	}
	outputs := []io.Writer{os.Stdout}
	if cfg.Logger.File != "" {
		file, err := os.OpenFile(cfg.Logger.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer func() {
			if err := file.Close(); err != nil && result == nil {
				result = err
			}
		}()
		outputs = append(outputs, file)
	}
	log := logger.New(cfg.Logger.Level, outputs...)
	broker, err := rabbitmq.Open(ctx, rabbitmq.Options{
		URL: cfg.RabbitMQ.URL, Queue: cfg.RabbitMQ.Queue,
		Prefetch: cfg.RabbitMQ.Prefetch, Timeout: cfg.RabbitMQ.Timeout,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := broker.Close(); err != nil {
			log.Error("close RabbitMQ", "error", err)
		}
	}()
	log.Info("worker starting", "role", role, "queue", cfg.RabbitMQ.Queue)
	if role != config.Sender && role != config.Scheduler {
		return fmt.Errorf("unknown worker role: %s", role)
	}
	store := sqlstorage.New(cfg.Database.DSN)
	startup, cancel := context.WithTimeout(ctx, cfg.Scheduler.Timeout)
	defer cancel()
	if err := store.Connect(startup); err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Error("close PostgreSQL", "error", err)
		}
	}()
	if err := store.Migrate(startup); err != nil {
		return err
	}
	cancel()
	switch role {
	case config.Sender:
		delivery := sender.NewRecordedDelivery(
			sender.NewLogDelivery(io.MultiWriter(outputs...)), store, cfg.RabbitMQ.Timeout,
		)
		return sender.New(broker, delivery).Run(ctx)
	case config.Scheduler:
		process := scheduler.New(store, broker, log, cfg.Scheduler.Interval, cfg.Scheduler.BatchSize, cfg.Scheduler.Timeout)
		return process.Run(ctx)
	case config.API:
		return fmt.Errorf("API is not a worker")
	default:
		return fmt.Errorf("unknown worker role")
	}
}
