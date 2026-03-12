package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSQLiteStore(t *testing.T) {
	ctx := context.Background()

	t.Run("Schema initialization success", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "test.db")

		store, err := NewSQLiteStore(ctx, dbPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() { _ = store.Close() }()

		tables := []string{"activities", "daily_tasks", "sync_state"}
		for _, tableName := range tables {
			var name string
			query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
			err := store.db.QueryRowContext(ctx, query, tableName).Scan(&name)
			if err != nil {
				t.Fatalf("table %s not found: %v", tableName, err)
			}
		}
	})

	t.Run("Initialization is idempotent", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "idempotent.db")

		store1, err := NewSQLiteStore(ctx, dbPath)
		if err != nil {
			t.Fatalf("first initialization failed: %v", err)
		}

		_, err = store1.db.ExecContext(ctx, "INSERT INTO activities (id, title, channel_name, type, page_url) VALUES ('test1', 'T', 'C', 'U', 'http')")
		if err != nil {
			t.Fatalf("failed to insert test data: %v", err)
		}
		_ = store1.Close()

		store2, err := NewSQLiteStore(ctx, dbPath)
		if err != nil {
			t.Fatalf("second initialization failed: %v", err)
		}
		defer func() { _ = store2.Close() }()

		var count int
		err = store2.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM activities").Scan(&count)
		if err != nil {
			t.Fatalf("failed to query test data: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 record after re-initialization, got %d", count)
		}
	})

	t.Run("DB path not writable", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.Chmod(tmpDir, 0o400); err != nil {
			t.Fatalf("failed to chmod tmp dir: %v", err)
		}
		defer func() { _ = os.Chmod(tmpDir, 0o700) }() //nolint:gosec

		dbPath := filepath.Join(tmpDir, "readonly", "test.db")
		_, err := NewSQLiteStore(ctx, dbPath)
		if err == nil {
			t.Fatal("expected error for read-only path, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create database") && !strings.Contains(err.Error(), "failed to open sqlite database") {
			t.Errorf("expected error about file path, got %v", err)
		}
	})

	t.Run("Close success", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "close.db")

		store, err := NewSQLiteStore(ctx, dbPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = store.Close()
		if err != nil {
			t.Fatalf("unexpected error on close: %v", err)
		}

		err = store.db.PingContext(ctx)
		if err == nil {
			t.Fatal("expected error when pinging closed db, got nil")
		}
	})

	t.Run("Pragma execution fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "pragma_fail.db")

		// wal mode needs write permission
		if err := os.WriteFile(dbPath, []byte(""), 0o400); err != nil {
			t.Fatalf("failed to create dummy db: %v", err)
		}

		_, err := NewSQLiteStore(ctx, dbPath)
		if err == nil {
			t.Fatal("expected error for read-only connection when executing pragma, got nil")
		}
		if !strings.Contains(err.Error(), "failed to execute pragma") {
			t.Errorf("expected pragma error, got: %v", err)
		}
	})

	t.Run("Schema initialization syntax error scenario", func(t *testing.T) {
		// This branch is hard-coded in NewSQLiteStore and not easily triggerable.
	})
}
