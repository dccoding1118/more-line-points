package notify

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/discord"
	"github.com/dccoding1118/more-line-points/internal/email"
	"github.com/dccoding1118/more-line-points/internal/model"
	"github.com/dccoding1118/more-line-points/internal/storage"
)

// Notifier orchestrates the notification pipeline.
type Notifier struct {
	activityStore storage.ActivityStore
	taskStore     storage.DailyTaskStore
	dcSender      discord.Sender
	emailSender   email.Sender
	mapping       *config.ChannelMapping
}

// NewNotifier creates a new Notifier.
func NewNotifier(
	as storage.ActivityStore,
	ts storage.DailyTaskStore,
	dc discord.Sender,
	em email.Sender,
	cm *config.ChannelMapping,
) *Notifier {
	return &Notifier{
		activityStore: as,
		taskStore:     ts,
		dcSender:      dc,
		emailSender:   em,
		mapping:       cm,
	}
}

// Run executes the full notification data flow.
func (n *Notifier) Run(ctx context.Context, targetDate time.Time) error {
	if n.dcSender == nil && n.emailSender == nil {
		return nil
	}

	// 1. Fetch active activities
	activities, err := n.activityStore.GetActivitiesByDate(ctx, targetDate)
	if err != nil {
		return fmt.Errorf("failed to get activities: %w", err)
	}

	// 2. Fetch daily tasks and build map
	tasks, err := n.taskStore.GetDailyTasksByDate(ctx, targetDate)
	if err != nil {
		return fmt.Errorf("failed to get daily tasks: %w", err)
	}

	log.Printf("[Notify] Target Date: %s | Activities Found: %d | Tasks Found: %d",
		targetDate.Format("2006-01-02"), len(activities), len(tasks))

	taskMap := make(map[string]model.DailyTask, len(tasks))
	for _, t := range tasks {
		taskMap[t.ActivityID] = t
	}

	// 3. Classify into 8 categories
	classified := make(map[string][]displayItem)

	for _, act := range activities {
		log.Printf("[Notify] Processing [%s] Type: %s, Title: %s", act.ID, act.Type, act.Title)
		if act.Type == model.ActivityTypeUnknown {
			log.Printf("[Notify] Skipping [%s]: Type is unknown", act.ID)
			continue
		}

		switch act.Type {
		case model.ActivityTypeKeyword:
			task, ok := taskMap[act.ID]
			if !ok {
				continue
			}
			item, err := n.buildKeywordItem(act, task)
			if err != nil {
				return err
			}
			if item != nil {
				classified[act.Type] = append(classified[act.Type], *item)
			}

		case model.ActivityTypeShopCollect:
			task, ok := taskMap[act.ID]
			if !ok {
				continue
			}
			label := act.Title
			taskURL := task.URL
			if taskURL == "" {
				taskURL = act.ActionURL
			}
			classified[act.Type] = append(classified[act.Type], displayItem{
				label: label,
				url:   taskURL,
			})

		default:
			// For other known types, use ActionURL directly
			classified[act.Type] = append(classified[act.Type], displayItem{
				label: act.Title,
				url:   act.ActionURL,
			})
		}
	}

	// 4. Format
	dateLabel := targetDate.Format("01/02")
	discordMsg := formatDiscord(dateLabel, classified)
	emailHTML := formatEmail(dateLabel, classified)
	subject := fmt.Sprintf("📅 %s LINE 任務清單", dateLabel)

	// 5. Dispatch
	if n.dcSender != nil {
		log.Printf("[Notify] Sending to Discord... (length: %d chars)", len(discordMsg))
		if err := n.dcSender.SendMessage(ctx, discordMsg); err != nil {
			log.Printf("[Notify] Discord error: %v (Message content truncated in log)", err)
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

// buildKeywordItem constructs a displayItem for a keyword activity,
// applying channel mapping degradation rules.
func (n *Notifier) buildKeywordItem(act model.Activity, task model.DailyTask) (*displayItem, error) {
	keyword := task.Keyword
	label := fmt.Sprintf("%s: 輸入 %s", act.ChannelName, keyword)

	channelID, found := n.mapping.LookupChannelID(act.ChannelName)
	if !found {
		onMissing := n.mapping.OnMissing
		if onMissing == "" {
			onMissing = "warn"
		}

		switch onMissing {
		case "skip":
			log.Printf("channel mapping not found for %q, skipping", act.ChannelName)
			return nil, nil
		case "error":
			return nil, fmt.Errorf("channel mapping not found for %q", act.ChannelName)
		default: // "warn"
			log.Printf("channel mapping not found for %q, showing with warning", act.ChannelName)
			return &displayItem{
				label:   label,
				warning: true,
			}, nil
		}
	}

	// url.QueryEscape produces '+'. LINE expects '%20' instead of '+' for spaces.
	escapedKeyword := strings.ReplaceAll(url.QueryEscape(keyword), "+", "%20")

	deepLink := fmt.Sprintf("https://line.me/R/oaMessage/%s/?%s",
		channelID, escapedKeyword)

	return &displayItem{
		label: label,
		url:   deepLink,
	}, nil
}
