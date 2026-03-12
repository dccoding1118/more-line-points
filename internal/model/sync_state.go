package model

import "time"

// SyncState represents the sync state for a given hash level.
type SyncState struct {
	Key      string
	Hash     string
	SyncedAt time.Time
}
