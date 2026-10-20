package sender

import (
	"context"
	"encoding/json"
	"io"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/notification"
)

// LogDelivery is the delivery effect, independent of diagnostic log filtering.
type LogDelivery struct{ output io.Writer }

func NewLogDelivery(output io.Writer) *LogDelivery { return &LogDelivery{output: output} }
func (d *LogDelivery) Send(ctx context.Context, n notification.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return json.NewEncoder(d.output).Encode(struct {
		Message string `json:"message"`
		notification.Notification
	}{Message: "notification sent", Notification: n})
}
