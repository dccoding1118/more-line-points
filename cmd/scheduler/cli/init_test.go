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

func TestInitCmd(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	mapPath := filepath.Join(tmpDir, "map.yaml")
	dbPath := filepath.Join(tmpDir, "test.db")

	rulesPath := filepath.Join(tmpDir, "rules.yaml")

	cfgData := fmt.Sprintf(`taskpage:
  output_path: "data/tasks.json"
  github_pages_url: "https://test.io"
database:
  path: %s
channel_mapping:
  path: %s
api:
  base_url: http
  region: tw
  headers:
    origin: o
    referer: r
    user-agent: u
parser:
  rules_path: %s
`, dbPath, mapPath, rulesPath)

	if err := os.WriteFile(cfgPath, []byte(cfgData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, []byte("mappings:\n  'LINE': '@line'\n"), 0o600); err != nil {
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
		setup       func()
	}{
		{
			name:    "Valid init subcommand",
			args:    []string{"init", "--config", cfgPath},
			wantErr: false,
		},
		{
			name:        "Missing --config parameter",
			args:        []string{"init"},
			wantErr:     true,
			errContains: "usage: scheduler init --config <path>",
		},
		{
			name:        "Config file not found",
			args:        []string{"init", "--config", filepath.Join(tmpDir, "not_exist.yaml")},
			wantErr:     true,
			errContains: "failed to read config file",
		},
		{
			name:        "Config format error",
			args:        []string{"init", "--config", createBadFile(t, tmpDir, "bad.yaml", ":::invalid_yaml\n:what:is\tthis")},
			wantErr:     true,
			errContains: "failed to parse config",
		},
		{
			name:        "Channel mapping not found",
			args:        []string{"init", "--config", createCfgWithDbPathOnly(t, tmpDir, "cfg_no_map.yaml", dbPath, "invalid_map.yaml", rulesPath)},
			wantErr:     true,
			errContains: "failed to read channel mapping",
		},
		{
			name: "Storage initialization fails",
			args: []string{"init", "--config", createCfgWithDbPathOnly(t, tmpDir, "cfg_bad_db.yaml", filepath.Join(tmpDir, "readonly", "data.db"), mapPath, rulesPath)},
			setup: func() {
				// Create readonly directory to trigger db init failure
				p := filepath.Join(tmpDir, "readonly")
				_ = os.MkdirAll(p, 0o400)
			},
			wantErr:     true,
			errContains: "unable to open database file",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			// Rebuild init Command for testing (isolation)
			root := newInitCmd(ctx)
			// Redirect output to buffer to prevent stdout noise
			b := bytes.NewBufferString("")
			root.SetOut(b)
			root.SetErr(b)

			// Execute subcommand and arguments (removing 'init' prefix)
			root.SetArgs(tc.args[1:])

			err := root.Execute()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error to contain %q, but got %v", tc.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
			}
		})
	}
}

func createBadFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func createCfgWithDbPathOnly(t *testing.T, dir, name, db, m, r string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	c := fmt.Sprintf(`taskpage:
  output_path: "data/tasks.json"
  github_pages_url: "https://test.io"
database:
  path: %s
channel_mapping:
  path: %s
api:
  base_url: http
  region: tw
  headers:
    origin: o
    referer: r
    user-agent: u
parser:
  rules_path: %s
`, db, m, r)
	if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}
