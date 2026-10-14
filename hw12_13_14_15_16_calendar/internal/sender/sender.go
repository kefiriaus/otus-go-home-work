// Package sender consumes reminders without depending on an AMQP client.
package sender

import (
	"context"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

type (
	Delivery interface {
		Send(context.Context, notification.Notification) error
	}
	Sender struct {
		consumer notification.Consumer
		delivery Delivery
	}
)

func New(consumer notification.Consumer, delivery Delivery) *Sender {
	return &Sender{consumer: consumer, delivery: delivery}
}

func (s *Sender) Run(ctx context.Context) error {
	return s.consumer.Consume(ctx, func(ctx context.Context, n notification.Notification) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := n.Validate(); err != nil {
			return err
		}
		return s.delivery.Send(ctx, n)
	})
}
