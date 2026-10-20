package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

func TestRabbitDelivery(t *testing.T) {
	url := os.Getenv("CALENDAR_TEST_RABBITMQ_URL")
	if url == "" {
		t.Skip("set CALENDAR_TEST_RABBITMQ_URL for RabbitMQ integration tests")
	}
	queue := fmt.Sprintf("calendar.test.%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	producer, err := Open(ctx, Options{URL: url, Queue: queue, Prefetch: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = producer.Close() }()
	consumer, err := Open(ctx, Options{URL: url, Queue: queue, Prefetch: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = consumer.Close() }()
	want := notification.Notification{EventID: "one", Title: "Meeting", UserID: "alice", StartsAt: time.Now().UTC()}
	if err := producer.Publish(ctx, want); err != nil {
		t.Fatal(err)
	}
	received := make(chan notification.Notification, 1)
	consumeContext, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- consumer.Consume(consumeContext, func(_ context.Context, n notification.Notification) error {
			received <- n
			return nil
		})
	}()
	select {
	case got := <-received:
		if got.EventID != want.EventID || got.Title != want.Title || !got.StartsAt.Equal(want.StartsAt) {
			t.Fatalf("payload: %+v", got)
		}
	case <-ctx.Done():
		t.Fatal("message not received")
	}
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("consumer did not stop")
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	for _, body := range []string{"null", "{}", "{", `{"eventId":"one","unknown":true}`, "{}{}"} {
		if _, err := decode([]byte(body)); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestDecodePreservesTrailingSyntaxError(t *testing.T) {
	_, err := decode([]byte(`{} garbage`))
	var syntaxError *json.SyntaxError
	if !errors.As(err, &syntaxError) {
		t.Fatalf("expected wrapped JSON syntax error, got %v", err)
	}
}

func TestDecodeRejectsSecondJSONValue(t *testing.T) {
	_, err := decode([]byte(`{} {}`))
	if err == nil || err.Error() != "one notification required" {
		t.Fatalf("expected single-notification error, got %v", err)
	}
}

func TestUnroutablePublication(t *testing.T) {
	address := os.Getenv("CALENDAR_TEST_RABBITMQ_URL")
	if address == "" {
		t.Skip("set CALENDAR_TEST_RABBITMQ_URL for routing regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	options := Options{
		URL: address, Queue: fmt.Sprintf("calendar.test.routing.%d", time.Now().UnixNano()),
		Prefetch: 1, Timeout: time.Second,
	}
	client, err := Open(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.publisher.QueueDelete(options.Queue, false, false, false); err != nil {
		t.Fatal(err)
	}
	notice := notification.Notification{EventID: "one", Title: "Meeting", UserID: "alice", StartsAt: time.Now()}
	if err := client.Publish(ctx, notice); err == nil {
		t.Fatal("unroutable notification reported as published")
	}
}
