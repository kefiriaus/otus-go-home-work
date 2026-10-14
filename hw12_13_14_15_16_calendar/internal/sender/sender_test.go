package sender

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

type testConsumer struct{ notice notification.Notification }

func (c testConsumer) Consume(ctx context.Context,
	handle func(context.Context, notification.Notification) error,
) error {
	return handle(ctx, c.notice)
}

func TestSenderLogsNotification(t *testing.T) {
	var output bytes.Buffer
	notice := notification.Notification{EventID: "one", Title: "Meeting", UserID: "alice", StartsAt: time.Now()}
	consumer := testConsumer{notice: notice}
	worker := New(consumer, NewLogDelivery(&output))
	if err := worker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"notification sent", "one", "Meeting", "alice", "startsAt"} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("missing %s: %s", field, output.String())
		}
	}
}

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }
func TestSenderOutputFailure(t *testing.T) {
	failure := errors.New("stdout failed")
	notice := notification.Notification{
		EventID: "failed-output", Title: "Output failure", UserID: "bob", StartsAt: time.Now(),
	}
	worker := New(testConsumer{notice: notice}, NewLogDelivery(failedWriter{err: failure}))
	if err := worker.Run(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("output failure: %v", err)
	}
}
