package factory

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestFactoryImpl_ExecuteTask(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())
	executor := NewBoundedExecutor()
	powGen := NewSimpleProofOfWorkGenerator()
	factory := NewFactory(workspaceManager, executor, powGen)

	// Create task spec
	spec := &FactoryTaskSpec{
		ID:         "test-task-1",
		SessionID:   "test-session-1",
		WorkItemID:  "PROJ-123",
		Title:       "Test Task",
		Objective:   "Execute test objective",
		WorkType:    contracts.WorkTypeImplementation,
		WorkDomain:  contracts.DomainFactory,
		Priority:    contracts.PriorityMedium,
		TimeoutSeconds: 300,
		MaxRetries:    2,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Execute
	ctx := context.Background()
	result, err := factory.ExecuteTask(ctx, spec)

	// Verify
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.TaskID != spec.ID {
		t.Errorf("TaskID mismatch: got %s, want %s", result.TaskID, spec.ID)
	}

	if result.SessionID != spec.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", result.SessionID, spec.SessionID)
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Status mismatch: got %s, want %s", result.Status, ExecutionStatusCompleted)
	}

	if !result.Success {
		t.Errorf("Success should be true")
	}

	if result.WorkspacePath == "" {
		t.Error("WorkspacePath should not be empty")
	}

	if result.CompletedAt.IsZero() {
		t.Error("CompletedAt should not be zero")
	}

	if result.Duration == 0 {
		t.Error("Duration should be non-zero")
	}
}

func TestFactoryImpl_AllocateWorkspace(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())
	executor := NewBoundedExecutor()
	powGen := NewSimpleProofOfWorkGenerator()
	factory := NewFactory(workspaceManager, executor, powGen)

	// Allocate
	ctx := context.Background()
	metadata, err := factory.AllocateWorkspace(ctx, "test-task", "test-session")

	// Verify
	if err != nil {
		t.Fatalf("AllocateWorkspace failed: %v", err)
	}

	if metadata == nil {
		t.Fatal("metadata should not be nil")
	}

	if metadata.TaskID != "test-task" {
		t.Errorf("TaskID mismatch: got %s, want test-task", metadata.TaskID)
	}

	if metadata.SessionID != "test-session" {
		t.Errorf("SessionID mismatch: got %s, want test-session", metadata.SessionID)
	}

	if metadata.Path == "" {
		t.Error("Path should not be empty")
	}
}

func TestFactoryImpl_ListTasks(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())
	executor := NewBoundedExecutor()
	powGen := NewSimpleProofOfWorkGenerator()
	factory := NewFactory(workspaceManager, executor, powGen)

	// Add tasks
	spec1 := &FactoryTaskSpec{
		ID:        "task-1",
		SessionID:  "session-1",
		Title:      "Task 1",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	spec2 := &FactoryTaskSpec{
		ID:        "task-2",
		SessionID:  "session-2",
		Title:      "Task 2",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	_, _ = factory.ExecuteTask(ctx, spec1)
	_, _ = factory.ExecuteTask(ctx, spec2)

	// List
	tasks, err := factory.ListTasks(ctx)

	// Verify
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Task count mismatch: got %d, want 2", len(tasks))
	}
}

func TestFactoryImpl_GetTask(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())
	executor := NewBoundedExecutor()
	powGen := NewSimpleProofOfWorkGenerator()
	factory := NewFactory(workspaceManager, executor, powGen)

	// Add task
	spec := &FactoryTaskSpec{
		ID:        "get-test-task",
		SessionID:  "get-session",
		Title:      "Get Test Task",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	_, _ = factory.ExecuteTask(ctx, spec)

	// Get
	task, err := factory.GetTask(ctx, "get-test-task")

	// Verify
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if task == nil {
		t.Fatal("task should not be nil")
	}

	if task.ID != "get-test-task" {
		t.Errorf("ID mismatch: got %s, want get-test-task", task.ID)
	}

	if task.Title != "Get Test Task" {
		t.Errorf("Title mismatch: got %s, want Get Test Task", task.Title)
	}
}

func TestFactoryImpl_CancelTask(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())
	executor := NewBoundedExecutor()
	powGen := NewSimpleProofOfWorkGenerator()
	factory := NewFactory(workspaceManager, executor, powGen)

	// Add task
	spec := &FactoryTaskSpec{
		ID:        "cancel-test-task",
		SessionID:  "cancel-session",
		Title:      "Cancel Test Task",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	_, _ = factory.ExecuteTask(ctx, spec)

	// Cancel
	err := factory.CancelTask(ctx, "cancel-test-task")

	// Verify
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	// Verify task still exists (just marked as canceled)
	task, err := factory.GetTask(ctx, "cancel-test-task")
	if err != nil {
		t.Fatalf("GetTask after cancel failed: %v", err)
	}

	if task.ID != "cancel-test-task" {
		t.Error("Task should still exist after cancel")
	}
}

func TestWorkspaceManagerImpl_CreateWorkspace(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())

	// Create
	ctx := context.Background()
	metadata, err := workspaceManager.CreateWorkspace(ctx, "task-1", "session-1")

	// Verify
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	if metadata == nil {
		t.Fatal("metadata should not be nil")
	}

	if metadata.TaskID != "task-1" {
		t.Errorf("TaskID mismatch: got %s, want task-1", metadata.TaskID)
	}

	if metadata.SessionID != "session-1" {
		t.Errorf("SessionID mismatch: got %s, want session-1", metadata.SessionID)
	}

	if metadata.Path == "" {
		t.Error("Path should not be empty")
	}
}

func TestWorkspaceManagerImpl_LockUnlockWorkspace(t *testing.T) {
	// Setup
	workspaceManager := NewWorkspaceManager(t.TempDir())

	// Create workspace
	ctx := context.Background()
	metadata, _ := workspaceManager.CreateWorkspace(ctx, "task-1", "session-1")

	// Lock
	err := workspaceManager.LockWorkspace(ctx, metadata.Path)
	if err != nil {
		t.Fatalf("LockWorkspace failed: %v", err)
	}

	// Verify lock
	lockedMetadata, err := workspaceManager.GetWorkspaceMetadata(ctx, metadata.Path)
	if err != nil {
		t.Fatalf("GetWorkspaceMetadata failed: %v", err)
	}

	if !lockedMetadata.Locked {
		t.Error("workspace should be locked")
	}

	// Unlock
	err = workspaceManager.UnlockWorkspace(ctx, metadata.Path)
	if err != nil {
		t.Fatalf("UnlockWorkspace failed: %v", err)
	}

	// Verify unlock
	unlockedMetadata, err := workspaceManager.GetWorkspaceMetadata(ctx, metadata.Path)
	if err != nil {
		t.Fatalf("GetWorkspaceMetadata after unlock failed: %v", err)
	}

	if unlockedMetadata.Locked {
		t.Error("workspace should be unlocked")
	}
}

func TestBoundedExecutor_ExecuteStep(t *testing.T) {
	// Setup
	executor := NewBoundedExecutor()

	// Create step
	step := &ExecutionStep{
		StepID:      "step-1",
		TaskID:      "task-1",
		Name:        "Test Step",
		Description: "Execute test step",
		Status:      StepStatusPending,
		TimeoutSeconds: 10,
		MaxRetries:    0,
	}

	// Execute
	ctx := context.Background()
	result, err := executor.ExecuteStep(ctx, step, "/tmp")

	// Verify
	if err != nil {
		t.Fatalf("ExecuteStep failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.Status != StepStatusCompleted {
		t.Errorf("Status mismatch: got %s, want %s", result.Status, StepStatusCompleted)
	}

	if result.StepID != "step-1" {
		t.Errorf("StepID mismatch: got %s, want step-1", result.StepID)
	}

	if result.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}

	if result.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestBoundedExecutor_ExecutePlan(t *testing.T) {
	// Setup
	executor := NewBoundedExecutor()

	// Create steps
	steps := []*ExecutionStep{
		{
			StepID:      "step-1",
			TaskID:      "task-1",
			Name:        "Step 1",
			Description: "Execute step 1",
			Status:      StepStatusPending,
			TimeoutSeconds: 10,
			MaxRetries:    0,
		},
		{
			StepID:      "step-2",
			TaskID:      "task-1",
			Name:        "Step 2",
			Description: "Execute step 2",
			Status:      StepStatusPending,
			TimeoutSeconds: 10,
			MaxRetries:    0,
		},
	}

	// Execute
	ctx := context.Background()
	result, err := executor.ExecutePlan(ctx, steps, "/tmp")

	// Verify
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if result.Status != ExecutionStatusCompleted {
		t.Errorf("Status mismatch: got %s, want %s", result.Status, ExecutionStatusCompleted)
	}

	if result.TotalSteps != 2 {
		t.Errorf("TotalSteps mismatch: got %d, want 2", result.TotalSteps)
	}

	if result.CompletedSteps != 2 {
		t.Errorf("CompletedSteps mismatch: got %d, want 2", result.CompletedSteps)
	}

	if result.Success != true {
		t.Error("Success should be true")
	}
}

func TestSimpleProofOfWorkGenerator_Generate(t *testing.T) {
	// Setup
	powGen := NewSimpleProofOfWorkGenerator()

	// Create result
	result := &ExecutionResult{
		TaskID:       "task-1",
		SessionID:    "session-1",
		WorkItemID:   "PROJ-123",
		Status:       ExecutionStatusCompleted,
		Success:      true,
		CompletedAt:  time.Now(),
		Duration:     5 * time.Minute,
		FilesChanged: []string{"file1.go", "file2.go"},
		TestsRun:     []string{"test1", "test2"},
		TestsPassed:  true,
		SREDEvidence: []contracts.EvidenceItem{},
		Recommendation: "merge",
	}

	// Generate
	ctx := context.Background()
	proof, err := powGen.Generate(ctx, result)

	// Verify
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if proof == nil {
		t.Fatal("proof should not be nil")
	}

	if proof.TaskID != "task-1" {
		t.Errorf("TaskID mismatch: got %s, want task-1", proof.TaskID)
	}

	if proof.SessionID != "session-1" {
		t.Errorf("SessionID mismatch: got %s, want session-1", proof.SessionID)
	}

	if proof.Result != string(ExecutionStatusCompleted) {
		t.Errorf("Result mismatch: got %s, want %s", proof.Result, ExecutionStatusCompleted)
	}

	if proof.RecommendedAction != "merge" {
		t.Errorf("RecommendedAction mismatch: got %s, want merge", proof.RecommendedAction)
	}

	if proof.RequiresApproval {
		t.Error("Should not require approval for merge recommendation")
	}
}

func TestSimpleProofOfWorkGenerator_SerializeToMarkdown(t *testing.T) {
	// Setup
	powGen := NewSimpleProofOfWorkGenerator()

	// Create proof
	proof := &ProofOfWorkSummary{
		TaskID:        "task-1",
		SessionID:      "session-1",
		WorkItemID:    "PROJ-123",
		Title:          "Test Task",
		Objective:      "Test objective",
		Result:         "completed",
		StartedAt:      time.Now(),
		CompletedAt:    time.Now(),
		Duration:       5 * time.Minute,
		ModelUsed:      "model-v1",
		AgentRole:      "factory",
		FilesChanged:   []string{"file1.go"},
		TestsRun:       []string{"test1"},
		TestsPassed:    true,
		RecommendedAction: "merge",
		RequiresApproval: false,
		GeneratedAt:    time.Now(),
	}

	// Serialize
	md, err := powGen.SerializeToMarkdown(proof)

	// Verify
	if err != nil {
		t.Fatalf("SerializeToMarkdown failed: %v", err)
	}

	if md == "" {
		t.Fatal("markdown should not be empty")
	}

	// Check for key sections
	if len(md) < 50 {
		t.Error("markdown should contain meaningful content")
	}

	// Check for task ID presence
	// In real tests, we'd parse the markdown more thoroughly
}
