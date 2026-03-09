package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	config := &Config{}
	connector, _ := New("test", "cluster-1", config)

	tests := []struct {
		jiraType   string
		expected   contracts.WorkType
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
	config := &Config{}
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
	config := &Config{}
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

	if len(workItem.Tags.HumanOrg) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(workItem.Tags.HumanOrg))
	}
}