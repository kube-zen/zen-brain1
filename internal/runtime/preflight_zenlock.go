package runtime

import (
	"context"
	"fmt"
	"os"
	"time"
)

// PreflightZenLock checks ZenLock health and Jira credential availability.
func PreflightZenLock(ctx context.Context) *EnhancedPreflightCheck {
	start := time.Now()

	// Check if running in cluster mode
	inCluster := os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("CONTAINER_NAME") != "" ||
		os.Getenv("CLUSTER_ID") != ""

	// In cluster mode, ZenLock must be healthy
	if inCluster {
		// Check ZenLock mount path
		zenlockPath := "/zen-lock/secrets"
		if _, err := os.Stat(zenlockPath); os.IsNotExist(err) {
			return &EnhancedPreflightCheck{
				Name:       "zenlock_jira_credentials",
				Category:   "core",
				Healthy:    false,
				Required:   true,
				Mode:       ModeReal,
				StrictMode: ModeReal,
				Message:    "ZenLock mount missing: /zen-lock/secrets not found",
				Duration:   time.Since(start),
				Error:      err.Error(),
			}
		}

		// Check Jira credentials in ZenLock
		jiraTokenPath := zenlockPath + "/JIRA_API_TOKEN"
		if _, err := os.Stat(jiraTokenPath); os.IsNotExist(err) {
			return &EnhancedPreflightCheck{
				Name:       "zenlock_jira_credentials",
				Category:   "core",
				Healthy:    false,
				Required:   true,
				Mode:       ModeReal,
				StrictMode: ModeReal,
				Message:    "Jira credentials not found in ZenLock: JIRA_API_TOKEN missing",
				Duration:   time.Since(start),
				Error:      "ZenLock credentials not available",
			}
		}

		// Check if plaintext bootstrap file still exists (SECURITY VIOLATION)
		plaintextPath := os.ExpandEnv("$HOME/zen/DONOTASKMOREFORTHISSHIT.txt")
		if _, err := os.Stat(plaintextPath); err == nil {
			return &EnhancedPreflightCheck{
				Name:       "zenlock_jira_credentials",
				Category:   "core",
				Healthy:    false,
				Required:   true,
				Mode:       ModeReal,
				StrictMode: ModeReal,
				Message:    "SECURITY VIOLATION: Plaintext bootstrap token file still exists after successful bootstrap",
				Duration:   time.Since(start),
				Error:      "Plaintext token not deleted: " + plaintextPath,
			}
		}

		// Success
		return &EnhancedPreflightCheck{
			Name:       "zenlock_jira_credentials",
			Category:   "core",
			Healthy:    true,
			Required:   true,
			Mode:       ModeReal,
			StrictMode: ModeReal,
			Message:    "ZenLock healthy, Jira credentials available",
			Duration:   time.Since(start),
		}
	}

	// Local dev mode - skip ZenLock check
	return &EnhancedPreflightCheck{
		Name:       "zenlock_jira_credentials",
		Category:   "core",
		Healthy:    true,
		Required:   false,
		Mode:       ModeStub,
		StrictMode: ModeReal,
		Message:    "Local dev mode - ZenLock not required",
		Duration:   time.Since(start),
		Skipped:    true,
	}
}

// PreflightLocalModel checks local LLM configuration.
func PreflightLocalModel(ctx context.Context, expectedModel string, expectedTimeout int, expectedKeepAlive string) *EnhancedPreflightCheck {
	start := time.Now()

	// Check local model configuration
	actualModel := os.Getenv("ZEN_FOREMAN_LLM_MODEL")
	if actualModel == "" {
		actualModel = "qwen3.5:0.8b" // Default
	}

	actualTimeout := os.Getenv("ZEN_FOREMAN_LLM_TIMEOUT_SECONDS")
	if actualTimeout == "" {
		actualTimeout = "2700" // Default 45m
	}

	actualKeepAlive := os.Getenv("ZEN_FOREMAN_LLM_KEEP_ALIVE")
	if actualKeepAlive == "" {
		actualKeepAlive = "45m" // Default
	}

	// Check if configuration matches expected values
	issues := []string{}

	if actualModel != expectedModel {
		issues = append(issues, fmt.Sprintf("model mismatch: expected=%s, actual=%s", expectedModel, actualModel))
	}

	if actualTimeout != fmt.Sprintf("%d", expectedTimeout) {
		issues = append(issues, fmt.Sprintf("timeout mismatch: expected=%ds, actual=%ss", expectedTimeout, actualTimeout))
	}

	if actualKeepAlive != expectedKeepAlive {
		issues = append(issues, fmt.Sprintf("keep_alive mismatch: expected=%s, actual=%s", expectedKeepAlive, actualKeepAlive))
	}

	if len(issues) > 0 {
		message := fmt.Sprintf("Local model configuration mismatch: %v", issues)
		return &EnhancedPreflightCheck{
			Name:       "local_model_config",
			Category:   "core",
			Healthy:    false,
			Required:   true,
			Mode:       ModeReal,
			StrictMode: ModeReal,
			Message:    message,
			Duration:   time.Since(start),
			Error:      "Configuration drift detected",
		}
	}

	return &EnhancedPreflightCheck{
		Name:       "local_model_config",
		Category:   "core",
		Healthy:    true,
		Required:   true,
		Mode:       ModeReal,
		StrictMode: ModeReal,
		Message:    fmt.Sprintf("Local model config OK: model=%s, timeout=%ss, keep_alive=%s", actualModel, actualTimeout, actualKeepAlive),
		Duration:   time.Since(start),
	}
}
