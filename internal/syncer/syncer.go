package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/dccoding1118/more-line-points/internal/apiclient"
	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/htmlparser"
	"github.com/dccoding1118/more-line-points/internal/model"
	"github.com/dccoding1118/more-line-points/internal/storage"
)

// Syncer orchestrates the data fetching from API and synchronization with DB.
type Syncer struct {
	apiClient      *apiclient.Client
	activityStore  storage.ActivityStore
	syncStateStore storage.SyncStateStore
	dailyTaskStore storage.DailyTaskStore
	htmlParser     *htmlparser.Parser
	channelMapping *config.ChannelMapping
	parseRules     *config.ParseRules
}

// NewSyncer creates a new Syncer instance.
func NewSyncer(
	api *apiclient.Client,
	actStore storage.ActivityStore,
	syncStore storage.SyncStateStore,
	dtStore storage.DailyTaskStore,
	parser *htmlparser.Parser,
	cm *config.ChannelMapping,
	rules *config.ParseRules,
) *Syncer {
	return &Syncer{
		apiClient:      api,
		activityStore:  actStore,
		syncStateStore: syncStore,
		dailyTaskStore: dtStore,
		htmlParser:     parser,
		channelMapping: cm,
		parseRules:     rules,
	}
}

// Sync performs a full data synchronization cycle.
// Returns (anyChangeFound, error).
func (s *Syncer) Sync(ctx context.Context) (bool, error) {
	// 1. Fetch from API
	rawItems, err := s.apiClient.FetchActivities(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to fetch activities: %w", err)
	}

	// 2. Compute L1 Hash (Overall Hash)
	l1Hash := computeL1Hash(rawItems)

	// 3. Check if L1 Hash changed (T1 Mechanism)
	existingL1, err := s.syncStateStore.GetHash(ctx, "activity_list")
	if err != nil {
		return false, fmt.Errorf("failed to get L1 hash: %w", err)
	}

	now := time.Now().UTC()

	if existingL1 == l1Hash {
		// T1: Hash identical, only update SyncedAt
		if err1 := s.syncStateStore.UpdateSyncedAt(ctx, "activity_list", now); err1 != nil {
			return false, fmt.Errorf("failed to update L1 synced_at: %w", err1)
		}
		return false, nil
	}

	// 4. L1 changed, perform details update (T2 Mechanism)
	var activeIDs []string
	hasChange := false

	// Process each item from API
	for _, item := range rawItems {
		id := item.EventID
		activeIDs = append(activeIDs, id)

		l2Key := "activity:" + id
		l2Hash := computeL2Hash(item)

		existingL2, errL2 := s.syncStateStore.GetHash(ctx, l2Key)
		if errL2 != nil {
			return false, fmt.Errorf("failed to get L2 hash for %s: %w", id, errL2)
		}

		act := s.convertToModel(id, item)

		// T2-L2 Check: Basic metadata
		if existingL2 != l2Hash {
			hasChange = true
			if err2 := s.activityStore.UpsertActivity(ctx, act); err2 != nil {
				return false, fmt.Errorf("failed to upsert activity %s: %w", id, err2)
			}
			if err3 := s.syncStateStore.SetHash(ctx, l2Key, l2Hash, now); err3 != nil {
				return false, fmt.Errorf("failed to set L2 hash for %s: %w", id, err3)
			}
			log.Printf("[L1/L2 Sync] Created/Updated activity id=%s title=%s page_url=%s", act.ID, act.Title, act.PageURL)
		} else {
			_ = s.syncStateStore.UpdateSyncedAt(ctx, l2Key, now)
		}

		// T2-L3 Check: Detailed content (Patch 3)
		if changed, errL3 := s.syncL3(ctx, act, now); errL3 != nil {
			log.Printf("failed to sync L3 for %s: %v", id, errL3)
		} else if changed {
			hasChange = true
		}
	}

	// 5. Handle deletions (T3 Mechanism)
	allIDs, err := s.activityStore.ListAllActivityIDs(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to list all activities: %w", err)
	}

	inactiveIDs := findMissingIDs(allIDs, activeIDs)
	if len(inactiveIDs) > 0 {
		hasChange = true
		if err4 := s.activityStore.MarkInactive(ctx, inactiveIDs); err4 != nil {
			return false, fmt.Errorf("failed to mark inactive: %w", err4)
		}
	}

	// 6. Update T1 (L1)
	if err5 := s.syncStateStore.SetHash(ctx, "activity_list", l1Hash, now); err5 != nil {
		return false, fmt.Errorf("failed to set L1 hash: %w", err5)
	}

	return hasChange, nil
}

// computeL1Hash 透過將整個 API 回傳的 items 抽取 EventID 排序後計算 SHA256。
func computeL1Hash(items []apiclient.RawActivity) string {
	// L1 Hash (根據設計檔: 所有活動 ID 排序後串接，取 SHA-256)
	if len(items) == 0 {
		return ""
	}
	var ids []string
	for _, item := range items {
		ids = append(ids, item.EventID)
	}
	sort.Strings(ids)

	raw := strings.Join(ids, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// computeL2Hash 計算單筆 Item 的 SHA256 (根據設計檔規則)。
func computeL2Hash(item apiclient.RawActivity) string {
	// L2 Hash 規則: eventId|eventTitle|channelName|clickUrl|eventStartTime|eventEndTime
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		item.EventID, item.EventTitle, item.ChannelName, item.ClickURL, item.EventStartTime, item.EventEndTime)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *Syncer) convertToModel(id string, raw apiclient.RawActivity) *model.Activity {
	channelID := ""
	if s.channelMapping != nil {
		if cid, ok := s.channelMapping.LookupChannelID(raw.ChannelName); ok {
			channelID = cid
		}
	}

	// Parse "2006-01-02 15:04:05" (Asia/Taipei)
	layout := "2006-01-02 15:04:05"
	loc, _ := time.LoadLocation("Asia/Taipei")
	vf, _ := time.ParseInLocation(layout, raw.EventStartTime, loc)
	vu, _ := time.ParseInLocation(layout, raw.EventEndTime, loc)

	return &model.Activity{
		ID:          id,
		Title:       strings.TrimSpace(raw.EventTitle),
		ChannelName: raw.ChannelName,
		ChannelID:   channelID,
		Type:        model.ActivityTypeUnknown,
		PageURL:     raw.ClickURL,
		ValidFrom:   vf,
		ValidUntil:  vu,
	}
}

// computeL3Hash computes SHA256 of the extracted TaskHTML.
func computeL3Hash(taskHTML string) string {
	if taskHTML == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(taskHTML))
	return hex.EncodeToString(sum[:])
}

// syncL3 handles detail page parsing and task synchronization.
func (s *Syncer) syncL3(ctx context.Context, act *model.Activity, now time.Time) (bool, error) {
	// L3 Pre-filter (URL-only based short circuit)
	if s.parseRules != nil {
		for _, rule := range s.parseRules.Rules {
			if rule.URLOnly && rule.URLPattern != "" && strings.Contains(act.PageURL, rule.URLPattern) {
				if err := s.activityStore.UpdateType(ctx, act.ID, rule.Type); err == nil {
					if rule.UseClickURL {
						_ = s.activityStore.UpdateActionURL(ctx, act.ID, act.PageURL)
					}
					return false, nil
				}
			}
		}
	}

	// HTML-based Identification
	res, err := s.htmlParser.Parse(ctx, act)
	if err != nil {
		return false, fmt.Errorf("l3 parse failed for %s: %w", act.ID, err)
	}

	// Update Activity Type and ActionURL unconditionally
	_ = s.activityStore.UpdateType(ctx, act.ID, res.Type)
	if res.ActionURL != "" {
		_ = s.activityStore.UpdateActionURL(ctx, act.ID, res.ActionURL)
	}

	l3Key := "detail:" + act.ID
	l3Hash := computeL3Hash(res.TaskHTML)
	existingL3, _ := s.syncStateStore.GetHash(ctx, l3Key)

	hasChange := false

	// Update Daily Tasks if TaskHTML changed
	if l3Hash != "" && existingL3 != l3Hash {
		if err := s.dailyTaskStore.ReplaceDailyTasks(ctx, act.ID, res.DailyTasks); err == nil {
			_ = s.syncStateStore.SetHash(ctx, l3Key, l3Hash, now)
			hasChange = true
		}
	} else if l3Hash != "" {
		_ = s.syncStateStore.UpdateSyncedAt(ctx, l3Key, now)
	}

	return hasChange, nil
}

func findMissingIDs(allIDs, activeIDs []string) []string {
	activeMap := make(map[string]bool, len(activeIDs))
	for _, id := range activeIDs {
		activeMap[id] = true
	}

	var missing []string
	for _, id := range allIDs {
		if !activeMap[id] {
			missing = append(missing, id)
		}
	}
	return missing
}
