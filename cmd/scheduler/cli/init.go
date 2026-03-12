package cli

import (
	"context"
	"fmt"

	"github.com/dccoding1118/more-line-points/internal/config"
	"github.com/dccoding1118/more-line-points/internal/storage"

	"github.com/spf13/cobra"
)

type initOptions struct {
	configPath string
}

func newInitCmd(ctx context.Context) *cobra.Command {
	opts := &initOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the database and schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.configPath == "" {
				return fmt.Errorf("usage: scheduler init --config <path>")
			}
			return runInit(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.configPath, "config", "", "path to config file (required)")
	return cmd
}

func runInit(ctx context.Context, opts *initOptions) error {
	cfg, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}

	_, err = config.LoadChannelMapping(cfg.ChannelMapping.Path)
	if err != nil {
		return err
	}

	store, err := storage.NewSQLiteStore(ctx, cfg.Database.Path)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	fmt.Println("Initialization complete. Database:", cfg.Database.Path)
	return nil
}
