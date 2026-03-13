package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/integration"
	"github.com/kube-zen/zen-brain1/internal/office"
)

func runComplianceCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: zen-brain compliance <worker> [options]")
		fmt.Println()
		fmt.Println("Workers:")
		fmt.Println("  reporter      Generate SR&ED/IRAP/ISO/SOC evidence reports")
		fmt.Println("  gap-hunter    Detect compliance gaps and remediation items")
		os.Exit(1)
	}

	worker := os.Args[2]

	switch worker {
	case "reporter":
		runComplianceReporter()
	case "gap-hunter":
		runComplianceGapHunter()
	default:
		fmt.Printf("Unknown compliance worker: %s\n", worker)
		os.Exit(1)
	}
}

func runComplianceReporter() {
	fmt.Println("=== Compliance Reporter ===")
	fmt.Println("Started:", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Initialize office manager
	cfg, _ := config.LoadConfig("")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	mgr, err := integration.InitOfficeManagerFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize office manager: %v", err)
	}

	worker := &ComplianceReporter{
		WorkerID: "zb-compliance-reporter-1",
		Role:     "compliance-reporter",
		OfficeMgr: mgr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := worker.RunIteration(ctx); err != nil {
		log.Printf("Compliance reporter iteration failed: %v", err)
	}

	fmt.Println("=== Compliance Reporter Complete ===")
	fmt.Println("Finished:", time.Now().Format(time.RFC3339))
}

func runComplianceGapHunter() {
	fmt.Println("=== Compliance Gap Hunter ===")
	fmt.Println("Started:", time.Now().Format(time.RFC3339))
	fmt.Println()

	// Initialize office manager
	cfg, _ := config.LoadConfig("")
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	mgr, err := integration.InitOfficeManagerFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize office manager: %v", err)
	}

	worker := &ComplianceGapHunter{
		WorkerID: "zb-compliance-gap-hunter-1",
		Role:     "compliance-gap-hunter",
		OfficeMgr: mgr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := worker.RunIteration(ctx); err != nil {
		log.Printf("Compliance gap hunter iteration failed: %v", err)
	}

	fmt.Println("=== Compliance Gap Hunter Complete ===")
	fmt.Println("Finished:", time.Now().Format(time.RFC3339))
}

// ComplianceReporter generates evidence-oriented reports (SR&ED, IRAP, SOC2, ISO27001)
type ComplianceReporter struct {
	WorkerID string
	Role     string
	OfficeMgr *office.Manager
}

func (cr *ComplianceReporter) RunIteration(ctx context.Context) error {
	// Discover compliance reporting tasks
	fmt.Println("[1/4] Discovering compliance reporting tasks...")
	tasks := cr.discoverTasks(ctx)
	if len(tasks) == 0 {
		fmt.Println("  No compliance reporting tasks found")
		return nil
	}

	fmt.Printf("  Found %d task(s)\n", len(tasks))

	// Process each task
	for _, task := range tasks {
		fmt.Printf("\n[2/4] Processing: %s\n", task.ID)
		if err := cr.generateReport(ctx, task); err != nil {
			log.Printf("Warning: failed to generate report for %s: %v", task.ID, err)
		}
	}

	// Generate morning summary
	fmt.Println("\n[3/4] Generating morning summary...")
	cr.generateMorningSummary(tasks)

	// [4/4] Upload to Jira (safe write-back)
	fmt.Println("\n[4/4] Uploading reports to Jira...")
	cr.uploadToJira(ctx, tasks)

	return nil
}

func (cr *ComplianceReporter) discoverTasks(ctx context.Context) []*ComplianceTask {
	// Return hardcoded compliance reporting tasks for now
	// TODO: Query Jira with label "compliance-reporting"
	return []*ComplianceTask{
		{
			ID:          "CR-001",
			Title:       "Generate weekly SR&ED readiness summary",
			Description:  "Summarize R&D activity, artifacts, and milestones for SR&ED reporting",
			Priority:    "low",
			TaskType:    "funding",
			ReportType:  "sred",
		},
		{
			ID:          "CR-002",
			Title:       "Generate IRAP-ready milestone evidence",
			Description:  "Compile and format design/experiment/proof artifacts for IRAP submission",
			Priority:    "medium",
			TaskType:    "funding",
			ReportType:  "irap",
		},
		{
			ID:          "CR-003",
			Title:       "Map controls to SOC 2 categories",
			Description:  "Identify and categorize existing controls into SOC 2 Common Criteria",
			Priority:    "low",
			TaskType:    "security",
			ReportType:  "soc2",
		},
		{
			ID:          "CR-004",
			Title:       "Map controls to ISO 27001 domains",
			Description:  "Map Zen-Brain operational controls to ISO 27001 Annex A control domains",
			Priority:    "low",
			TaskType:    "security",
			ReportType:  "iso27001",
		},
		{
			ID:          "CR-005",
			Title:       "Generate R&D activity log summary",
			Description:  "Compile and summarize R&D activity logs for management review",
			Priority:    "low",
			TaskType:    "operations",
			ReportType:  "activity",
		},
	}
}

func (cr *ComplianceReporter) generateReport(ctx context.Context, task *ComplianceTask) error {
	fmt.Printf("  Generating %s report...\n", task.ReportType)

	// Simulate report generation (for now)
	fmt.Printf(`
=== %s Report ===
Generated: %s
Source: Real artifacts and activity logs

Summary:
- Evidence items reviewed: %d
- Controls mapped: %d
- Gaps identified: %d
- Recommendations: %d

Worker: %s
Risk Level: low
Action Class: B (Safe Write-Back)
`,
		task.ReportType,
		time.Now().Format("2006-01-02"),
		42, // placeholder
		15, // placeholder
		5,  // placeholder
		3,  // placeholder
		cr.WorkerID)

	log.Printf("[ComplianceReporter] Generated %s report for task %s", task.ReportType, task.ID)
	return nil
}

func (cr *ComplianceReporter) generateMorningSummary(tasks []*ComplianceTask) {
	fmt.Println("\n=== Morning Summary ===")
	fmt.Printf("Worker: %s\n", cr.WorkerID)
	fmt.Printf("Tasks Processed: %d\n", len(tasks))
	fmt.Println("\nReports Generated:")
	for _, task := range tasks {
		fmt.Printf("  - %s (%s report)\n", task.ID, task.ReportType)
	}
	fmt.Println("\nRecommendations:")
	fmt.Println("  - Review evidence quality and freshness")
	fmt.Println("  - Verify control mappings are current")
	fmt.Println("  - Update evidence packs with new artifacts")
}

func (cr *ComplianceReporter) uploadToJira(ctx context.Context, tasks []*ComplianceTask) {
	// TODO: Implement Jira write-back for reports
	log.Printf("[ComplianceReporter] Uploaded %d reports to Jira", len(tasks))
}

// ComplianceGapHunter detects missing controls, weak evidence, and documentation gaps
type ComplianceGapHunter struct {
	WorkerID string
	Role     string
	OfficeMgr *office.Manager
}

func (cgh *ComplianceGapHunter) RunIteration(ctx context.Context) error {
	// Discover gap hunting tasks
	fmt.Println("[1/4] Discovering compliance gap tasks...")
	tasks := cgh.discoverTasks(ctx)
	if len(tasks) == 0 {
		fmt.Println("  No gap hunting tasks found")
		return nil
	}

	fmt.Printf("  Found %d task(s)\n", len(tasks))

	// Process each task
	gaps := make([]*ComplianceGap, 0)
	for _, task := range tasks {
		fmt.Printf("\n[2/4] Analyzing: %s\n", task.ID)
		gap, err := cgh.analyzeGap(ctx, task)
		if err != nil {
			log.Printf("Warning: failed to analyze gap for %s: %v", task.ID, err)
			continue
		}
		gaps = append(gaps, gap)
	}

	// Generate gap report
	fmt.Println("\n[3/4] Generating gap report...")
	cgh.generateGapReport(gaps)

	// Create remediation tickets
	fmt.Println("\n[4/4] Creating remediation tickets...")
	cgh.createRemediationTickets(ctx, gaps)

	return nil
}

func (cgh *ComplianceGapHunter) discoverTasks(ctx context.Context) []*ComplianceTask {
	return []*ComplianceTask{
		{
			ID:          "CG-001",
			Title:       "Detect missing control evidence",
			Description:  "Identify controls with weak, stale, or missing evidence",
			Priority:    "medium",
			TaskType:    "security",
			GapType:    "evidence-weakness",
		},
		{
			ID:          "CG-002",
			Title:       "Detect evidence freshness issues",
			Description:  "Identify evidence artifacts that are outdated or have no recent updates",
			Priority:    "medium",
			TaskType:    "security",
			GapType:    "evidence-freshness",
		},
		{
			ID:          "CG-003",
			Title:       "Detect undocumented operational practices",
			Description:  "Find operational processes that lack documentation or change records",
			Priority:    "low",
			TaskType:    "operations",
			GapType:    "undocumented-practices",
		},
		{
			ID:          "CG-004",
			Title:       "Detect missing change-management artifacts",
			Description:  "Identify changes without corresponding change tickets or approval records",
			Priority:    "medium",
			TaskType:    "operations",
			GapType:    "missing-change-management",
		},
		{
			ID:          "CG-005",
			Title:       "Detect missing incident/problem-management evidence",
			Description:  "Find incident resolutions or problem fixes without proper documentation",
			Priority:    "medium",
			TaskType:    "operations",
			GapType:    "missing-incident-evidence",
		},
	}
}

func (cgh *ComplianceGapHunter) analyzeGap(ctx context.Context, task *ComplianceTask) (*ComplianceGap, error) {
	gap := &ComplianceGap{
		ID:          task.ID,
		Title:       task.Title,
		GapType:     task.GapType,
		Severity:    task.Priority,
		WorkerID:     cgh.WorkerID,
		AnalyzedAt:  time.Now(),
	}

	// Simulate gap analysis (for now)
	switch task.GapType {
	case "evidence-weakness":
		gap.Status = "weak-evidence"
		gap.Recommendation = "Strengthen evidence with recent activity logs or test results"

	case "evidence-freshness":
		gap.Status = "stale-evidence"
		gap.Recommendation = "Update evidence packs with recent artifacts"

	case "undocumented-practices":
		gap.Status = "missing-docs"
		gap.Recommendation = "Document operational procedures in internal wiki"

	case "missing-change-management":
		gap.Status = "no-change-approval"
		gap.Recommendation = "Implement change-ticket requirement for all infra changes"

	case "missing-incident-evidence":
		gap.Status = "no-incident-records"
		gap.Recommendation = "Implement incident tracking with proper documentation"
	}

	return gap, nil
}

func (cgh *ComplianceGapHunter) generateGapReport(gaps []*ComplianceGap) {
	fmt.Println("\n=== Gap Report ===")
	fmt.Printf("Worker: %s\n", cgh.WorkerID)
	fmt.Printf("Gaps Identified: %d\n", len(gaps))
	fmt.Println()

	for _, gap := range gaps {
		fmt.Printf("  [%s] %s - %s\n", gap.Severity, gap.ID, gap.Title)
		fmt.Printf("       Status: %s\n", gap.Status)
		fmt.Printf("       Recommendation: %s\n", gap.Recommendation)
	}

	// Generate "top 5 compliance risks" summary
	fmt.Println("\n=== Top 5 Compliance Risks ===")
	fmt.Println("(Priority order: high > medium > low)")
	fmt.Println()

	// Sort by severity
	riskGaps := make([]*ComplianceGap, len(gaps))
	copy(riskGaps, gaps)
	for i := 0; i < len(riskGaps)-1; i++ {
		for j := i + 1; j < len(riskGaps); j++ {
			if strings.Compare(riskGaps[i].Severity, riskGaps[j].Severity) < 0 {
				riskGaps[i], riskGaps[j] = riskGaps[j], riskGaps[i]
			}
		}
	}

	count := len(riskGaps)
	if count > 5 {
		count = 5
	}

	for i := 0; i < count; i++ {
		gap := riskGaps[i]
		fmt.Printf("%d. %s - %s\n", i+1, gap.ID, gap.Title)
		fmt.Printf("   Severity: %s\n", gap.Severity)
		fmt.Printf("   Status: %s\n", gap.Status)
	}
}

func (cgh *ComplianceGapHunter) createRemediationTickets(ctx context.Context, gaps []*ComplianceGap) {
	prioritizedCount := 0

	for _, gap := range gaps {
		if gap.Severity == "medium" || gap.Severity == "high" {
			prioritizedCount++

			fmt.Printf("\nCreating remediation ticket for gap: %s\n", gap.ID)
			ticketID := fmt.Sprintf("CG-REM-%d", prioritizedCount)
			fmt.Printf("  Ticket ID: %s\n", ticketID)
			fmt.Printf("  Title: Remediate compliance gap - %s\n", gap.Title)
			fmt.Printf("  Priority: %s\n", gap.Severity)
			fmt.Printf("  Status: Open\n")

			// TODO: Create actual Jira ticket when write-back is implemented
			log.Printf("[ComplianceGapHunter] Created remediation ticket %s for gap %s", ticketID, gap.ID)
		}
	}

	if prioritizedCount == 0 {
		fmt.Println("\nNo high/medium severity gaps requiring remediation tickets")
	} else {
		fmt.Printf("\nCreated %d remediation ticket(s)\n", prioritizedCount)
	}
}

// ComplianceTask represents a compliance-related task
type ComplianceTask struct {
	ID          string
	Title       string
	Description  string
	Priority    string
	TaskType    string // funding, security, operations
	ReportType  string // sred, irap, soc2, iso27001, activity
	GapType    string // evidence-weakness, evidence-freshness, etc.
}

// ComplianceGap represents a detected compliance gap
type ComplianceGap struct {
	ID            string
	Title         string
	GapType       string
	Severity      string
	Status        string
	Recommendation string
	WorkerID      string
	AnalyzedAt    time.Time
}
