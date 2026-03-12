package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// Execute 啟動 CLI 程式的根節點。
func Execute(ctx context.Context) error {
	rootCmd := &cobra.Command{
		Use:   "scheduler",
		Short: "A notification system for LINE promotional activities",
		Long:  "Fetches LINE activities, parses keywords, and sends tasks summaries.",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}

	rootCmd.AddCommand(newInitCmd(ctx))
	rootCmd.AddCommand(newSyncCmd(ctx))
	rootCmd.AddCommand(newNotifyCmd(ctx))
	return rootCmd.ExecuteContext(ctx)
}
