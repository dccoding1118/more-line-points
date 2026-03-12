package htmlparser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/model"
)

type mockFetcher struct {
	html []byte
	err  error
}

func (m *mockFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	return m.html, m.err
}

func testRules() *config.ParseRules {
	return &config.ParseRules{
		Rules: []config.TypeRule{
			{
				Type:          model.ActivityTypeKeyword,
				TextPatterns:  []string{"輸入關鍵字"},
				URLPattern:    "line.me/R/oaMessage/",
				HasDailyTasks: true,
				HasKeyword:    true,
			},
			{
				Type:         model.ActivityTypeShare,
				TextPatterns: []string{"分享連結"},
				URLPattern:   "event.line.me/s/",
			},
			{
				Type:          model.ActivityTypeShopCollect,
				TextPatterns:  []string{"收藏指定店家"},
				URLPattern:    "buy.line.me/u/partner/",
				HasDailyTasks: true,
			},
		},
		DatePatterns: []string{
			`(\d+)月(\d+)日`,
			`(\d+)/(\d+)`,
		},
	}
}

func TestParser_Parse(t *testing.T) {
	ctx := context.Background()
	rules := testRules()

	t.Run("P1: keyword page", func(t *testing.T) {
		html := `
			<html>
			<body>
				<p>請輸入關鍵字：</p>
				<ul>
					<li>3月5日：<a href="https://line.me/R/oaMessage/@id/?0305KEY">K1</a></li>
				</ul>
			</body>
			</html>
		`
		fetcher := &mockFetcher{html: []byte(html)}
		p := NewParser(fetcher, rules)
		act := &model.Activity{PageURL: "http://test", ValidFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

		res, err := p.Parse(ctx, act)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Type != model.ActivityTypeKeyword {
			t.Errorf("expected keyword, got %q", res.Type)
		}
		if len(res.DailyTasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(res.DailyTasks))
		}
		if res.DailyTasks[0].Keyword != "0305KEY" {
			t.Errorf("expected 0305KEY, got %q", res.DailyTasks[0].Keyword)
		}
		if res.DailyTasks[0].UseDate.Month() != 3 || res.DailyTasks[0].UseDate.Day() != 5 {
			t.Errorf("expected 3/5, got %v", res.DailyTasks[0].UseDate)
		}
	})

	t.Run("P2: share page", func(t *testing.T) {
		html := `<html><body>分享連結拿好禮 <a href="https://event.line.me/s/123">GO</a></body></html>`
		p := NewParser(&mockFetcher{html: []byte(html)}, rules)
		res, _ := p.Parse(ctx, &model.Activity{})
		if res.Type != model.ActivityTypeShare || res.ActionURL != "https://event.line.me/s/123" {
			t.Errorf("expected share, got %q, action %q", res.Type, res.ActionURL)
		}
	})

	t.Run("P4: shop-collect page", func(t *testing.T) {
		html := `
			<html>
			<body>
				收藏指定店家
				<div>3/10 <a href="https://buy.line.me/u/partner/p1">Shop 1</a></div>
				<div>3/11 <a href="https://buy.line.me/u/partner/p2">Shop 2</a></div>
			</body>
			</html>
		`
		p := NewParser(&mockFetcher{html: []byte(html)}, rules)
		res, _ := p.Parse(ctx, &model.Activity{ValidFrom: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)})
		if res.Type != model.ActivityTypeShopCollect {
			t.Fatalf("expected shop-collect, got %q", res.Type)
		}
		if len(res.DailyTasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res.DailyTasks))
		}
	})

	t.Run("P8: other (no match)", func(t *testing.T) {
		html := `<html><body>nothing here</body></html>`
		p := NewParser(&mockFetcher{html: []byte(html)}, rules)
		res, _ := p.Parse(ctx, &model.Activity{})
		if res.Type != model.ActivityTypeOther {
			t.Errorf("expected other, got %q", res.Type)
		}
	})

	t.Run("P9: fetch fail", func(t *testing.T) {
		p := NewParser(&mockFetcher{err: fmt.Errorf("network fail")}, rules)
		_, err := p.Parse(ctx, &model.Activity{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("TaskHTML extraction", func(t *testing.T) {
		html := `<html><body>輸入關鍵字 <li>3/05 <a href="https://line.me/R/oaMessage/@id/?K">Link</a></li></body></html>`
		p := NewParser(&mockFetcher{html: []byte(html)}, rules)
		res, _ := p.Parse(ctx, &model.Activity{})
		if res.TaskHTML == "" {
			t.Error("expected TaskHTML not empty")
		}
		if !strings.Contains(res.TaskHTML, "3/05") || !strings.Contains(res.TaskHTML, "Link") {
			t.Errorf("TaskHTML mismatch: %q", res.TaskHTML)
		}
	})

	t.Run("Year wrapping", func(t *testing.T) {
		// ValidFrom = 2025-12-01, task date text = "1月5日" -> should be 2026-01-05
		html := `<html><body>輸入關鍵字 <li>1月5日 <a href="https://line.me/R/oaMessage/@id/?K">K</a></li></body></html>`
		p := NewParser(&mockFetcher{html: []byte(html)}, rules)
		act := &model.Activity{ValidFrom: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)}
		res, _ := p.Parse(ctx, act)
		if res.DailyTasks[0].UseDate.Year() != 2026 {
			t.Errorf("expected year 2026, got %d", res.DailyTasks[0].UseDate.Year())
		}
	})

	t.Run("Wildcard match", func(t *testing.T) {
		r := &config.ParseRules{
			Rules: []config.TypeRule{
				{Type: "wc", TextPatterns: []string{"點我*試手氣"}, URLPattern: "line.me"},
			},
		}
		html := `<html><body>點我週三試手氣 <a href="https://line.me">Link</a></body></html>`
		p := NewParser(&mockFetcher{html: []byte(html)}, r)
		res, _ := p.Parse(ctx, &model.Activity{})
		if res.Type != "wc" {
			t.Errorf("wildcard match failed")
		}
	})
}

func TestDefaultFetcher(t *testing.T) {
	ctx := context.Background()
	cfg := config.APIConfig{
		Headers: config.HeadersConfig{
			Origin:    "o",
			Referer:   "r",
			UserAgent: "u",
		},
	}
	fetcher := NewDefaultFetcher(cfg)

	t.Run("Fetch Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") != "o" {
				t.Errorf("expected origin o")
			}
			_, _ = w.Write([]byte("ok"))
		}))
		defer ts.Close()

		b, err := fetcher.Fetch(ctx, ts.URL)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if string(b) != "ok" {
			t.Errorf("expected ok, got %s", string(b))
		}
	})

	t.Run("Fetch HTTP Error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		_, err := fetcher.Fetch(ctx, ts.URL)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("Fetch Bad URL", func(t *testing.T) {
		_, err := fetcher.Fetch(ctx, "://bad-url")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}
