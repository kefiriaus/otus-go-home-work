// Package migrations embeds the calendar schema for automatic scheduler startup.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"
)

//go:embed *.up.sql
var scripts embed.FS

func Apply(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(141414)"); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "ROLLBACK")
		_, _ = conn.ExecContext(cleanup, "SELECT pg_advisory_unlock(141414)")
	}()
	files, err := fs.Glob(scripts, "*.up.sql")
	if err != nil {
		return err
	}
	for _, file := range files {
		body, err := scripts.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := conn.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", file, err)
		}
	}
	return nil
}
