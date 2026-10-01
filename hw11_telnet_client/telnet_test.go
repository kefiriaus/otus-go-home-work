package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTelnetClient(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		var config net.ListenConfig
		l, err := config.Listen(context.Background(), "tcp", "127.0.0.1:")
		require.NoError(t, err)
		defer func() { require.NoError(t, l.Close()) }()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()

			in := &bytes.Buffer{}
			out := &bytes.Buffer{}

			timeout, err := time.ParseDuration("10s")
			require.NoError(t, err)

			client := NewTelnetClient(l.Addr().String(), timeout, io.NopCloser(in), out)
			require.NoError(t, client.Connect())
			defer func() { require.NoError(t, client.Close()) }()

			in.WriteString("hello\n")
			err = client.Send()
			require.NoError(t, err)

			err = client.Receive()
			require.NoError(t, err)
			require.Equal(t, "world\n", out.String())
		}()

		go func() {
			defer wg.Done()

			conn, err := l.Accept()
			require.NoError(t, err)
			require.NotNil(t, conn)
			defer func() { require.NoError(t, conn.Close()) }()

			request := make([]byte, 1024)
			n, err := conn.Read(request)
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(request)[:n])

			n, err = conn.Write([]byte("world\n"))
			require.NoError(t, err)
			require.NotEqual(t, 0, n)
		}()

		wg.Wait()
	})
}

func TestConnectionFailure(t *testing.T) {
	client := NewTelnetClient("127.0.0.1:invalid", time.Second, io.NopCloser(&bytes.Buffer{}), io.Discard)
	require.NotNil(t, client)
	require.Error(t, client.Connect())
	require.NoError(t, client.Close())
}

func TestCloseUnblocksTransfers(t *testing.T) {
	var config net.ListenConfig
	l, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	in, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	client := NewTelnetClient(l.Addr().String(), time.Second, in, io.Discard)
	require.NotNil(t, client)
	require.NoError(t, client.Connect())
	peer, err := l.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	results := make(chan error, 2)
	go func() { results <- client.Send() }()
	go func() { results <- client.Receive() }()
	require.NoError(t, client.Close())
	for range 2 {
		select {
		case <-results:
		case <-time.After(time.Second):
			t.Fatal("Close did not unblock a transfer")
		}
	}
	require.NoError(t, client.Close())
}
