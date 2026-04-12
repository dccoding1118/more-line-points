package taskpage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

type mockActivityStore struct {
	activities []model.Activity
	err        error
}

func (m *mockActivityStore) GetActivitiesByDate(ctx context.Context, targetDate time.Time) ([]model.Activity, error) {
	return m.activities, m.err
}

func (m *mockActivityStore) UpsertActivity(ctx context.Context, a *model.Activity) error { return nil }
func (m *mockActivityStore) GetActivity(ctx context.Context, id string) (*model.Activity, error) {
	return nil, nil
}

func (m *mockActivityStore) ListAllActivityIDs(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockActivityStore) MarkInactive(ctx context.Context, ids []string) error { return nil }
func (m *mockActivityStore) CleanExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (m *mockActivityStore) UpdateType(ctx context.Context, id, actType string) error { return nil }
func (m *mockActivityStore) UpdateActionURL(ctx context.Context, id, actionURL string) error {
	return nil
}

type mockTaskStore struct {
	tasks []model.DailyTask
	err   error
}

func (m *mockTaskStore) GetDailyTasksByDate(ctx context.Context, targetDate time.Time) ([]model.DailyTask, error) {
	return m.tasks, m.err
}

func (m *mockTaskStore) ReplaceDailyTasks(ctx context.Context, activityID string, tasks []model.DailyTask) error {
	return nil
}

func TestJSONExporter_Export(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "tasks.json")

	now := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	from0 := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	from1 := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)

	as := &mockActivityStore{
		activities: []model.Activity{
			{ID: "A1", Title: "Act 1", Type: "keyword", IsActive: true, ValidFrom: from0},
			{ID: "A2", Title: "Act 2", Type: "share", IsActive: true, ValidFrom: from1, ActionURL: "https://act2.link"},
			{ID: "A3", Title: "Act 3", Type: "unknown", IsActive: true, ValidFrom: from0},
			{ID: "A4", Title: "Act 4", Type: "keyword", IsActive: true, ValidFrom: from0}, // No matching task, should skip
		},
	}

	ts := &mockTaskStore{
		tasks: []model.DailyTask{
			{ActivityID: "A1", Keyword: "KEY0412", URL: "https://line.me/R/oaMessage/@ch/?KEY0412"},
		},
	}

	exporter := NewJSONExporter(as, ts)
	err := exporter.Export(context.Background(), now, outPath)
	if err != nil {
		t.Fatalf("Export() failed: %v", err)
	}

	//nolint:gosec // outPath is defined in a test TempDir, safe to read
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() failed: %v", err)
	}

	var page TaskPage
	if err := json.Unmarshal(b, &page); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if page.Date != "2026-04-12" {
		t.Errorf("expected Date 2026-04-12, got %s", page.Date)
	}

	if len(page.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(page.Tasks))
	}

	if page.Tasks[0].ID != "A1" || page.Tasks[0].Day != 0 || page.Tasks[0].DeepLink != "https://line.me/R/oaMessage/@ch/?KEY0412" {
		t.Errorf("A1 task incorrect: %+v", page.Tasks[0])
	}
	if page.Tasks[1].ID != "A2" || page.Tasks[1].Day != 1 || page.Tasks[1].DeepLink != "https://act2.link" {
		t.Errorf("A2 task incorrect: %+v", page.Tasks[1])
	}
}
