package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dccoding1118/more-line-points/internal/apiclient"
)

func TestSyncCmd(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Mock LINE API Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiclient.APIResponse{Status: "OK"}
		resp.Result.DataList = []apiclient.RawActivity{
			{ChannelName: "LINE 購物", EventTitle: "Act 1"},
		}
		resp.Result.PageToken = nil
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	mapPath := filepath.Join(tmpDir, "map.yaml")
	dbPath := filepath.Join(tmpDir, "test.db")
	rulesPath := filepath.Join(tmpDir, "rules.yaml")

	cfgData := fmt.Sprintf(`database:
  path: %s
channel_mapping:
  path: %s
api:
  base_url: %s
  region: tw
  headers:
    origin: o
    referer: r
    user-agent: u
parser:
  rules_path: %s
`, dbPath, mapPath, ts.URL, rulesPath)

	if err := os.WriteFile(cfgPath, []byte(cfgData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mapPath, []byte("mappings:\n  'LINE 購物': '@shopping'\n"), 0o600); err != nil {
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
			name:    "Valid sync execution",
			args:    []string{"sync", "--config", cfgPath},
			wantErr: false,
		},
		{
			name:        "Missing --config",
			args:        []string{"sync"},
			wantErr:     true,
			errContains: "usage: scheduler sync --config <path>",
		},
		{
			name:        "Config error",
			args:        []string{"sync", "--config", createBadFile(t, tmpDir, "bad_sync.yaml", ":::xxx")},
			wantErr:     true,
			errContains: "failed to parse config",
		},
		{
			name:        "API error triggers Sync Failed",
			args:        []string{"sync", "--config", createCfgWithDbPathOnly(t, tmpDir, "cfg_bad_api.yaml", dbPath, mapPath, rulesPath)},
			wantErr:     true,
			errContains: "sync failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}

			cmd := newSyncCmd(ctx)
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
