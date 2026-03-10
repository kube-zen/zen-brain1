package integration

import (
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
)

func TestInitOfficeManagerFromConfig_Disabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jira.Enabled = false
	mgr, err := InitOfficeManagerFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	if len(mgr.ListConnectors()) != 0 {
		t.Errorf("expected no connectors when Jira disabled, got %v", mgr.ListConnectors())
	}
}

func TestInitOfficeManagerFromConfig_EnabledAndRequiredMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jira.Enabled = true
	cfg.Jira.BaseURL = ""
	cfg.Jira.APIToken = ""
	_, err := InitOfficeManagerFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error when enabled but required fields missing")
	}
}

func TestInitOfficeManagerFromConfig_EnabledWithRequired(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jira.Enabled = true
	cfg.Jira.BaseURL = "https://test.atlassian.net"
	cfg.Jira.APIToken = "test-token"
	cfg.Jira.Email = "test@example.com"
	cfg.Jira.ProjectKey = "TEST"
	mgr, err := InitOfficeManagerFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
	connectors := mgr.ListConnectors()
	if len(connectors) != 1 || connectors[0] != "jira" {
		t.Errorf("expected one connector named jira, got %v", connectors)
	}
	// Cluster default -> jira
	conn, err := mgr.GetConnectorForCluster("default")
	if err != nil {
		t.Fatalf("GetConnectorForCluster(default): %v", err)
	}
	if conn == nil {
		t.Fatal("connector for default should not be nil")
	}
}

func TestBuildJiraConfig_ProjectKeyFromProject(t *testing.T) {
	cfg := &config.Config{}
	cfg.Jira.Enabled = true
	cfg.Jira.Project = "LEGACY"
	cfg.Jira.ProjectKey = ""
	jiraCfg := BuildJiraConfig(cfg)
	if jiraCfg == nil {
		t.Fatal("expected non-nil jira config when enabled")
	}
	if jiraCfg.ProjectKey != "LEGACY" {
		t.Errorf("expected ProjectKey LEGACY from Project, got %q", jiraCfg.ProjectKey)
	}
}
