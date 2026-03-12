package storage

import (
	"context"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

func TestReplaceDailyTasks(t *testing.T) {
	ctx := context.Background()
	store := newTestDB(t)
	defer func() { _ = store.Close() }()

	actID := "act-task"
	_ = store.UpsertActivity(ctx, &model.Activity{ID: actID, Title: "Task Test"})

	t.Run("R2: New tasks", func(t *testing.T) {
		tasks := []model.DailyTask{
			{UseDate: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), Keyword: "K1", URL: "U1"},
			{UseDate: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC), Keyword: "K2", URL: "U2"},
		}
		err := store.ReplaceDailyTasks(ctx, actID, tasks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := store.GetDailyTasksByDate(ctx, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
		if len(got) != 1 || got[0].Keyword != "K1" {
			t.Errorf("expected K1 on 3/5, got %v", got)
		}
	})

	t.Run("R1: Replace existing", func(t *testing.T) {
		tasks := []model.DailyTask{
			{UseDate: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), Keyword: "K1-NEW", URL: "U1-NEW"},
		}
		err := store.ReplaceDailyTasks(ctx, actID, tasks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := store.GetDailyTasksByDate(ctx, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
		if len(got) != 1 || got[0].Keyword != "K1-NEW" {
			t.Errorf("expected K1-NEW, got %v", got)
		}
		// check if 3/6 was deleted
		got6, _ := store.GetDailyTasksByDate(ctx, time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC))
		if len(got6) != 0 {
			t.Errorf("expected 3/6 to be deleted, got %v", got6)
		}
	})

	t.Run("R3: Clear with empty slice", func(t *testing.T) {
		err := store.ReplaceDailyTasks(ctx, actID, []model.DailyTask{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := store.GetDailyTasksByDate(ctx, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC))
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})
}

func TestGetDailyTasksByDate(t *testing.T) {
	ctx := context.Background()
	store := newTestDB(t)
	defer func() { _ = store.Close() }()

	date := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	_ = store.UpsertActivity(ctx, &model.Activity{ID: "act-a", Title: "A"})
	_ = store.UpsertActivity(ctx, &model.Activity{ID: "act-b", Title: "B"})

	_ = store.ReplaceDailyTasks(ctx, "act-a", []model.DailyTask{{UseDate: date, Keyword: "KA"}})
	_ = store.ReplaceDailyTasks(ctx, "act-b", []model.DailyTask{{UseDate: date, Keyword: "KB"}})

	t.Run("D1: Data exists", func(t *testing.T) {
		got, err := store.GetDailyTasksByDate(ctx, date)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(got))
		}
	})

	t.Run("D2: No data", func(t *testing.T) {
		got, err := store.GetDailyTasksByDate(ctx, date.Add(24*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty non-nil slice, got %v", got)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		_ = store.Close()
		_, err := store.GetDailyTasksByDate(ctx, time.Now())
		if err == nil {
			t.Errorf("expected error after db close")
		}
		err = store.ReplaceDailyTasks(ctx, "a", []model.DailyTask{})
		if err == nil {
			t.Errorf("expected error after db close")
		}
	})
}
