BEGIN;

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY CHECK (btrim(id) <> ''),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    duration_ns BIGINT NOT NULL CHECK (duration_ns > 0 AND duration_ns % 1000 = 0),
    description TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL CHECK (btrim(user_id) <> ''),
    notify_before_ns BIGINT CHECK (notify_before_ns >= 0),
    CHECK (ends_at > starts_at),
    CHECK (ends_at = starts_at + ((duration_ns / 1000)::text || ' microseconds')::interval),
    CONSTRAINT events_user_time_exclusion EXCLUDE USING gist (
        user_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
);

CREATE INDEX IF NOT EXISTS events_starts_at_idx ON events (starts_at, id);

COMMIT;
