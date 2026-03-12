package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
)

func TestFetchActivities(t *testing.T) {
	ctx := context.Background()

	t.Run("C1: 單頁面成功回傳全資料", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("region") != "tw" {
				t.Errorf("expected region=tw")
			}
			resp := APIResponse{Status: "OK"}
			resp.Result.DataList = []RawActivity{
				{ChannelName: "LINE 購物", EventTitle: "Act 1"},
				{ChannelName: "LINE 旅遊", EventTitle: "Act 2"},
			}
			resp.Result.PageToken = nil
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)

		items, err := client.FetchActivities(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
	})

	t.Run("C2: 多頁面回傳", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageToken := r.URL.Query().Get("pageToken")
			resp := APIResponse{Status: "OK"}

			switch pageToken {
			case "":
				resp.Result.DataList = []RawActivity{{EventTitle: "Page 1"}}
				nextPage := "cursor2"
				resp.Result.PageToken = &nextPage
			case "cursor2":
				resp.Result.DataList = []RawActivity{{EventTitle: "Page 2"}}
				resp.Result.PageToken = nil
			default:
				t.Errorf("unexpected pageToken: %s", pageToken)
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)
		client.pageDelay = 10 * time.Millisecond // 加速測試

		items, err := client.FetchActivities(ctx)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(items) != 2 || items[0].EventTitle != "Page 1" || items[1].EventTitle != "Page 2" {
			t.Errorf("wrong items: %v", items)
		}
	})

	t.Run("C3: API HTTP 非 200", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)

		_, err := client.FetchActivities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("C4: JSON 解析失敗", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintln(w, "invalid json")
		}))
		defer ts.Close()

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)

		_, err := client.FetchActivities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("C5: API Result Code != 200", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := APIResponse{Status: "FAILED"}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)

		_, err := client.FetchActivities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("C6: URL 解析錯誤", func(t *testing.T) {
		cfg := config.APIConfig{BaseURL: "::invalid-url", Region: "tw"}
		client := NewClient(cfg)

		_, err := client.FetchActivities(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("C7: Context Canceled", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond) // Ensure the handler blocks long enough for test
			resp := APIResponse{Status: "OK"}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		ctxCancel, cancel := context.WithCancel(context.Background())
		cancel() // 立即中斷

		cfg := config.APIConfig{BaseURL: ts.URL, Region: "tw"}
		client := NewClient(cfg)

		_, err := client.FetchActivities(ctxCancel)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
