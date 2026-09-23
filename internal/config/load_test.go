package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv_JiraEmailUsername(t *testing.T) {
	// ZB-025A law: Jira credentials resolve ONLY from ZenLock secrets /
	// credentials sources. JIRA_EMAIL / JIRA_USERNAME env vars must NOT
	// populate credentials (env fallback removed in 41f1b70a/d0e41164).
	t.Run("JIRA_EMAIL ignored", func(t *testing.T) {
		os.Setenv("JIRA_EMAIL", "e@test.com")
		os.Setenv("JIRA_USERNAME", "u")
		defer func() {
			os.Unsetenv("JIRA_EMAIL")
			os.Unsetenv("JIRA_USERNAME")
		}()
		cfg := DefaultConfig()
		cfg.loadFromEnv()
		if cfg.Jira.Email != "" {
			t.Errorf("Jira.Email must stay empty without a sanctioned credential source, got %q", cfg.Jira.Email)
		}
	})
	t.Run("JIRA_USERNAME ignored", func(t *testing.T) {
		os.Unsetenv("JIRA_EMAIL")
		os.Setenv("JIRA_USERNAME", "fallback@u")
		defer os.Unsetenv("JIRA_USERNAME")
		cfg := DefaultConfig()
		cfg.loadFromEnv()
		if cfg.Jira.Email != "" {
			t.Errorf("Jira.Email must stay empty without a sanctioned credential source, got %q", cfg.Jira.Email)
		}
	})
}

func TestLoadFromEnv_JiraAPITokenToken(t *testing.T) {
	// ZB-025A law: JIRA_API_TOKEN / JIRA_TOKEN env vars must NOT populate
	// credentials (env fallback removed; ZenLock-only resolution).
	t.Run("JIRA_API_TOKEN ignored", func(t *testing.T) {
		os.Setenv("JIRA_API_TOKEN", "api-secret")
		os.Setenv("JIRA_TOKEN", "legacy")
		defer func() {
			os.Unsetenv("JIRA_API_TOKEN")
			os.Unsetenv("JIRA_TOKEN")
		}()
		cfg := DefaultConfig()
		cfg.loadFromEnv()
		if cfg.Jira.APIToken != "" {
			t.Errorf("Jira.APIToken must stay empty without a sanctioned credential source, got %q", cfg.Jira.APIToken)
		}
	})
	t.Run("JIRA_TOKEN ignored", func(t *testing.T) {
		os.Unsetenv("JIRA_API_TOKEN")
		os.Setenv("JIRA_TOKEN", "legacy-secret")
		defer os.Unsetenv("JIRA_TOKEN")
		cfg := DefaultConfig()
		cfg.loadFromEnv()
		if cfg.Jira.APIToken != "" {
			t.Errorf("Jira.APIToken must stay empty without a sanctioned credential source, got %q", cfg.Jira.APIToken)
		}
	})
}

func TestLoadFromEnv_ProjectProjectKeyCompatibility(t *testing.T) {
	// When ProjectKey is empty and Project is set, ProjectKey becomes Project
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// YAML with project: PROJ only (no project_key)
	err := os.WriteFile(path, []byte(`
jira:
  enabled: true
  base_url: "https://jira.example.com"
  project: "PROJ"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	// Temporarily override findConfigPath by loading from our file
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jira.ProjectKey != "PROJ" {
		t.Errorf("expected ProjectKey PROJ from Project compatibility, got %q", cfg.Jira.ProjectKey)
	}
	if cfg.Jira.Project != "PROJ" {
		t.Errorf("expected Project PROJ, got %q", cfg.Jira.Project)
	}
}
