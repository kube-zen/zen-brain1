package mlq

import (
	"fmt"
	"log"
	"os"

	"sigs.k8s.io/yaml"
)

// Level represents an MLQ level with backend and concurrency.
type Level struct {
	Level       int    `json:"level"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Enabled     bool   `json:"enabled"`

	Backend       BackendConfig `json:"backend"`
	Concurrency   ConcurrencyConfig `json:"concurrency"`
	TaskClass     []string `json:"task_class"`
	Capabilities  LevelCapabilities `json:"capabilities"`
	Performance   PerformanceConfig `json:"performance"`
}

// BackendConfig defines a backend connection.
type BackendConfig struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	ModelFile   string `json:"model_file"`
	APIEndpoint string `json:"api_endpoint"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

// ConcurrencyConfig defines worker pool sizing.
type ConcurrencyConfig struct {
	MaxWorkers int `json:"max_workers"`
	MinWorkers int `json:"min_workers"`
}

// LevelCapabilities defines what the level can do.
type LevelCapabilities struct {
	SupportsStreaming   bool `json:"supports_streaming"`
	SupportsFunctions  bool `json:"supports_functions"`
	MaxContextTokens   int `json:"max_context_tokens"`
	MaxOutputTokens    int `json:"max_output_tokens"`
}

// PerformanceConfig defines performance targets.
type PerformanceConfig struct {
	TargetRPS          float64 `json:"target_rps"`
	MaxLatencySeconds   int `json:"max_latency_seconds"`
}

// EscalationRule defines when to move between levels.
type EscalationRule struct {
	Trigger              string   `json:"trigger"`
	FromLevel            int       `json:"from_level"`
	ToLevel              int       `json:"to_level"`
	MaxRetries           int       `json:"max_retries"`
	TimeoutThresholdSec  int       `json:"timeout_threshold_seconds"`
	RequireManualApproval bool      `json:"require_manual_approval"`
	AllowedForTaskClass []string `json:"allowed_for_task_class,omitempty"`
}

// SelectionPolicy defines default level mapping and overrides.
type SelectionPolicy struct {
	DefaultLevelMapping map[string]int  `json:"default_level_mapping"`
	JiraKeyOverrides    map[string]int  `json:"jira_key_overrides"`
	FallbackBehavior    FallbackConfig  `json:"fallback_behavior"`
}

// FallbackConfig defines fallback behavior.
type FallbackConfig struct {
	Strategy              string `json:"strategy"`
	MaxFallbackAttempts   int    `json:"max_fallback_attempts"`
	FallbackDelaySeconds  int    `json:"fallback_delay_seconds"`
}

// MLQConfig is the full MLQ configuration.
type MLQConfig struct {
	MLQLevels        []Level           `json:"mlq_levels"`
	EscalationRules  []EscalationRule  `json:"escalation_rules"`
	SelectionPolicy  SelectionPolicy    `json:"selection_policy"`
	HealthChecks     HealthCheckConfig  `json:"health_checks"`
	Logging         LoggingConfig      `json:"logging"`
}

// HealthCheckConfig defines backend health monitoring.
type HealthCheckConfig struct {
	Enabled            bool               `json:"enabled"`
	IntervalSeconds    int                `json:"interval_seconds"`
	Backends          []BackendHealthCheck `json:"backends"`
}

// BackendHealthCheck defines health check for a backend.
type BackendHealthCheck struct {
	Level               int    `json:"level"`
	CheckEndpoint       string `json:"check_endpoint"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	HealthyThreshold    int    `json:"healthy_threshold"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
}

// LoggingConfig defines MLQ logging.
type LoggingConfig struct {
	Level              string `json:"level"`
	LogSelection       bool   `json:"log_selection"`
	SelectionFormat    string `json:"selection_format"`
	LogEscalation     bool   `json:"log_escalation"`
	EscalationFormat  string `json:"escalation_format"`
	LogFallback       bool   `json:"log_fallback"`
	FallbackFormat    string `json:"fallback_format"`
}

// MLQ manages multi-level backend selection.
type MLQ struct {
	config     *MLQConfig
	levels     map[int]*Level
	healthy     map[int]bool
}

// NewMLQFromConfig creates MLQ from config file.
func NewMLQFromConfig(configPath string) (*MLQ, error) {
	config, err := LoadMLQConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	return NewMLQ(config), nil
}

// LoadMLQConfig loads MLQ config from YAML file.
func LoadMLQConfig(configPath string) (*MLQConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var config MLQConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &config, nil
}

// NewMLQ creates MLQ from config struct.
func NewMLQ(config *MLQConfig) *MLQ {
	m := &MLQ{
		config:  config,
		levels:  make(map[int]*Level),
		healthy: make(map[int]bool),
	}

	// Build level index
	for i := range config.MLQLevels {
		level := &config.MLQLevels[i]
		m.levels[level.Level] = level
		m.healthy[level.Level] = level.Enabled // Assume enabled = healthy initially
		log.Printf("[MLQ] Loaded level %d: name=%s enabled=%v backend=%s/%s",
			level.Level, level.DisplayName, level.Enabled, level.Backend.Provider, level.Backend.Name)
	}

	return m
}

// SelectLevel chooses MLQ level for a task.
func (m *MLQ) SelectLevel(taskID, jiraKey, taskClass string) (*Level, error) {
	config := m.config

	// 1. Check Jira key overrides first
	if jiraKey != "" {
		if levelNum, ok := config.SelectionPolicy.JiraKeyOverrides[jiraKey]; ok {
			if level, ok := m.levels[levelNum]; ok && level.Enabled {
				m.logSelection(taskID, jiraKey, level, "jira_override")
				return level, nil
			}
			log.Printf("[MLQ] Jira override for %s points to disabled level %d", jiraKey, levelNum)
		}
	}

	// 2. Check task class mapping
	if taskClass != "" {
		if levelNum, ok := config.SelectionPolicy.DefaultLevelMapping[taskClass]; ok {
			if level, ok := m.levels[levelNum]; ok && level.Enabled {
				m.logSelection(taskID, jiraKey, level, "task_class")
				return level, nil
			}
			log.Printf("[MLQ] Task class %s maps to disabled level %d", taskClass, levelNum)
		}
	}

	// 3. Use fallback (Level 0) if nothing matched
	if level, ok := m.levels[0]; ok && level.Enabled {
		m.logSelection(taskID, jiraKey, level, "fallback")
		return level, nil
	}

	return nil, fmt.Errorf("no enabled level available for task=%s jira=%s", taskID, jiraKey)
}

// GetBackend returns backend configuration for a level.
func (l *Level) GetBackend() (provider, model, baseURL string, timeoutSeconds int) {
	return l.Backend.Provider,
		l.Backend.Name,
		l.Backend.APIEndpoint,
		l.Backend.TimeoutSeconds
}

// logSelection logs level selection according to config.
func (m *MLQ) logSelection(taskID, jiraKey string, level *Level, reason string) {
	if !m.config.Logging.LogSelection {
		return
	}

	format := m.config.Logging.SelectionFormat
	logStr := fmt.Sprintf(format,
		level.Level, taskID, jiraKey,
		level.Backend.Provider, level.Backend.Name, level.Backend.TimeoutSeconds)

	log.Printf("[MLQ] Selection: %s (reason=%s)", logStr, reason)
}

// GetLevel returns level by number.
func (m *MLQ) GetLevel(levelNum int) (*Level, bool) {
	level, ok := m.levels[levelNum]
	return level, ok
}

// ListLevels returns all enabled levels.
func (m *MLQ) ListLevels() []int {
	var levels []int
	for levelNum, level := range m.levels {
		if level.Enabled {
			levels = append(levels, levelNum)
		}
	}
	return levels
}
