package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func runSelfImprovementCommand() {
	fmt.Println("=== Zen-Brain Self-Improvement Loop ===")
	fmt.Println("Started:", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Initialize action policy
	policy := office.NewActionPolicy(log.Default())

	// Initialize office manager (same fallback pattern as office.go)
	mgr, err := getOfficeManager()
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

	// Track metrics for morning report
	metrics := &NightShiftMetrics{
		StartTime: time.Now(),
	}

	// Track processed tasks to avoid duplicates
	processedTasks := make(map[string]bool)

	// Run continuous loop for 45-60 minutes (Phase 1)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	fmt.Println("[Self-Improvement] Starting continuous loop (60-minute timeout)...")

	// Loop until timeout or no more tasks
	iterationCount := 0
	for {
		iterationCount++
		fmt.Printf("[Self-Improvement] Iteration %d started...\n", iterationCount)

		if err := worker.RunIteration(ctx, metrics, processedTasks); err != nil {
			log.Printf("Self-improvement iteration failed: %v", err)
			break
		}

		// Check if context is cancelled (timeout)
		select {
		case <-ctx.Done():
			fmt.Println("[Self-Improvement] Timeout reached, stopping loop...")
			break
		default:
			// Continue to next iteration
		}

		// Small delay between iterations to avoid rapid-fire processing
		time.Sleep(5 * time.Second)
	}

	// Generate morning report
	fmt.Println()
	fmt.Println("=== Morning Report ===")
	generateMorningReport(metrics)

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

// RunIteration executes one iteration of self-improvement loop
func (w *SelfImprovementWorker) RunIteration(ctx context.Context, metrics *NightShiftMetrics, processedTasks map[string]bool) error {
	// Step 1: Discover eligible tasks
	fmt.Println("[1/7] Discovering eligible self-improvement tasks...")
	tasks, err := w.discoverTasks(ctx, processedTasks)
	if err != nil {
		metrics.TasksDiscovered = 0
		return fmt.Errorf("failed to discover tasks: %w", err)
	}

	metrics.TasksDiscovered = len(tasks)

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

	metrics.TasksClaimed++
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
		metrics.TasksEscalated++
		metrics.EscalatedTasks = append(metrics.EscalatedTasks, task)
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
	metrics.TasksProcessed++
	metrics.ProcessedTasks = append(metrics.ProcessedTasks, task)

	// Mark task as processed to avoid re-processing in next iteration
	processedTasks[task.ID] = true

	return nil
}

// discoverTasks finds eligible self-improvement tasks from Jira
func (w *SelfImprovementWorker) discoverTasks(ctx context.Context, processedTasks map[string]bool) ([]*SelfImprovementTask, error) {
	// Jira discovery query for safe Night Shift work
	// Filter: zen-brain1 backlog, small, non-critical, self-contained, unblocked, not claimed
	jql := `project = ZB AND labels = "zen-brain-nightshift" AND status NOT IN ("Done", "Closed", "Blocked") AND priority IN ("Low", "Medium") ORDER BY created ASC`

	// Search Jira
	items, err := w.OfficeMgr.Search(ctx, "default", jql)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jira for nightshift tasks: %w", err)
	}

	// Convert to self-improvement tasks
	var tasks []*SelfImprovementTask
	for _, item := range items {
		// Check if already claimed (by checking for worker-claim label in tags)
		claimed := false
		for _, tag := range item.Tags.Policy {
			if tag == "worker-claimed" {
				claimed = true
				break
			}
		}
		if claimed {
			continue // Skip claimed tasks
		}

		// Skip if already processed
		if processedTasks[item.ID] {
			continue
		}

		// Determine action class based on task content
		actionClass := "A"
		riskLevel := "none"

		// Class C tasks (approval required) - SKIP these
		title := strings.ToLower(item.Title)
		body := strings.ToLower(item.Body)

		if strings.Contains(title, "deploy") ||
			strings.Contains(title, "merge") ||
			strings.Contains(title, "secret") ||
			strings.Contains(title, "infra") ||
			strings.Contains(body, "kubernetes") ||
			strings.Contains(body, "cloud") {
			actionClass = "C"
			riskLevel = "medium"
			continue // Skip Class C tasks for now
		}

		// Class B tasks (safe write-back)
		if strings.Contains(title, "format") ||
			strings.Contains(title, "report") ||
			strings.Contains(title, "comment") ||
			strings.Contains(body, "jira") {
			actionClass = "B"
			riskLevel = "low"
		}

		// Map priority
		priority := "medium"
		if item.Priority == contracts.PriorityLow {
			priority = "low"
		} else if item.Priority == contracts.PriorityMedium {
			priority = "medium"
		}

		tasks = append(tasks, &SelfImprovementTask{
			ID:          item.ID,
			Title:       item.Title,
			Description:  item.Body, // WorkItem has Body, not Description
			Priority:    priority,
			Type:         "nightshift",
			ActionClass:  actionClass,
			RiskLevel:    riskLevel,
		})
	}

	if len(tasks) == 0 {
		fmt.Println("  No eligible tasks found matching nightshift criteria")
		fmt.Println("  Expected JQL:", jql)
		fmt.Println("  Note: Create Zen-Brain tickets with label 'zen-brain-nightshift'")
	}

	return tasks, nil
}

// claimTask claims one task with lease ownership
func (w *SelfImprovementWorker) claimTask(ctx context.Context, tasks []*SelfImprovementTask) (*SelfImprovementTask, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	// Claim first available task
	task := tasks[0]
	task.WorkerID = w.WorkerID
	task.ClaimedAt = time.Now()
	task.LeaseExpires = time.Now().Add(30 * time.Minute) // 30-minute lease

	// TODO: Add Jira label "worker-claimed" when Jira write-back is implemented
	// For now, tracking is in-memory only

	return task, nil
}

// classifyTask analyzes task and creates an action with classification
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

// executeAction executes only allowed actions for task
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

// generateReport creates proof/report for task execution
func (w *SelfImprovementWorker) generateReport(ctx context.Context, task *SelfImprovementTask, action *office.Action, result *ActionResult) error {
	// TODO: Upload proof artifact to Jira when write-back is implemented
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

// completeTask marks task as complete and releases ownership
func (w *SelfImprovementWorker) completeTask(ctx context.Context, task *SelfImprovementTask, action *office.Action, result *ActionResult) error {
	// TODO: Update Jira status or label
	// TODO: Post final comment with worker identity and action class
	log.Printf("[Complete] Task %s completed by worker %s (duration: %v)",
		task.ID, w.WorkerID, result.Duration)
	return nil
}

// generateMorningReport creates a concise morning summary
func generateMorningReport(metrics *NightShiftMetrics) {
	duration := time.Since(metrics.StartTime)

	fmt.Println("Worker: zb-self-improvement-1")
	fmt.Println("Duration:", duration.Round(time.Second))
	fmt.Println()
	fmt.Printf("Tasks Discovered: %d\n", metrics.TasksDiscovered)
	fmt.Printf("Tasks Claimed:   %d\n", metrics.TasksClaimed)
	fmt.Printf("Tasks Processed: %d\n", metrics.TasksProcessed)
	fmt.Printf("Tasks Escalated: %d\n", metrics.TasksEscalated)
	fmt.Println()

	if metrics.TasksProcessed > 0 {
		fmt.Println("Successfully processed:")
		for _, task := range metrics.ProcessedTasks {
			fmt.Printf("  - %s (%s)\n", task.ID, task.Title)
		}
	}

	if metrics.TasksEscalated > 0 {
		fmt.Println("Escalated for approval:")
		for _, task := range metrics.EscalatedTasks {
			fmt.Printf("  - %s (Class C)\n", task.ID)
		}
	}

	if metrics.TasksDiscovered == 0 {
		fmt.Println("No eligible tasks found.")
		fmt.Println("Recommendation: Create Zen-Brain tickets with label 'zen-brain-nightshift'")
	}
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

// NightShiftMetrics tracks night shift performance
type NightShiftMetrics struct {
	StartTime       time.Time
	TasksDiscovered int
	TasksClaimed   int
	TasksProcessed  int
	TasksEscalated int
	ProcessedTasks  []*SelfImprovementTask
	EscalatedTasks  []*SelfImprovementTask
}

// ActionResult represents result of executing an action
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
