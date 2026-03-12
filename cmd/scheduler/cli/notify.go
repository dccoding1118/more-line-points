package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/discord"
	"github.com/dccoding1118/more-line-points/internal/email"
	"github.com/dccoding1118/more-line-points/internal/notify"
	"github.com/dccoding1118/more-line-points/internal/storage"
	"github.com/spf13/cobra"
)

type notifyOptions struct {
	configPath string
	date       string
}

func newNotifyCmd(ctx context.Context) *cobra.Command {
	opts := &notifyOptions{}

	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Push daily tasks to Discord and/or Email",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.configPath == "" {
				return fmt.Errorf("usage: scheduler notify --config <path>")
			}
			return runNotify(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.configPath, "config", "", "path to config file (required)")
	cmd.Flags().StringVar(&opts.date, "date", "", "target date in YYYY-MM-DD format (defaults to tomorrow)")
	return cmd
}

func runNotify(ctx context.Context, opts *notifyOptions) error {
	// Parse target date
	var targetDate time.Time
	if opts.date != "" {
		var err error
		targetDate, err = time.Parse("2006-01-02", opts.date)
		if err != nil {
			return fmt.Errorf("failed to parse date %q: %w", opts.date, err)
		}
	} else {
		targetDate = time.Now().AddDate(0, 0, 1)
	}

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}

	cm, err := config.LoadChannelMapping(cfg.ChannelMapping.Path)
	if err != nil {
		return err
	}

	store, err := storage.NewSQLiteStore(ctx, cfg.Database.Path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var dcSender discord.Sender
	if cfg.Discord.Enabled {
		var err error
		dcSender, err = discord.NewSender(cfg.Discord.BotToken, cfg.Discord.NotifyChannelID, cfg.Discord.APIEndpoint)
		if err != nil {
			return fmt.Errorf("failed to create discord sender: %w", err)
		}
		defer func() { _ = dcSender.Close() }()
	}

	var emailSender email.Sender
	if cfg.Email.Enabled {
		emailSender = email.NewSender(
			cfg.Email.CredentialsPath, cfg.Email.TokenPath,
			cfg.Email.SenderMail,
			cfg.Email.Recipients,
		)
	}

	n := notify.NewNotifier(store, store, dcSender, emailSender, cm)
	if err := n.Run(ctx, targetDate); err != nil {
		return fmt.Errorf("notify failed: %w", err)
	}

	log.Printf("Notification sent for %s", targetDate.Format("2006-01-02"))
	return nil
}
