package cli

import (
	"context"
	"fmt"
	"log"

	"github.com/dccoding1118/more-line-points/internal/apiclient"
	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/htmlparser"
	"github.com/dccoding1118/more-line-points/internal/storage"
	"github.com/dccoding1118/more-line-points/internal/syncer"
	"github.com/spf13/cobra"
)

type syncOptions struct {
	configPath string
}

func newSyncCmd(ctx context.Context) *cobra.Command {
	opts := &syncOptions{}

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch and synchronize LINE activities",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.configPath == "" {
				return fmt.Errorf("usage: scheduler sync --config <path>")
			}
			return runSync(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.configPath, "config", "", "path to config file (required)")
	return cmd
}

func runSync(ctx context.Context, opts *syncOptions) error {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}

	cm, err := config.LoadChannelMapping(cfg.ChannelMapping.Path)
	if err != nil {
		return err
	}

	rules, err := config.LoadParseRules(cfg.Parser.RulesPath)
	if err != nil {
		return err
	}

	store, err := storage.NewSQLiteStore(ctx, cfg.Database.Path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	apiClient := apiclient.NewClient(cfg.API)
	fetcher := htmlparser.NewDefaultFetcher(cfg.API)
	p := htmlparser.NewParser(fetcher, rules)
	s := syncer.NewSyncer(apiClient, store, store, store, p, cm, rules)

	hasChange, err := s.Sync(ctx)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if hasChange {
		log.Println("Sync completed. Changes detected and updated.")
	} else {
		log.Println("Sync completed. No changes detected.")
	}

	return nil
}
