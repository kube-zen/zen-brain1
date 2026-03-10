// Package config provides configuration loading for zen-brain.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the full zen-brain configuration.
type Config struct {
	HomeDir string `yaml:"home_dir"`

	Logging LoggingConfig `yaml:"logging"`

	KB         KBConfig         `yaml:"kb"`
	QMD        QMDConfig        `yaml:"qmd"`
	Jira       JiraConfig       `yaml:"jira"`
	Confluence ConfluenceConfig `yaml:"confluence"`
	Clusters   []ClusterConfig  `yaml:"clusters"`
	SRED       SREDConfig       `yaml:"sred"`
	Ledger     LedgerConfig     `yaml:"ledger"`

	ZenContext ZenContextConfig `yaml:"zen_context"`

	Planner PlannerConfig `yaml:"planner"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// KBConfig holds knowledge base configuration.
type KBConfig struct {
	DocsRepo    string `yaml:"docs_repo"`
	SearchLimit int    `yaml:"search_limit"`
}

// QMDConfig holds QMD configuration.
type QMDConfig struct {
	BinaryPath      string        `yaml:"binary_path"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// JiraConfig holds Jira connector configuration.
type JiraConfig struct {
	Enabled   bool              `yaml:"enabled"`
	BaseURL   string            `yaml:"base_url"`
	Project   string            `yaml:"project"`    // Legacy; prefer ProjectKey
	ProjectKey string           `yaml:"project_key"`
	Username  string            `yaml:"-"`          // From env var
	APIToken  string            `yaml:"-"`          // From env var
	Email     string            `yaml:"email"`
	WebhookURL    string        `yaml:"webhook_url"`
	WebhookSecret string        `yaml:"webhook_secret"`
	WebhookPort   int           `yaml:"webhook_port"`
	WebhookPath   string        `yaml:"webhook_path"`
	StatusMapping      map[string]string `yaml:"status_mapping"`
	WorkTypeMapping    map[string]string `yaml:"worktype_mapping"`
	PriorityMapping    map[string]string `yaml:"priority_mapping"`
	CustomFieldMapping map[string]string `yaml:"custom_field_mapping"`
}

// ConfluenceConfig holds Confluence integration configuration.
type ConfluenceConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BaseURL  string `yaml:"base_url"`
	Space    string `yaml:"space"`
	Username string `yaml:"-"` // From env var
	APIToken string `yaml:"-"` // From env var
}

// ClusterConfig holds multi-cluster configuration.
type ClusterConfig struct {
	ID         string `yaml:"id"`
	Type       string `yaml:"type"`
	Kubeconfig string `yaml:"kubeconfig"`
}

// SREDConfig holds SR&ED evidence collection configuration.
type SREDConfig struct {
	Enabled             bool     `yaml:"enabled"`
	DefaultTags         []string `yaml:"default_tags"`
	EvidenceRequirement string   `yaml:"evidence_requirement"`
}

// LedgerConfig holds ZenLedger configuration.
type LedgerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	SSLMode  string `yaml:"ssl_mode"`
}

// ZenContextConfig holds three-tier memory configuration.
type ZenContextConfig struct {
	Tier1Redis RedisTierConfig `yaml:"tier1_redis"`
	Tier2QMD   QMDTierConfig   `yaml:"tier2_qmd"`
	Tier3S3    S3TierConfig    `yaml:"tier3_s3"`
	Journal    JournalConfig   `yaml:"journal"`
	ClusterID  string          `yaml:"cluster_id"`
	Verbose    bool            `yaml:"verbose"`
}

// RedisTierConfig holds Redis configuration.
type RedisTierConfig struct {
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// QMDTierConfig holds QMD tier configuration.
type QMDTierConfig struct {
	RepoPath      string `yaml:"repo_path"`
	QMDBinaryPath string `yaml:"qmd_binary_path"`
	Verbose       bool   `yaml:"verbose"`
}

// S3TierConfig holds S3 tier configuration.
type S3TierConfig struct {
	Bucket            string        `yaml:"bucket"`
	Region            string        `yaml:"region"`
	Endpoint          string        `yaml:"endpoint"`
	AccessKeyID       string        `yaml:"access_key_id"`
	SecretAccessKey   string        `yaml:"secret_access_key"`
	SessionToken      string        `yaml:"session_token"`
	UsePathStyle      bool          `yaml:"use_path_style"`
	DisableSSL        bool          `yaml:"disable_ssl"`
	ForceRenameBucket bool          `yaml:"force_rename_bucket"`
	MaxRetries        int           `yaml:"max_retries"`
	Timeout           time.Duration `yaml:"timeout"`
	PartSize          int64         `yaml:"part_size"`
	Concurrency       int           `yaml:"concurrency"`
	Verbose           bool          `yaml:"verbose"`
}

// JournalConfig holds journal configuration.
type JournalConfig struct {
	JournalPath      string `yaml:"journal_path"`
	EnableQueryIndex bool   `yaml:"enable_query_index"`
}

// PlannerConfig holds planner configuration.
type PlannerConfig struct {
	DefaultModel    string  `yaml:"default_model"`
	MaxCostPerTask  float64 `yaml:"max_cost_per_task"`
	RequireApproval bool    `yaml:"require_approval"`
}

// LoadConfig loads configuration from a YAML file.
// If path is empty, tries default paths.
func LoadConfig(path string) (*Config, error) {
	// Determine config path
	if path == "" {
		path = findConfigPath()
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Load sensitive values from environment variables
	config.loadFromEnv()

	// Set defaults
	config.setDefaults()

	return &config, nil
}

// findConfigPath searches for the config file in standard locations.
func findConfigPath() string {
	// Try current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}
	if _, err := os.Stat("config.dev.yaml"); err == nil {
		return "config.dev.yaml"
	}

	// Try home directory
	homeDir := HomeDir()
	homeConfig := homeDir + "/config.yaml"
	if _, err := os.Stat(homeConfig); err == nil {
		return homeConfig
	}

	// Try configs directory
	configsDir := "../configs"
	for _, name := range []string{"config.yaml", "config.dev.yaml"} {
		if _, err := os.Stat(configsDir + "/" + name); err == nil {
			return configsDir + "/" + name
		}
	}

	// Default to config.dev.yaml
	return "../configs/config.dev.yaml"
}

// loadFromEnv loads sensitive configuration from environment variables.
// Supports both legacy and unified Jira env names: JIRA_URL, JIRA_EMAIL, JIRA_USERNAME,
// JIRA_TOKEN, JIRA_API_TOKEN, JIRA_PROJECT_KEY. APIToken prefers JIRA_API_TOKEN then JIRA_TOKEN;
// Email prefers JIRA_EMAIL then JIRA_USERNAME.
func (c *Config) loadFromEnv() {
	// Jira: unified env support
	if c.Jira.BaseURL == "" {
		c.Jira.BaseURL = os.Getenv("JIRA_URL")
	}
	if c.Jira.Email == "" {
		c.Jira.Email = os.Getenv("JIRA_EMAIL")
	}
	if c.Jira.Email == "" {
		c.Jira.Email = os.Getenv("JIRA_USERNAME")
	}
	if c.Jira.Username == "" {
		c.Jira.Username = os.Getenv("JIRA_USERNAME")
	}
	if c.Jira.APIToken == "" {
		c.Jira.APIToken = os.Getenv("JIRA_API_TOKEN")
	}
	if c.Jira.APIToken == "" {
		c.Jira.APIToken = os.Getenv("JIRA_TOKEN")
	}
	if c.Jira.ProjectKey == "" {
		c.Jira.ProjectKey = os.Getenv("JIRA_PROJECT_KEY")
	}
	// Compatibility: Project -> ProjectKey if ProjectKey empty
	if c.Jira.ProjectKey == "" && c.Jira.Project != "" {
		c.Jira.ProjectKey = c.Jira.Project
	}
	if c.Jira.Email == "" && c.Jira.Username != "" {
		c.Jira.Email = c.Jira.Username
	}

	// Confluence credentials
	c.Confluence.Username = os.Getenv("CONFLUENCE_USERNAME")
	c.Confluence.APIToken = os.Getenv("CONFLUENCE_API_TOKEN")

	// AWS credentials for S3
	if c.ZenContext.Tier3S3.AccessKeyID == "" {
		c.ZenContext.Tier3S3.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if c.ZenContext.Tier3S3.SecretAccessKey == "" {
		c.ZenContext.Tier3S3.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if c.ZenContext.Tier3S3.SessionToken == "" {
		c.ZenContext.Tier3S3.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}
}

// setDefaults sets default values for missing configuration.
func (c *Config) setDefaults() {
	if c.HomeDir == "" {
		c.HomeDir = HomeDir()
	}

	if c.KB.SearchLimit == 0 {
		c.KB.SearchLimit = 10
	}

	if c.QMD.RefreshInterval == 0 {
		c.QMD.RefreshInterval = 3600 * time.Second
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}

	if c.ZenContext.ClusterID == "" {
		c.ZenContext.ClusterID = "default"
	}
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		HomeDir: HomeDir(),
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		KB: KBConfig{
			DocsRepo:    "../zen-docs",
			SearchLimit: 10,
		},
		QMD: QMDConfig{
			BinaryPath:      "qmd",
			RefreshInterval: 3600 * time.Second,
		},
		SRED: SREDConfig{
			Enabled:             true,
			DefaultTags:         []string{"experimental_general"},
			EvidenceRequirement: "summary",
		},
		ZenContext: ZenContextConfig{
			ClusterID: "default",
			Verbose:   false,
		},
		Planner: PlannerConfig{
			DefaultModel:    "glm-4.7",
			MaxCostPerTask:  10.0,
			RequireApproval: false,
		},
	}
}
