package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/factory"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestBlock4_ExecuteTaskWithProof tests execution and proof-of-work generation
func TestBlock4_ExecuteTaskWithProof(t *testing.T) {
	// Create temporary runtime directory
	tmpDir := t.TempDir()

	// Build factory
	workspaceManager := factory.NewWorkspaceManager(tmpDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(tmpDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		tmpDir,
	)

	// Create task spec
	taskID := "TEST-EXECUTE-001"
	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Title:      "Test task execution",
		Objective:  "Demonstrate factory execution with proof-of-work generation",
		WorkType:   contracts.WorkTypeImplementation,
		Priority:   contracts.PriorityHigh,
		TemplateKey: "default",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()

	// Execute task
	result, err := factoryInst.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}

	// Verify basic result
	if result.TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, result.TaskID)
	}
	if result.SessionID == "" {
		t.Error("Expected SessionID to be set")
	}
	if result.WorkspacePath == "" {
		t.Error("Expected WorkspacePath to be set")
	}

	// Generate proof-of-work
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		t.Fatalf("GenerateProofOfWork: %v", err)
	}

	// Verify proof-of-work
	if proof.TaskID != taskID {
		t.Errorf("Expected proof task ID %s, got %s", taskID, proof.TaskID)
	}
	if proof.WorkspacePath == "" {
		t.Error("Expected WorkspacePath in proof")
	}
	if proof.TemplateKey == "" {
		t.Error("Expected TemplateKey in proof")
	}

	// Verify artifacts were created
	if len(proof.ArtifactPaths) == 0 {
		t.Error("Expected artifact paths to be set")
	}

	// Verify checksums
	if proof.Checksums == nil || len(proof.Checksums) == 0 {
		t.Error("Expected checksums to be generated")
	}
}

// TestBlock4_WorkspaceManagement tests workspace creation and cleanup
func TestBlock4_WorkspaceManagement(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceManager := factory.NewWorkspaceManager(tmpDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(tmpDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		tmpDir,
	)

	taskID := "TEST-WORKSPACE-001"
	sessionID := "session-test"

	ctx := context.Background()

	// Allocate workspace
	workspace, err := factoryInst.AllocateWorkspace(ctx, taskID, sessionID)
	if err != nil {
		t.Fatalf("AllocateWorkspace: %v", err)
	}

	// Verify workspace
	if workspace.TaskID != taskID {
		t.Errorf("Expected task ID %s, got %s", taskID, workspace.TaskID)
	}
	if workspace.Path == "" {
		t.Error("Expected workspace path to be set")
	}
	if !workspace.Initialized {
		t.Error("Expected workspace to be initialized")
	}

	// Verify workspace exists on disk
	if _, err := os.Stat(workspace.Path); os.IsNotExist(err) {
		t.Errorf("Workspace path %s does not exist", workspace.Path)
	}

	// Get workspace metadata
	metadata, err := factoryInst.GetWorkspaceMetadata(ctx, workspace.Path)
	if err != nil {
		t.Fatalf("GetWorkspaceMetadata: %v", err)
	}

	if metadata.TaskID != taskID {
		t.Errorf("Expected metadata task ID %s, got %s", taskID, metadata.TaskID)
	}

	// Cleanup workspace
	if err := factoryInst.CleanupWorkspace(ctx, workspace.Path); err != nil {
		t.Fatalf("CleanupWorkspace: %v", err)
	}

	// Verify workspace is cleaned up
	if _, err := os.Stat(workspace.Path); !os.IsNotExist(err) {
		t.Errorf("Workspace path %s should not exist after cleanup", workspace.Path)
	}
}

// TestBlock4_ProofOfWorkArtifacts tests proof-of-work artifact generation
func TestBlock4_ProofOfWorkArtifacts(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceManager := factory.NewWorkspaceManager(tmpDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(tmpDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		tmpDir,
	)

	taskID := "TEST-PROOF-001"
	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Title:      "Test proof generation",
		Objective:  "Verify proof-of-work artifact generation",
		WorkType:   contracts.WorkTypeImplementation,
		Priority:   contracts.PriorityMedium,
		TemplateKey: "default",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()

	// Execute task
	result, err := factoryInst.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}

	// Generate proof
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		t.Fatalf("GenerateProofOfWork: %v", err)
	}

	// Verify proof has all required fields
	if proof.TaskID == "" {
		t.Error("Expected TaskID to be set")
	}
	if proof.SessionID == "" {
		t.Error("Expected SessionID to be set")
	}
	if proof.WorkItemID == "" {
		t.Error("Expected WorkItemID to be set")
	}
	if proof.WorkspacePath == "" {
		t.Error("Expected WorkspacePath to be set")
	}
	if proof.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}
	if proof.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set")
	}
	if proof.Result == "" {
		t.Error("Expected Result to be set")
	}

	// Verify template metadata
	if proof.TemplateKey == "" {
		t.Error("Expected TemplateKey to be set")
	}

	// Verify artifact paths
	if len(proof.ArtifactPaths) == 0 {
		t.Error("Expected ArtifactPaths to be set")
	}

	// Verify checksums exist
	if proof.Checksums == nil {
		t.Error("Expected Checksums to be initialized")
	}

	// Test JSON serialization
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatalf("JSON marshal: %v", err)
	}
	if len(data) == 0 {
		t.Error("Expected non-empty JSON output")
	}

	// Verify JSON round-trip
	var decoded factory.ProofOfWorkSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if decoded.TaskID != proof.TaskID {
		t.Error("Expected TaskID to survive JSON round-trip")
	}
}

// TestBlock4_TaskListing tests task listing and retrieval
func TestBlock4_TaskListing(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceManager := factory.NewWorkspaceManager(tmpDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(tmpDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		tmpDir,
	)

	ctx := context.Background()

	// Execute multiple tasks
	taskIDs := []string{"LIST-001", "LIST-002", "LIST-003"}
	for _, taskID := range taskIDs {
		spec := &factory.FactoryTaskSpec{
			ID:         taskID,
			SessionID:  "session-" + taskID,
			WorkItemID: "WORK-" + taskID,
			Title:      "Test task " + taskID,
			Objective:  "Test objective",
			WorkType:   contracts.WorkTypeImplementation,
			Priority:   contracts.PriorityMedium,
			TemplateKey: "default",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		_, err := factoryInst.ExecuteTask(ctx, spec)
		if err != nil {
			t.Fatalf("ExecuteTask %s: %v", taskID, err)
		}
	}

	// List all tasks
	tasks, err := factoryInst.ListTasks(ctx)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}

	if len(tasks) < len(taskIDs) {
		t.Errorf("Expected at least %d tasks, got %d", len(taskIDs), len(tasks))
	}

	// Get specific task
	task, err := factoryInst.GetTask(ctx, taskIDs[0])
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}

	if task.ID != taskIDs[0] {
		t.Errorf("Expected task ID %s, got %s", taskIDs[0], task.ID)
	}

	// Verify task has workspace
	if task.WorkspacePath == "" {
		t.Error("Expected task to have workspace path")
	}
}

// TestBlock4_ProofVerification tests proof checksum verification
func TestBlock4_ProofVerification(t *testing.T) {
	tmpDir := t.TempDir()

	proofManager := factory.NewProofOfWorkManager(tmpDir)

	taskID := "TEST-VERIFY-001"

	// Create execution result
	result := &factory.ExecutionResult{
		TaskID:     taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Status:     factory.ExecutionStatusCompleted,
		Success:    true,
		CompletedAt: time.Now(),
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
		TemplateKey: "default",
	}

	// Create workspace
	if err := os.MkdirAll(result.WorkspacePath, 0755); err != nil {
		t.Fatalf("Create workspace: %v", err)
	}

	// Create test file
	testFile := filepath.Join(result.WorkspacePath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Write test file: %v", err)
	}

	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  result.SessionID,
		WorkItemID: result.WorkItemID,
		Title:      "Test verification",
		Objective:  "Verify proof checksums",
		WorkType:   contracts.WorkTypeImplementation,
		Priority:   contracts.PriorityHigh,
		TemplateKey: "default",
	}

	ctx := context.Background()

	// Create proof
	artifact, err := proofManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		t.Fatalf("CreateProofOfWork: %v", err)
	}

	// Verify artifact exists
	if artifact == nil {
		t.Fatal("Expected artifact to be created")
	}

	// Verify files exist
	if _, err := os.Stat(artifact.JSONPath); os.IsNotExist(err) {
		t.Errorf("JSON path %s does not exist", artifact.JSONPath)
	}
	if _, err := os.Stat(artifact.MarkdownPath); os.IsNotExist(err) {
		t.Errorf("Markdown path %s does not exist", artifact.MarkdownPath)
	}
	if _, err := os.Stat(artifact.LogPath); os.IsNotExist(err) {
		t.Errorf("Log path %s does not exist", artifact.LogPath)
	}

	// Verify checksums
	if artifact.Summary.Checksums == nil || len(artifact.Summary.Checksums) == 0 {
		t.Error("Expected checksums to be generated")
	}

	// Verify artifact integrity
	valid, err := proofManager.VerifyArtifact(ctx, artifact)
	if err != nil {
		t.Fatalf("VerifyArtifact: %v", err)
	}
	if !valid {
		t.Error("Expected artifact to be valid")
	}
}

// TestBlock4_WorkspaceIsolation tests workspace isolation
func TestBlock4_WorkspaceIsolation(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceManager := factory.NewWorkspaceManager(tmpDir)

	ctx := context.Background()

	// Create two workspaces
	task1ID := "ISOLATE-001"
	task2ID := "ISOLATE-002"

	ws1, err := workspaceManager.CreateWorkspace(ctx, task1ID, "session-1")
	if err != nil {
		t.Fatalf("CreateWorkspace 1: %v", err)
	}

	ws2, err := workspaceManager.CreateWorkspace(ctx, task2ID, "session-1")
	if err != nil {
		t.Fatalf("CreateWorkspace 2: %v", err)
	}

	// Verify workspaces are different
	if ws1.Path == ws2.Path {
		t.Error("Expected different workspace paths")
	}

	// Verify workspaces are isolated
	if ws1.TaskID == ws2.TaskID {
		t.Error("Expected different task IDs")
	}

	// Create file in workspace 1
	file1 := filepath.Join(ws1.Path, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Write file1: %v", err)
	}

	// Verify file is not in workspace 2
	file2 := filepath.Join(ws2.Path, "file1.txt")
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Error("Expected file1 to NOT exist in workspace 2")
	}
}

// TestBlock4_ProofMetadata tests proof metadata completeness
func TestBlock4_ProofMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceManager := factory.NewWorkspaceManager(tmpDir)
	executor := factory.NewBoundedExecutor()
	proofManager := factory.NewProofOfWorkManager(tmpDir)

	factoryInst := factory.NewFactory(
		workspaceManager,
		executor,
		proofManager,
		tmpDir,
	)

	taskID := "TEST-METADATA-001"
	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Title:      "Test metadata completeness",
		Objective:  "Verify all proof metadata is captured",
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityHigh,
		TemplateKey: "implementation:real",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()

	// Execute task
	result, err := factoryInst.ExecuteTask(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteTask: %v", err)
	}

	// Generate proof
	proof, err := factoryInst.GenerateProofOfWork(ctx, result)
	if err != nil {
		t.Fatalf("GenerateProofOfWork: %v", err)
	}

	// Verify all metadata fields
	if proof.TaskID == "" {
		t.Error("Expected TaskID")
	}
	if proof.SessionID == "" {
		t.Error("Expected SessionID")
	}
	if proof.WorkItemID == "" {
		t.Error("Expected WorkItemID")
	}
	if proof.WorkType == "" {
		t.Error("Expected WorkType")
	}
	if proof.WorkDomain == "" {
		t.Error("Expected WorkDomain")
	}
	if proof.Title == "" {
		t.Error("Expected Title")
	}
	if proof.Objective == "" {
		t.Error("Expected Objective")
	}
	if proof.Result == "" {
		t.Error("Expected Result")
	}
	if proof.WorkspacePath == "" {
		t.Error("Expected WorkspacePath")
	}
	if proof.TemplateKey == "" {
		t.Error("Expected TemplateKey")
	}
	if proof.Version == "" {
		t.Error("Expected Version")
	}
	if proof.StartedAt.IsZero() {
		t.Error("Expected StartedAt")
	}
	if proof.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt")
	}

	// Verify timestamps are reasonable
	if proof.CompletedAt.Before(proof.StartedAt) {
		t.Error("Expected CompletedAt >= StartedAt")
	}

	// Verify duration is set
	if proof.Duration == 0 {
		t.Error("Expected Duration to be non-zero")
	}
}

// TestBlock4_ArtifactPersistence tests that artifacts persist correctly
func TestBlock4_ArtifactPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	proofManager := factory.NewProofOfWorkManager(tmpDir)

	taskID := "PERSIST-001"

	result := &factory.ExecutionResult{
		TaskID:     taskID,
		SessionID:  "session-" + taskID,
		WorkItemID: "WORK-" + taskID,
		Status:     factory.ExecutionStatusCompleted,
		Success:    true,
		CompletedAt: time.Now(),
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
		TemplateKey: "default",
	}

	// Create workspace
	if err := os.MkdirAll(result.WorkspacePath, 0755); err != nil {
		t.Fatalf("Create workspace: %v", err)
	}

	spec := &factory.FactoryTaskSpec{
		ID:         taskID,
		SessionID:  result.SessionID,
		WorkItemID: result.WorkItemID,
		Title:      "Test persistence",
		Objective:  "Verify artifacts persist",
		WorkType:   contracts.WorkTypeImplementation,
		Priority:   contracts.PriorityHigh,
		TemplateKey: "default",
	}

	ctx := context.Background()

	// Create proof
	artifact, err := proofManager.CreateProofOfWork(ctx, result, spec)
	if err != nil {
		t.Fatalf("CreateProofOfWork: %v", err)
	}

	// Verify JSON file exists and is readable
	jsonData, err := os.ReadFile(artifact.JSONPath)
	if err != nil {
		t.Fatalf("Read JSON: %v", err)
	}

	var loadedProof factory.ProofOfWorkSummary
	if err := json.Unmarshal(jsonData, &loadedProof); err != nil {
		t.Fatalf("Unmarshal JSON: %v", err)
	}

	if loadedProof.TaskID != taskID {
		t.Errorf("Expected TaskID %s, got %s", taskID, loadedProof.TaskID)
	}

	// Verify markdown file exists and is readable
	mdData, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("Read Markdown: %v", err)
	}

	if len(mdData) == 0 {
		t.Error("Expected non-empty markdown file")
	}

	// Verify log file exists
	if _, err := os.Stat(artifact.LogPath); os.IsNotExist(err) {
		t.Error("Expected log file to exist")
	}
}
