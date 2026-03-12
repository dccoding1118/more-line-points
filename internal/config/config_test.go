package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()

	cases := []struct {
		name              string
		yamlContent       string
		expectErr         bool
		errContains       string
		expectedDBPath    string
		expectedRulesPath string
	}{
		{
			name: "Valid YAML Config",
			yamlContent: `
database:
  path: data/test.db
channel_mapping:
  path: config/test_map.yaml
api:
  base_url: https://api.line.me
  region: tw
  headers:
    origin: expected-origin
    referer: expected-referer
    user-agent: expected-user-agent
parser:
  rules_path: config/rules.yaml
`,
			expectErr:         false,
			expectedDBPath:    "data/test.db",
			expectedRulesPath: "config/rules.yaml",
		},
		{
			name: "YAML with unknown fields",
			yamlContent: `
database:
  path: data/test.db
api:
  base_url: ok
  region: ok
  headers:
    origin: ok
    referer: ok
    user-agent: ok
parser:
  rules_path: rules
unknown:
  foo: bar
`,
			expectErr:         false,
			expectedDBPath:    "data/test.db",
			expectedRulesPath: "rules",
		},
		{
			name:        "Invalid YAML format",
			yamlContent: "database:\n  path: data/test.db\n\tinvalid_tab",
			expectErr:   true,
			errContains: "failed to parse config",
		},
		{
			name: "Missing required field",
			yamlContent: `
database:
  path: ""
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "database.path is required",
			expectedRulesPath: "rules",
		},
		{
			name: "Missing api base_url",
			yamlContent: `
database:
  path: db
api:
  region: tw
  headers:
    origin: ok
    referer: ok
    user-agent: ok
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "api.base_url is required",
			expectedRulesPath: "rules",
		},
		{
			name: "Missing api region",
			yamlContent: `
database:
  path: db
api:
  base_url: ok
  headers:
    origin: ok
    referer: ok
    user-agent: ok
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "api.region is required",
			expectedRulesPath: "rules",
		},
		{
			name: "Missing api headers origin",
			yamlContent: `
database:
  path: db
api:
  base_url: ok
  region: tw
  headers:
    referer: ok
    user-agent: ok
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "api.headers.origin is required",
			expectedRulesPath: "rules",
		},
		{
			name: "Missing api headers referer",
			yamlContent: `
database:
  path: db
api:
  base_url: ok
  region: tw
  headers:
    origin: ok
    user-agent: ok
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "api.headers.referer is required",
			expectedRulesPath: "rules",
		},
		{
			name: "Missing api headers user-agent",
			yamlContent: `
database:
  path: db
api:
  base_url: ok
  region: tw
  headers:
    origin: ok
    referer: ok
parser:
  rules_path: rules
`,
			expectErr:         true,
			errContains:       "api.headers.user-agent is required",
			expectedRulesPath: "rules",
		},
		{
			name:        "Empty database path (field completely missing)",
			yamlContent: "something_else: ok\nparser:\n  rules_path: rules",
			expectErr:   true,
			errContains: "database.path is required",
		},
		{
			name: "Missing parser rules_path",
			yamlContent: `
database:
  path: db
api:
  base_url: ok
  region: tw
  headers:
    origin: ok
    referer: ok
    user-agent: ok
`,
			expectErr:   true,
			errContains: "parser.rules_path is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace spaces in test name for file path compatibility
			filePath := filepath.Join(tmpDir, strings.ReplaceAll(tc.name, " ", "_")+".yaml")
			err := os.WriteFile(filePath, []byte(tc.yamlContent), 0o600)
			if err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			cfg, err := Load(filePath)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Database.Path != tc.expectedDBPath {
				t.Errorf("expected db path %q, got %q", tc.expectedDBPath, cfg.Database.Path)
			}
			if tc.expectedRulesPath != "" && cfg.Parser.RulesPath != tc.expectedRulesPath {
				t.Errorf("expected rules path %q, got %q", tc.expectedRulesPath, cfg.Parser.RulesPath)
			}
		})
	}

	t.Run("Discord and Email config parsing", func(t *testing.T) {
		content := `
database:
  path: data/test.db
api:
  base_url: http://api
  region: tw
  headers:
    origin: o
    referer: r
    user-agent: u
parser:
  rules_path: rules.yaml
discord:
  enabled: true
  bot_token: "token123"
  guild_id: "guild123"
  notify_channel_id: "notify123"
  admin_channel_id: "admin123"
email:
  enabled: true
  credentials_path: creds.json
  token_path: token.json
  sender_mail: test@gmail.com
  recipients:
    - user1@gmail.com
    - user2@gmail.com
`
		filePath := filepath.Join(tmpDir, "tg_email_config.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		cfg, err := Load(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.Discord.Enabled || cfg.Discord.BotToken != "token123" || cfg.Discord.NotifyChannelID != "notify123" {
			t.Errorf("discord config mismatch: %+v", cfg.Discord)
		}
		if !cfg.Email.Enabled || cfg.Email.CredentialsPath != "creds.json" || cfg.Email.TokenPath != "token.json" {
			t.Errorf("email config mismatch: %+v", cfg.Email)
		}
		if len(cfg.Email.Recipients) != 2 || cfg.Email.Recipients[0] != "user1@gmail.com" {
			t.Errorf("email recipients mismatch: %v", cfg.Email.Recipients)
		}
	})

	t.Run("File not found", func(t *testing.T) {
		_, err := Load(filepath.Join(tmpDir, "nonexistent.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read config file") {
			t.Errorf("expected error containing 'failed to read', got %q", err.Error())
		}
	})
}

func TestLoadParseRules(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("Valid Rules", func(t *testing.T) {
		content := `
rules:
  - type: keyword
    text_patterns: ["輸入關鍵字"]
    url_pattern: "line.me"
    has_daily_tasks: true
date_patterns:
  - '\d+月\d+日'
`
		path := filepath.Join(tmpDir, "rules.yaml")
		// Write the valid rules content to a temporary file
		_ = os.WriteFile(path, []byte(content), 0o600)

		pr, err := LoadParseRules(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Check if rules are loaded correctly
		if len(pr.Rules) != 1 || pr.Rules[0].Type != "keyword" {
			t.Errorf("expected 1 rule of type keyword, got %v", pr.Rules)
		}
		// Check if date patterns are loaded correctly
		if len(pr.DatePatterns) != 1 || pr.DatePatterns[0] != `\d+月\d+日` {
			t.Errorf("expected date pattern, got %v", pr.DatePatterns)
		}
	})

	t.Run("File not found", func(t *testing.T) {
		// Attempt to load a non-existent file
		_, err := LoadParseRules(filepath.Join(tmpDir, "none.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("Invalid YAML", func(t *testing.T) {
		path := filepath.Join(tmpDir, "bad.yaml")
		// Write invalid YAML content to a temporary file
		_ = os.WriteFile(path, []byte("rules:\n  - : bad"), 0o600)
		// Attempt to load invalid YAML
		_, err := LoadParseRules(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Check if the error message indicates a parsing failure
		if !strings.Contains(err.Error(), "failed to parse rules") {
			t.Errorf("expected parse error, got %v", err)
		}
	})
}

func TestLoadChannelMapping(t *testing.T) {
	tmpDir := t.TempDir()

	cases := []struct {
		name        string
		yamlContent string
		expectErr   bool
		errContains string
		expectedKey string
		expectedVal string
	}{
		{
			name: "Valid YAML Mapping",
			yamlContent: `
mappings:
  "LINE 購物": "@lineshopping"
on_missing: warn
`,
			expectErr:   false,
			expectedKey: "LINE 購物",
			expectedVal: "@lineshopping",
		},
		{
			name: "Empty Mappings",
			yamlContent: `
on_missing: warn
`,
			expectErr: false,
		},
		{
			name:        "Invalid YAML format",
			yamlContent: "mappings:\n  invalid_tab\n",
			expectErr:   true,
			errContains: "failed to parse channel mapping",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Replace spaces in test name for file path compatibility
			filePath := filepath.Join(tmpDir, strings.ReplaceAll(tc.name, " ", "_")+".yaml")
			err := os.WriteFile(filePath, []byte(tc.yamlContent), 0o600)
			if err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			cm, err := LoadChannelMapping(filePath)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("expected error containing %q, got %q", tc.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cm.Mappings == nil {
				t.Fatal("expected mappings to be initialized, got nil")
			}
			if tc.expectedKey != "" {
				if val, ok := cm.Mappings[tc.expectedKey]; !ok || val != tc.expectedVal {
					t.Errorf("expected mapping %q -> %q, got %q", tc.expectedKey, tc.expectedVal, val)
				}
			}
		})
	}

	t.Run("File not found", func(t *testing.T) {
		_, err := LoadChannelMapping(filepath.Join(tmpDir, "nonexistent.yaml"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read channel mapping") {
			t.Errorf("expected error containing 'failed to read', got %q", err.Error())
		}
	})
}

func TestChannelMapping_LookupChannelID(t *testing.T) {
	cm := &ChannelMapping{
		Mappings: map[string]string{
			"活動 A": "@actA",
		},
	}

	// Test case 1: Existing channel ID
	id, ok := cm.LookupChannelID("活動 A")
	if !ok || id != "@actA" {
		t.Errorf("expected @actA and true, got %q and %v", id, ok)
	}

	// Test case 2: Non-existent channel ID
	id, ok = cm.LookupChannelID("NotExist")
	if ok || id != "" {
		t.Errorf("expected empty string and false, got %q and %v", id, ok)
	}
}
