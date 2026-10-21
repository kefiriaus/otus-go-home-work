package sender

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

type deliveryFunc func(context.Context, notification.Notification) error

func (f deliveryFunc) Send(ctx context.Context, n notification.Notification) error { return f(ctx, n) }

type receiptFunc func(context.Context, notification.Notification) error

func (f receiptFunc) RecordDelivery(ctx context.Context, n notification.Notification) error {
	return f(ctx, n)
}

func TestRecordedDelivery(t *testing.T) {
	failure := errors.New("effect failed")
	const (
		outputStage  = "output"
		successStage = "success"
	)
	for _, stage := range []string{successStage, outputStage, "receipt"} {
		t.Run(stage, func(t *testing.T) {
			outputDone := false
			recorded := false
			delivery := deliveryFunc(func(context.Context, notification.Notification) error {
				if stage == outputStage {
					return failure
				}
				outputDone = true
				return nil
			})
			receipts := receiptFunc(func(context.Context, notification.Notification) error {
				if !outputDone {
					t.Fatal("receipt before delivery")
				}
				recorded = true
				if stage == "receipt" {
					return failure
				}
				return nil
			})
			recordedDelivery := NewRecordedDelivery(delivery, receipts, 10*time.Millisecond)
			err := recordedDelivery.Send(context.Background(), notification.Notification{})
			if stage == successStage && err != nil {
				t.Fatal(err)
			}
			if stage != successStage && !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
			if recorded != (stage != outputStage) {
				t.Fatalf("recorded=%v", recorded)
			}
		})
	}
}

func TestRecordedDeliveryBoundsReceiptWrite(t *testing.T) {
	delivery := deliveryFunc(func(context.Context, notification.Notification) error { return nil })
	receipts := receiptFunc(func(ctx context.Context, _ notification.Notification) error {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("receipt write has no deadline")
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	})
	recordedDelivery := NewRecordedDelivery(delivery, receipts, 10*time.Millisecond)
	err := recordedDelivery.Send(context.Background(), notification.Notification{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error: %v", err)
	}
}
