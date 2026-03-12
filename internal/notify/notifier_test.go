package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/model"
)

// --- Mock implementations ---

type mockActivityStore struct {
	activities []model.Activity
	err        error
}

func (m *mockActivityStore) GetActivitiesByDate(_ context.Context, _ time.Time) ([]model.Activity, error) {
	return m.activities, m.err
}

func (m *mockActivityStore) UpsertActivity(_ context.Context, _ *model.Activity) error { return nil }
func (m *mockActivityStore) GetActivity(_ context.Context, _ string) (*model.Activity, error) {
	return nil, nil
}
func (m *mockActivityStore) ListAllActivityIDs(_ context.Context) ([]string, error) { return nil, nil }
func (m *mockActivityStore) MarkInactive(_ context.Context, _ []string) error       { return nil }
func (m *mockActivityStore) CleanExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (m *mockActivityStore) UpdateType(_ context.Context, _, _ string) error      { return nil }
func (m *mockActivityStore) UpdateActionURL(_ context.Context, _, _ string) error { return nil }

type mockTaskStore struct {
	tasks []model.DailyTask
	err   error
}

func (m *mockTaskStore) ReplaceDailyTasks(_ context.Context, _ string, _ []model.DailyTask) error {
	return nil
}

func (m *mockDcSender) Close() error {
	return nil
}

func (m *mockTaskStore) GetDailyTasksByDate(_ context.Context, _ time.Time) ([]model.DailyTask, error) {
	return m.tasks, m.err
}

type mockDcSender struct {
	lastMsg string
	err     error
	called  bool
}

func (m *mockDcSender) SendMessage(_ context.Context, text string) error {
	m.called = true
	m.lastMsg = text
	return m.err
}

type mockEmailSender struct {
	lastSubject string
	lastBody    string
	err         error
	called      bool
}

func (m *mockEmailSender) SendHTML(_ context.Context, subject, htmlBody string) error {
	m.called = true
	m.lastSubject = subject
	m.lastBody = htmlBody
	return m.err
}

// --- Tests ---

func TestNotifierRun(t *testing.T) {
	targetDate := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		activities  []model.Activity
		tasks       []model.DailyTask
		onMissing   string
		mappings    map[string]string
		dcSender    *mockDcSender
		emailSender *mockEmailSender
		wantErr     bool
		errContains string
		checkDc     func(t *testing.T, msg string)
		checkEmail  func(t *testing.T, body string)
		checkNoSend bool
	}{
		{
			name: "N1: Mixed types with complete task mapping",
			activities: []model.Activity{
				{ID: "a1", Title: "LINE 購物活動", ChannelName: "LINE 購物", Type: model.ActivityTypeKeyword, ActionURL: "http://x"},
				{ID: "a2", Title: "抽獎活動", Type: model.ActivityTypeLuckyDraw, ActionURL: "http://lucky"},
				{ID: "a3", Title: "投票活動", Type: model.ActivityTypePoll, ActionURL: "http://poll"},
			},
			tasks: []model.DailyTask{
				{ActivityID: "a1", Keyword: "0305關鍵字"},
			},
			onMissing:   "warn",
			mappings:    map[string]string{"LINE 購物": "@lineshopping"},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     false,
			checkDc: func(t *testing.T, msg string) {
				t.Helper()
				if !strings.Contains(msg, "🔑 關鍵字任務") {
					t.Error("expected keyword section")
				}
				if !strings.Contains(msg, "🎁 點我試手氣") {
					t.Error("expected lucky-draw section")
				}
				if !strings.Contains(msg, "🗳️ 投票任務") {
					t.Error("expected poll section")
				}
				if !strings.Contains(msg, "@lineshopping") {
					t.Error("expected deep link with channel ID")
				}
			},
			checkEmail: func(t *testing.T, body string) {
				t.Helper()
				if !strings.Contains(body, "<h3>🔑 關鍵字任務</h3>") {
					t.Error("expected keyword h3 section")
				}
				if !strings.Contains(body, "<ul>") {
					t.Error("expected ul tags")
				}
			},
		},
		{
			name: "N2: keyword activity without DailyTask excluded",
			activities: []model.Activity{
				{ID: "a1", Title: "No task keyword", ChannelName: "LINE 購物", Type: model.ActivityTypeKeyword},
			},
			tasks:       []model.DailyTask{},
			onMissing:   "warn",
			mappings:    map[string]string{"LINE 購物": "@lineshopping"},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     false,
			checkDc: func(t *testing.T, msg string) {
				t.Helper()
				if !strings.Contains(msg, "當日無需執行任務") {
					t.Errorf("expected no-task message, got: %s", msg)
				}
			},
		},
		{
			name:        "N3: Zero activities",
			activities:  []model.Activity{},
			tasks:       []model.DailyTask{},
			onMissing:   "warn",
			mappings:    map[string]string{},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     false,
			checkDc: func(t *testing.T, msg string) {
				t.Helper()
				if !strings.Contains(msg, "當日無需執行任務") {
					t.Errorf("expected no-task message, got: %s", msg)
				}
			},
		},
		{
			name: "N4: LookupChannelID fails + on_missing=skip",
			activities: []model.Activity{
				{ID: "a1", Title: "Missing Channel", ChannelName: "Unknown Channel", Type: model.ActivityTypeKeyword},
			},
			tasks: []model.DailyTask{
				{ActivityID: "a1", Keyword: "test"},
			},
			onMissing:   "skip",
			mappings:    map[string]string{},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     false,
			checkDc: func(t *testing.T, msg string) {
				t.Helper()
				if strings.Contains(msg, "Missing Channel") {
					t.Error("expected activity to be skipped")
				}
				if !strings.Contains(msg, "當日無需執行任務") {
					t.Error("expected no-task message")
				}
			},
		},
		{
			name: "N5: LookupChannelID fails + on_missing=warn",
			activities: []model.Activity{
				{ID: "a1", Title: "Warn Channel", ChannelName: "Unknown Channel", Type: model.ActivityTypeKeyword},
			},
			tasks: []model.DailyTask{
				{ActivityID: "a1", Keyword: "testkey"},
			},
			onMissing:   "warn",
			mappings:    map[string]string{},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     false,
			checkDc: func(t *testing.T, msg string) {
				t.Helper()
				if !strings.Contains(msg, "⚠️") {
					t.Error("expected warning emoji")
				}
				if !strings.Contains(msg, "需手動前往頻道") {
					t.Error("expected manual channel note")
				}
			},
		},
		{
			name: "N6: LookupChannelID fails + on_missing=error",
			activities: []model.Activity{
				{ID: "a1", Title: "Error Channel", ChannelName: "Unknown Channel", Type: model.ActivityTypeKeyword},
			},
			tasks: []model.DailyTask{
				{ActivityID: "a1", Keyword: "testkey"},
			},
			onMissing:   "error",
			mappings:    map[string]string{},
			dcSender:    &mockDcSender{},
			emailSender: &mockEmailSender{},
			wantErr:     true,
			errContains: "channel mapping not found",
			checkNoSend: true,
		},
		{
			name:       "N7: Both senders nil",
			activities: []model.Activity{},
			tasks:      []model.DailyTask{},
			onMissing:  "warn",
			mappings:   map[string]string{},
			wantErr:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			as := &mockActivityStore{activities: tc.activities}
			ts := &mockTaskStore{tasks: tc.tasks}
			cm := &config.ChannelMapping{
				Mappings:  tc.mappings,
				OnMissing: tc.onMissing,
			}

			var dcSender *mockDcSender
			var emailSender *mockEmailSender

			n := NewNotifier(as, ts, nil, nil, cm)
			if tc.dcSender != nil {
				dcSender = tc.dcSender
				n.dcSender = dcSender
			}
			if tc.emailSender != nil {
				emailSender = tc.emailSender
				n.emailSender = emailSender
			}

			err := n.Run(context.Background(), targetDate)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				if tc.checkNoSend {
					if dcSender != nil && dcSender.called {
						t.Error("expected discord sender NOT called")
					}
					if emailSender != nil && emailSender.called {
						t.Error("expected email sender NOT called")
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkDc != nil && dcSender != nil {
				tc.checkDc(t, dcSender.lastMsg)
			}
			if tc.checkEmail != nil && emailSender != nil {
				tc.checkEmail(t, emailSender.lastBody)
			}
		})
	}
}

func TestNotifierRunStoreErrors(t *testing.T) {
	cm := &config.ChannelMapping{Mappings: map[string]string{}, OnMissing: "warn"}

	t.Run("Activity store error", func(t *testing.T) {
		as := &mockActivityStore{err: errors.New("db error")}
		ts := &mockTaskStore{}
		n := NewNotifier(as, ts, &mockDcSender{}, nil, cm)
		err := n.Run(context.Background(), time.Now())
		if err == nil || !strings.Contains(err.Error(), "failed to get activities") {
			t.Errorf("expected activity store error, got %v", err)
		}
	})

	t.Run("Task store error", func(t *testing.T) {
		as := &mockActivityStore{activities: []model.Activity{}}
		ts := &mockTaskStore{err: errors.New("db error")}
		n := NewNotifier(as, ts, &mockDcSender{}, nil, cm)
		err := n.Run(context.Background(), time.Now())
		if err == nil || !strings.Contains(err.Error(), "failed to get daily tasks") {
			t.Errorf("expected task store error, got %v", err)
		}
	})
}
