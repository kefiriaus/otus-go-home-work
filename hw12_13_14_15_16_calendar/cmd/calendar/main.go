package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/app"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/logger"
	internalgrpc "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server/grpc"
	internalhttp "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/server/http"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	memorystorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/memory"
	sqlstorage "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage/sql"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "configs/config.yaml", "Path to YAML configuration file")
	flag.Parse()
	if flag.Arg(0) == "version" {
		printVersion()
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	err := run(ctx, configFile)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, path string) (result error) {
	config, err := LoadConfig(path)
	if err != nil {
		return err
	}
	outputs := []io.Writer{os.Stdout}
	if config.Logger.File != "" {
		f, err := os.OpenFile(config.Logger.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open log file: %w", err)
		}
		defer func() {
			if err := f.Close(); err != nil && result == nil {
				result = fmt.Errorf("close log file: %w", err)
			}
		}()
		outputs = append(outputs, f)
	}
	log := logger.New(config.Logger.Level, outputs...)
	var store storage.Storage
	if config.Storage.Type == storageSQL {
		sqlStore := sqlstorage.New(config.Database.DSN)
		connect, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := sqlStore.Connect(connect)
		if err == nil {
			err = sqlStore.Migrate(connect)
		}
		cancel()
		if err != nil {
			_ = sqlStore.Close()
			return err
		}
		defer func() {
			if err := sqlStore.Close(); err != nil {
				log.Error("close PostgreSQL", "error", err)
			}
		}()
		store = sqlStore
	} else {
		store = memorystorage.New()
	}
	calendar := app.New(store)
	address := net.JoinHostPort(config.HTTP.Host, strconv.Itoa(config.HTTP.Port))
	httpServer := internalhttp.NewServer(log, address, calendar)
	grpcAddress := net.JoinHostPort(config.GRPC.Host, strconv.Itoa(config.GRPC.Port))
	grpcServer := internalgrpc.NewServer(log, grpcAddress, calendar)
	log.Info("calendar starting", "http_address", address, "grpc_address", grpcAddress, "storage", config.Storage.Type)
	if err := runServers(ctx, httpServer, grpcServer); err != nil {
		return err
	}
	log.Info("calendar stopped")
	return nil
}

type lifecycleServer interface{ Start(context.Context) error }

func runServers(ctx context.Context, first, second lifecycleServer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, 2)
	go func() { results <- first.Start(ctx) }()
	go func() { results <- second.Start(ctx) }()
	firstError := <-results
	cancel()
	secondError := <-results
	return errors.Join(firstError, secondError)
}
