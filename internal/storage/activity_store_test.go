package storage

import (
	"context"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

func newTestDB(t *testing.T) *SQLiteStore {
	ctx := context.Background()
	// Each connection to ":memory:" gets its own private database.
	store, err := NewSQLiteStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("failed to create memory db: %v", err)
	}
	return store
}

func TestUpsertActivity(t *testing.T) {
	ctx := context.Background()

	t.Run("U1: New activity", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		now := time.Now().Truncate(time.Second)
		a := &model.Activity{
			ID:          "act1",
			Title:       "Test Act 1",
			ChannelName: "LINE 購物",
			ChannelID:   "@shopping",
			PageURL:     "http://example.com/1",
			ValidFrom:   now,
			ValidUntil:  now.Add(24 * time.Hour),
		}

		err := store.UpsertActivity(ctx, a)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := store.GetActivity(ctx, "act1")
		if err != nil {
			t.Fatalf("unexpected get error: %v", err)
		}
		if got.IsActive != true {
			t.Errorf("expected is_active=true, got %v", got.IsActive)
		}
		if got.Type != "unknown" {
			t.Errorf("expected type='unknown', got %q", got.Type)
		}
		if got.Title != "Test Act 1" {
			t.Errorf("expected title mismatch")
		}
	})

	t.Run("U2: Update existing activity", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		a := &model.Activity{
			ID:          "act2",
			Title:       "Old Title",
			ChannelName: "Old",
			Type:        "custom_type",
			PageURL:     "http://old.com",
		}
		_ = store.UpsertActivity(ctx, a)
		// Manually change type in DB to test if U2 overwrites it
		_, _ = store.db.ExecContext(ctx, `UPDATE activities SET type = 'custom', created_at = '2020-01-01 00:00:00' WHERE id = 'act2'`)

		a.Title = "New Title"
		a.Type = "unknown"
		err := store.UpsertActivity(ctx, a)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := store.GetActivity(ctx, "act2")
		if err != nil {
			t.Fatalf("unexpected get err: %v", err)
		}
		if got.Title != "New Title" {
			t.Errorf("expected title updated")
		}
		if got.Type != "custom" {
			t.Errorf("expected type to remain 'custom', got %q", got.Type)
		}
		// Verify created_at remains unchanged
		if got.CreatedAt.Year() != 2020 {
			t.Errorf("expected created_at unaltered, got %v", got.CreatedAt)
		}
	})

	t.Run("U3: Activity reappears", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		a := &model.Activity{ID: "act3", Title: "Title 3"}
		_ = store.UpsertActivity(ctx, a)
		_ = store.MarkInactive(ctx, []string{"act3"})

		// Reappears and updates
		a.Title = "Title 3 Updated"
		err := store.UpsertActivity(ctx, a)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := store.GetActivity(ctx, "act3")
		if got.IsActive != true {
			t.Errorf("expected is_active=true")
		}
		if got.Title != "Title 3 Updated" {
			t.Errorf("expected title updated")
		}
	})
}

func TestGetActivity(t *testing.T) {
	ctx := context.Background()
	store := newTestDB(t)
	defer func() { _ = store.Close() }()

	t.Run("G1: Exists", func(t *testing.T) {
		a := &model.Activity{
			ID:          "act-get",
			Title:       "Test Get",
			ChannelName: "Get Channel",
			PageURL:     "url",
			ActionURL:   "action_url",
			ValidFrom:   time.Now().Truncate(time.Second),
		}
		_ = store.UpsertActivity(ctx, a)
		_ = store.UpdateActionURL(ctx, a.ID, a.ActionURL)

		got, err := store.GetActivity(ctx, "act-get")
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.ID != a.ID || got.Title != a.Title || got.ActionURL != a.ActionURL || !got.ValidFrom.Equal(a.ValidFrom) {
			t.Errorf("got %v, want matching struct", got)
		}
	})

	t.Run("G2: Not exists", func(t *testing.T) {
		_, err := store.GetActivity(ctx, "not-exist")
		if err != ErrActivityNotFound {
			t.Errorf("expected ErrActivityNotFound, got %v", err)
		}
	})
}

func TestUpdateType(t *testing.T) {
	ctx := context.Background()
	store := newTestDB(t)
	defer func() { _ = store.Close() }()

	a := &model.Activity{ID: "act-type", Title: "Type Test"}
	_ = store.UpsertActivity(ctx, a)

	t.Run("Update success", func(t *testing.T) {
		err := store.UpdateType(ctx, "act-type", model.ActivityTypeKeyword)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := store.GetActivity(ctx, "act-type")
		if got.Type != model.ActivityTypeKeyword {
			t.Errorf("expected type 'keyword', got %q", got.Type)
		}
	})

	t.Run("Not found", func(t *testing.T) {
		err := store.UpdateType(ctx, "none", model.ActivityTypeKeyword)
		if err != ErrActivityNotFound {
			t.Errorf("expected ErrActivityNotFound, got %v", err)
		}
	})
}

func TestUpdateActionURL(t *testing.T) {
	ctx := context.Background()
	store := newTestDB(t)
	defer func() { _ = store.Close() }()

	a := &model.Activity{ID: "act-action", Title: "Action Test"}
	_ = store.UpsertActivity(ctx, a)

	t.Run("Update success", func(t *testing.T) {
		url := "https://example.com/action"
		err := store.UpdateActionURL(ctx, "act-action", url)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := store.GetActivity(ctx, "act-action")
		if got.ActionURL != url {
			t.Errorf("expected action_url %q, got %q", url, got.ActionURL)
		}
	})

	t.Run("Not found", func(t *testing.T) {
		err := store.UpdateActionURL(ctx, "none", "http://ok")
		if err != ErrActivityNotFound {
			t.Errorf("expected ErrActivityNotFound, got %v", err)
		}
	})
}

func TestListAllActivityIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("A1: Has data", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		_ = store.UpsertActivity(ctx, &model.Activity{ID: "b", Title: "b"})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "a", Title: "a"})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "c", Title: "c"})

		ids, err := store.ListAllActivityIDs(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
			t.Errorf("expected [a, b, c], got %v", ids)
		}
	})

	t.Run("A2: No data", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		ids, err := store.ListAllActivityIDs(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if ids == nil || len(ids) != 0 {
			t.Errorf("expected empty non-nil slice, got %v", ids)
		}
	})
}

func TestMarkInactive(t *testing.T) {
	ctx := context.Background()

	t.Run("I1-I3: Various scenarios", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		_ = store.UpsertActivity(ctx, &model.Activity{ID: "1", Title: "1"})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "2", Title: "2"})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "3", Title: "3"})

		// I2: Empty list
		err := store.MarkInactive(ctx, []string{})
		if err != nil {
			t.Errorf("unexpected error for empty slice: %v", err)
		}

		// I1, I3: Valid and non-existent IDs
		err = store.MarkInactive(ctx, []string{"1", "3", "not-exist"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		act1, _ := store.GetActivity(ctx, "1")
		if act1.IsActive != false {
			t.Errorf("expected act1 inactive")
		}

		act2, _ := store.GetActivity(ctx, "2")
		if act2.IsActive != true {
			t.Errorf("expected act2 active")
		}

		act3, _ := store.GetActivity(ctx, "3")
		if act3.IsActive != false {
			t.Errorf("expected act3 inactive")
		}
	})
}

func TestGetActivitiesByDate(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name      string
		setup     func(store *SQLiteStore)
		target    time.Time
		wantCount int
		wantIDs   []string
	}{
		{
			name: "S1: Active within date range",
			setup: func(store *SQLiteStore) {
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s1", Title: "Active", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
					ValidUntil: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
				})
			},
			target:    time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantIDs:   []string{"s1"},
		},
		{
			name: "S2: Expired activity not returned",
			setup: func(store *SQLiteStore) {
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s2", Title: "Expired", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
					ValidUntil: time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
				})
			},
			target:    time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "S3: Not started activity not returned",
			setup: func(store *SQLiteStore) {
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s3", Title: "Future", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
					ValidUntil: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
				})
			},
			target:    time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "S4: Inactive activity not returned",
			setup: func(store *SQLiteStore) {
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s4", Title: "Inactive", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
					ValidUntil: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC),
				})
				_ = store.MarkInactive(ctx, []string{"s4"})
			},
			target:    time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name:      "S5: No matching activities",
			setup:     func(store *SQLiteStore) {},
			target:    time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantCount: 0,
		},
		{
			name: "S6: valid_from equals target date with Asia/Taipei timezone",
			setup: func(store *SQLiteStore) {
				loc, _ := time.LoadLocation("Asia/Taipei")
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s6", Title: "SameDay Start", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 3, 13, 14, 0, 0, 0, loc), // 2026-03-13 14:00 +0800
					ValidUntil: time.Date(2026, 3, 18, 23, 59, 59, 0, loc),
				})
			},
			target:    time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantIDs:   []string{"s6"},
		},
		{
			name: "S7: valid_until equals target date with Asia/Taipei timezone",
			setup: func(store *SQLiteStore) {
				loc, _ := time.LoadLocation("Asia/Taipei")
				_ = store.UpsertActivity(ctx, &model.Activity{
					ID: "s7", Title: "SameDay End", ChannelName: "ch",
					PageURL:    "http://x",
					ValidFrom:  time.Date(2026, 3, 1, 0, 0, 0, 0, loc),
					ValidUntil: time.Date(2026, 3, 13, 23, 59, 59, 0, loc), // ends on target day
				})
			},
			target:    time.Date(2026, 3, 13, 0, 0, 0, 0, time.UTC),
			wantCount: 1,
			wantIDs:   []string{"s7"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestDB(t)
			defer func() { _ = store.Close() }()

			tc.setup(store)

			got, err := store.GetActivitiesByDate(ctx, tc.target)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("expected %d activities, got %d", tc.wantCount, len(got))
			}
			for i, wantID := range tc.wantIDs {
				if i < len(got) && got[i].ID != wantID {
					t.Errorf("expected activity ID %q at index %d, got %q", wantID, i, got[i].ID)
				}
			}
		})
	}
}

func TestCleanExpired(t *testing.T) {
	ctx := context.Background()

	t.Run("E1-E2: Clean expired", func(t *testing.T) {
		store := newTestDB(t)
		defer func() { _ = store.Close() }()

		now := time.Now()
		yesterday := now.Add(-24 * time.Hour)
		tomorrow := now.Add(24 * time.Hour)

		_ = store.UpsertActivity(ctx, &model.Activity{ID: "not-expired", Title: "A", ValidUntil: tomorrow})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "expired1", Title: "B", ValidUntil: yesterday})
		_ = store.UpsertActivity(ctx, &model.Activity{ID: "expired2", Title: "C", ValidUntil: yesterday})

		// E1: Has expired activities
		count, err := store.CleanExpired(ctx, now)
		if err != nil || count != 2 {
			t.Errorf("expected 2 affected rows, got %d, err: %v", count, err)
		}

		_, err = store.GetActivity(ctx, "expired1")
		if err != ErrActivityNotFound {
			t.Errorf("expected expired1 to be deleted")
		}

		_, err = store.GetActivity(ctx, "not-expired")
		if err != nil {
			t.Errorf("expected not-expired to still exist")
		}

		// E2: No expired activities
		count, err = store.CleanExpired(ctx, now)
		if err != nil || count != 0 {
			t.Errorf("expected 0 affected rows, got %d, err: %v", count, err)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		store := newTestDB(t)
		_ = store.Close()

		if err := store.UpdateType(ctx, "a", "t"); err == nil {
			t.Errorf("expected error after db close")
		}
		if err := store.UpdateActionURL(ctx, "a", "u"); err == nil {
			t.Errorf("expected error after db close")
		}
		if _, err := store.ListAllActivityIDs(ctx); err == nil {
			t.Errorf("expected error after db close")
		}
		if err := store.MarkInactive(ctx, []string{"a"}); err == nil {
			t.Errorf("expected error after db close")
		}
		if _, err := store.CleanExpired(ctx, time.Now()); err == nil {
			t.Errorf("expected error after db close")
		}
	})
}
