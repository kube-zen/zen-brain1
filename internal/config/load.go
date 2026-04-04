// Package config provides configuration loading for zen-brain.
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/secrets"
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

	MessageBus MessageBusConfig `yaml:"message_bus"`

	Planner PlannerConfig `yaml:"planner"`
}

// MessageBusConfig holds Block 3 message bus configuration.
type MessageBusConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Kind     string `yaml:"kind"` // "redis"
	RedisURL string `yaml:"redis_url"`
	Stream   string `yaml:"stream"`
	Required bool   `yaml:"required"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// KBConfig holds knowledge base configuration.
type KBConfig struct {
	Enabled     bool   `yaml:"enabled"`
	DocsRepo    string `yaml:"docs_repo"`
	SearchLimit int    `yaml:"search_limit"`
	Required    bool   `yaml:"required"`
}

// QMDConfig holds QMD configuration.
type QMDConfig struct {
	BinaryPath      string        `yaml:"binary_path"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

// JiraConfig holds Jira connector configuration.
type JiraConfig struct {
	Enabled            bool              `yaml:"enabled"`
	BaseURL            string            `yaml:"base_url"`
	Project            string            `yaml:"project"` // Legacy; prefer ProjectKey
	ProjectKey         string            `yaml:"project_key"`
	Username           string            `yaml:"-"` // From env var
	APIToken           string            `yaml:"-"` // From env var
	Email              string            `yaml:"email"`
	WebhookURL         string            `yaml:"webhook_url"`
	WebhookSecret      string            `yaml:"webhook_secret"`
	WebhookPort        int               `yaml:"webhook_port"`
	WebhookPath        string            `yaml:"webhook_path"`
	StatusMapping      map[string]string `yaml:"status_mapping"`
	WorkTypeMapping    map[string]string `yaml:"worktype_mapping"`
	PriorityMapping    map[string]string `yaml:"priority_mapping"`
	CustomFieldMapping map[string]string `yaml:"custom_field_mapping"`

	// Canonical secret source fields
	CredentialsFile   string `yaml:"credentials_file"` // Path to host credential file (default: ~/.zen-brain/secrets/jira.yaml)
	CredentialsDir    string `yaml:"credentials_dir"`  // Path to ZenLock mounted dir (default: /zen-lock/secrets)
	CredentialsSource string `yaml:"-"`                // Populated after resolution
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
	Required bool   `yaml:"required"`
}

// ZenContextConfig holds three-tier memory configuration.
type ZenContextConfig struct {
	Tier1Redis RedisTierConfig `yaml:"tier1_redis"`
	Tier2QMD   QMDTierConfig   `yaml:"tier2_qmd"`
	Tier3S3    S3TierConfig    `yaml:"tier3_s3"`
	Journal    JournalConfig   `yaml:"journal"`
	ClusterID  string          `yaml:"cluster_id"`
	Verbose    bool            `yaml:"verbose"`
	Required   bool            `yaml:"required"`
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

	// Load Jira credentials from canonical sources
	config.loadJiraCredentials()

	// Set defaults
	config.setDefaults()

	return &config, nil
}

// findConfigPath returns the canonical runtime config path when path is not explicitly set.
// Runtime config lives under ZEN_BRAIN_HOME only; repo configs/ is for templates/examples.
// Use --config <path> to override. If the file does not exist, LoadConfig will fail and callers may use DefaultConfig().
func findConfigPath() string {
	return filepath.Join(HomeDir(), "config.yaml")
}

// loadFromEnv loads non-Jira environment variables (Confluence, AWS, Redis, etc.).
// Jira credentials are loaded via loadJiraCredentials() using the canonical resolver.
func (c *Config) loadFromEnv() {
	// Confluence: TODO - implement secrets.ResolveConfluence() when needed
	c.Confluence.Username = os.Getenv("CONFLUENCE_USERNAME")
	c.Confluence.APIToken = os.Getenv("CONFLUENCE_API_TOKEN")

	// AWS: TODO - implement secrets.ResolveAWS() when needed
	if c.ZenContext.Tier3S3.AccessKeyID == "" {
		c.ZenContext.Tier3S3.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if c.ZenContext.Tier3S3.SecretAccessKey == "" {
		c.ZenContext.Tier3S3.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if c.ZenContext.Tier3S3.SessionToken == "" {
		c.ZenContext.Tier3S3.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}
	// ZenContext Tier1 Redis (env override for sandbox deployments)
	if c.ZenContext.Tier1Redis.Addr == "" {
		c.ZenContext.Tier1Redis.Addr = os.Getenv("TIER1_REDIS_ADDR")
		if c.ZenContext.Tier1Redis.Addr != "" {
			fmt.Fprintf(os.Stderr, "[Config] TIER1_REDIS_ADDR loaded from env: %s\n", c.ZenContext.Tier1Redis.Addr)
		}
	}
	// Message bus
	if c.MessageBus.RedisURL == "" {
		c.MessageBus.RedisURL = os.Getenv("REDIS_URL")
	}
	if c.MessageBus.Kind == "" && os.Getenv("ZEN_BRAIN_MESSAGE_BUS") == "redis" {
		c.MessageBus.Enabled = true
		c.MessageBus.Kind = "redis"
	}
	// Cluster ID for session/context/office lookups (config file overrides this when set)
	if c.ZenContext.ClusterID == "" {
		c.ZenContext.ClusterID = os.Getenv("CLUSTER_ID")
	}
}

// loadJiraCredentials resolves Jira credentials from canonical sources.
// In cluster mode: ONLY accepts credentials from /zen-lock/secrets.
// In local dev mode: MAY use credentials_file for debugging only.
// Resolution order: credentials_dir → credentials_file.
// Populates BaseURL, Email, APIToken, ProjectKey, and CredentialsSource.
func (c *Config) loadJiraCredentials() {
	ctx := context.Background()

	// Detect cluster mode: if running in Kubernetes, use ONLY ZenLock secrets
	inClusterMode := os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("CONTAINER_NAME") != "" ||
		os.Getenv("CLUSTER_ID") != ""

	// Set defaults for credential paths
	if c.Jira.CredentialsDir == "" {
		c.Jira.CredentialsDir = "/zen-lock/secrets"
	}

	// ZB-025A: In cluster mode, credentials_file is FORBIDDEN
	// Only /zen-lock/secrets is allowed as source of truth
	if inClusterMode && c.Jira.CredentialsFile != "" {
		// Check if operator explicitly set credentials_file (not default)
		defaultFilePath := filepath.Join(HomeDir(), "secrets", "jira.yaml")
		if c.Jira.CredentialsFile != defaultFilePath {
			// Non-default credentials_file in cluster mode is forbidden
			fmt.Fprintf(os.Stderr, "ERROR: credentials_file is forbidden in cluster mode\n")
			fmt.Fprintf(os.Stderr, "Cluster mode MUST use zenlock-dir:/zen-lock/secrets only\n")
			fmt.Fprintf(os.Stderr, "Run: deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh\n")
			c.Jira.CredentialsSource = "error: forbidden in cluster mode"
			return
		}
		// Fall through - will resolve from credentials_dir
	}

	// Only set credentials_file for local dev mode (not cluster mode)
	if !inClusterMode && c.Jira.CredentialsFile == "" {
		c.Jira.CredentialsFile = filepath.Join(HomeDir(), "secrets", "jira.yaml")
	} else if !inClusterMode && c.Jira.CredentialsFile != "" {
		// Expand tilde in credentials_file path
		if strings.HasPrefix(c.Jira.CredentialsFile, "~/") {
			// Get user home directory and expand tilde
			homeDir, err := os.UserHomeDir()
			if err == nil {
				c.Jira.CredentialsFile = filepath.Join(homeDir, strings.TrimPrefix(c.Jira.CredentialsFile, "~/"))
			}
		}
	}

	// Debug output
	if os.Getenv("DEBUG_JIRA_CREDS") == "1" {
		fmt.Fprintf(os.Stderr, "DEBUG Jira config:\n")
		fmt.Fprintf(os.Stderr, "  InClusterMode: %v\n", inClusterMode)
		fmt.Fprintf(os.Stderr, "  CredentialsFile: %s\n", c.Jira.CredentialsFile)
		fmt.Fprintf(os.Stderr, "  CredentialsDir: %s\n", c.Jira.CredentialsDir)
	}

	// ZB-025A: In cluster mode, ONLY use credentials_dir, never credentials_file
	opts := secrets.JiraResolveOptions{
		DirPath:     c.Jira.CredentialsDir,
		FilePath:    "", // Empty in cluster mode
		ClusterMode: inClusterMode,
	}

	// In local dev mode, allow credentials_file
	if !inClusterMode {
		opts.FilePath = c.Jira.CredentialsFile
	}

	// Resolve credentials from canonical sources
	material, err := secrets.ResolveJira(ctx, opts)

	// ZB-025A: Hard-fail in cluster mode if Jira enabled but credentials not found
	if inClusterMode && c.Jira.Enabled && (err != nil || material == nil || material.Source == "none") {
		fmt.Fprintf(os.Stderr, "\nERROR: Jira credentials not loaded from ZenLock\n")
		fmt.Fprintf(os.Stderr, "Source: %s\n", c.Jira.CredentialsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "\nResolution: Run bootstrap script to set up Jira integration\n")
		fmt.Fprintf(os.Stderr, "  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh\n")
		fmt.Fprintf(os.Stderr, "\nRequired files:\n")
		fmt.Fprintf(os.Stderr, "  - ~/zen/DONOTASKMOREFORTHISSHIT.txt\n")
		fmt.Fprintf(os.Stderr, "  - ~/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age\n")
		fmt.Fprintf(os.Stderr, "  - ~/zen/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age\n")
		c.Jira.CredentialsSource = "error: missing zenlock-dir credentials"
		return
	}

	if err != nil {
		// Log warning but don't fail config load
		// Credentials may not be configured yet
		c.Jira.CredentialsSource = "error: " + err.Error()
		if os.Getenv("DEBUG_JIRA_CREDS") == "1" {
			fmt.Fprintf(os.Stderr, "DEBUG ResolveJira error: %v\n", err)
		}
		return
	}

	if os.Getenv("DEBUG_JIRA_CREDS") == "1" {
		fmt.Fprintf(os.Stderr, "DEBUG Material:\n")
		fmt.Fprintf(os.Stderr, "  Source: %s\n", material.Source)
		fmt.Fprintf(os.Stderr, "  BaseURL: %s\n", material.BaseURL)
		fmt.Fprintf(os.Stderr, "  Email: %s\n", material.Email)
		fmt.Fprintf(os.Stderr, "  ProjectKey: %s\n", material.ProjectKey)
	}

	if material != nil && material.Source != "none" {
		// Only populate fields from material if they're empty
		// This preserves values from config compatibility (Project -> ProjectKey)
		if c.Jira.BaseURL == "" {
			c.Jira.BaseURL = material.BaseURL
		}
		if c.Jira.Email == "" {
			c.Jira.Email = material.Email
		}
		if c.Jira.APIToken == "" {
			c.Jira.APIToken = material.APIToken
		}
		// Only set ProjectKey if it's empty AND material has a value
		// This preserves config compatibility for Project -> ProjectKey
		if c.Jira.ProjectKey == "" && material.ProjectKey != "" {
			c.Jira.ProjectKey = material.ProjectKey
		}
		c.Jira.CredentialsSource = material.Source

		if os.Getenv("DEBUG_JIRA_CREDS") == "1" {
			fmt.Fprintf(os.Stderr, "DEBUG After population:\n")
			fmt.Fprintf(os.Stderr, "  BaseURL: %s\n", c.Jira.BaseURL)
			fmt.Fprintf(os.Stderr, "  Email: %s\n", c.Jira.Email)
			fmt.Fprintf(os.Stderr, "  APIToken (present): %v\n", c.Jira.APIToken != "")
			fmt.Fprintf(os.Stderr, "  ProjectKey: %s\n", c.Jira.ProjectKey)
		}
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
	if c.MessageBus.Stream == "" {
		c.MessageBus.Stream = "zen-brain.events"
	}

	// ZB-025A: Set ONLY canonical runtime path for Jira
	// credentials_file is FORBIDDEN as default - use ZenLock only
	if c.Jira.CredentialsDir == "" {
		c.Jira.CredentialsDir = "/zen-lock/secrets"
	}
	// ZB-025A: DO NOT set default credentials_file path
	// This eliminates ambiguity and prevents silent fallback
	// If credentials_file is needed for local debug, operator must explicitly set it
}

// ApplyEnvOverrides applies environment variable overrides and loads Jira credentials.
// This MUST be called after DefaultConfig() to ensure env vars are absorbed.
// LoadConfig() calls this automatically, but DefaultConfig() does not.
func (c *Config) ApplyEnvOverrides() {
	c.loadFromEnv()
	c.loadJiraCredentials()
	c.setDefaults()
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
		MessageBus: MessageBusConfig{
			Enabled: false,
			Kind:    "redis",
			Stream:  "zen-brain.events",
		},
		Planner: PlannerConfig{
			DefaultModel:    "glm-4.7",
			MaxCostPerTask:  10.0,
			RequireApproval: false,
		},
	}
}
