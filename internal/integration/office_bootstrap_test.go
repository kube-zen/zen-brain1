package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/office/jira"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
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

// TestOfficeManager_FetchAndSearchAgainstTestServer ensures office Manager Fetch and Search
// work against a mock Jira server (Block 2: office fetch/search commands work against test server).
func TestOfficeManager_FetchAndSearchAgainstTestServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/issue/TEST-1":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"key": "TEST-1",
				"id": "10001",
				"fields": {
					"summary": "Office test issue",
					"description": "",
					"created": "2026-01-01T00:00:00.000+0000",
					"updated": "2026-01-01T00:00:00.000+0000",
					"status": {"name": "Open"},
					"priority": {"name": "Medium"},
					"issuetype": {"name": "Task"},
					"project": {"key": "TEST"},
					"reporter": {"displayName": ""},
					"assignee": {"displayName": ""},
					"labels": []
				}
			}`))
		case r.URL.Path == "/rest/api/3/search/jql":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"issues":[{"key":"TEST-1","id":"10001","fields":{"summary":"Office test issue","status":{"name":"Open"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"project":{"key":"TEST"},"reporter":{},"assignee":{},"labels":[],"created":"2026-01-01T00:00:00.000+0000","updated":"2026-01-01T00:00:00.000+0000"}}],"total":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	jiraCfg := &jira.Config{
		BaseURL:    server.URL,
		Email:      "test@example.com",
		APIToken:   "test-token",
		ProjectKey: "TEST",
	}
	conn, err := jira.New("jira", "default", jiraCfg)
	if err != nil {
		t.Fatalf("jira.New: %v", err)
	}
	mgr := NewOfficeManagerForTest(conn)
	ctx := context.Background()

	// Fetch
	item, err := mgr.Fetch(ctx, "default", "TEST-1")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if item.ID != "TEST-1" || item.Title != "Office test issue" {
		t.Errorf("Fetch: got ID=%q Title=%q", item.ID, item.Title)
	}
	if item.Status != contracts.StatusRequested {
		t.Errorf("Fetch: expected status requested, got %s", item.Status)
	}

	// Search
	items, err := mgr.Search(ctx, "default", "status = Open")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(items) != 1 || items[0].ID != "TEST-1" {
		t.Errorf("Search: expected one item TEST-1, got %d items %v", len(items), items)
	}
}

// NewOfficeManagerForTest creates an office Manager with the given connector registered for "default".
// Used by integration tests that need Fetch/Search against a test server.
func NewOfficeManagerForTest(conn *jira.JiraOffice) *office.Manager {
	mgr := office.NewManager()
	_ = mgr.Register("jira", conn)
	_ = mgr.RegisterForCluster("default", "jira")
	return mgr
}
