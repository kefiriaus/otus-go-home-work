package sqlstorage

import (
	"context"
	"fmt"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/migrations"
)

func (s *Storage) Migrate(ctx context.Context) error {
	if s.db == nil {
		return ErrNotConnected
	}
	return migrations.Apply(ctx, s.db)
}

// DispatchDue holds event locks until confirmed publications have been marked.
// A rollback after publishing can redeliver: PostgreSQL and AMQP are not atomic.
func (s *Storage) DispatchDue(ctx context.Context, now time.Time, limit int,
	publish func(context.Context, notification.Notification) error,
) (int, error) {
	if s.db == nil {
		return 0, ErrNotConnected
	}
	if now.IsZero() || limit < 1 || publish == nil {
		return 0, fmt.Errorf("invalid notification scan")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT id,title,starts_at,user_id FROM events
 WHERE notify_before_ns IS NOT NULL AND notification_sent_at IS NULL AND ends_at > $1
 AND EXTRACT(EPOCH FROM (starts_at - $1::timestamptz)) * 1000000000 <= notify_before_ns
 ORDER BY starts_at,id LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return 0, err
	}
	notices := make([]notification.Notification, 0, limit)
	for rows.Next() {
		var n notification.Notification
		if err := rows.Scan(&n.EventID, &n.Title, &n.StartsAt, &n.UserID); err != nil {
			_ = rows.Close()
			return 0, err
		}
		notices = append(notices, n)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil {
		return 0, rowErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	for _, n := range notices {
		if err := publish(ctx, n); err != nil {
			return 0, err
		}
		_, err := tx.ExecContext(ctx, "UPDATE events SET notification_sent_at=$2 WHERE id=$1", n.EventID, now)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(notices), nil
}

func (s *Storage) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if s.db == nil {
		return 0, ErrNotConnected
	}
	if cutoff.IsZero() {
		return 0, fmt.Errorf("cleanup cutoff required")
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE ends_at < $1", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
