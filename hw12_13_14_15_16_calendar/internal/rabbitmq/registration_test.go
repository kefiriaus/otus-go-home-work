package rabbitmq

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

// Drop only basic.consume-ok while keeping the AMQP connection alive.
func TestConsumerRegistrationTimeout(t *testing.T) {
	address := os.Getenv("CALENDAR_TEST_RABBITMQ_URL")
	if address == "" {
		t.Skip("set CALENDAR_TEST_RABBITMQ_URL for registration timeout regression")
	}
	upstreamURL, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamURL.Scheme != "amqp" {
		t.Skip("AMQP proxy requires unencrypted test URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var listen net.ListenConfig
	proxy, err := listen.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proxy.Close() }()
	backend := upstreamURL.Host
	upstreamURL.Host = proxy.Addr().String()
	dropped := make(chan struct{}, 1)
	go func() {
		downstream, err := proxy.Accept()
		if err != nil {
			return
		}
		defer func() { _ = downstream.Close() }()
		var dial net.Dialer
		upstream, err := dial.DialContext(ctx, "tcp", backend)
		if err != nil {
			return
		}
		defer func() { _ = upstream.Close() }()
		go func() { _, _ = io.Copy(upstream, downstream) }()
		relayWithoutConsumeOK(downstream, upstream, dropped)
	}()
	options := Options{URL: upstreamURL.String(), Queue: "calendar.test.registration", Prefetch: 1, Timeout: time.Second}
	client, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	client.options.Timeout = 100 * time.Millisecond
	result := make(chan error, 1)
	go func() {
		result <- client.Consume(ctx, func(context.Context, notification.Notification) error { return nil })
	}()
	select {
	case <-dropped:
	case <-ctx.Done():
		t.Fatal("proxy did not intercept consume-ok")
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("registration unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Error("consumer registration ignored configured timeout")
	}
}

func relayWithoutConsumeOK(downstream, upstream net.Conn, dropped chan<- struct{}) {
	for {
		header := make([]byte, 7)
		if _, err := io.ReadFull(upstream, header); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(header[3:7])
		if length > 1<<20 {
			return
		}
		body := make([]byte, int(length)+1)
		if _, err := io.ReadFull(upstream, body); err != nil {
			return
		}
		if header[0] == 1 && length >= 4 && binary.BigEndian.Uint16(body[:2]) == 60 &&
			binary.BigEndian.Uint16(body[2:4]) == 21 {
			select {
			case dropped <- struct{}{}:
			default:
			}
			continue
		}
		if _, err := io.Copy(downstream, io.MultiReader(bytes.NewReader(header), bytes.NewReader(body))); err != nil {
			return
		}
	}
}
