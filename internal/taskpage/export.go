package taskpage

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
	"github.com/dccoding1118/more-line-points/internal/storage"
)

// JSONExporter handles exporting daily tasks to JSON for the static site.
type JSONExporter struct {
	activityStore storage.ActivityStore
	taskStore     storage.DailyTaskStore
}

// NewJSONExporter initializes a new JSONExporter.
func NewJSONExporter(as storage.ActivityStore, ts storage.DailyTaskStore) *JSONExporter {
	return &JSONExporter{
		activityStore: as,
		taskStore:     ts,
	}
}

// TaskItem represents a single task in the JSON feed.
type TaskItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ChannelName string `json:"channel_name"`
	ChannelID   string `json:"channel_id"`
	Type        string `json:"type"`
	Day         int    `json:"day"`
	Keyword     string `json:"keyword,omitempty"`
	DeepLink    string `json:"deep_link,omitempty"`
	PageURL     string `json:"page_url,omitempty"`
}

// TaskPage represents the structure of tasks.json.
type TaskPage struct {
	Date        string     `json:"date"`
	GeneratedAt string     `json:"generated_at"`
	Tasks       []TaskItem `json:"tasks"`
}

// Export runs the export process and writes to outputPath.
func (e *JSONExporter) Export(ctx context.Context, targetDate time.Time, outputPath string) error {
	activities, err := e.activityStore.GetActivitiesByDate(ctx, targetDate)
	if err != nil {
		return fmt.Errorf("failed to get activities: %w", err)
	}

	tasks, err := e.taskStore.GetDailyTasksByDate(ctx, targetDate)
	if err != nil {
		return fmt.Errorf("failed to get daily tasks: %w", err)
	}

	// Group tasks by activity_id
	taskMap := make(map[string]model.DailyTask)
	for _, t := range tasks {
		taskMap[t.ActivityID] = t
	}

	dateStr := targetDate.Format("2006-01-02")
	outTasks := make([]TaskItem, 0, len(activities))

	for _, act := range activities {
		if !act.IsActive || targetDate.Before(act.ValidFrom) {
			continue // Skip inactive or not yet started
		}

		day := calculateDay(targetDate, act.ValidFrom)
		item := TaskItem{
			ID:          act.ID,
			Title:       act.Title,
			ChannelName: act.ChannelName,
			ChannelID:   act.ChannelID,
			Type:        act.Type,
			Day:         day,
			PageURL:     act.PageURL,
		}

		switch act.Type {
		case model.ActivityTypeKeyword, model.ActivityTypeShopCollect:
			t, ok := taskMap[act.ID]
			if !ok {
				continue // Skip if no specific daily task mapped
			}
			item.Keyword = t.Keyword
			if t.URL != "" {
				item.DeepLink = t.URL
			}
		case model.ActivityTypeUnknown:
			continue
		default:
			// other, poll, share, lucky-draw, app-checkin, passport
			// ActionURL is usually where user is led for direct click tasks
			if act.ActionURL != "" {
				item.DeepLink = act.ActionURL
			}
		}

		outTasks = append(outTasks, item)
	}

	page := TaskPage{
		Date:        dateStr,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Tasks:       outTasks,
	}

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write tasks.json: %w", err)
	}

	return nil
}

// calculateDay returns the difference in days. If target=validFrom, returns 0.
func calculateDay(target, validFrom time.Time) int {
	tDay := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	vDay := time.Date(validFrom.Year(), validFrom.Month(), validFrom.Day(), 0, 0, 0, 0, validFrom.Location())
	diff := tDay.Sub(vDay).Hours() / 24
	if diff < 0 {
		return 0
	}
	return int(math.Round(diff))
}
