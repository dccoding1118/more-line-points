package notify

import (
	"fmt"
	"strings"
)

// displayItem represents a single notification line item.
type displayItem struct {
	label   string // Display text
	url     string // Link URL (empty for degraded items)
	warning bool   // True when channel lookup failed with on_missing=warn
}

// categoryDef defines a notification category.
type categoryDef struct {
	key   string
	emoji string
	title string
}

var categories = []categoryDef{
	{key: "keyword", emoji: "🔑", title: "關鍵字任務"},
	{key: "shop-collect", emoji: "🛍️", title: "收藏指定店家"},
	{key: "lucky-draw", emoji: "🎁", title: "點我試手氣"},
	{key: "poll", emoji: "🗳️", title: "投票任務"},
	{key: "app-checkin", emoji: "📱", title: "App 簽到任務"},
	{key: "passport", emoji: "📗", title: "購物護照任務"},
	{key: "share", emoji: "🔗", title: "分享好友任務"},
	{key: "other", emoji: "📌", title: "其他任務"},
}

// formatDiscord produces Discord-compatible Markdown.
func formatDiscord(dateLabel string, classified map[string][]displayItem) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**📅 %s LINE 任務清單**", dateLabel))

	hasContent := false
	for _, cat := range categories {
		items, ok := classified[cat.key]
		if !ok || len(items) == 0 {
			continue
		}
		hasContent = true
		sb.WriteString(fmt.Sprintf("\n\n**%s %s**", cat.emoji, cat.title))
		for _, item := range items {
			sb.WriteString("\n")
			if item.warning {
				sb.WriteString(fmt.Sprintf("• ⚠️ %s (需手動前往頻道)", item.label))
			} else if item.url != "" {
				sb.WriteString(fmt.Sprintf("• [%s](%s)", item.label, item.url))
			} else {
				sb.WriteString(fmt.Sprintf("• %s", item.label))
			}
		}
	}

	if !hasContent {
		sb.WriteString("\n\n當日無需執行任務 ✅")
	}

	return sb.String()
}

// formatEmail produces full HTML with ul/li lists.
func formatEmail(dateLabel string, classified map[string][]displayItem) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<h2>📅 %s LINE 任務清單</h2>", dateLabel))

	hasContent := false
	for _, cat := range categories {
		items, ok := classified[cat.key]
		if !ok || len(items) == 0 {
			continue
		}
		hasContent = true
		sb.WriteString(fmt.Sprintf("\n<h3>%s %s</h3>\n<ul>", cat.emoji, cat.title))
		for _, item := range items {
			if item.warning {
				sb.WriteString(fmt.Sprintf("\n  <li>⚠️ %s (需手動前往頻道)</li>", item.label))
			} else if item.url != "" {
				sb.WriteString(fmt.Sprintf("\n  <li><a href=\"%s\">%s</a></li>", item.url, item.label))
			} else {
				sb.WriteString(fmt.Sprintf("\n  <li>%s</li>", item.label))
			}
		}
		sb.WriteString("\n</ul>")
	}

	if !hasContent {
		sb.WriteString("\n<p>當日無需執行任務 ✅</p>")
	}

	return sb.String()
}
