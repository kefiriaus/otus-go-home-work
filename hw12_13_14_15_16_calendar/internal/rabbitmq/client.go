// Package rabbitmq implements reminder delivery with publisher confirms and manual ACKs.
package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Options struct {
	URL, Queue string
	Prefetch   int
	Timeout    time.Duration
}
type Client struct {
	conn                *amqp.Connection
	raw                 net.Conn
	publisher, consumer *amqp.Channel
	returns             <-chan amqp.Return
	gate                chan struct{}
	options             Options
}

func Open(ctx context.Context, options Options) (*Client, error) {
	if options.Timeout <= 0 || options.Prefetch < 1 || strings.TrimSpace(options.Queue) == "" {
		return nil, errors.New("invalid RabbitMQ options")
	}
	operation, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	var raw net.Conn
	conn, err := amqp.DialConfig(options.URL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial: func(network, address string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: options.Timeout}
			socket, err := dialer.DialContext(operation, network, address)
			if err != nil {
				return nil, err
			}
			deadline, _ := operation.Deadline()
			if err := socket.SetDeadline(deadline); err != nil {
				_ = socket.Close()
				return nil, err
			}
			raw = socket
			return socket, nil
		},
	})
	if err != nil {
		if operation.Err() != nil {
			return nil, operation.Err()
		}
		return nil, errors.New("cannot connect to RabbitMQ; check connection settings and availability")
	}
	client := &Client{conn: conn, raw: raw, options: options, gate: make(chan struct{}, 1)}
	deadline, _ := operation.Deadline()
	_ = raw.SetDeadline(deadline)
	stop := context.AfterFunc(operation, func() { _ = raw.Close() })
	defer stop()
	if err := client.setup(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("initialize RabbitMQ topology: %w", err)
	}
	if operation.Err() != nil {
		_ = raw.Close()
		return nil, operation.Err()
	}
	_ = raw.SetDeadline(time.Time{})
	return client, nil
}

func (c *Client) setup() error {
	var err error
	c.publisher, err = c.conn.Channel()
	if err != nil {
		return err
	}
	c.consumer, err = c.conn.Channel()
	if err != nil {
		return err
	}
	deadExchange := c.options.Queue + ".dead"
	if err := c.publisher.ExchangeDeclare(deadExchange, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := c.publisher.QueueDeclare(c.options.Queue+".rejected", true, false, false, false, nil); err != nil {
		return err
	}
	if err := c.publisher.QueueBind(c.options.Queue+".rejected", "", deadExchange, false, nil); err != nil {
		return err
	}
	if _, err := c.publisher.QueueDeclare(c.options.Queue, true, false, false, false,
		amqp.Table{"x-dead-letter-exchange": deadExchange}); err != nil {
		return err
	}
	if err := c.publisher.Confirm(false); err != nil {
		return err
	}
	c.returns = c.publisher.NotifyReturn(make(chan amqp.Return, 1))
	return c.consumer.Qos(c.options.Prefetch, 0, false)
}

func (c *Client) Close() error {
	if c.conn.IsClosed() {
		return nil
	}
	err := c.conn.CloseDeadline(time.Now().Add(c.options.Timeout))
	if errors.Is(err, amqp.ErrClosed) {
		return nil
	}
	return err
}

func (c *Client) Publish(ctx context.Context, n notification.Notification) (result error) {
	if err := n.Validate(); err != nil {
		return err
	}
	operation, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()
	select {
	case c.gate <- struct{}{}:
	case <-operation.Done():
		return operation.Err()
	}
	defer func() { <-c.gate }()
	deadline, _ := operation.Deadline()
	_ = c.raw.SetDeadline(deadline)
	stop := context.AfterFunc(operation, func() { _ = c.raw.Close() })
	defer stop()
	defer func() {
		if result != nil {
			_ = c.raw.Close()
		} else {
			_ = c.raw.SetDeadline(time.Time{})
		}
	}()
	body, err := json.Marshal(n)
	if err != nil {
		return err
	}
	confirmation, err := c.publisher.PublishWithDeferredConfirmWithContext(operation, "", c.options.Queue, true, false,
		amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: n.EventID, Body: body})
	if err != nil {
		return err
	}
	if confirmation == nil {
		return errors.New("publisher confirms not enabled")
	}
	ack, err := confirmation.WaitContext(operation)
	if err != nil {
		return err
	}
	if !ack {
		return errors.New("RabbitMQ rejected notification")
	}
	select {
	case returned := <-c.returns:
		if returned.ReplyCode != 0 {
			return errors.New("notification was not routed to its queue")
		}
	default:
	}
	return nil
}

func (c *Client) Consume(ctx context.Context, handle func(context.Context, notification.Notification) error) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	stop := context.AfterFunc(ctx, func() { _ = c.raw.Close() })
	defer stop()
	registration, cancelRegistration := context.WithTimeout(ctx, c.options.Timeout)
	deadline, _ := registration.Deadline()
	_ = c.raw.SetDeadline(deadline)
	stopRegistration := context.AfterFunc(registration, func() { _ = c.raw.Close() })
	deliveries, err := c.consumer.ConsumeWithContext(ctx, c.options.Queue, "", false, false, false, false, nil)
	registrationErr := registration.Err()
	stopRegistration()
	cancelRegistration()
	_ = c.raw.SetDeadline(time.Time{})
	if registrationErr != nil {
		if ctx.Err() != nil {
			return nil
		}
		return registrationErr
	}

	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, open := <-deliveries:
			if !open {
				if ctx.Err() != nil {
					return nil
				}
				return errors.New("RabbitMQ consumer connection closed")
			}
			if err := c.process(ctx, delivery, handle); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}
}

func (c *Client) process(ctx context.Context, delivery amqp.Delivery,
	handle func(context.Context, notification.Notification) error,
) error {
	n, err := decode(delivery.Body)
	_ = c.raw.SetWriteDeadline(time.Now().Add(c.options.Timeout))
	defer func() { _ = c.raw.SetWriteDeadline(time.Time{}) }()
	if err != nil {
		return delivery.Nack(false, false)
	}
	if err := handle(ctx, n); err != nil {
		if nackErr := delivery.Nack(false, true); nackErr != nil {
			return errors.Join(err, nackErr)
		}
		return err
	}
	return delivery.Ack(false)
}

func decode(body []byte) (notification.Notification, error) {
	if len(body) > 1<<20 {
		return notification.Notification{}, errors.New("notification too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var n *notification.Notification
	if err := decoder.Decode(&n); err != nil {
		return notification.Notification{}, err
	}
	if n == nil {
		return notification.Notification{}, errors.New("notification required")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return notification.Notification{}, errors.New("one notification required")
	}
	return *n, n.Validate()
}
