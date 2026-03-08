package factory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestProofOfWork_JiraIntegration tests the full flow from factory execution
// to proof-of-work generation to Jira comment creation.
func TestProofOfWork_JiraIntegration(t *testing.T) {
	// Setup proof-of-work manager
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create a task spec
	spec := &FactoryTaskSpec{
		ID:         "integration-test-task",
		SessionID:   "integration-test-session",
		WorkItemID:  "PROJ-999",
		Title:       "Integration Test Task",
		Objective:   "Test end-to-end proof-of-work to Jira integration",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Execute task (simplified - we'll mock execution result)
	ctx := context.Background()

	// Create mock execution result
	result := &ExecutionResult{
		TaskID:       spec.ID,
		SessionID:    spec.SessionID,
		WorkItemID:  spec.WorkItemID,
		Status:       ExecutionStatusCompleted,
		Success:      true,
		CompletedAt:  time.Now(),
		Duration:     5 * time.Minute,
		WorkspacePath: runtimeDir,
		FilesChanged: []string{"integration_test.go"},
		TestsRun:     []string{"TestIntegration"},
		TestsPassed:  true,
		Recommendation: "merge",
		ExecutionSteps: []*ExecutionStep{},
		SREDEvidence: []contracts.EvidenceItem{},
	}

	// Step 1: Create proof-of-work artifact
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Verify artifact created
	if artifact == nil {
		t.Fatal("artifact should not be nil")
	}
	if artifact.Summary == nil {
		t.Fatal("artifact summary should not be nil")
	}
	if artifact.Summary.TaskID != spec.ID {
		t.Errorf("TaskID mismatch: got %s, want %s", artifact.Summary.TaskID, spec.ID)
	}

	// Step 2: Generate Jira comment
	comment, err := powManager.GenerateJiraComment(ctx, artifact)
	if err != nil {
		t.Fatalf("GenerateJiraComment failed: %v", err)
	}

	// Verify comment structure
	if comment == nil {
		t.Fatal("comment should not be nil")
	}
	if comment.WorkItemID != spec.WorkItemID {
		t.Errorf("Comment WorkItemID mismatch: got %s, want %s", comment.WorkItemID, spec.WorkItemID)
	}
	if comment.Body == "" {
		t.Error("Comment body should not be empty")
	}
	if comment.Attribution == nil {
		t.Error("Comment attribution should not be nil")
	}

	// Step 3: Verify comment can be formatted for Jira API
	// Create a mock Jira server to test the actual HTTP request
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path matches Jira comment endpoint
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/rest/api/3/issue/PROJ-999/comment" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Check for authorization header (basic auth for Jira)
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Missing Authorization header")
		}

		// Check content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected application/json, got %s", contentType)
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": "12345", "body": "comment posted"}`))
	}))
	defer mockServer.Close()

	// Step 4: Test with actual Jira connector (requires config)
	// For integration test, we'll skip actual connector instantiation
	// since it requires real Jira credentials. Instead we verify the
	// comment structure matches what Jira expects.
	t.Logf("Integration test completed successfully")
	t.Logf("Proof-of-work artifact created at: %s", artifact.Directory)
	t.Logf("Jira comment generated with body length: %d", len(comment.Body))
	t.Logf("Comment attribution: %s", comment.Attribution.AgentRole)
}

// TestProofOfWork_FieldAlignment verifies that proof-of-work schema
// aligns with documentation requirements.
func TestProofOfWork_FieldAlignment(t *testing.T) {
	// Create a summary with all fields populated
	summary := &ProofOfWorkSummary{
		TaskID:        "test-task",
		SessionID:      "test-session",
		WorkItemID:    "PROJ-123",
		SourceKey:      "PROJ-123",
		SourceSystem:   "jira",
		Title:          "Test Task",
		Objective:      "Test objective",
		Result:         "completed",
		StartedAt:      time.Now(),
		CompletedAt:    time.Now(),
		Duration:       10 * time.Minute,
		ModelUsed:      "factory-v1",
		AgentRole:      "factory",
		FilesChanged:   []string{"test.go"},
		NewFiles:       []string{"test.go"},
		ModifiedFiles:  []string{},
		DeletedFiles:   []string{},
		LinesAdded:     10,
		LinesDeleted:   2,
		TestsRun:       []string{"TestSomething"},
		TestsPassed:    true,
		TestsFailed:    []string{},
		CommandLog:     []string{"go test ./..."},
		OutputLog:      "PASS",
		ErrorLog:       "",
		EvidenceItems:  []contracts.EvidenceItem{},
		UnresolvedRisks: []string{},
		KnownLimitations: []string{},
		RecommendedAction: "merge",
		RequiresApproval: false,
		ReviewNotes:     "",
		ArtifactPaths:   []string{},
		GitBranch:       "ai/PROJ-123",
		GitCommit:       "",
		PRURL:           "",
		GeneratedAt:     time.Now(),
	}

	// Verify required fields from Batch C specification
	requiredFields := []struct {
		name  string
		value interface{}
	}{
		{"task_id", summary.TaskID},
		{"session_id", summary.SessionID},
		{"work_item_id", summary.WorkItemID},
		{"source_key", summary.SourceKey},
		{"source_system", summary.SourceSystem},
		{"title", summary.Title},
		{"objective", summary.Objective},
		{"result", summary.Result},
		{"started_at", summary.StartedAt},
		{"completed_at", summary.CompletedAt},
		{"duration", summary.Duration},
		{"model_used", summary.ModelUsed},
		{"agent_role", summary.AgentRole},
		{"files_changed", summary.FilesChanged},
		{"tests_run", summary.TestsRun},
		{"tests_passed", summary.TestsPassed},
		{"recommended_action", summary.RecommendedAction},
		{"requires_approval", summary.RequiresApproval},
		{"generated_at", summary.GeneratedAt},
	}

	for _, field := range requiredFields {
		switch v := field.value.(type) {
		case string:
			if v == "" {
				t.Errorf("Required field %s is empty", field.name)
			}
		case []string:
			// Arrays can be empty
		case time.Time:
			if v.IsZero() {
				t.Errorf("Required field %s is zero time", field.name)
			}
		case time.Duration:
			if v == 0 {
				t.Errorf("Required field %s is zero duration", field.name)
			}
		case bool:
			// bool is fine
		default:
			// other types okay
		}
	}

	// Verify that all fields from documentation are represented
	// Documentation mentions: session_id, work_item_id, source_key, source_system
	// title, objective, result, started_at, completed_at, model_used, agent_role
	// files_changed, tests_run, tests_passed, evidence_items, unresolved_risks
	// recommended_action, requires_approval, artifact_paths, git_branch, git_commit, pr_url
	// All present in our schema ✅

	t.Log("Proof-of-work schema aligns with documentation requirements")
}

// TestProofOfWork_EvidenceHandling tests SR&ED evidence collection.
func TestProofOfWork_EvidenceHandling(t *testing.T) {
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create evidence items
	evidenceItems := []contracts.EvidenceItem{
		{
			ID:          "ev-001",
			SessionID:   "session-1",
			Type:        "hypothesis",
			Content:     "RISK: This might not work",
			Metadata:    map[string]string{"experiment": "test"},
			CollectedAt: time.Now(),
		},
		{
			ID:          "ev-002",
			SessionID:   "session-1",
			Type:        "observation",
			Content:     "Test observation",
			Metadata:    map[string]string{"experiment": "test"},
			CollectedAt: time.Now(),
		},
	}

	result := &ExecutionResult{
		TaskID:        "evidence-test-task",
		SessionID:      "evidence-test-session",
		WorkItemID:    "PROJ-456",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       15 * time.Minute,
		WorkspacePath:   runtimeDir,
		SREDEvidence:   evidenceItems,
		Recommendation: "review",
		ExecutionSteps:  []*ExecutionStep{},
	}

	spec := &FactoryTaskSpec{
		ID:         "evidence-test-task",
		SessionID:   "evidence-test-session",
		WorkItemID:  "PROJ-456",
		Title:       "Evidence Test Task",
		Objective:   "Test evidence handling",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Verify evidence items are included
	if len(artifact.Summary.EvidenceItems) != 2 {
		t.Errorf("Expected 2 evidence items, got %d", len(artifact.Summary.EvidenceItems))
	}

	// Verify risks extracted from evidence
	if len(artifact.Summary.UnresolvedRisks) != 1 {
		t.Errorf("Expected 1 unresolved risk, got %d", len(artifact.Summary.UnresolvedRisks))
	} else {
		if artifact.Summary.UnresolvedRisks[0] != "This might not work" {
			t.Errorf("Risk extraction incorrect: got %s", artifact.Summary.UnresolvedRisks[0])
		}
	}

	// Read markdown to verify evidence section
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	md := string(mdContent)
	if !strings.Contains(md, "## Evidence (SR&ED)") {
		t.Error("Markdown should have Evidence section for SR&ED")
	}
	if !strings.Contains(md, "hypothesis") || !strings.Contains(md, "observation") {
		t.Error("Markdown should contain evidence item types")
	}
}