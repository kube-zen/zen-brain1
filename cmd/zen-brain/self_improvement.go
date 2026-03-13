package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/integration"
	"github.com/kube-zen/zen-brain1/internal/office"
)

func runSelfImprovementCommand() {
	fmt.Println("=== Zen-Brain Self-Improvement Loop ===")
	fmt.Println("Started:", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Initialize action policy
	policy := office.NewActionPolicy(log.Default())

	// Load config
	cfg, _ := config.LoadConfig("")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Initialize office manager
	mgr, err := integration.InitOfficeManagerFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize office manager: %v", err)
	}

	// Create self-improvement worker
	worker := &SelfImprovementWorker{
		WorkerID:    "zb-self-improvement-1",
		Role:        "self-improvement",
		OfficeMgr:    mgr,
		ActionPolicy: policy,
	}

	// Run one iteration of the loop
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := worker.RunIteration(ctx); err != nil {
		log.Fatalf("Self-improvement iteration failed: %v", err)
	}

	fmt.Println()
	fmt.Println("=== Self-Improvement Complete ===")
	fmt.Println("Finished:", time.Now().Format(time.RFC3339))
}

// SelfImprovementWorker executes safe self-improvement tasks
type SelfImprovementWorker struct {
	WorkerID    string
	Role        string
	OfficeMgr    *office.Manager
	ActionPolicy *office.ActionPolicy
}

// RunIteration executes one iteration of the self-improvement loop
func (w *SelfImprovementWorker) RunIteration(ctx context.Context) error {
	// Step 1: Discover eligible tasks
	fmt.Println("[1/7] Discovering eligible self-improvement tasks...")
	tasks, err := w.discoverTasks(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("  No eligible tasks found")
		return nil
	}

	fmt.Printf("  Found %d eligible task(s)\n", len(tasks))

	// Step 2: Claim one task
	fmt.Println("[2/7] Claiming task...")
	task, err := w.claimTask(ctx, tasks)
	if err != nil {
		return fmt.Errorf("failed to claim task: %w", err)
	}

	if task == nil {
		fmt.Println("  No task available to claim")
		return nil
	}

	fmt.Printf("  Claimed: %s (worker: %s)\n", task.ID, w.WorkerID)

	// Step 3: Analyze/classify task
	fmt.Println("[3/7] Analyzing and classifying task...")
	action, err := w.classifyTask(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to classify task: %w", err)
	}

	fmt.Printf("  Task ID: %s\n", action.ID)
	fmt.Printf("  Action Type: %s\n", action.Type)
	fmt.Printf("  Action Class: %s\n", action.Class)
	fmt.Printf("  Risk Level: %s\n", action.RiskLevel)

	// Step 4: Check action policy
	fmt.Println("[4/7] Checking action policy...")
	if !w.ActionPolicy.CanExecute(action) {
		fmt.Printf("  Action not allowed: %s (requires approval)\n", action.ID)
		return w.escalateTask(ctx, task, action)
	}

	// Step 5: Execute only allowed actions
	fmt.Println("[5/7] Executing allowed action...")
	result, err := w.executeAction(ctx, task, action)
	if err != nil {
		return fmt.Errorf("failed to execute action: %w", err)
	}

	fmt.Printf("  Execution complete: %s\n", result.Status)

	// Step 6: Generate proof/report/comment
	fmt.Println("[6/7] Generating proof and report...")
	if err := w.generateReport(ctx, task, action, result); err != nil {
		log.Printf("Warning: failed to generate report: %v", err)
	}

	// Step 7: Release/complete/escalate
	fmt.Println("[7/7] Completing task...")
	if err := w.completeTask(ctx, task, action, result); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	fmt.Printf("  Task completed: %s\n", task.ID)
	return nil
}

// discoverTasks finds eligible self-improvement tasks
func (w *SelfImprovementWorker) discoverTasks(ctx context.Context) ([]*SelfImprovementTask, error) {
	// For now, return hardcoded safe tasks
	// TODO: Query Jira with label "self-improvement" or similar
	return []*SelfImprovementTask{
		{
			ID:          "SI-001",
			Title:       "Improve runtime doctor output clarity",
			Description:  "Make runtime doctor output more human-readable and actionable",
			Priority:    "medium",
			Type:        "improvement",
			ActionClass:  "A", // Always allowed
			RiskLevel:   "none",
		},
		{
			ID:          "SI-002",
			Title:       "Format proof-of-work artifacts consistently",
			Description:  "Ensure all proof artifacts follow the same schema and structure",
			Priority:    "low",
			Type:        "improvement",
			ActionClass:  "B", // Safe write-back
			RiskLevel:   "low",
		},
		{
			ID:          "SI-003",
			Title:       "Hunt for TODO/FIXME comments in core paths",
			Description:  "Find and classify TODO/FIXME comments in internal/ and pkg/ directories",
			Priority:    "medium",
			Type:        "hunting",
			ActionClass:  "A", // Always allowed
			RiskLevel:   "none",
		},
	}, nil
}

// claimTask claims one task with lease ownership
func (w *SelfImprovementWorker) claimTask(ctx context.Context, tasks []*SelfImprovementTask) (*SelfImprovementTask, error) {
	// Simple round-robin for now
	// TODO: Implement proper claim/lease with Jira label or field update
	if len(tasks) == 0 {
		return nil, nil
	}

	// Claim first available task
	task := tasks[0]
	task.WorkerID = w.WorkerID
	task.ClaimedAt = time.Now()
	task.LeaseExpires = time.Now().Add(30 * time.Minute) // 30-minute lease

	return task, nil
}

// classifyTask analyzes the task and creates an action with classification
func (w *SelfImprovementWorker) classifyTask(ctx context.Context, task *SelfImprovementTask) (*office.Action, error) {
	// Map task to action class
	actionClass := office.ActionClassAlwaysAllowed
	switch task.ActionClass {
	case "A":
		actionClass = office.ActionClassAlwaysAllowed
	case "B":
		actionClass = office.ActionClassSafeWriteBack
	case "C":
		actionClass = office.ActionClassApprovalRequired
	}

	// Create action
	action := &office.Action{
		ID:           fmt.Sprintf("action-%s-%d", task.ID, time.Now().Unix()),
		Type:         "self_improvement",
		Class:        actionClass,
		Description:  task.Description,
		RiskLevel:    task.RiskLevel,
		BusinessImpact: "none",
	}

	return action, nil
}

// executeAction executes only allowed actions for the task
func (w *SelfImprovementWorker) executeAction(ctx context.Context, task *SelfImprovementTask, action *office.Action) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  action.ID,
		WorkerID:  w.WorkerID,
		TaskID:    task.ID,
		Status:     "completed",
		StartedAt:  time.Now(),
	}

	// Execute based on action class
	switch action.Class {
	case office.ActionClassAlwaysAllowed:
		// Class A: Read/analyze/recommend
		result.Message = "Analyzed task, generated recommendations"
		result.Output = generateRecommendation(task)

	case office.ActionClassSafeWriteBack:
		// Class B: Safe write-back to Jira
		result.Message = "Generated safe Jira comment/attachment"
		result.Output = generateSafeWriteback(task)

	case office.ActionClassApprovalRequired:
		// Class C: Not allowed without approval
		return nil, fmt.Errorf("class C actions require explicit approval")
	}

	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)
	return result, nil
}

// generateReport creates proof/report for the task execution
func (w *SelfImprovementWorker) generateReport(ctx context.Context, task *SelfImprovementTask, action *office.Action, result *ActionResult) error {
	// TODO: Upload proof artifact to Jira
	// TODO: Post comment to Jira with worker identity and action class
	log.Printf("[Report] Generated proof for task %s by worker %s (action class: %s)",
		task.ID, w.WorkerID, action.Class)
	return nil
}

// escalateTask escalates a task that requires approval
func (w *SelfImprovementWorker) escalateTask(ctx context.Context, task *SelfImprovementTask, action *office.Action) error {
	log.Printf("[Escalate] Task %s requires approval (class: %s, worker: %s)",
		task.ID, action.Class, w.WorkerID)
	// TODO: Add Jira label "approval-required" and comment
	return nil
}

// completeTask marks the task as complete and releases ownership
func (w *SelfImprovementWorker) completeTask(ctx context.Context, task *SelfImprovementTask, action *office.Action, result *ActionResult) error {
	// TODO: Update Jira status or label
	// TODO: Post final comment with worker identity and action class
	log.Printf("[Complete] Task %s completed by worker %s (duration: %v)",
		task.ID, w.WorkerID, result.Duration)
	return nil
}

// SelfImprovementTask represents a self-improvement task
type SelfImprovementTask struct {
	ID           string
	Title        string
	Description  string
	Priority     string
	Type         string
	ActionClass  string // A, B, or C
	RiskLevel    string // none, low, medium, high
	WorkerID     string
	ClaimedAt    time.Time
	LeaseExpires time.Time
}

// ActionResult represents the result of executing an action
type ActionResult struct {
	ActionID    string
	WorkerID    string
	TaskID      string
	Status      string
	Message     string
	Output      string
	StartedAt   time.Time
	FinishedAt  time.Time
	Duration    time.Duration
}

// generateRecommendation generates recommendations for a task
func generateRecommendation(task *SelfImprovementTask) string {
	return fmt.Sprintf("Recommendations for %s:\n- Review current implementation in internal/runtime/\n- Add clearer section headers\n- Include actionable next steps\n- Format output as table for easier scanning", task.Title)
}

// generateSafeWriteback generates safe Jira content for a task
func generateSafeWriteback(task *SelfImprovementTask) string {
	return fmt.Sprintf("Analysis completed for %s\n\nFindings:\n- Current proof format has inconsistent schema\n- Recommend standardizing to proof-of-work.json\n- Update proof formatter in internal/factory/\n\nWorker: %s\nAction Class: B (Safe Write-Back)\nRisk Level: %s", task.Title, "zb-self-improvement-1", task.RiskLevel)
}
