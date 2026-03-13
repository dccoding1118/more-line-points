package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

var ErrActivityNotFound = errors.New("activity not found")

// ActivityStore manages the persistence of activity data.
type ActivityStore interface {
	UpsertActivity(ctx context.Context, a *model.Activity) error
	GetActivity(ctx context.Context, id string) (*model.Activity, error)
	ListAllActivityIDs(ctx context.Context) ([]string, error)
	MarkInactive(ctx context.Context, ids []string) error
	CleanExpired(ctx context.Context, cutoff time.Time) (int64, error)
	UpdateType(ctx context.Context, id, actType string) error
	UpdateActionURL(ctx context.Context, id, actionURL string) error
	GetActivitiesByDate(ctx context.Context, targetDate time.Time) ([]model.Activity, error)
}

func (s *SQLiteStore) UpsertActivity(ctx context.Context, a *model.Activity) error {
	query := `
		INSERT INTO activities (
			id, title, channel_name, channel_id, type, page_url, valid_from, valid_until, is_active
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			channel_name = excluded.channel_name,
			channel_id = excluded.channel_id,
			page_url = excluded.page_url,
			valid_from = excluded.valid_from,
			valid_until = excluded.valid_until,
			is_active = 1,
			updated_at = CURRENT_TIMESTAMP;
	`
	t := a.Type
	if t == "" {
		t = "unknown"
	}

	// Store valid_from/valid_until as Unix timestamps (int64) for correct
	// numeric comparison regardless of timezone formatting.
	vfUnix := a.ValidFrom.Unix()
	vuUnix := a.ValidUntil.Unix()

	_, err := s.db.ExecContext(ctx, query,
		a.ID, a.Title, a.ChannelName, a.ChannelID, t, a.PageURL, vfUnix, vuUnix,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert activity: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetActivity(ctx context.Context, id string) (*model.Activity, error) {
	query := `
		SELECT id, title, channel_name, channel_id, type, page_url, action_url, valid_from, valid_until, is_active, created_at, updated_at
		FROM activities
		WHERE id = ?;
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var a model.Activity
	var chanID, actionURL sql.NullString
	var vfUnix, vuUnix sql.NullInt64

	err := row.Scan(
		&a.ID, &a.Title, &a.ChannelName, &chanID, &a.Type, &a.PageURL, &actionURL,
		&vfUnix, &vuUnix, &a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrActivityNotFound
		}
		return nil, fmt.Errorf("failed to get activity: %w", err)
	}

	if chanID.Valid {
		a.ChannelID = chanID.String
	}
	if actionURL.Valid {
		a.ActionURL = actionURL.String
	}
	if vfUnix.Valid {
		a.ValidFrom = time.Unix(vfUnix.Int64, 0)
	}
	if vuUnix.Valid {
		a.ValidUntil = time.Unix(vuUnix.Int64, 0)
	}

	return &a, nil
}

func (s *SQLiteStore) ListAllActivityIDs(ctx context.Context) ([]string, error) {
	query := `SELECT id FROM activities ORDER BY id ASC;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list activity ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan activity id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error when listing activity ids: %w", err)
	}

	return ids, nil
}

func (s *SQLiteStore) MarkInactive(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	//nolint:gosec // placeholders contains only "?" and is safe from SQL injection
	query := fmt.Sprintf(`UPDATE activities SET is_active = 0, updated_at = CURRENT_TIMESTAMP WHERE id IN (%s);`, strings.Join(placeholders, ","))

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark inactive: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CleanExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `DELETE FROM activities WHERE valid_until < ?;`
	res, err := s.db.ExecContext(ctx, query, cutoff.Unix())
	if err != nil {
		return 0, fmt.Errorf("failed to clean expired activities: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return affected, nil
}

func (s *SQLiteStore) UpdateType(ctx context.Context, id, actType string) error {
	query := `UPDATE activities SET type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	res, err := s.db.ExecContext(ctx, query, actType, id)
	if err != nil {
		return fmt.Errorf("failed to update activity type: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrActivityNotFound
	}
	return nil
}

func (s *SQLiteStore) UpdateActionURL(ctx context.Context, id, actionURL string) error {
	query := `UPDATE activities SET action_url = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;`
	res, err := s.db.ExecContext(ctx, query, actionURL, id)
	if err != nil {
		return fmt.Errorf("failed to update action url: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrActivityNotFound
	}
	return nil
}

func (s *SQLiteStore) GetActivitiesByDate(ctx context.Context, targetDate time.Time) ([]model.Activity, error) {
	query := `
		SELECT id, title, channel_name, channel_id, type, page_url, action_url, valid_from, valid_until, is_active, created_at, updated_at
		FROM activities
		WHERE is_active = 1 AND valid_from <= ? AND valid_until >= ?;
	`

	// Compute start/end of the target date in Asia/Taipei timezone,
	// so the query correctly handles timezone-aware second-precision comparison.
	loc, _ := time.LoadLocation("Asia/Taipei")
	date := targetDate.In(loc)
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 0, loc)

	rows, err := s.db.QueryContext(ctx, query, endOfDay.Unix(), startOfDay.Unix())
	if err != nil {
		return nil, fmt.Errorf("failed to query activities by date: %w", err)
	}
	defer func() { _ = rows.Close() }()

	activities := []model.Activity{}
	for rows.Next() {
		var a model.Activity
		var chanID, actionURL sql.NullString
		var vfUnix, vuUnix sql.NullInt64

		if err := rows.Scan(
			&a.ID, &a.Title, &a.ChannelName, &chanID, &a.Type, &a.PageURL, &actionURL,
			&vfUnix, &vuUnix, &a.IsActive, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan activity: %w", err)
		}

		if chanID.Valid {
			a.ChannelID = chanID.String
		}
		if actionURL.Valid {
			a.ActionURL = actionURL.String
		}
		if vfUnix.Valid {
			a.ValidFrom = time.Unix(vfUnix.Int64, 0)
		}
		if vuUnix.Valid {
			a.ValidUntil = time.Unix(vuUnix.Int64, 0)
		}

		activities = append(activities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error when querying activities by date: %w", err)
	}

	return activities, nil
}
