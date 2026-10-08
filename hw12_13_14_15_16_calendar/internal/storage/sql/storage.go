package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/storage"
	"github.com/lib/pq"
)

type Storage struct {
	dsn string
	db  *sql.DB
}

var (
	_               storage.Storage = (*Storage)(nil)
	ErrNotConnected                 = errors.New("PostgreSQL storage is not connected")
)

func New(dsn string) *Storage { return &Storage{dsn: dsn} }

// Connect and Close belong to startup/shutdown; event operations may run concurrently.
func (s *Storage) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.db != nil {
		return s.db.PingContext(ctx)
	}
	db, err := sql.Open("postgres", s.dsn)
	if err != nil {
		return errors.New("invalid PostgreSQL connection configuration")
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Driver errors can include connection settings; do not log credentials.
		return errors.New("cannot connect to PostgreSQL; check connection settings and availability")
	}
	s.db = db
	return nil
}

func (s *Storage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Storage) CreateEvent(ctx context.Context, e storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if s.db == nil {
		return ErrNotConnected
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events
  (id,title,starts_at,ends_at,duration_ns,description,user_id,notify_before_ns)
  VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		e.ID, e.Title, e.StartsAt, e.EndsAt(), int64(e.Duration), e.Description, e.UserID, reminder(e.NotifyBefore))
	return operationError(ctx, err)
}

func (s *Storage) UpdateEvent(ctx context.Context, id string, e storage.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id != e.ID {
		return storage.ErrInvalidEvent
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if s.db == nil {
		return ErrNotConnected
	}
	result, err := s.db.ExecContext(ctx, `UPDATE events SET title=$2,starts_at=$3,ends_at=$4,
  duration_ns=$5,description=$6,user_id=$7,notify_before_ns=$8 WHERE id=$1`,
		id, e.Title, e.StartsAt, e.EndsAt(), int64(e.Duration), e.Description, e.UserID, reminder(e.NotifyBefore))
	return changedRows(ctx, result, err)
}

func (s *Storage) DeleteEvent(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.db == nil {
		return ErrNotConnected
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE id=$1", id)
	return changedRows(ctx, result, err)
}

func (s *Storage) ListEvents(ctx context.Context, from, to time.Time) ([]storage.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := storage.ValidateRange(from, to); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, ErrNotConnected
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,title,starts_at,duration_ns,description,user_id,notify_before_ns
  FROM events WHERE starts_at >= $1 AND starts_at < $2 ORDER BY starts_at,id COLLATE "C"`,
		ceilTimestamp(from), ceilTimestamp(to))
	if err != nil {
		return nil, operationError(ctx, err)
	}
	defer func() { _ = rows.Close() }()
	events := make([]storage.Event, 0)
	for rows.Next() {
		var e storage.Event
		var duration int64
		var notify sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Title, &e.StartsAt, &duration, &e.Description, &e.UserID, &notify); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Duration = time.Duration(duration)
		if notify.Valid {
			d := time.Duration(notify.Int64)
			e.NotifyBefore = &d
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, operationError(ctx, err)
	}
	return events, nil
}

func reminder(d *time.Duration) any {
	if d == nil {
		return nil
	}
	return int64(*d)
}

func changedRows(ctx context.Context, result sql.Result, err error) error {
	if err != nil {
		return operationError(ctx, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count affected events: %w", err)
	}
	if count == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func operationError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return storage.ErrAlreadyExists
		case "23P01":
			return storage.ErrDateBusy
		case "23514", "23502":
			return storage.ErrInvalidEvent
		}
	}
	return fmt.Errorf("PostgreSQL event operation: %w", err)
}

// Event starts are microsecond-aligned. Ceiling both bounds preserves [from, to)
// comparisons even when callers supply nanoseconds; PostgreSQL otherwise rounds.
func ceilTimestamp(t time.Time) time.Time {
	floor := t.Truncate(time.Microsecond)
	if floor.Equal(t) {
		return t
	}
	return floor.Add(time.Microsecond)
}
