package main

import (
	"context"
	"errors"
	"testing"
)

type serverFunc func(context.Context) error

func (s serverFunc) Start(ctx context.Context) error { return s(ctx) }

func TestServersFailTogether(t *testing.T) {
	failed := errors.New("listen failed")
	stopped := make(chan struct{})
	err := runServers(context.Background(),
		serverFunc(func(context.Context) error { return failed }),
		serverFunc(func(ctx context.Context) error { <-ctx.Done(); close(stopped); return nil }))
	if !errors.Is(err, failed) {
		t.Fatalf("lost startup failure: %v", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("second server not stopped")
	}
}
