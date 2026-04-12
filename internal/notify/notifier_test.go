package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/model"
)

type mockDailyTaskStore struct {
	tasks []model.DailyTask
	err   error
}

func (m *mockDailyTaskStore) GetDailyTasksByDate(ctx context.Context, date time.Time) ([]model.DailyTask, error) {
	return m.tasks, m.err
}

func (m *mockDailyTaskStore) ReplaceDailyTasks(ctx context.Context, activityID string, tasks []model.DailyTask) error {
	return nil
}

type mockDiscordSender struct {
	sentMsg string
	err     error
}

func (m *mockDiscordSender) SendMessage(ctx context.Context, text string) error {
	m.sentMsg = text
	return m.err
}

func (m *mockDiscordSender) Close() error { return nil }

type mockEmailSender struct {
	sentSubject string
	sentHTML    string
	err         error
}

func (m *mockEmailSender) SendHTML(ctx context.Context, subject, htmlBody string) error {
	m.sentSubject = subject
	m.sentHTML = htmlBody
	return m.err
}

func TestNotifier_Run(t *testing.T) {
	targetDate := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	pagesURL := "https://example.com/pages"

	tests := []struct {
		name       string
		tasks      []model.DailyTask
		storeErr   error
		dcErr      error
		emErr      error
		wantErr    bool
		wantDcText string
	}{
		{
			name: "N1: multi tasks",
			tasks: []model.DailyTask{
				{ID: 1}, {ID: 2},
			},
			wantDcText: "2 項任務",
		},
		{
			name:       "N2: zero tasks",
			tasks:      []model.DailyTask{},
			wantDcText: "今日沒有需要執行",
		},
		{
			name:     "Store error",
			storeErr: errors.New("db error"),
			wantErr:  true,
		},
		{
			name: "Sender error does not block",
			tasks: []model.DailyTask{
				{ID: 1},
			},
			dcErr:      errors.New("discord down"),
			emErr:      errors.New("email down"),
			wantDcText: "1 項任務",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &mockDailyTaskStore{tasks: tc.tasks, err: tc.storeErr}
			dc := &mockDiscordSender{err: tc.dcErr}
			em := &mockEmailSender{err: tc.emErr}

			n := NewNotifier(store, dc, em, pagesURL)
			err := n.Run(context.Background(), targetDate)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// basic validation
			if tc.wantDcText != "" && dc.err == nil {
				if !strings.Contains(dc.sentMsg, tc.wantDcText) {
					t.Errorf("expected msg containing %q, got %q", tc.wantDcText, dc.sentMsg)
				}
			}
		})
	}
}
