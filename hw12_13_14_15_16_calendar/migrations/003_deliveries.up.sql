CREATE TABLE IF NOT EXISTS notification_deliveries (
    event_id TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, starts_at)
);
