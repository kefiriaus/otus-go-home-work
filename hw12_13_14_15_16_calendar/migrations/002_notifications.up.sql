BEGIN;
ALTER TABLE events ADD COLUMN IF NOT EXISTS notification_sent_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS events_pending_notification_idx ON events (starts_at)
    WHERE notify_before_ns IS NOT NULL AND notification_sent_at IS NULL;
CREATE INDEX IF NOT EXISTS events_ends_at_idx ON events (ends_at);

CREATE OR REPLACE FUNCTION reset_event_notification() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.starts_at IS DISTINCT FROM OLD.starts_at OR
       NEW.notify_before_ns IS DISTINCT FROM OLD.notify_before_ns THEN
        NEW.notification_sent_at := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS events_reset_notification ON events;
CREATE TRIGGER events_reset_notification BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION reset_event_notification();
COMMIT;
