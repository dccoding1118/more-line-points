package model

import "time"

// DailyTask represents an entry in the daily task list (e.g., keyword or specific shop link).
type DailyTask struct {
	ID         int64
	ActivityID string
	UseDate    time.Time
	Keyword    string // Populated for 'keyword' type
	URL        string // oaMessage link or shop link, added for Patch 3 (L3)
	Note       string
}
