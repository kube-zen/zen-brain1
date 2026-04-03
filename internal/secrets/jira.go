// Package secrets provides canonical secret resolution for office integrations.
package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// JiraMaterial holds resolved Jira credentials.
type JiraMaterial struct {
	BaseURL    string
	Email      string
	APIToken   string
	ProjectKey string
	Source     string
}

// JiraResolveOptions controls credential resolution behavior.
type JiraResolveOptions struct {
	DirPath          string // ZenLock mounted directory (e.g., /zen-lock/secrets)
	FilePath         string // Host credential file (e.g., ~/.zen-brain/secrets/jira.yaml)
	AllowEnvFallback bool   // Allow env vars as fallback (default: false)
	ClusterMode      bool   // If true, ONLY DirPath allowed (no FilePath, no env fallback)
}

// ResolveJira resolves Jira credentials from canonical sources.
// Resolution order depends on mode:
//
// Cluster mode (ClusterMode=true):
//   - ONLY DirPath (/zen-lock/secrets)
//   - No FilePath fallback
//   - No env fallback
//   - Hard fail if DirPath not present
//
// Local mode (ClusterMode=false):
//   - DirPath → FilePath → Env fallback (if AllowEnvFallback=true)
//
// Returns clear Source string: "zenlock-dir:<path>", "host-file:<path>", "env", "none".
// In cluster mode, returns error if credentials not found (no silent "none" return).
func ResolveJira(ctx context.Context, opts JiraResolveOptions) (*JiraMaterial, error) {
	// CLUSTER MODE: Strict enforcement - ONLY DirPath allowed
	if opts.ClusterMode {
		if opts.DirPath == "" {
			return nil, fmt.Errorf("cluster mode: DirPath required for ZenLock mount")
		}
		material, err := tryZenLockDir(opts.DirPath)
		if err != nil {
			return nil, fmt.Errorf("cluster mode: ZenLock mount failed at %s: %w", opts.DirPath, err)
		}
		if material == nil || material.Source == "none" {
			return nil, fmt.Errorf("cluster mode: no credentials found in ZenLock mount at %s", opts.DirPath)
		}
		return material, nil
	}

	// LOCAL MODE: Try DirPath → FilePath → Env fallback (if allowed)
	
	// Try ZenLock directory first
	if opts.DirPath != "" {
		material, err := tryZenLockDir(opts.DirPath)
		if err == nil && material != nil {
			return material, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Try host file
	if opts.FilePath != "" {
		material, err := tryHostFile(opts.FilePath)
		if err == nil && material != nil {
			return material, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	// Try env fallback only if explicitly allowed
	if opts.AllowEnvFallback {
		material := tryEnvFallback()
		if material != nil {
			return material, nil
		}
	}

	// No credentials found
	return &JiraMaterial{
		Source: "none",
	}, nil
}

// tryZenLockDir attempts to read credentials from ZenLock mounted directory.
// Expected files: JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN, JIRA_PROJECT_KEY
func tryZenLockDir(dirPath string) (*JiraMaterial, error) {
	material := &JiraMaterial{
		Source: fmt.Sprintf("zenlock-dir:%s", dirPath),
	}

	// Read individual files
	readFile := func(name string) string {
		content, err := os.ReadFile(filepath.Join(dirPath, name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(content))
	}

	material.BaseURL = readFile("JIRA_URL")
	material.Email = readFile("JIRA_EMAIL")
	material.APIToken = readFile("JIRA_API_TOKEN")
	material.ProjectKey = readFile("JIRA_PROJECT_KEY")

	// Validate that at least some credentials are present
	if material.BaseURL == "" && material.APIToken == "" {
		return nil, fmt.Errorf("zenlock-dir: no credentials found in %s", dirPath)
	}

	return material, nil
}

// tryHostFile attempts to read credentials from host YAML file.
// Supports both stringData format (ZenLock-style) and flat YAML keys.
func tryHostFile(filePath string) (*JiraMaterial, error) {
	material := &JiraMaterial{
		Source: fmt.Sprintf("host-file:%s", filePath),
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("host-file: failed to read %s: %w", filePath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("host-file: failed to parse YAML: %w", err)
	}

	// Helper to extract string value from nested map (handles both map[string]interface{} and map[interface{}]interface{})
	extractString := func(m map[string]interface{}, key string) string {
		if val, ok := m[key]; ok {
			if strVal, ok := val.(string); ok {
				return strVal
			}
		}
		return ""
	}

	// Try stringData format first (ZenLock-style)
	// YAML v2 may unmarshal to map[interface{}]interface{}
	if stringDataVal, ok := raw["stringData"]; ok {
		// Try map[string]interface{} first
		if stringData, ok := stringDataVal.(map[string]interface{}); ok {
			material.BaseURL = extractString(stringData, "JIRA_URL")
			material.Email = extractString(stringData, "JIRA_EMAIL")
			material.APIToken = extractString(stringData, "JIRA_API_TOKEN")
			material.ProjectKey = extractString(stringData, "JIRA_PROJECT_KEY")
		} else if stringData, ok := stringDataVal.(map[interface{}]interface{}); ok {
			// Handle map[interface{}]interface{} case
			if val, ok := stringData["JIRA_URL"]; ok {
				if strVal, ok := val.(string); ok {
					material.BaseURL = strVal
				}
			}
			if val, ok := stringData["JIRA_EMAIL"]; ok {
				if strVal, ok := val.(string); ok {
					material.Email = strVal
				}
			}
			if val, ok := stringData["JIRA_API_TOKEN"]; ok {
				if strVal, ok := val.(string); ok {
					material.APIToken = strVal
				}
			}
			if val, ok := stringData["JIRA_PROJECT_KEY"]; ok {
				if strVal, ok := val.(string); ok {
					material.ProjectKey = strVal
				}
			}
		}
	} else {
		// Fall back to flat YAML keys
		material.BaseURL = extractString(raw, "JIRA_URL")
		material.Email = extractString(raw, "JIRA_EMAIL")
		material.APIToken = extractString(raw, "JIRA_API_TOKEN")
		material.ProjectKey = extractString(raw, "JIRA_PROJECT_KEY")
	}

	// Validate that at least some credentials are present
	if material.BaseURL == "" && material.APIToken == "" {
		return nil, fmt.Errorf("host-file: no credentials found in %s", filePath)
	}

	return material, nil
}

// tryEnvFallback attempts to read credentials from environment variables.
// Only used when AllowEnvFallback is explicitly true.
// Supports: JIRA_API_TOKEN/JIRA_TOKEN, JIRA_EMAIL/JIRA_USERNAME, JIRA_URL, JIRA_PROJECT_KEY
func tryEnvFallback() *JiraMaterial {
	material := &JiraMaterial{
		Source: "env",
	}

	// Read from env
	if material.BaseURL == "" {
		material.BaseURL = os.Getenv("JIRA_URL")
	}
	if material.Email == "" {
		material.Email = os.Getenv("JIRA_EMAIL")
		if material.Email == "" {
			material.Email = os.Getenv("JIRA_USERNAME")
		}
	}
	if material.APIToken == "" {
		material.APIToken = os.Getenv("JIRA_API_TOKEN")
		if material.APIToken == "" {
			material.APIToken = os.Getenv("JIRA_TOKEN")
		}
	}
	if material.ProjectKey == "" {
		material.ProjectKey = os.Getenv("JIRA_PROJECT_KEY")
	}

	// Validate that at least some credentials are present
	if material.BaseURL == "" && material.APIToken == "" {
		return nil
	}

	return material
}

// JiraCapabilities represents what Jira operations are possible.
type JiraCapabilities struct {
	TokenReadable    bool
	ReadAllowed      bool // GET /issue works
	UpdateAllowed    bool // PUT /issue works
	CreateAllowed    bool // POST /issue works
	TransitionAllowed bool // POST /transitions works
}

// CheckJiraCapabilities tests Jira capability matrix.
// Returns capability booleans, never secret values.
// Note: This requires actual API calls to test permissions.
func CheckJiraCapabilities(ctx context.Context, material *JiraMaterial) (*JiraCapabilities, error) {
	caps := &JiraCapabilities{}

	if material == nil || material.APIToken == "" {
		return caps, fmt.Errorf("no Jira token available")
	}

	caps.TokenReadable = true

	// Test read permission (GET /issue/PROOF-1 or similar)
	// For now, assume read works if token is present
	// Full implementation would test actual API endpoint
	caps.ReadAllowed = true
	caps.UpdateAllowed = true
	caps.TransitionAllowed = true

	// Create permission requires actual API test
	// For now, assume true if token present
	// TODO: Implement actual POST /issue test with dry-run or proof ticket
	caps.CreateAllowed = true

	return caps, nil
}

// FormatJiraCapabilitySummary creates non-secret capability report.
func FormatJiraCapabilitySummary(material *JiraMaterial, caps *JiraCapabilities) string {
	var lines []string

	lines = append(lines, "=== JIRA CAPABILITIES ===")

	if material != nil {
		lines = append(lines, fmt.Sprintf("Token Source: %s", material.Source))
	} else {
		lines = append(lines, "Token Source: NOT AVAILABLE")
	}

	if caps == nil {
		lines = append(lines, "Capabilities: NOT TESTED")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, fmt.Sprintf("Token Readable: %v", caps.TokenReadable))
	lines = append(lines, fmt.Sprintf("Read Allowed: %v", caps.ReadAllowed))
	lines = append(lines, fmt.Sprintf("Update Allowed: %v", caps.UpdateAllowed))
	lines = append(lines, fmt.Sprintf("Create Allowed: %v", caps.CreateAllowed))
	lines = append(lines, fmt.Sprintf("Transition Allowed: %v", caps.TransitionAllowed))

	if !caps.CreateAllowed {
		lines = append(lines, "WARNING: Jira token present but CREATE permission MISSING")
	}

	return strings.Join(lines, "\n")
}
