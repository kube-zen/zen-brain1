package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestProofOfWorkManager_CreateProofOfWork(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create execution result
	result := &ExecutionResult{
		TaskID:       "task-1",
		SessionID:    "session-1",
		WorkItemID:   "PROJ-123",
		Status:       ExecutionStatusCompleted,
		Success:      true,
		CompletedAt:  time.Now(),
		Duration:     5 * time.Minute,
		WorkspacePath: "/tmp/workspace",
		FilesChanged: []string{"file1.go", "file2.go"},
		TestsRun:     []string{"test1", "test2"},
		TestsPassed:  true,
		SREDEvidence: []contracts.EvidenceItem{
			{
				ID:          "evidence-1",
				SessionID:   "session-1",
				Type:        "hypothesis",
				Content:     "Test hypothesis",
				Metadata:    map[string]string{"key": "value"},
				CollectedAt: time.Now(),
			},
		},
		ExecutionSteps: []*ExecutionStep{},
	}

	// Create task spec
	spec := &FactoryTaskSpec{
		ID:         "task-1",
		SessionID:   "session-1",
		WorkItemID:  "PROJ-123",
		Title:       "Test Task",
		Objective:   "Execute test objective",
	}

	// Create proof-of-work
	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	if artifact == nil {
		t.Fatal("artifact should not be nil")
	}

	if artifact.Summary == nil {
		t.Fatal("summary should not be nil")
	}

	if artifact.Directory == "" {
		t.Error("directory should not be empty")
	}

	if artifact.JSONPath == "" {
		t.Error("json_path should not be empty")
	}

	if artifact.MarkdownPath == "" {
		t.Error("markdown_path should not be empty")
	}

	if artifact.LogPath == "" {
		t.Error("log_path should not be empty")
	}

	// Verify files exist
	if _, err := os.Stat(artifact.JSONPath); os.IsNotExist(err) {
		t.Error("JSON file should exist")
	}

	if _, err := os.Stat(artifact.MarkdownPath); os.IsNotExist(err) {
		t.Error("Markdown file should exist")
	}

	if _, err := os.Stat(artifact.LogPath); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}

	// Verify summary content
	if artifact.Summary.TaskID != "task-1" {
		t.Errorf("TaskID mismatch: got %s, want task-1", artifact.Summary.TaskID)
	}

	if artifact.Summary.SessionID != "session-1" {
		t.Errorf("SessionID mismatch: got %s, want session-1", artifact.Summary.SessionID)
	}

	if artifact.Summary.Result != "completed" {
		t.Errorf("Result mismatch: got %s, want completed", artifact.Summary.Result)
	}
}

func TestProofOfWorkManager_ListProofOfWorks(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create a proof-of-work artifact first
	result := &ExecutionResult{
		TaskID:      "list-test-task",
		SessionID:   "list-test-session",
		WorkItemID:  "PROJ-456",
		Status:      ExecutionStatusCompleted,
		Success:     true,
		CompletedAt:  time.Now(),
		Duration:     1 * time.Minute,
		WorkspacePath: "/tmp/workspace",
		ExecutionSteps: []*ExecutionStep{},
		SREDEvidence: []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "list-test-task",
		SessionID:   "list-test-session",
		WorkItemID:  "PROJ-456",
		Title:       "List Test Task",
		Objective:   "List test objective",
	}

	ctx := context.Background()
	_, _ = powManager.CreateProofOfWork(ctx, result, spec)

	// List proof-of-works
	artifacts, err := powManager.ListProofOfWorks(ctx, "list-test-task")

	// Verify
	if err != nil {
		t.Fatalf("ListProofOfWorks failed: %v", err)
	}

	if len(artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(artifacts))
	}

	if artifacts[0].Summary.TaskID != "list-test-task" {
		t.Errorf("TaskID mismatch: got %s, want list-test-task", artifacts[0].Summary.TaskID)
	}
}

func TestProofOfWorkManager_GetProofOfWork(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create a proof-of-work artifact
	result := &ExecutionResult{
		TaskID:      "get-test-task",
		SessionID:   "get-test-session",
		WorkItemID:  "PROJ-789",
		Status:      ExecutionStatusCompleted,
		Success:     true,
		CompletedAt:  time.Now(),
		Duration:     2 * time.Minute,
		WorkspacePath: "/tmp/workspace",
		ExecutionSteps: []*ExecutionStep{},
		SREDEvidence: []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "get-test-task",
		SessionID:   "get-test-session",
		WorkItemID:  "PROJ-789",
		Title:       "Get Test Task",
		Objective:   "Get test objective",
	}

	ctx := context.Background()
	artifact, _ := powManager.CreateProofOfWork(ctx, result, spec)

	// Get proof-of-work
	retrievedArtifact, err := powManager.GetProofOfWork(ctx, artifact.Directory)

	// Verify
	if err != nil {
		t.Fatalf("GetProofOfWork failed: %v", err)
	}

	if retrievedArtifact == nil {
		t.Fatal("retrieved artifact should not be nil")
	}

	if retrievedArtifact.Summary.TaskID != "get-test-task" {
		t.Errorf("TaskID mismatch: got %s, want get-test-task", retrievedArtifact.Summary.TaskID)
	}

	if retrievedArtifact.Directory != artifact.Directory {
		t.Errorf("Directory mismatch: got %s, want %s", retrievedArtifact.Directory, artifact.Directory)
	}
}

func TestProofOfWorkManager_CleanupProofOfWorks(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create multiple proof-of-work artifacts
	for i := 0; i < 3; i++ {
		result := &ExecutionResult{
			TaskID:       fmt.Sprintf("cleanup-task-%d", i),
			SessionID:    "cleanup-session",
			WorkItemID:  "PROJ-123",
			Status:       ExecutionStatusCompleted,
			Success:      true,
			CompletedAt:  time.Now(),
			Duration:     time.Duration(i) * time.Minute,
			WorkspacePath: "/tmp/workspace",
			ExecutionSteps: []*ExecutionStep{},
			SREDEvidence: []contracts.EvidenceItem{},
		}

		spec := &FactoryTaskSpec{
			ID:       fmt.Sprintf("cleanup-task-%d", i),
			SessionID: "cleanup-session",
			Title:    fmt.Sprintf("Cleanup Task %d", i),
			Objective: fmt.Sprintf("Cleanup objective %d", i),
		}

		ctx := context.Background()
		_, _ = powManager.CreateProofOfWork(ctx, result, spec)
	}

	// Wait a moment to ensure timestamps differ
	time.Sleep(10 * time.Millisecond)

	// Cleanup proof-of-works older than 5 minutes
	ctx := context.Background()
	err := powManager.CleanupProofOfWorks(ctx, 5*time.Minute)

	// Verify
	if err != nil {
		t.Fatalf("CleanupProofOfWorks failed: %v", err)
	}

	// List remaining artifacts
	artifacts, err := powManager.ListProofOfWorks(ctx, "")

	// Verify
	if err != nil {
		t.Fatalf("ListProofOfWorks after cleanup failed: %v", err)
	}

	// Should still have artifacts (none are older than 5 minutes)
	if len(artifacts) == 0 {
		t.Error("Expected artifacts to remain after cleanup of very old items")
	}
}

func TestProofOfWorkManager_GenerateMarkdown(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)
	// Generate markdown
	// We need to test the private method indirectly through CreateProofOfWork
	result := &ExecutionResult{
		TaskID:        "markdown-test-task",
		SessionID:      "markdown-test-session",
		WorkItemID:    "PROJ-111",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       1 * time.Hour,
		WorkspacePath:   runtimeDir,
		ExecutionSteps:  []*ExecutionStep{},
		SREDEvidence:   []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "markdown-test-task",
		SessionID:   "markdown-test-session",
		WorkItemID:  "PROJ-111",
		Title:       "Markdown Test Task",
		Objective:   "Test markdown generation",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Read markdown file
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	md := string(mdContent)

	// Verify key sections
	if len(md) < 100 {
		t.Error("Markdown should be substantial")
	}

	// Check for required sections
	requiredSections := []string{
		"# Proof of Work",
		"## Summary",
		"## Objective",
		"## Result",
		"## Recommendation",
	}

	for _, section := range requiredSections {
		if !containsString(md, section) {
			t.Errorf("Missing required section: %s", section)
		}
	}
}

func TestProofOfWorkManager_GenerateJSON(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Generate JSON
	result := &ExecutionResult{
		TaskID:        "json-test-task",
		SessionID:      "json-test-session",
		WorkItemID:    "PROJ-222",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       30 * time.Minute,
		WorkspacePath:   runtimeDir,
		ExecutionSteps:  []*ExecutionStep{},
		SREDEvidence:   []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "json-test-task",
		SessionID:   "json-test-session",
		WorkItemID:  "PROJ-222",
		Title:       "JSON Test Task",
		Objective:   "Test JSON generation",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Read JSON file
	jsonContent, err := os.ReadFile(artifact.JSONPath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	// Verify it's valid JSON
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(jsonContent, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify required fields
	requiredFields := []string{
		"task_id",
		"session_id",
		"work_item_id",
		"result",
		"duration",
		"model_used",
		"agent_role",
		"recommended_action",
	}

	for _, field := range requiredFields {
		if _, exists := jsonMap[field]; !exists {
			t.Errorf("Missing required field in JSON: %s", field)
		}
	}

	// Verify task ID
	if taskID, ok := jsonMap["task_id"].(string); ok {
		if taskID != "json-test-task" {
			t.Errorf("task_id mismatch: got %s, want json-test-task", taskID)
		}
	}
}

func TestProofOfWorkManager_FailedTask(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create failed execution result
	result := &ExecutionResult{
		TaskID:        "failed-task",
		SessionID:      "failed-session",
		WorkItemID:    "PROJ-333",
		Status:         ExecutionStatusFailed,
		Success:        false,
		CompletedAt:    time.Now(),
		Duration:       10 * time.Minute,
		WorkspacePath:   "/tmp/workspace",
		Error:          "Step execution failed after retries",
		ErrorCode:      "STEP_EXECUTION_FAILED",
		NeedsRetry:     true,
		Recommendation:  "retry",
		ExecutionSteps:  []*ExecutionStep{},
		SREDEvidence:   []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "failed-task",
		SessionID:   "failed-session",
		WorkItemID:  "PROJ-333",
		Title:       "Failed Task",
		Objective:   "Test failed task proof-of-work",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Read markdown
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	md := string(mdContent)

	// Should show failed status
	if !containsString(md, "❌ **Task failed**") {
		t.Error("Markdown should show failed status for failed task")
	}

	// Verify recommendation is retry
	if !containsString(md, "**Action:** **retry**") {
		t.Error("Recommendation should be retry for failed task")
	}
}

func TestProofOfWorkManager_EvidenceItems(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create result with evidence items
	evidenceItems := []contracts.EvidenceItem{
		{
			ID:          "evidence-1",
			SessionID:   "evidence-session",
			Type:        "hypothesis",
			Content:     "Test hypothesis for SR&ED",
			Metadata:    map[string]string{"experiment": "test-1"},
			CollectedAt: time.Now(),
		},
		{
			ID:          "evidence-2",
			SessionID:   "evidence-session",
			Type:        "observation",
			Content:     "Test observation",
			Metadata:    map[string]string{"experiment": "test-2"},
			CollectedAt: time.Now(),
		},
		{
			ID:          "evidence-3",
			SessionID:   "evidence-session",
			Type:        "experiment",
			Content:     "Test experiment result",
			Metadata:    map[string]string{"experiment": "test-3"},
			CollectedAt: time.Now(),
		},
	}

	result := &ExecutionResult{
		TaskID:        "evidence-task",
		SessionID:      "evidence-session",
		WorkItemID:    "PROJ-444",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       15 * time.Minute,
		WorkspacePath:   "/tmp/workspace",
		SREDEvidence:   evidenceItems,
		ExecutionSteps:  []*ExecutionStep{},
	}

	spec := &FactoryTaskSpec{
		ID:         "evidence-task",
		SessionID:   "evidence-session",
		WorkItemID:  "PROJ-444",
		Title:       "Evidence Task",
		Objective:   "Test evidence collection",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Read JSON to verify evidence items
	jsonContent, err := os.ReadFile(artifact.JSONPath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(jsonContent, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify evidence_items field
	if evidence, exists := jsonMap["evidence_items"]; exists {
		if evidenceSlice, ok := evidence.([]interface{}); ok {
			if len(evidenceSlice) != 3 {
				t.Errorf("Expected 3 evidence items, got %d", len(evidenceSlice))
			}
		}
	}

	// Read markdown to verify evidence section
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	md := string(mdContent)

	if !containsString(md, "## Evidence (SR&ED)") {
		t.Error("Markdown should have Evidence section for SR&ED")
	}

	// Verify evidence items are present
	if !containsString(md, "hypothesis") || !containsString(md, "observation") || !containsString(md, "experiment") {
		t.Error("Markdown should contain all evidence item types")
	}
}

func TestProofOfWorkManager_NoChanges(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create result with no changes
	result := &ExecutionResult{
		TaskID:        "no-changes-task",
		SessionID:      "no-changes-session",
		WorkItemID:    "PROJ-555",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       5 * time.Minute,
		WorkspacePath:   "/tmp/workspace",
		FilesChanged:   []string{},
		TestsRun:       []string{},
		TestsPassed:    true,
		SREDEvidence:   []contracts.EvidenceItem{},
		ExecutionSteps:  []*ExecutionStep{},
	}

	spec := &FactoryTaskSpec{
		ID:         "no-changes-task",
		SessionID:   "no-changes-session",
		WorkItemID:  "PROJ-555",
		Title:       "No Changes Task",
		Objective:   "Test task with no changes",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)

	// Verify
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Read markdown
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Failed to read markdown file: %v", err)
	}

	md := string(mdContent)

	// Should not have Files Changed section if no changes
	if containsString(md, "## Files Changed") {
		t.Error("Markdown should not have Files Changed section when no files changed")
	}

	// Should not have Tests section if no tests
	if containsString(md, "## Tests") {
		t.Error("Markdown should not have Tests section when no tests run")
	}
}

func TestProofOfWorkManager_GenerateJiraComment(t *testing.T) {
	// Setup
	runtimeDir := t.TempDir()
	powManager := NewProofOfWorkManager(runtimeDir)

	// Create proof-of-work artifact
	result := &ExecutionResult{
		TaskID:        "jira-comment-task",
		SessionID:      "jira-comment-session",
		WorkItemID:    "PROJ-999",
		Status:         ExecutionStatusCompleted,
		Success:        true,
		CompletedAt:    time.Now(),
		Duration:       10 * time.Minute,
		WorkspacePath:   runtimeDir,
		FilesChanged:   []string{"test.go"},
		TestsRun:       []string{"test1"},
		TestsPassed:    true,
		Recommendation: "merge",
		ExecutionSteps:  []*ExecutionStep{},
		SREDEvidence:   []contracts.EvidenceItem{},
	}

	spec := &FactoryTaskSpec{
		ID:         "jira-comment-task",
		SessionID:   "jira-comment-session",
		WorkItemID:  "PROJ-999",
		Title:       "Jira Comment Test Task",
		Objective:   "Test Jira comment generation",
	}

	ctx := context.Background()
	artifact, err := powManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		t.Fatalf("CreateProofOfWork failed: %v", err)
	}

	// Generate Jira comment
	comment, err := powManager.GenerateJiraComment(ctx, artifact)
	if err != nil {
		t.Fatalf("GenerateJiraComment failed: %v", err)
	}

	// Verify comment structure
	if comment == nil {
		t.Fatal("comment should not be nil")
	}

	if comment.ID != "jira-comment-task" {
		t.Errorf("Comment ID mismatch: got %s, want jira-comment-task", comment.ID)
	}

	if comment.WorkItemID != "PROJ-999" {
		t.Errorf("Comment WorkItemID mismatch: got %s, want PROJ-999", comment.WorkItemID)
	}

	if comment.Body == "" {
		t.Error("Comment body should not be empty")
	}

	if comment.Author != "zen-brain" {
		t.Errorf("Comment author mismatch: got %s, want zen-brain", comment.Author)
	}

	if comment.Attribution == nil {
		t.Error("Comment attribution should not be nil")
	}

	if comment.Attribution.AgentRole != "factory" {
		t.Errorf("Attribution agent role mismatch: got %s, want factory", comment.Attribution.AgentRole)
	}

	if comment.Attribution.ModelUsed != "factory-v1" {
		t.Errorf("Attribution model used mismatch: got %s, want factory-v1", comment.Attribution.ModelUsed)
	}

	if comment.Attribution.SessionID != "jira-comment-session" {
		t.Errorf("Attribution session ID mismatch: got %s, want jira-comment-session", comment.Attribution.SessionID)
	}

	if comment.Attribution.TaskID != "jira-comment-task" {
		t.Errorf("Attribution task ID mismatch: got %s, want jira-comment-task", comment.Attribution.TaskID)
	}

	// Check that body contains markdown content
	if !containsString(comment.Body, "# Proof of Work") {
		t.Error("Comment body should contain markdown content")
	}

	if !containsString(comment.Body, "Jira Comment Test Task") {
		t.Error("Comment body should contain task title")
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (len(s) >= len(substr)) && func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()
}
