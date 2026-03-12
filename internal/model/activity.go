package model

import (
	"time"
)

// ActivityType represents the categorized type of an activity.
// It is defined as a string to allow extensibility via parse_rules.yaml.
const (
	ActivityTypeUnknown     = "unknown"
	ActivityTypeOther       = "other"
	ActivityTypeKeyword     = "keyword"
	ActivityTypeShare       = "share"
	ActivityTypeShopCollect = "shop-collect"
	ActivityTypePoll        = "poll"
	ActivityTypeTask        = "task"
	ActivityTypeLuckyDraw   = "lucky-draw"
	ActivityTypeAppCheckin  = "app-checkin"
	ActivityTypePassport    = "passport"
)

// Activity represents a LINE promotional activity.
type Activity struct {
	ID          string
	Title       string
	ChannelName string
	ChannelID   string
	Type        string
	PageURL     string
	ActionURL   string // Added for Patch 3 (L3)
	ValidFrom   time.Time
	ValidUntil  time.Time
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
