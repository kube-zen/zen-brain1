package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestNewJiraOffice(t *testing.T) {
	config := &Config{
		BaseURL:  "https://test.atlassian.net",
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("Failed to create JiraOffice: %v", err)
	}

	if connector == nil {
		t.Fatal("Connector is nil")
	}

	if connector.Name != "test-jira" {
		t.Errorf("Expected name 'test-jira', got %s", connector.Name)
	}
}

func TestMapWorkType(t *testing.T) {
	config := &Config{
		BaseURL:  "https://test.atlassian.net",
		APIToken: "test-token",
	}
	connector, _ := New("test", "cluster-1", config)

	tests := []struct {
		jiraType string
		expected contracts.WorkType
	}{
		{"Bug", contracts.WorkTypeDebug},
		{"bug", contracts.WorkTypeDebug},
		{"Defect", contracts.WorkTypeDebug},
		{"Task", contracts.WorkTypeImplementation},
		{"Chore", contracts.WorkTypeImplementation},
		{"Story", contracts.WorkTypeDesign},
		{"Feature", contracts.WorkTypeDesign},
		{"Epic", contracts.WorkTypeResearch},
		{"Spike", contracts.WorkTypeResearch},
		{"Improvement", contracts.WorkTypeRefactor},
		{"Unknown", contracts.WorkTypeImplementation},
	}

	for _, test := range tests {
		result := connector.mapWorkType(test.jiraType)
		if result != test.expected {
			t.Errorf("mapWorkType(%q) = %v, expected %v", test.jiraType, result, test.expected)
		}
	}
}

func TestMapPriority(t *testing.T) {
	config := &Config{
		BaseURL:  "https://test.atlassian.net",
		APIToken: "test-token",
	}
	connector, _ := New("test", "cluster-1", config)

	tests := []struct {
		jiraPriority string
		expected     contracts.Priority
	}{
		{"Highest", contracts.PriorityCritical},
		{"Critical", contracts.PriorityCritical},
		{"1", contracts.PriorityCritical},
		{"High", contracts.PriorityHigh},
		{"2", contracts.PriorityHigh},
		{"Medium", contracts.PriorityMedium},
		{"3", contracts.PriorityMedium},
		{"Low", contracts.PriorityLow},
		{"4", contracts.PriorityLow},
		{"Lowest", contracts.PriorityBackground},
		{"5", contracts.PriorityBackground},
		{"Unknown", contracts.PriorityMedium},
	}

	for _, test := range tests {
		result := connector.mapPriority(test.jiraPriority)
		if result != test.expected {
			t.Errorf("mapPriority(%q) = %v, expected %v", test.jiraPriority, result, test.expected)
		}
	}
}

func TestMapStatus(t *testing.T) {
	config := &Config{
		BaseURL:  "https://test.atlassian.net",
		APIToken: "test-token",
	}
	connector, _ := New("test", "cluster-1", config)

	tests := []struct {
		jiraStatus string
		expected   contracts.WorkStatus
	}{
		{"To Do", contracts.StatusRequested},
		{"Backlog", contracts.StatusRequested},
		{"Requested", contracts.StatusRequested},
		{"In Progress", contracts.StatusRunning},
		{"In Development", contracts.StatusRunning},
		{"Review", contracts.StatusRunning},
		{"Testing", contracts.StatusRunning},
		{"Done", contracts.StatusCompleted},
		{"Completed", contracts.StatusCompleted},
		{"Closed", contracts.StatusCompleted},
		{"Blocked", contracts.StatusBlocked},
		{"On Hold", contracts.StatusBlocked},
		{"Paused", contracts.StatusBlocked},
		{"Failed", contracts.StatusFailed},
		{"Canceled", contracts.StatusCanceled},
		{"Unknown", contracts.StatusRequested},
	}

	for _, test := range tests {
		result := connector.mapStatus(test.jiraStatus)
		if result != test.expected {
			t.Errorf("mapStatus(%q) = %v, expected %v", test.jiraStatus, result, test.expected)
		}
	}
}

func TestExtractJiraKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PROJ-123", "PROJ-123"},
		{"ABC-456", "ABC-456"},
		{"workitem-123", "workitem-123"}, // Not a valid Jira key but passes simple check
		{"123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174000"}, // UUID
	}

	for _, test := range tests {
		result := extractJiraKey(test.input)
		if result != test.expected {
			t.Errorf("extractJiraKey(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// Mock server tests would go here
func TestFetchWithMockServer(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"key": "TEST-123",
				"id": "10001",
				"self": "https://test.atlassian.net/rest/api/3/issue/10001",
				"fields": {
					"summary": "Test Issue",
					"description": "Test description",
					"created": "2026-03-07T10:00:00.000+0000",
					"updated": "2026-03-07T11:00:00.000+0000",
					"status": {"name": "To Do"},
					"priority": {"name": "Medium"},
					"issuetype": {"name": "Task"},
					"project": {"key": "TEST"},
					"reporter": {"displayName": "Test User"},
					"assignee": {"displayName": "Assignee"},
					"labels": ["test", "bug"]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}

	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("Failed to create connector: %v", err)
	}

	ctx := context.Background()
	workItem, err := connector.FetchBySourceKey(ctx, "cluster-1", "TEST-123")
	if err != nil {
		t.Fatalf("Failed to fetch issue: %v", err)
	}

	if workItem.ID != "TEST-123" {
		t.Errorf("Expected ID TEST-123, got %s", workItem.ID)
	}

	if workItem.Title != "Test Issue" {
		t.Errorf("Expected title 'Test Issue', got %s", workItem.Title)
	}

	if workItem.Status != contracts.StatusRequested {
		t.Errorf("Expected status StatusRequested, got %v", workItem.Status)
	}

	if workItem.WorkType != contracts.WorkTypeImplementation {
		t.Errorf("Expected work type WorkTypeImplementation, got %v", workItem.WorkType)
	}

	// HumanOrg includes original labels plus structured tags (jira:project:, jira:status:, jira:type:)
	if len(workItem.Tags.HumanOrg) < 2 {
		t.Errorf("Expected at least 2 labels, got %d", len(workItem.Tags.HumanOrg))
	}
	// Original labels must be present
	hasTest, hasBug := false, false
	for _, tag := range workItem.Tags.HumanOrg {
		if tag == "test" {
			hasTest = true
		}
		if tag == "bug" {
			hasBug = true
		}
	}
	if !hasTest || !hasBug {
		t.Errorf("Expected labels 'test' and 'bug' in tags, got %v", workItem.Tags.HumanOrg)
	}
}

func TestPing_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/myself" || strings.HasPrefix(r.URL.Path, "/rest/api/3/project/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}
	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := connector.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestSearch_WrapsPlainQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "jql=") {
			// Should contain project = KEY AND (plain)
			if !strings.Contains(r.URL.RawQuery, "project") {
				t.Error("expected project in JQL for plain query")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"issues":[],"total":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:    server.URL,
		Email:      "test@example.com",
		APIToken:   "test-token",
		ProjectKey: "PROJ",
	}
	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	_, err = connector.Search(ctx, "cluster-1", "status = Open")
	if err != nil {
		t.Errorf("Search: %v", err)
	}
}

func TestSearch_DoesNotDoubleWrapJQL(t *testing.T) {
	var capturedJQL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "jql=") {
			// Decode jql param
			capturedJQL = r.URL.Query().Get("jql")
			// Full JQL should be used as-is (no extra "project = X AND (...)")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"issues":[],"total":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:    server.URL,
		Email:      "test@example.com",
		APIToken:   "test-token",
		ProjectKey: "PROJ",
	}
	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	fullJQL := "project = OTHER ORDER BY created DESC"
	_, err = connector.Search(ctx, "cluster-1", fullJQL)
	if err != nil {
		t.Errorf("Search: %v", err)
	}
	if capturedJQL != fullJQL {
		t.Errorf("expected JQL as-is %q, got %q", fullJQL, capturedJQL)
	}
}

func TestAddAttachment_SendsXAtlassianToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/issue/TEST-123/attachments" {
			if r.Header.Get("X-Atlassian-Token") != "no-check" {
				t.Error("expected X-Atlassian-Token: no-check header")
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"id":"1"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &Config{
		BaseURL:  server.URL,
		Email:    "test@example.com",
		APIToken: "test-token",
	}
	connector, err := New("test-jira", "cluster-1", config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	att := &contracts.Attachment{
		ID: "1", WorkItemID: "TEST-123", Filename: "proof.json",
		ContentType: "application/json", Size: 10, CreatedAt: time.Now(),
	}
	err = connector.AddAttachment(ctx, "cluster-1", "TEST-123", att, []byte(`{"x":1}`))
	if err != nil {
		t.Errorf("AddAttachment: %v", err)
	}
}

func TestConvertToWorkItem_WorkDomainFromComponents(t *testing.T) {
	// Build issue via JSON so Fields is fully populated
	raw := `{"key":"X","fields":{"summary":"Test","status":{"name":"Open"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"project":{"key":"T"},"created":"2026-01-01T00:00:00.000Z","updated":"2026-01-01T00:00:00.000Z","labels":[],"components":[{"name":"observability"}]}}`
	var issue JiraIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	config := &Config{
		BaseURL:            "https://test.atlassian.net",
		APIToken:           "test-token",
		CustomFieldMapping: map[string]string{"customfield_10001": "core"},
	}
	connector, _ := New("test", "cluster-1", config)
	// With custom field present, mapping to "core" -> DomainCore
	wi := connector.convertToWorkItem(&issue, map[string]interface{}{"customfield_10001": "backend"})
	if wi.WorkDomain != contracts.DomainCore {
		t.Errorf("expected WorkDomain from custom field mapping, got %s", wi.WorkDomain)
	}
	// Without custom field match, component "observability" -> DomainObservability
	wi2 := connector.convertToWorkItem(&issue, nil)
	if wi2.WorkDomain != contracts.DomainObservability {
		t.Errorf("expected WorkDomain from component observability, got %s", wi2.WorkDomain)
	}
}

func TestConvertToWorkItem_CustomFieldTagTruncation(t *testing.T) {
	raw := `{"key":"T-1","fields":{"summary":"","status":{"name":"Open"},"priority":{"name":"Medium"},"issuetype":{"name":"Task"},"project":{"key":"T"},"created":"2026-01-01T00:00:00.000Z","updated":"2026-01-01T00:00:00.000Z"}}`
	var issue JiraIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	config := &Config{BaseURL: "https://test.atlassian.net", APIToken: "test-token"}
	connector, _ := New("test", "cluster-1", config)
	longVal := strings.Repeat("x", 500)
	wi := connector.convertToWorkItem(&issue, map[string]interface{}{"customfield_99": longVal})
	var found bool
	for _, tag := range wi.Tags.HumanOrg {
		if strings.HasPrefix(tag, "customfield_99:") {
			found = true
			if len(tag) > 250 {
				t.Errorf("custom field tag should be truncated, got length %d", len(tag))
			}
			break
		}
	}
	if !found {
		t.Error("expected custom field tag to be present")
	}
}
