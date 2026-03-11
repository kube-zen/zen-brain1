package config

import (
	"fmt"
	"os"
	"strings"
)

// ValidationLevel represents how strict validation should be.
type ValidationLevel string

const (
	ValidationLevelStrict    ValidationLevel = "strict"    // Prod: no hardcoded defaults
	ValidationLevelStandard  ValidationLevel = "standard"  // Staging: minimal defaults
	ValidationLevelRelaxed   ValidationLevel = "relaxed"   // Dev: allow defaults
)

// ValidationMode represents what to validate.
type ValidationMode string

const (
	ValidateModeNone      ValidationMode = "none"       // No validation
	ValidateModeCritical  ValidationMode = "critical"   // Only critical services
	ValidateModeAll       ValidationMode = "all"        // All services
)

// ConfigValidator validates configuration with different strictness levels.
type ConfigValidator struct {
	level ValidationLevel
	mode  ValidationMode
}

// ValidationResult contains validation results.
type ValidationResult struct {
	Valid       bool                  `json:"valid"`
	Level       ValidationLevel       `json:"level"`
	Errors      []ValidationError     `json:"errors"`
	Warnings    []ValidationWarning   `json:"warnings"`
	Suggestions []string              `json:"suggestions"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // critical, high, medium, low
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewConfigValidator creates a validator based on runtime profile.
func NewConfigValidator() *ConfigValidator {
	level := detectValidationLevel()
	mode := detectValidationMode()

	return &ConfigValidator{
		level: level,
		mode:  mode,
	}
}

// detectValidationLevel determines validation strictness from environment.
func detectValidationLevel() ValidationLevel {
	profile := strings.ToLower(os.Getenv("ZEN_RUNTIME_PROFILE"))
	strict := os.Getenv("ZEN_BRAIN_STRICT_RUNTIME") != ""
	strictConfig := os.Getenv("ZEN_CONFIG_STRICT") != ""

	// Strict mode enabled
	if strict || strictConfig || profile == "prod" {
		return ValidationLevelStrict
	}

	// Staging
	if profile == "staging" {
		return ValidationLevelStandard
	}

	// Dev/test/ci
	return ValidationLevelRelaxed
}

// detectValidationMode determines what to validate.
func detectValidationMode() ValidationMode {
	mode := strings.ToLower(os.Getenv("ZEN_CONFIG_VALIDATE"))

	switch mode {
	case "none":
		return ValidateModeNone
	case "critical":
		return ValidateModeCritical
	case "all":
		return ValidateModeAll
	default:
		// Default: validate critical in prod, all otherwise
		if os.Getenv("ZEN_RUNTIME_PROFILE") == "prod" {
			return ValidateModeCritical
		}
		return ValidateModeAll
	}
}

// Validate validates the configuration based on level and mode.
func (v *ConfigValidator) Validate(cfg *Config) *ValidationResult {
	result := &ValidationResult{
		Valid:       true,
		Level:       v.level,
		Errors:      []ValidationError{},
		Warnings:    []ValidationWarning{},
		Suggestions: []string{},
	}

	if cfg == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:    "config",
			Message:  "configuration is nil",
			Severity: "critical",
		})
		return result
	}

	// Validate based on mode
	if v.mode != ValidateModeNone {
		v.validateCore(cfg, result)
		v.validateServices(cfg, result)
		v.validateIntegration(cfg, result)
	}

	// Determine overall validity
	for _, err := range result.Errors {
		if err.Severity == "critical" {
			result.Valid = false
			break
		}
	}

	return result
}

// validateCore validates core configuration.
func (v *ConfigValidator) validateCore(cfg *Config, result *ValidationResult) {
	// Home directory
	if cfg.HomeDir == "" {
		if v.level == ValidationLevelStrict {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "home_dir",
				Message:  "home_dir must be set in strict mode (ZEN_BRAIN_HOME or config)",
				Severity: "high",
			})
		} else {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "home_dir",
				Message: "using default home directory",
			})
		}
	}

	// Logging configuration
	if cfg.Logging.Level == "" {
		if v.level == ValidationLevelStrict {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "logging.level",
				Message:  "logging level must be set in strict mode",
				Severity: "medium",
			})
		}
	}

	// ZenContext configuration
	v.validateZenContext(cfg, result)
}

// validateZenContext validates ZenContext configuration.
func (v *ConfigValidator) validateZenContext(cfg *Config, result *ValidationResult) {
	// Cluster ID should not be hardcoded "default" in prod
	if cfg.ZenContext.ClusterID == "default" && v.level == ValidationLevelStrict {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "zen_context.cluster_id",
			Message: "using default cluster ID in production - set CLUSTER_ID env var",
		})
		result.Suggestions = append(result.Suggestions,
			"Set CLUSTER_ID environment variable for production deployments")
	}

	// Redis address should not be localhost in prod
	if v.containsHardcodedLocalhost(cfg.ZenContext.Tier1Redis.Addr) && v.level == ValidationLevelStrict {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "zen_context.tier1_redis.addr",
			Message:  "localhost addresses not allowed in strict mode - set TIER1_REDIS_ADDR",
			Severity: "high",
		})
	}
}

// validateServices validates service configurations.
func (v *ConfigValidator) validateServices(cfg *Config, result *ValidationResult) {
	// Jira configuration
	if cfg.Jira.Enabled {
		v.validateJira(cfg, result)
	}

	// QMD configuration
	v.validateQMD(cfg, result)

	// Ledger configuration
	v.validateLedger(cfg, result)

	// Message bus configuration
	if cfg.MessageBus.Enabled {
		v.validateMessageBus(cfg, result)
	}
}

// validateJira validates Jira configuration.
func (v *ConfigValidator) validateJira(cfg *Config, result *ValidationResult) {
	// Base URL required if enabled
	if cfg.Jira.BaseURL == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "jira.base_url",
			Message:  "jira.base_url required when jira.enabled=true",
			Severity: "high",
		})
	} else if v.containsHardcodedLocalhost(cfg.Jira.BaseURL) && v.level == ValidationLevelStrict {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "jira.base_url",
			Message: "localhost URL in production config",
		})
	}

	// Project key
	if cfg.Jira.ProjectKey == "" && cfg.Jira.Project == "" {
		if v.level != ValidationLevelRelaxed {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "jira.project_key",
				Message:  "jira.project_key or jira.project required",
				Severity: "medium",
			})
		}
	}

	// Credentials
	if cfg.Jira.Email == "" {
		// Check env vars
		if os.Getenv("JIRA_EMAIL") == "" && os.Getenv("JIRA_USERNAME") == "" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "jira.email",
				Message: "JIRA_EMAIL or JIRA_USERNAME env var not set",
			})
		}
	}

	if cfg.Jira.APIToken == "" {
		if os.Getenv("JIRA_API_TOKEN") == "" && os.Getenv("JIRA_TOKEN") == "" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "jira.api_token",
				Message: "JIRA_API_TOKEN or JIRA_TOKEN env var not set",
			})
		}
	}
}

// validateQMD validates QMD configuration.
func (v *ConfigValidator) validateQMD(cfg *Config, result *ValidationResult) {
	// QMD binary path
	if cfg.QMD.BinaryPath == "" {
		if v.level == ValidationLevelStrict {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "qmd.binary_path",
				Message:  "qmd.binary_path must be set in strict mode",
				Severity: "medium",
			})
		}
	} else if cfg.QMD.BinaryPath == "qmd" && v.level == ValidationLevelStrict {
		// Using bare "qmd" command in prod
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "qmd.binary_path",
			Message: "using bare 'qmd' command - consider full path in production",
		})
		result.Suggestions = append(result.Suggestions,
			"Set QMD_BINARY_PATH to full path (e.g., /usr/local/bin/qmd)")
	}

	// Refresh interval
	if cfg.QMD.RefreshInterval == 0 {
		if v.level == ValidationLevelStrict {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "qmd.refresh_interval",
				Message:  "qmd.refresh_interval must be set in strict mode",
				Severity: "low",
			})
		}
	}
}

// validateLedger validates Ledger configuration.
func (v *ConfigValidator) validateLedger(cfg *Config, result *ValidationResult) {
	// Ledger DSN (constructed from Host/Port/Database)
	if cfg.Ledger.Host == "" {
		// Check env var
		if os.Getenv("ZEN_LEDGER_HOST") == "" {
			if v.level == ValidationLevelStrict {
				result.Errors = append(result.Errors, ValidationError{
					Field:    "ledger.host",
					Message:  "ledger.host must be set in strict mode (or ZEN_LEDGER_HOST env var)",
					Severity: "high",
				})
			} else {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "ledger.host",
					Message: "ledger host not configured - using stub implementation",
				})
			}
		}
	} else if v.containsHardcodedLocalhost(cfg.Ledger.Host) && v.level == ValidationLevelStrict {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "ledger.host",
			Message: "localhost in ledger host for production",
		})
	}
}

// validateMessageBus validates message bus configuration.
func (v *ConfigValidator) validateMessageBus(cfg *Config, result *ValidationResult) {
	// Redis URL
	if cfg.MessageBus.RedisURL == "" {
		if os.Getenv("ZEN_BRAIN_MESSAGE_BUS_URL") == "" {
			if cfg.MessageBus.Required {
				result.Errors = append(result.Errors, ValidationError{
					Field:    "message_bus.redis_url",
					Message:  "message_bus.redis_url required when message_bus.required=true",
					Severity: "high",
				})
			} else if v.level != ValidationLevelRelaxed {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Field:   "message_bus.redis_url",
					Message: "message bus URL not configured",
				})
			}
		}
	} else if v.containsHardcodedLocalhost(cfg.MessageBus.RedisURL) && v.level == ValidationLevelStrict {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "message_bus.redis_url",
			Message:  "localhost not allowed in message_bus.redis_url for strict mode",
			Severity: "high",
		})
	}

	// Stream name
	if cfg.MessageBus.Stream == "" {
		if v.level == ValidationLevelStrict {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "message_bus.stream",
				Message:  "message_bus.stream must be set in strict mode",
				Severity: "medium",
			})
		}
	} else if cfg.MessageBus.Stream == "zen-brain.events" && v.level == ValidationLevelStrict {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "message_bus.stream",
			Message: "using default stream name in production",
		})
	}
}

// validateIntegration validates integration configurations.
func (v *ConfigValidator) validateIntegration(cfg *Config, result *ValidationResult) {
	// Confluence
	if cfg.Confluence.Enabled {
		if cfg.Confluence.BaseURL == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "confluence.base_url",
				Message:  "confluence.base_url required when confluence.enabled=true",
				Severity: "high",
			})
		}
	}

	// SRED
	if cfg.SRED.Enabled && len(cfg.SRED.DefaultTags) == 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Field:   "sred.default_tags",
			Message: "no default tags for SR&ED evidence collection",
		})
	}
}

// containsHardcodedLocalhost checks if a string contains localhost references.
func (v *ConfigValidator) containsHardcodedLocalhost(s string) bool {
	return strings.Contains(s, "localhost") ||
		strings.Contains(s, "127.0.0.1") ||
		strings.Contains(s, "::1")
}

// ValidateConfig is a convenience function for quick validation.
func ValidateConfig(cfg *Config) *ValidationResult {
	validator := NewConfigValidator()
	return validator.Validate(cfg)
}

// MustValidate validates config and panics on critical errors in strict mode.
func MustValidate(cfg *Config) *ValidationResult {
	validator := NewConfigValidator()
	result := validator.Validate(cfg)

	// In strict mode, panic on critical errors
	if validator.level == ValidationLevelStrict && !result.Valid {
		panic(fmt.Sprintf("configuration validation failed in strict mode: %v", result.Errors))
	}

	return result
}

// StrictConfigLoader wraps config loading with validation.
type StrictConfigLoader struct {
	validator *ConfigValidator
}

// NewStrictConfigLoader creates a strict config loader.
func NewStrictConfigLoader() *StrictConfigLoader {
	return &StrictConfigLoader{
		validator: NewConfigValidator(),
	}
}

// LoadAndValidate loads config and validates it.
func (l *StrictConfigLoader) LoadAndValidate(path string) (*Config, *ValidationResult, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	result := l.validator.Validate(cfg)

	// Return error if validation failed in strict mode
	if !result.Valid && l.validator.level == ValidationLevelStrict {
		return cfg, result, fmt.Errorf("configuration validation failed: %v", result.Errors)
	}

	return cfg, result, nil
}
