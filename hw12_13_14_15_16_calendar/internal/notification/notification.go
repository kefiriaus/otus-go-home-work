// Package notification defines reminder messages independently of the broker.
package notification

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Notification struct {
	EventID  string    `json:"eventId"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"startsAt"`
	UserID   string    `json:"userId"`
}

func (n Notification) Validate() error {
	if strings.TrimSpace(n.EventID) == "" || strings.TrimSpace(n.Title) == "" ||
		strings.TrimSpace(n.UserID) == "" || n.StartsAt.IsZero() {
		return errors.New("invalid notification")
	}
	return nil
}

type Publisher interface {
	Publish(context.Context, Notification) error
}
type Consumer interface {
	Consume(context.Context, func(context.Context, Notification) error) error
}
