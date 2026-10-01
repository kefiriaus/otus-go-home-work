package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunInvalidArguments(t *testing.T) {
	const host = "localhost"
	for _, args := range [][]string{
		nil,
		{host},
		{host, "80", "extra"},
		{"--timeout=invalid", host, "80"},
		{"--timeout=0s", host, "80"},
	} {
		t.Run("args="+strings.Join(args, " "), func(t *testing.T) {
			require.Error(t, run(context.Background(), args, io.NopCloser(&bytes.Buffer{}), io.Discard, io.Discard))
		})
	}
}

func TestRunShutdown(t *testing.T) {
	const peerClosed = "peer closed"
	for _, reason := range []string{"input EOF", peerClosed, "cancelled"} {
		t.Run(reason, func(t *testing.T) {
			var config net.ListenConfig
			l, err := config.Listen(context.Background(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = l.Close() })
			host, port, err := net.SplitHostPort(l.Addr().String())
			require.NoError(t, err)
			in, writer := io.Pipe()
			t.Cleanup(func() { _ = writer.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- run(ctx, []string{host, port}, in, io.Discard, io.Discard) }()
			require.NoError(t, l.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second)))
			peer, err := l.Accept()
			require.NoError(t, err)
			t.Cleanup(func() { _ = peer.Close() })
			switch reason {
			case "input EOF":
				require.NoError(t, writer.Close())
			case peerClosed:
				require.NoError(t, peer.Close())
			case "cancelled":
				cancel()
			}
			select {
			case err := <-result:
				require.NoError(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("client did not terminate")
			}
			if reason != peerClosed {
				require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
				_, err = peer.Read(make([]byte, 1))
				require.ErrorIs(t, err, io.EOF)
			}
		})
	}
}
