package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
)

// RawActivity 是由 LINE API 回傳的單個活動結構
type RawActivity struct {
	EventID        string `json:"eventId"`
	EventTitle     string `json:"eventTitle"`
	ChannelName    string `json:"channelName"`
	ClickURL       string `json:"clickUrl"`
	EventStartTime string `json:"eventStartTime"`
	EventEndTime   string `json:"eventEndTime"`
}

// APIResponse 代表 API 解析時的外層與 result 結構
type APIResponse struct {
	Status string `json:"status"`
	Result struct {
		DataList  []RawActivity `json:"dataList"`
		PageToken *string       `json:"pageToken"`
	} `json:"result"`
}

// Client 負責與 LINE API 進行溝通
type Client struct {
	cfg        config.APIConfig
	httpClient *http.Client
	pageDelay  time.Duration // 分頁請求間隔 (通常為 1s)
}

// NewClient 建立新的 apiclient 實體
func NewClient(cfg config.APIConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		pageDelay: 1 * time.Second,
	}
}

// FetchActivities 根據 config 中的 BaseURL 與 Region 進行分頁抓取直到 hasMore=false。
func (c *Client) FetchActivities(ctx context.Context) ([]RawActivity, error) {
	var allItems []RawActivity
	var pageToken *string

	baseURL, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 組合請求 URL
		q := baseURL.Query()
		q.Set("region", c.cfg.Region)
		if pageToken != nil {
			q.Set("pageToken", *pageToken)
		}
		baseURL.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// 設置 Headers
		req.Header.Set("Origin", c.cfg.Headers.Origin)
		req.Header.Set("Referer", c.cfg.Headers.Referer)
		req.Header.Set("User-Agent", c.cfg.Headers.UserAgent)

		// 執行要求
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read body: %w", err)
		}

		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		if apiResp.Status != "OK" {
			return nil, fmt.Errorf("api returned status: %s", apiResp.Status)
		}

		allItems = append(allItems, apiResp.Result.DataList...)

		if apiResp.Result.PageToken == nil {
			break
		}
		pageToken = apiResp.Result.PageToken

		// 防止過於頻繁請求，延遲一秒
		time.Sleep(c.pageDelay)
	}

	return allItems, nil
}
