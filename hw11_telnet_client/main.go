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
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, in io.ReadCloser, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("go-telnet", flag.ContinueOnError)
	flags.SetOutput(stderr)
	timeout := flags.Duration("timeout", 10*time.Second, "connection timeout")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("usage: go-telnet [--timeout=10s] host port")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	address := net.JoinHostPort(flags.Arg(0), flags.Arg(1))
	client := NewTelnetClient(address, *timeout, in, out)
	connected := make(chan error)
	go func() {
		err := client.Connect()
		select {
		case connected <- err:
		case <-ctx.Done():
			_ = client.Close()
		}
	}()
	select {
	case <-ctx.Done():
		fmt.Fprintln(stderr, "Interrupted")
		return nil
	case err := <-connected:
		defer func() { _ = client.Close() }()
		if err != nil {
			return fmt.Errorf("connect to %s: %w", address, err)
		}
	}
	fmt.Fprintf(stderr, "Connected to %s\n", address)
	sent, received := make(chan error, 1), make(chan error, 1)
	go func() { sent <- client.Send() }()
	go func() { received <- client.Receive() }()
	select {
	case <-ctx.Done():
		fmt.Fprintln(stderr, "Interrupted")
	case err := <-sent:
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		fmt.Fprintln(stderr, "EOF")
	case err := <-received:
		if err != nil {
			return fmt.Errorf("receive: %w", err)
		}
		fmt.Fprintln(stderr, "Connection was closed by peer")
	}
	return nil
}
