BEGIN;
DROP TRIGGER IF EXISTS events_reset_notification ON events;
DROP FUNCTION IF EXISTS reset_event_notification();
DROP INDEX IF EXISTS events_pending_notification_idx;
DROP INDEX IF EXISTS events_ends_at_idx;
ALTER TABLE events DROP COLUMN IF EXISTS notification_sent_at;
COMMIT;
