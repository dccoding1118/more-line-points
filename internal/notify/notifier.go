package notify

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dccoding1118/more-line-points/internal/discord"
	"github.com/dccoding1118/more-line-points/internal/email"
	"github.com/dccoding1118/more-line-points/internal/storage"
)

// Notifier orchestrates the notification pipeline.
type Notifier struct {
	taskStore   storage.DailyTaskStore
	dcSender    discord.Sender
	emailSender email.Sender
	pagesURL    string
}

// NewNotifier creates a new Notifier.
func NewNotifier(
	ts storage.DailyTaskStore,
	dc discord.Sender,
	em email.Sender,
	pagesURL string,
) *Notifier {
	return &Notifier{
		taskStore:   ts,
		dcSender:    dc,
		emailSender: em,
		pagesURL:    pagesURL,
	}
}

// Run executes the full notification data flow.
func (n *Notifier) Run(ctx context.Context, targetDate time.Time) error {
	if n.dcSender == nil && n.emailSender == nil {
		return nil
	}

	// 1. Fetch daily tasks to count them
	tasks, err := n.taskStore.GetDailyTasksByDate(ctx, targetDate)
	if err != nil {
		return fmt.Errorf("failed to get daily tasks: %w", err)
	}

	dateLabel := targetDate.Format("01/02")
	count := len(tasks)

	log.Printf("[Notify] Target Date: %s | Tasks Found: %d", targetDate.Format("2006-01-02"), count)

	var discordMsg, emailHTML string
	if count == 0 {
		discordMsg = fmt.Sprintf("📅 **%s LINE 任務清單**\n今日沒有需要執行的任務！", dateLabel)
		emailHTML = fmt.Sprintf("<h2>📅 %s LINE 任務清單</h2><p>今日沒有需要執行的任務！</p>", dateLabel)
	} else {
		discordMsg = fmt.Sprintf("📅 **%s LINE 任務清單已更新**\n共有 %d 項任務等待完成！\n\n👉 [點我前往任務首頁](%s)", dateLabel, count, n.pagesURL)
		emailHTML = fmt.Sprintf("<h2>📅 %s LINE 任務清單已更新</h2><p>共有 %d 項任務等待完成！</p><br/><p>👉 <a href=\"%s\">點我前往任務首頁</a></p>", dateLabel, count, n.pagesURL)
	}

	subject := fmt.Sprintf("📅 %s LINE 任務清單", dateLabel)

	// Dispatch
	if n.dcSender != nil {
		log.Printf("[Notify] Sending to Discord... (length: %d chars)", len(discordMsg))
		if err := n.dcSender.SendMessage(ctx, discordMsg); err != nil {
			log.Printf("[Notify] Discord error: %v", err)
		} else {
			log.Printf("[Notify] Discord message sent successfully")
		}
	}

	if n.emailSender != nil {
		log.Printf("[Notify] Sending Email... (subject=%q, body_len=%d)", subject, len(emailHTML))
		if err := n.emailSender.SendHTML(ctx, subject, emailHTML); err != nil {
			log.Printf("[Notify] Email error: %v", err)
		} else {
			log.Printf("[Notify] Email sent successfully")
		}
	}

	return nil
}
