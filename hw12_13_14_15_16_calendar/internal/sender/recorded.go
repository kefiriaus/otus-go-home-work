package sender

import (
	"context"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

// ReceiptStore records delivery completion before the consumer acknowledges a message.
type ReceiptStore interface {
	RecordDelivery(context.Context, notification.Notification) error
}

type RecordedDelivery struct {
	delivery Delivery
	receipts ReceiptStore
	timeout  time.Duration
}

func NewRecordedDelivery(delivery Delivery, receipts ReceiptStore, timeout time.Duration) *RecordedDelivery {
	return &RecordedDelivery{delivery: delivery, receipts: receipts, timeout: timeout}
}

func (d *RecordedDelivery) Send(ctx context.Context, n notification.Notification) error {
	if err := d.delivery.Send(ctx, n); err != nil {
		return err
	}
	operation, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	return d.receipts.RecordDelivery(operation, n)
}
