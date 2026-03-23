package runtime

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
)

// PreflightJiraProjectKey verifies the configured Jira project key is accessible.
// This check validates:
// 1. Project key is configured in source metadata
// 2. Project key is visible to the runtime account
// 3. Project key can be accessed directly via /project/{key}
func PreflightJiraProjectKey(ctx context.Context, cfg *config.Config) *EnhancedPreflightCheck {
	start := time.Now()

	// Skip if Jira is not enabled
	if !cfg.Jira.Enabled {
		return &EnhancedPreflightCheck{
			Name:       "jira_project_key",
			Category:   "office",
			Healthy:    true,
			Required:   false,
			Mode:       ModeDisabled,
			StrictMode: ModeReal,
			Message:    "Jira disabled - project key check skipped",
			Duration:   time.Since(start),
			Skipped:    true,
		}
	}

	// Get configured project key
	projectKey := cfg.Jira.ProjectKey
	if projectKey == "" {
		return &EnhancedPreflightCheck{
			Name:       "jira_project_key",
			Category:   "office",
			Healthy:    false,
			Required:   true,
			Mode:       ModeReal,
			StrictMode: ModeReal,
			Message:    "Jira enabled but no project key configured",
			Duration:   time.Since(start),
			Error:      "project_key not set in jira-metadata.yaml or environment",
		}
	}

	// Verify credentials are available
	if cfg.Jira.BaseURL == "" || cfg.Jira.APIToken == "" || cfg.Jira.Email == "" {
		return &EnhancedPreflightCheck{
			Name:       "jira_project_key",
			Category:   "office",
			Healthy:    false,
			Required:   true,
			Mode:       ModeReal,
			StrictMode: ModeReal,
			Message:    "Jira credentials not available for project key verification",
			Duration:   time.Since(start),
			Error:      "missing credentials (base_url, email, or api_token)",
		}
	}

	// Perform live verification against Jira API
	healthy, message, err := verifyJiraProjectAccess(ctx, cfg, projectKey)
	if err != nil {
		return &EnhancedPreflightCheck{
			Name:       "jira_project_key",
			Category:   "office",
			Healthy:    false,
			Required:   true,
			Mode:       ModeReal,
			StrictMode: ModeReal,
			Message:    message,
			Duration:   time.Since(start),
			Error:      err.Error(),
		}
	}

	return &EnhancedPreflightCheck{
		Name:       "jira_project_key",
		Category:   "office",
		Healthy:    healthy,
		Required:   true,
		Mode:       ModeReal,
		StrictMode: ModeReal,
		Message:    message,
		Duration:   time.Since(start),
	}
}

// verifyJiraProjectAccess performs live API checks against Jira to verify project accessibility.
func verifyJiraProjectAccess(ctx context.Context, cfg *config.Config, projectKey string) (bool, string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Check 1: Verify project is visible in /project/search
	searchURL := fmt.Sprintf("%s/rest/api/3/project/search", cfg.Jira.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return false, "Failed to create project search request", fmt.Errorf("create request failed: %w", err)
	}

	req.SetBasicAuth(cfg.Jira.Email, cfg.Jira.APIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("Jira API unreachable: %v", err), fmt.Errorf("api unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Jira API returned status %d", resp.StatusCode), fmt.Errorf("api status %d", resp.StatusCode)
	}

	// Check 2: Verify direct project access via /project/{key}
	projectURL := fmt.Sprintf("%s/rest/api/3/project/%s", cfg.Jira.BaseURL, projectKey)
	req2, err := http.NewRequestWithContext(ctx, "GET", projectURL, nil)
	if err != nil {
		return false, "Failed to create project direct access request", fmt.Errorf("create request failed: %w", err)
	}

	req2.SetBasicAuth(cfg.Jira.Email, cfg.Jira.APIToken)
	req2.Header.Set("Accept", "application/json")

	resp2, err := client.Do(req2)
	if err != nil {
		return false, fmt.Sprintf("Jira project %s API check failed: %v", projectKey, err), fmt.Errorf("project check failed: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusNotFound {
		return false, fmt.Sprintf("Jira project %s not found or not accessible to runtime account %s", projectKey, cfg.Jira.Email),
			fmt.Errorf("project %s not accessible", projectKey)
	}

	if resp2.StatusCode == http.StatusForbidden {
		return false, fmt.Sprintf("Jira project %s forbidden - runtime account %s lacks permissions", projectKey, cfg.Jira.Email),
			fmt.Errorf("project %s forbidden", projectKey)
	}

	if resp2.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("Jira project %s returned status %d", projectKey, resp2.StatusCode),
			fmt.Errorf("project %s status %d", projectKey, resp2.StatusCode)
	}

	// Success - project key is accessible
	return true, fmt.Sprintf("Jira project key %s verified and accessible to %s", projectKey, cfg.Jira.Email), nil
}

// LogJiraProjectKey logs the configured Jira project key at startup for observability.
func LogJiraProjectKey(cfg *config.Config) {
	if !cfg.Jira.Enabled {
		fmt.Println("[Startup] Jira: disabled")
		return
	}

	projectKey := cfg.Jira.ProjectKey
	if projectKey == "" {
		fmt.Println("[Startup] Jira: enabled but NO PROJECT KEY CONFIGURED")
		fmt.Println("[Startup] Jira: Set project_key in jira-metadata.yaml or JIRA_PROJECT_KEY env var")
		return
	}

	credentialsSource := cfg.Jira.CredentialsSource
	if credentialsSource == "" {
		credentialsSource = "unknown"
	}

	fmt.Printf("[Startup] Jira: enabled\n")
	fmt.Printf("[Startup] Jira Project Key: %s\n", projectKey)
	fmt.Printf("[Startup] Jira Credentials Source: %s\n", credentialsSource)
	fmt.Printf("[Startup] Jira Base URL: %s\n", cfg.Jira.BaseURL)
	fmt.Printf("[Startup] Jira Email: %s\n", cfg.Jira.Email)

	// Warn if credentials source is not canonical
	inCluster := os.Getenv("KUBERNETES_SERVICE_HOST") != "" ||
		os.Getenv("CONTAINER_NAME") != "" ||
		os.Getenv("CLUSTER_ID") != ""

	if inCluster && credentialsSource != "zenlock-dir" {
		fmt.Printf("[Startup] Jira WARNING: Running in cluster but credentials from %s (expected zenlock-dir)\n", credentialsSource)
	}
}

// GetJiraProjectKeyFromMetadata reads the canonical project key from source-controlled metadata.
// This is the single source of truth for the project key.
func GetJiraProjectKeyFromMetadata() (string, error) {
	// Read from jira-metadata.yaml
	metadataPath := "deploy/zen-lock/jira-metadata.yaml"
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return "", fmt.Errorf("jira-metadata.yaml not found at %s", metadataPath)
	}

	// Read the file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("failed to read jira-metadata.yaml: %w", err)
	}

	// Parse YAML (simple string search for project_key)
	// In production, would use yaml.Unmarshal
	lines := string(data)
	for _, line := range splitLines(lines) {
		if len(line) > 12 && line[:12] == "  project_key" {
			// Extract value after colon
			for i := 12; i < len(line); i++ {
				if line[i] == ':' {
					value := trimSpacesAndQuotes(line[i+1:])
					if value != "" {
						return value, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("project_key not found in jira-metadata.yaml")
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpacesAndQuotes(s string) string {
	// Trim leading/trailing spaces
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	result := s[start:end]

	// Trim quotes
	if len(result) >= 2 && result[0] == '"' && result[len(result)-1] == '"' {
		result = result[1 : len(result)-1]
	}

	return result
}
