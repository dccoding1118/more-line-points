package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCmd(t *testing.T) {
	ctx := context.Background()

	// 測試未攜帶子命令時是否順利顯示 Help
	oldOut := executeWithOutput(ctx, []string{})
	if !bytes.Contains(oldOut, []byte("A notification system for LINE promotional activities")) &&
		!bytes.Contains(oldOut, []byte("Fetches LINE activities")) {
		t.Errorf("expected root help message, got %s", oldOut)
	}

	// 測試未知的子命令
	errOut := executeWithOutput(ctx, []string{"unknown-command"})
	if !bytes.Contains(errOut, []byte("unknown command")) {
		t.Errorf("expected unknown command error, got %s", errOut)
	}
}

func executeWithOutput(ctx context.Context, args []string) []byte {
	root := &cobra.Command{
		Use:   "scheduler",
		Short: "A notification system for LINE promotional activities",
		Long:  "Fetches LINE activities, parses keywords, and sends tasks summaries.",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	root.AddCommand(newInitCmd(ctx))

	b := bytes.NewBufferString("")
	root.SetOut(b)
	root.SetErr(b)
	root.SetArgs(args)

	_ = root.ExecuteContext(ctx)
	return b.Bytes()
}
