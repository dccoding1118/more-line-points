package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/apiclient"
	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/htmlparser"
	"github.com/dccoding1118/more-line-points/internal/model"
)

// mockActivityStore implements storage.ActivityStore for testing.
type mockActivityStore struct {
	UpsertCalled       int
	UpdateTypeCalled   int
	UpdateURLCalled    int
	MarkInactiveCalled int
	ListAllReturn      []string
}

func (m *mockActivityStore) UpsertActivity(ctx context.Context, a *model.Activity) error {
	m.UpsertCalled++
	return nil
}

func (m *mockActivityStore) GetActivity(ctx context.Context, id string) (*model.Activity, error) {
	return nil, nil
}

func (m *mockActivityStore) ListAllActivityIDs(ctx context.Context) ([]string, error) {
	return m.ListAllReturn, nil
}

func (m *mockActivityStore) MarkInactive(ctx context.Context, ids []string) error {
	m.MarkInactiveCalled++
	return nil
}

func (m *mockActivityStore) CleanExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

func (m *mockActivityStore) UpdateType(ctx context.Context, id, actType string) error {
	m.UpdateTypeCalled++
	return nil
}

func (m *mockActivityStore) UpdateActionURL(ctx context.Context, id, actionURL string) error {
	m.UpdateURLCalled++
	return nil
}

func (m *mockActivityStore) GetActivitiesByDate(ctx context.Context, targetDate time.Time) ([]model.Activity, error) {
	return nil, nil
}

// mockSyncStateStore implements storage.SyncStateStore for testing.
type mockSyncStateStore struct {
	Hashes      map[string]string
	SetCount    int
	UpdateCount int
}

func newMockSyncStore() *mockSyncStateStore {
	return &mockSyncStateStore{Hashes: make(map[string]string)}
}

func (m *mockSyncStateStore) GetHash(ctx context.Context, key string) (string, error) {
	return m.Hashes[key], nil
}

func (m *mockSyncStateStore) SetHash(ctx context.Context, key, hash string, syncedAt time.Time) error {
	m.Hashes[key] = hash
	m.SetCount++
	return nil
}

func (m *mockSyncStateStore) UpdateSyncedAt(ctx context.Context, key string, syncedAt time.Time) error {
	m.UpdateCount++
	return nil
}

// mockDailyTaskStore implements storage.DailyTaskStore for testing.
type mockDailyTaskStore struct {
	ReplaceCalled int
}

func (m *mockDailyTaskStore) ReplaceDailyTasks(ctx context.Context, activityID string, tasks []model.DailyTask) error {
	m.ReplaceCalled++
	return nil
}

func (m *mockDailyTaskStore) GetDailyTasksByDate(ctx context.Context, date time.Time) ([]model.DailyTask, error) {
	return nil, nil
}

// mockHTTPFetcher for testing HTML parsing.
type mockHTTPFetcher struct {
	htmlMap map[string]string
}

func (m *mockHTTPFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	if html, ok := m.htmlMap[url]; ok {
		return []byte(html), nil
	}
	return []byte("<html></html>"), nil
}

func TestSync(t *testing.T) {
	ctx := context.Background()

	// Mock API Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiclient.APIResponse{Status: "OK"}
		resp.Result.DataList = []apiclient.RawActivity{
			{EventID: "evt-123", ChannelName: "LINE 購物", EventTitle: "Act 1", EventStartTime: "2026-03-01 10:00:00", EventEndTime: "2026-03-01 20:00:00", ClickURL: "http://h1"},
		}
		resp.Result.PageToken = nil
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	apiClient := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL, Region: "tw"})
	fetcher := &mockHTTPFetcher{htmlMap: make(map[string]string)}
	parser := htmlparser.NewParser(fetcher, &config.ParseRules{})

	t.Run("First Sync", func(t *testing.T) {
		actStore := &mockActivityStore{}
		syncStore := newMockSyncStore()
		dtStore := &mockDailyTaskStore{}
		parserMock := htmlparser.NewParser(&mockHTTPFetcher{}, &config.ParseRules{})
		syncer := NewSyncer(apiClient, actStore, syncStore, dtStore, parserMock, nil, nil)
		hasChange, err := syncer.Sync(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !hasChange {
			t.Errorf("expected change on first sync")
		}
		if actStore.UpsertCalled != 1 {
			t.Errorf("expected 1 upsert, got %d", actStore.UpsertCalled)
		}
	})

	t.Run("L1 Exact Match", func(t *testing.T) {
		actStore := &mockActivityStore{}
		syncStore := newMockSyncStore()
		dtStore := &mockDailyTaskStore{}

		rawItems := []apiclient.RawActivity{{EventID: "evt-123", ChannelName: "LINE 購物", EventTitle: "Act 1", EventStartTime: "2026-03-01 10:00:00", EventEndTime: "2026-03-01 20:00:00"}}
		l1 := computeL1Hash(rawItems)
		syncStore.Hashes["activity_list"] = l1

		syncer := NewSyncer(apiClient, actStore, syncStore, dtStore, parser, nil, nil)
		hasChange, err := syncer.Sync(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if hasChange {
			t.Errorf("expected no change")
		}
		if syncStore.UpdateCount == 0 {
			t.Errorf("expected update synced_at")
		}
	})

	t.Run("L1 Change but L2 Match", func(t *testing.T) {
		// New ID appears, but existing ID matched L2. MarkInactive should trigger.
		rawItems := []apiclient.RawActivity{{EventID: "evt-1", ChannelName: "C", EventTitle: "T1", EventStartTime: "2026-03-01 10:00:00", EventEndTime: "2026-03-01 20:00:00", ClickURL: "http://h1"}}
		ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := apiclient.APIResponse{Status: "OK"}
			resp.Result.DataList = rawItems
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts2.Close()
		localAPI := apiclient.NewClient(config.APIConfig{BaseURL: ts2.URL, Region: "tw"})

		actStore := &mockActivityStore{ListAllReturn: []string{"evt-1", "evt-old"}}
		syncStore := newMockSyncStore()
		syncStore.Hashes["activity_list"] = "different"
		syncStore.Hashes["activity:evt-1"] = computeL2Hash(rawItems[0])
		parserMock := htmlparser.NewParser(&mockHTTPFetcher{}, &config.ParseRules{})
		syncer := NewSyncer(localAPI, actStore, syncStore, &mockDailyTaskStore{}, parserMock, nil, nil)
		hasChange, _ := syncer.Sync(ctx)

		if !hasChange {
			t.Errorf("expected change due to deactivation")
		}
		if actStore.UpsertCalled != 0 {
			t.Errorf("expected 0 upsert because L2 matched")
		}
		if actStore.MarkInactiveCalled != 1 {
			t.Errorf("expected mark inactive called")
		}
	})
}

func TestSync_L3(t *testing.T) {
	ctx := context.Background()

	t.Run("Poll detection via clickURL", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := apiclient.APIResponse{Status: "OK"}
			resp.Result.DataList = []apiclient.RawActivity{
				{EventID: "p1", EventTitle: "填問卷拿好禮", ClickURL: "http://event.line.me/poll/123"},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()
		api := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL})

		rules := &config.ParseRules{
			Rules: []config.TypeRule{
				{Type: model.ActivityTypePoll, URLPattern: "event.line.me/poll/", URLOnly: true, UseClickURL: true},
			},
		}

		actStore := &mockActivityStore{}
		syncStore := newMockSyncStore()
		syncer := NewSyncer(api, actStore, syncStore, &mockDailyTaskStore{}, htmlparser.NewParser(&mockHTTPFetcher{}, rules), nil, rules)

		_, _ = syncer.Sync(ctx)
		if actStore.UpdateTypeCalled != 1 {
			t.Errorf("expected UpdateType for poll")
		}
	})

	t.Run("L3 content change triggers task replacement", func(t *testing.T) {
		actID := "kw1"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := apiclient.APIResponse{Status: "OK"}
			resp.Result.DataList = []apiclient.RawActivity{{EventID: actID, ClickURL: "http://kw-page"}}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()
		api := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL})

		rules := &config.ParseRules{
			Rules: []config.TypeRule{
				{Type: model.ActivityTypeKeyword, TextPatterns: []string{"輸入"}, URLPattern: "line.me", HasDailyTasks: true, HasKeyword: true},
			},
			DatePatterns: []string{`(\d+)/(\d+)`},
		}
		fetcher := &mockHTTPFetcher{htmlMap: map[string]string{
			"http://kw-page": "<html><body>輸入關鍵字 <li>3/5 <a href='https://line.me/R/oaMessage/@id/?K1'>K1</a></li></body></html>",
		}}

		actStore := &mockActivityStore{}
		syncStore := newMockSyncStore()
		dtStore := &mockDailyTaskStore{}
		syncer := NewSyncer(api, actStore, syncStore, dtStore, htmlparser.NewParser(fetcher, rules), nil, rules)

		// 1. Initial L3 sync
		if hasChange, err := syncer.Sync(ctx); err != nil {
			t.Fatalf("first sync failed: %v", err)
		} else if !hasChange {
			t.Error("expected hasChange true on first L3 sync")
		}
		if dtStore.ReplaceCalled != 1 {
			t.Errorf("expected ReplaceDailyTasks called")
		}
		l3Hash := syncStore.Hashes["detail:"+actID]
		if l3Hash == "" {
			t.Fatal("expected L3 hash set")
		}

		// 2. Second sync with same content (L3 matched)
		syncStore.Hashes["activity_list"] = "force-mismatch-2"
		if _, err := syncer.Sync(ctx); err != nil {
			t.Fatalf("second sync failed: %v", err)
		}
		if dtStore.ReplaceCalled != 1 {
			t.Errorf("expected ReplaceDailyTasks NOT called again")
		}

		// 3. Third sync with changed content
		syncStore.Hashes["activity_list"] = "force-mismatch-3"
		fetcher.htmlMap["http://kw-page"] = "<html><body>輸入關鍵字 <li>3/6 <a href='https://line.me/R/oaMessage/@id/?K2'>K2</a></li></body></html>"
		if hasChange, err := syncer.Sync(ctx); err != nil {
			t.Fatalf("third sync failed: %v", err)
		} else if !hasChange {
			t.Errorf("expected hasChange true on third L3 sync")
		}
		if dtStore.ReplaceCalled != 2 {
			t.Errorf("expected ReplaceDailyTasks called again for changed content, count: %d", dtStore.ReplaceCalled)
		}
	})
}

func TestComputeL1Hash_Empty(t *testing.T) {
	hash := computeL1Hash(nil)
	if hash != "" {
		t.Errorf("expected empty hash for no items")
	}
}

// mockSyncStateStoreErr implements SyncStateStore and returns errors
type mockSyncStateStoreErr struct {
	*mockSyncStateStore
}

func (m *mockSyncStateStoreErr) GetHash(ctx context.Context, key string) (string, error) {
	return "", fmt.Errorf("mock error")
}

// mockActivityStoreErr implements ActivityStore and returns errors
type mockActivityStoreErr struct {
	*mockActivityStore
}

func (m *mockActivityStoreErr) UpsertActivity(ctx context.Context, a *model.Activity) error {
	return fmt.Errorf("mock error")
}

func (m *mockActivityStoreErr) MarkInactive(ctx context.Context, ids []string) error {
	return fmt.Errorf("mock error")
}

func (m *mockActivityStoreErr) ListAllActivityIDs(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("mock error")
}

func TestSync_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("API Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		api := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL})
		syncer := NewSyncer(api, &mockActivityStore{}, newMockSyncStore(), &mockDailyTaskStore{}, htmlparser.NewParser(&mockHTTPFetcher{}, &config.ParseRules{}), nil, nil)
		_, err := syncer.Sync(ctx)
		if err == nil {
			t.Error("expected sync error due to API failure")
		}
	})

	t.Run("GetHash L1 Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := apiclient.APIResponse{Status: "OK"}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()
		api := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL})
		syncer := NewSyncer(api, &mockActivityStore{}, &mockSyncStateStoreErr{newMockSyncStore()}, &mockDailyTaskStore{}, htmlparser.NewParser(&mockHTTPFetcher{}, &config.ParseRules{}), nil, nil)
		_, err := syncer.Sync(ctx)
		if err == nil {
			t.Error("expected sync error due to GetHash failure")
		}
	})

	t.Run("GetHash L2 Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := apiclient.APIResponse{Status: "OK"}
			resp.Result.DataList = []apiclient.RawActivity{{EventID: "e1"}}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()
		api := apiclient.NewClient(config.APIConfig{BaseURL: ts.URL})
		errStore := &mockSyncStateStoreErr{newMockSyncStore()}
		syncer := NewSyncer(api, &mockActivityStore{}, errStore, &mockDailyTaskStore{}, htmlparser.NewParser(&mockHTTPFetcher{}, &config.ParseRules{}), nil, nil)
		// We need to bypass L1 error
		syncer.syncStateStore = errStore

		_, err := syncer.Sync(ctx)
		if err == nil {
			t.Error("expected sync error due to L2 GetHash failure")
		}
	})
}
