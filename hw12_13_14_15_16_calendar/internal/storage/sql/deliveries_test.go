package sqlstorage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

func TestPostgresDeliveryReceipts(t *testing.T) {
	s := integrationStorage(t)
	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("receipt-%d", time.Now().UnixNano())
	t.Cleanup(func() { _, _ = s.db.ExecContext(ctx, "DELETE FROM notification_deliveries WHERE event_id=$1", id) })
	n := notification.Notification{
		EventID: id, Title: "Meeting", UserID: "alice",
		StartsAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	for i := 0; i < 2; i++ {
		if err := s.RecordDelivery(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	query := "SELECT count(*) FROM notification_deliveries WHERE event_id=$1"
	if err := s.db.QueryRowContext(ctx, query, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate receipt: %d %v", count, err)
	}
	n.StartsAt = n.StartsAt.Add(time.Hour)
	if err := s.RecordDelivery(ctx, n); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, query, id).Scan(&count); err != nil || count != 2 {
		t.Fatalf("rescheduled receipt: %d %v", count, err)
	}
	if err := s.RecordDelivery(ctx, notification.Notification{}); err == nil {
		t.Fatal("invalid receipt accepted")
	}
}
