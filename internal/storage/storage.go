package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SQLiteStore manages the connection to the SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite connection and ensures the directory and schema exist.
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, query := range pragmas {
		if _, err := db.ExecContext(ctx, query); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to execute pragma: %w", err)
		}
	}

	schemas := []string{
		`CREATE TABLE IF NOT EXISTS activities (
			id            TEXT PRIMARY KEY,
			title         TEXT NOT NULL,
			channel_name  TEXT NOT NULL,
			channel_id    TEXT,
			type          TEXT NOT NULL DEFAULT 'unknown',
			page_url      TEXT NOT NULL,
			action_url    TEXT,
			valid_from    DATETIME,
			valid_until   DATETIME,
			is_active     INTEGER NOT NULL DEFAULT 1,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS daily_tasks (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			activity_id   TEXT NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
			use_date      DATE NOT NULL,
			keyword       TEXT,
			url           TEXT,
			note          TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			key           TEXT PRIMARY KEY,
			hash          TEXT NOT NULL,
			synced_at     DATETIME NOT NULL
		);`,
	}

	for _, query := range schemas {
		if _, err := db.ExecContext(ctx, query); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to execute schema initialization: %w", err)
		}
	}

	return &SQLiteStore{db: db}, nil
}

// Close 關閉資料庫連線。
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
