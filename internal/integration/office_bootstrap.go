// Package integration provides Office bootstrap from config (avoids office->jira import cycle).
package integration

import (
	"fmt"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
)

// BuildJiraConfig builds jira.Config from application config.
// If cfg.Jira.ProjectKey is empty and cfg.Jira.Project is set, Project is used as ProjectKey.
func BuildJiraConfig(cfg *config.Config) *jira.Config {
	if cfg == nil || !cfg.Jira.Enabled {
		return nil
	}
	projectKey := cfg.Jira.ProjectKey
	if projectKey == "" && cfg.Jira.Project != "" {
		projectKey = cfg.Jira.Project
	}
	email := cfg.Jira.Email
	if email == "" && cfg.Jira.Username != "" {
		email = cfg.Jira.Username
	}
	return &jira.Config{
		BaseURL:            cfg.Jira.BaseURL,
		Email:              email,
		APIToken:           cfg.Jira.APIToken,
		ProjectKey:         projectKey,
		WebhookURL:         cfg.Jira.WebhookURL,
		WebhookSecret:      cfg.Jira.WebhookSecret,
		WebhookPort:        cfg.Jira.WebhookPort,
		WebhookPath:        cfg.Jira.WebhookPath,
		StatusMapping:      cfg.Jira.StatusMapping,
		WorkTypeMapping:    cfg.Jira.WorkTypeMapping,
		PriorityMapping:    cfg.Jira.PriorityMapping,
		CustomFieldMapping: cfg.Jira.CustomFieldMapping,
	}
}

// InitOfficeManagerFromConfig creates an Office Manager and registers the Jira connector
// when Jira is enabled and required fields are present. Cluster "default" is mapped to "jira".
// Returns (nil, nil) when Jira is disabled. Returns error when enabled but config invalid.
func InitOfficeManagerFromConfig(cfg *config.Config) (*office.Manager, error) {
	if cfg == nil || !cfg.Jira.Enabled {
		return office.NewManager(), nil
	}
	jiraCfg := BuildJiraConfig(cfg)
	if jiraCfg.BaseURL == "" || jiraCfg.APIToken == "" {
		return nil, fmt.Errorf("jira enabled but missing required fields: need base_url and api_token (from JIRA_URL and JIRA_API_TOKEN or JIRA_TOKEN)")
	}
	connector, err := jira.New("jira", "default", jiraCfg)
	if err != nil {
		return nil, fmt.Errorf("create jira connector: %w", err)
	}
	mgr := office.NewManager()
	if err := mgr.Register("jira", connector); err != nil {
		return nil, fmt.Errorf("register jira connector: %w", err)
	}
	if err := mgr.RegisterForCluster("default", "jira"); err != nil {
		return nil, fmt.Errorf("register jira for cluster default: %w", err)
	}
	return mgr, nil
}
