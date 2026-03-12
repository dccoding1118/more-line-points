package storage

import (
	"context"
	"testing"
	"time"
)

func TestSetAndGetHash(t *testing.T) {
	ctx := context.Background()

	t.Run("S1/H1/H2", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		// H2: 不存在
		hash, err := store.GetHash(ctx, "not-exist")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if hash != "" {
			t.Errorf("expected empty string, got %q", hash)
		}

		// S1: 新 key
		now := time.Now().Truncate(time.Second)
		err = store.SetHash(ctx, "key1", "hash1", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// H1: 存在
		hash, err = store.GetHash(ctx, "key1")
		if err != nil || hash != "hash1" {
			t.Errorf("expected hash1, got %q, err: %v", hash, err)
		}

		// S2: 更新
		later := now.Add(time.Hour)
		err = store.SetHash(ctx, "key1", "hash2", later)
		if err != nil {
			t.Fatalf("unexpected error on update: %v", err)
		}

		hash, _ = store.GetHash(ctx, "key1")
		if hash != "hash2" {
			t.Errorf("expected hash2, got %q", hash)
		}
	})
}

func TestUpdateSyncedAt(t *testing.T) {
	ctx := context.Background()

	t.Run("T1/T2", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		now := time.Now().Truncate(time.Second)

		// T2: 不存在
		err := store.UpdateSyncedAt(ctx, "not-exist", now)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// 準備存在的 key
		_ = store.SetHash(ctx, "key1", "hash1", now)

		// T1: 存在
		later := now.Add(time.Hour)
		err = store.UpdateSyncedAt(ctx, "key1", later)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// 驗證 hash 未改變，且沒有報錯
		hash, err := store.GetHash(ctx, "key1")
		if err != nil || hash != "hash1" {
			t.Errorf("expected hash1, got %q, err: %v", hash, err)
		}

		// 透過查詢檢查 dates
		var syncedAt time.Time
		_ = store.db.QueryRowContext(ctx, "SELECT synced_at FROM sync_state WHERE key = 'key1'").Scan(&syncedAt)
		if !syncedAt.Equal(later) {
			t.Errorf("expected date updated to %v, got %v", later, syncedAt)
		}
	})
}
