package main

import (
	"context"
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
		cancel()
		if err != nil {
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
	// The application is initialized, but business API routes belong to homework 13.
	_ = calendar
	address := net.JoinHostPort(config.HTTP.Host, strconv.Itoa(config.HTTP.Port))
	server := internalhttp.NewServer(log, address)
	log.Info("calendar starting", "address", address, "storage", config.Storage.Type)
	if err := server.Start(ctx); err != nil {
		return err
	}
	log.Info("calendar stopped")
	return nil
}
