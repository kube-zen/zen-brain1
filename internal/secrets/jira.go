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
}

// ResolveJira resolves Jira credentials from canonical sources.
// Resolution order: DirPath → FilePath → Env fallback (if allowed).
// Returns clear Source string: "zenlock-dir:<path>", "host-file:<path>", "env", "none".
func ResolveJira(ctx context.Context, opts JiraResolveOptions) (*JiraMaterial, error) {
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
