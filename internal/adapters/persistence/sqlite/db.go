package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open abre o banco em modo WAL, aplica pragmas e executa migrations.
func Open(dsn string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dsn), 0700); err != nil {
		return nil, fmt.Errorf("sqlite mkdir: %w", err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite = single-writer

	ctx := context.Background()
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return nil, fmt.Errorf("sqlite pragma %q: %w", p, err)
		}
	}
	return db, nil
}
