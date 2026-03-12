package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

// DailyTaskStore manages the daily_tasks table.
type DailyTaskStore interface {
	ReplaceDailyTasks(ctx context.Context, activityID string, tasks []model.DailyTask) error
	GetDailyTasksByDate(ctx context.Context, date time.Time) ([]model.DailyTask, error)
}

// ReplaceDailyTasks replaces all daily tasks for a specific activity within a transaction.
func (s *SQLiteStore) ReplaceDailyTasks(ctx context.Context, activityID string, tasks []model.DailyTask) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Delete existing tasks for this activity
	deleteQuery := `DELETE FROM daily_tasks WHERE activity_id = ?;`
	if _, err := tx.ExecContext(ctx, deleteQuery, activityID); err != nil {
		return fmt.Errorf("failed to delete old daily tasks: %w", err)
	}

	// 2. Insert new tasks
	if len(tasks) > 0 {
		insertQuery := `INSERT INTO daily_tasks (activity_id, use_date, keyword, url, note) VALUES (?, ?, ?, ?, ?);`
		stmt, err := tx.PrepareContext(ctx, insertQuery)
		if err != nil {
			return fmt.Errorf("failed to prepare insert statement: %w", err)
		}
		defer func() { _ = stmt.Close() }()

		for _, task := range tasks {
			useDateStr := task.UseDate.Format("2006-01-02")
			if _, err := stmt.ExecContext(ctx, activityID, useDateStr, task.Keyword, task.URL, task.Note); err != nil {
				return fmt.Errorf("failed to insert daily task: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// GetDailyTasksByDate retrieves all daily tasks for a given date.
func (s *SQLiteStore) GetDailyTasksByDate(ctx context.Context, date time.Time) ([]model.DailyTask, error) {
	query := `SELECT id, activity_id, use_date, keyword, url, note FROM daily_tasks WHERE use_date = ?;`
	dateStr := date.Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, query, dateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tasks := []model.DailyTask{}
	for rows.Next() {
		var t model.DailyTask
		var useDateStr string
		if err := rows.Scan(&t.ID, &t.ActivityID, &useDateStr, &t.Keyword, &t.URL, &t.Note); err != nil {
			return nil, fmt.Errorf("failed to scan daily task: %w", err)
		}
		parsedDate, err := time.Parse("2006-01-02", useDateStr)
		if err != nil {
			// fallback if sqlite returned it differently, but formatted as ISO in insert
			t.UseDate = time.Time{}
		} else {
			t.UseDate = parsedDate
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tasks, nil
}
