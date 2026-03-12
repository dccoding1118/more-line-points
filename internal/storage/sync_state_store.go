package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SyncStateStore 管理同步狀態的 Hash 存取。
type SyncStateStore interface {
	GetHash(ctx context.Context, key string) (string, error)
	SetHash(ctx context.Context, key, hash string, syncedAt time.Time) error
	UpdateSyncedAt(ctx context.Context, key string, syncedAt time.Time) error
}

func (s *SQLiteStore) GetHash(ctx context.Context, key string) (string, error) {
	query := `SELECT hash FROM sync_state WHERE key = ?;`
	row := s.db.QueryRowContext(ctx, query, key)

	var hash string
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // Key 不存在回傳空字串與無 error
		}
		return "", fmt.Errorf("failed to get sync state hash: %w", err)
	}

	return hash, nil
}

func (s *SQLiteStore) SetHash(ctx context.Context, key, hash string, syncedAt time.Time) error {
	query := `
		INSERT INTO sync_state (key, hash, synced_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			hash = excluded.hash,
			synced_at = excluded.synced_at;
	`
	_, err := s.db.ExecContext(ctx, query, key, hash, syncedAt)
	if err != nil {
		return fmt.Errorf("failed to set sync state hash: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateSyncedAt(ctx context.Context, key string, syncedAt time.Time) error {
	query := `UPDATE sync_state SET synced_at = ? WHERE key = ?;`
	_, err := s.db.ExecContext(ctx, query, syncedAt, key)
	if err != nil {
		return fmt.Errorf("failed to update synced at: %w", err)
	}
	return nil
}
