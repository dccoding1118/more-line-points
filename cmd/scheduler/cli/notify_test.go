package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotifyCmd(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test.db")
	mapPath := filepath.Join(tmpDir, "map.yaml")
	rulesPath := filepath.Join(tmpDir, "rules.yaml")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfgData := fmt.Sprintf(`database:
  path: %s
channel_mapping:
  path: %s
api:
  base_url: http://api
  region: tw
  headers:
    origin: o
    referer: r
    user-agent: u
parser:
  rules_path: %s
discord:
  enabled: false
email:
  enabled: false
`, dbPath, mapPath, rulesPath)

	if err := os.WriteFile(cfgPath, []byte(cfgData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, []byte("mappings: {}\non_missing: warn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulesPath, []byte("rules: []\ndate_patterns: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Missing --config",
			args:        []string{"notify"},
			wantErr:     true,
			errContains: "usage: scheduler notify --config <path>",
		},
		{
			name:        "Invalid --date format",
			args:        []string{"notify", "--config", cfgPath, "--date", "03-05-2026"},
			wantErr:     true,
			errContains: "failed to parse date",
		},
		{
			name:    "Valid config with disabled senders",
			args:    []string{"notify", "--config", cfgPath, "--date", "2026-03-05"},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newNotifyCmd(ctx)
			b := bytes.NewBufferString("")
			cmd.SetOut(b)
			cmd.SetErr(b)
			cmd.SetArgs(tc.args[1:])

			err := cmd.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error containing %q, got %v", tc.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
			}
		})
	}
}
