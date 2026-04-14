package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 代表 config.yaml 的設定內容。
// 每個 Patch 會在此 struct 中新增該 Patch 所需的欄位。
// Patch 0 僅包含 database 與 channel_mapping 區塊。
type Config struct {
	Database       DatabaseConfig       `yaml:"database"`
	TaskPage       TaskPageConfig       `yaml:"taskpage"`
	ChannelMapping ChannelMappingConfig `yaml:"channel_mapping"`
	API            APIConfig            `yaml:"api"`
	Parser         ParserConfig         `yaml:"parser"` // Added for Patch 3 (L3)
	Discord        DiscordConfig        `yaml:"discord"`
	Email          EmailConfig          `yaml:"email"`
}

// DiscordConfig holds the Discord bot notification settings.
type DiscordConfig struct {
	Enabled         bool   `yaml:"enabled"`
	BotToken        string `yaml:"bot_token"`
	GuildID         string `yaml:"guild_id"`
	NotifyChannelID string `yaml:"notify_channel_id"`
	AdminChannelID  string `yaml:"admin_channel_id"`
	APIEndpoint     string `yaml:"api_endpoint"`
}

// EmailConfig holds the email notification settings.
type EmailConfig struct {
	Enabled         bool     `yaml:"enabled"`
	CredentialsPath string   `yaml:"credentials_path"`
	TokenPath       string   `yaml:"token_path"`
	SenderMail      string   `yaml:"sender"`
	Recipients      []string `yaml:"recipients"`
	RecipientsEnv   string   `yaml:"recipients_env"` // Commas separated string for env injection
}

type DatabaseConfig struct {
	Path string `yaml:"path"` // SQLite 檔案路徑，必填
}

type TaskPageConfig struct {
	OutputPath     string `yaml:"output_path"`      // tasks.json 輸出路徑
	GithubPagesURL string `yaml:"github_pages_url"` // 推播顯示用之前端任務網頁網址
}

type ChannelMappingConfig struct {
	Path string `yaml:"path"` // channel_mapping.yaml 的檔案路徑
}

type APIConfig struct {
	BaseURL string        `yaml:"base_url"` // API 端點 URL，必填
	Region  string        `yaml:"region"`   // 地區碼（如 "tw"），必填
	Headers HeadersConfig `yaml:"headers"`
}

type HeadersConfig struct {
	Origin    string `yaml:"origin"`     // Required
	Referer   string `yaml:"referer"`    // Required
	UserAgent string `yaml:"user-agent"` // Required
}

// ParserConfig holds the path to HTML parsing rules.
type ParserConfig struct {
	RulesPath string `yaml:"rules_path"` // Path to parse_rules.yaml, Required
}

// ParseRules represents the content of parse_rules.yaml.
type ParseRules struct {
	Rules        []TypeRule `yaml:"rules"`
	DatePatterns []string   `yaml:"date_patterns"`
}

// TypeRule defines how to identify an activity type and extract data.
type TypeRule struct {
	Type          string   `yaml:"type"`
	TextPatterns  []string `yaml:"text_patterns"`
	URLPattern    string   `yaml:"url_pattern"`
	HasDailyTasks bool     `yaml:"has_daily_tasks"`
	HasKeyword    bool     `yaml:"has_keyword"`
	URLOnly       bool     `yaml:"url_only"`
	UseClickURL   bool     `yaml:"use_click_url"`
}

// ChannelMapping 代表 channel_mapping.yaml 的內容。
// 定義 channelName → @channel_id 的對應關係。
type ChannelMapping struct {
	Mappings  map[string]string `yaml:"mappings"`   // key=channelName, value=@channel_id
	OnMissing string            `yaml:"on_missing"` // "warn" | "skip" | "error"，預設是 "warn"
}

// LookupChannelID 在 Mappings 中查詢 channelName 對應的 channel_id。
// 回傳 (channelID, true) 表示找到對應；("", false) 表示無對應。
func (cm *ChannelMapping) LookupChannelID(channelName string) (string, bool) {
	id, ok := cm.Mappings[channelName]
	return id, ok
}

// Load 讀取並解析設定檔。
func Load(path string) (*Config, error) {
	cleanPath := filepath.Clean(path)
	b, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	content := os.ExpandEnv(string(b))

	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 支援從字串環境變數轉換為陣列
	if cfg.Email.RecipientsEnv != "" {
		parts := strings.Split(cfg.Email.RecipientsEnv, ",")
		var parsed []string
		for _, p := range parts {
			if v := strings.TrimSpace(p); v != "" {
				parsed = append(parsed, v)
			}
		}
		if len(parsed) > 0 {
			cfg.Email.Recipients = parsed
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate 驗證必填欄位。
func (c *Config) Validate() error {
	if c.Database.Path == "" {
		return errors.New("config validation: database.path is required")
	}
	if c.TaskPage.OutputPath == "" {
		return errors.New("config validation: taskpage.output_path is required")
	}
	if c.TaskPage.GithubPagesURL == "" {
		return errors.New("config validation: taskpage.github_pages_url is required")
	}
	if c.API.BaseURL == "" {
		return errors.New("config validation: api.base_url is required")
	}
	if c.API.Region == "" {
		return errors.New("config validation: api.region is required")
	}
	if c.API.Headers.Origin == "" {
		return errors.New("config validation: api.headers.origin is required")
	}
	if c.API.Headers.Referer == "" {
		return errors.New("config validation: api.headers.referer is required")
	}
	if c.API.Headers.UserAgent == "" {
		return errors.New("config validation: api.headers.user-agent is required")
	}
	if c.Parser.RulesPath == "" {
		return errors.New("config validation: parser.rules_path is required")
	}
	return nil
}

// LoadParseRules reads and parses the HTML parsing rules file.
func LoadParseRules(path string) (*ParseRules, error) {
	cleanPath := filepath.Clean(path)
	b, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read parse rules: %w", err)
	}

	var pr ParseRules
	if err := yaml.Unmarshal(b, &pr); err != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", err)
	}

	return &pr, nil
}

// LoadChannelMapping 讀取並解析頻道映射檔。
func LoadChannelMapping(path string) (*ChannelMapping, error) {
	cleanPath := filepath.Clean(path)
	b, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read channel mapping: %w", err)
	}

	var cm ChannelMapping
	if err := yaml.Unmarshal(b, &cm); err != nil {
		return nil, fmt.Errorf("failed to parse channel mapping: %w", err)
	}

	if cm.Mappings == nil {
		cm.Mappings = make(map[string]string)
	}

	return &cm, nil
}
