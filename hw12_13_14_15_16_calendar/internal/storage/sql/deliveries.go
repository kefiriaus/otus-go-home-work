package sqlstorage

import (
	"context"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

// RecordDelivery is idempotent for the same occurrence of an event.
func (s *Storage) RecordDelivery(ctx context.Context, n notification.Notification) error {
	if s.db == nil {
		return ErrNotConnected
	}
	if err := n.Validate(); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO notification_deliveries(event_id,starts_at) VALUES($1,$2)
 ON CONFLICT(event_id,starts_at) DO NOTHING`, n.EventID, n.StartsAt)
	return err
}
