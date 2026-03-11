package analyzer

import (
	"fmt"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// RichAnalysisResult extends AnalysisResult with operator-facing content.
type RichAnalysisResult struct {
	*contracts.AnalysisResult

	// Enhanced fields for operator-facing summaries
	ExecutiveSummary string               `json:"executive_summary,omitempty"`        // High-level overview for quick review
	TechnicalSummary  string               `json:"technical_summary,omitempty"`         // Detailed technical breakdown
	ActionItems     []*ActionItem         `json:"action_items,omitempty"`           // Concrete next steps
	RiskAssessment  *RiskAssessment       `json:"risk_assessment,omitempty"`       // Risk analysis
	ReplayID        string                 `json:"replay_id"`                // For replayability
	CorrelationID    string                 `json:"correlation_id"`           // Trace analysis → Jira results
	AuditTrail      *AnalysisAuditTrail     `json:"audit_trail,omitempty"`        // Full custody chain
}

// ActionItem represents a concrete action for the operator.
type ActionItem struct {
	ID          string    `json:"id"`                          // Unique action ID
	Priority    string    `json:"priority"`                      // high/medium/low
	Title       string    `json:"title"`                         // Action title
	Description string    `json:"description"`                  // Detailed description
	Category    string    `json:"category"`                      // code/infrastructure/testing/analysis/approval
	EstimatedEffort string    `json:"estimated_effort,omitempty"`   // Time/complexity estimate
	DependsOn  []string  `json:"depends_on,omitempty"`        // Task dependencies
	Status      string    `json:"status"`                       // pending/in_progress/completed/blocked
	CreatedAt   time.Time `json:"created_at"`
}

// RiskAssessment represents risk analysis for the analysis.
type RiskAssessment struct {
	OverallRisk      string             `json:"overall_risk"`         // low/medium/high/critical
	RiskFactors      []*RiskFactor       `json:"risk_factors"`
	MitigationSteps  []*MitigationStep   `json:"mitigation_steps"`
}

// RiskFactor represents an identified risk.
type RiskFactor struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`              // technical/operational/dependency/unknown
	Severity    string  `json:"severity"`              // low/medium/high/critical
	Description string  `json:"description"`
	Impact     string  `json:"impact"`               // schedule/quality/security/multiple
	Likelihood  string  `json:"likelihood"`           // unlikely/possible/likely/very_likely
}

// MitigationStep represents a mitigation action.
type MitigationStep struct {
	ID          string    `json:"id"`
	RiskID      string    `json:"risk_id"`              // Links to RiskFactor.ID
	Step        string    `json:"step"`                 // Mitigation action
	Owner       string    `json:"owner,omitempty"`        // Who should do it
	Deadline    *time.Time `json:"deadline,omitempty"`     // When it should be done
	Status      string    `json:"status"`               // pending/in_progress/completed
}

// AnalysisAuditTrail represents the full custody chain.
type AnalysisAuditTrail struct {
	AnalysisID      string              `json:"analysis_id"`           // Result.AnalysisID or generated
	WorkItemID     string              `json:"work_item_id"`         // Source work item ID
	JiraKey        string              `json:"jira_key,omitempty"`      // Jira issue key
	WorkItemSource  string              `json:"work_item_source"`      // Where work item came from
	AnalysisChain   []*AnalysisStep      `json:"analysis_chain"`        // Each stage's execution
	TaskChain       []*TaskLinkage      `json:"task_chain"`            // Generated tasks and their links
	JiraLinkage    []*JiraCorrelation   `json:"jira_linkage"`         // Links to Jira items
	CustodyStart    time.Time           `json:"custody_start"`        // When work item was received
	CustodyEnd      time.Time           `json:"custody_end"`          // When analysis completed
	ChainOfTrust    []string            `json:"chain_of_trust"`       // Actors/systems in the chain
	Verified        bool                `json:"verified"`             // If audit trail is verified
}

// AnalysisStep represents a single step in the analysis pipeline.
type AnalysisStep struct {
	Stage           string    `json:"stage"`                 // StageClassification, StageRequirements, etc.
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
	DurationMs      int64     `json:"duration_ms"`
	Status          string    `json:"status"`                // success/failure/skipped
	InputSummary    string    `json:"input_summary"`         // What went into this stage
	OutputSummary   string    `json:"output_summary"`        // What came out
	Actor           string    `json:"actor"`                 // zen-brain/llm:provider/human
	Errors          []string  `json:"errors,omitempty"`
}

// TaskLinkage represents generated tasks and their relationships.
type TaskLinkage struct {
	TaskID              string       `json:"task_id"`
	SourceWorkItemID    string       `json:"source_work_item_id"`
	SourceStage         string       `json:"source_stage"`         // Which analysis stage created this task
	DependencyType      string       `json:"dependency_type"`      // sequential/parallel/conditional
	DependsOnTaskIDs   []string     `json:"depends_on_task_ids,omitempty"`
	LinkedToJiraKeys   []string     `json:"linked_to_jira_keys,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
}

// JiraCorrelation represents links between analysis and Jira.
type JiraCorrelation struct {
	CorrelationType   string    `json:"correlation_type"`      // parent/child/related/blocks/is_blocked_by
	SourceID          string    `json:"source_id"`           // Analysis or task ID
	TargetJiraKey    string    `json:"target_jira_key"`     // Jira issue key
	RelationshipNotes string    `json:"relationship_notes"`   // Why this correlation exists
	CreatedAt         time.Time `json:"created_at"`
	Verified          bool      `json:"verified"`            // If correlation is verified
}

// TaskBreakdown represents a structured breakdown of work into tasks.
type TaskBreakdown struct {
	PrimaryTasks      []*contracts.BrainTaskSpec          `json:"primary_tasks"`           // Main implementation tasks
	SupportingTasks  []*contracts.BrainTaskSpec          `json:"supporting_tasks"`         // Setup/teardown/validation tasks
	Dependencies     []*TaskDependency     `json:"dependencies"`           // Task dependencies
	ExecutionOrder   [][]string            `json:"execution_order"`         // How tasks should run (parallel paths)
	EstimatedDuration string                 `json:"estimated_duration"`     // Total time estimate
	ResourceAllocation *ResourceAllocation    `json:"resource_allocation,omitempty"` // How resources are allocated
}

// TaskDependency represents a relationship between tasks.
type TaskDependency struct {
	FromTaskID   string `json:"from_task_id"`
	ToTaskID     string `json:"to_task_id"`
	DependencyType string `json:"dependency_type"`      // hard (must complete before) / soft (prefer completion before)
	Constraint     string `json:"constraint,omitempty"`    // Additional constraint notes
}

// ResourceAllocation represents how resources are assigned to tasks.
type ResourceAllocation struct {
	TotalTasks        int     `json:"total_tasks"`
	ParallelCapacity   int     `json:"parallel_capacity"`      // Max tasks that can run in parallel
	EstimatedCPUPct   float64 `json:"estimated_cpu_pct"`
	EstimatedMemoryMB int64   `json:"estimated_memory_mb"`
	PriorityQueue     bool    `json:"priority_queue"`       // Whether tasks are prioritized
}

// Replayability represents information for replaying analyses.
type Replayability struct {
	ReplayID          string            `json:"replay_id"`              // Unique replay identifier
	CanReplay          bool              `json:"can_replay"`              // Whether this analysis can be replayed
	ReplayParameters   *ReplayParameters  `json:"replay_parameters"`       // Parameters for replay
	PreviousReplayID   string            `json:"previous_replay_id,omitempty"` // Chain of replays
	ReplayCount        int               `json:"replay_count"`           // How many times replayed
	LastReplayedAt     *time.Time        `json:"last_replayed_at,omitempty"`
}

// ReplayParameters represents the configuration for replaying.
type ReplayParameters struct {
	LLMProvider        string            `json:"llm_provider"`
	LLMModel          string            `json:"llm_model"`
	Temperature        float64           `json:"temperature"`
	MaxTokens          int                `json:"max_tokens"`
	EnabledStages     []string           `json:"enabled_stages"`
	ConfigOverrides    map[string]any    `json:"config_overrides,omitempty"`
}

// EnrichForRichAnalysis creates a RichAnalysisResult from AnalysisResult.
func EnrichForRichAnalysis(base *contracts.AnalysisResult, workItem *contracts.WorkItem) *RichAnalysisResult {
	now := time.Now()

	// Generate unique IDs
	replayID := fmt.Sprintf("replay-%d-%s", now.Unix(), workItem.ID)
	correlationID := fmt.Sprintf("corr-%d-%s", now.Unix(), workItem.ID)

	// Build audit trail
	auditTrail := &AnalysisAuditTrail{
		AnalysisID:      generateAnalysisID(base, now),
		WorkItemID:     workItem.ID,
		JiraKey:        workItem.Source.IssueKey,
		WorkItemSource:  inferSource(workItem),
		AnalysisChain:   buildAnalysisChain(base),
		TaskChain:       buildTaskChain(base),
		JiraLinkage:    buildJiraLinkage(workItem, base),
		CustodyStart:   now,
		CustodyEnd:     now.Add(time.Minute), // Simple 1-minute estimate
		ChainOfTrust:    []string{"zen-brain", base.AnalyzerVersion, workItem.Source.System},
		Verified:        false,
	}

	// Build risk assessment
	riskAssessment := assessRisks(workItem, base)

	// Build action items
	actionItems := generateActionItems(workItem, base)

	// Build summaries
	executiveSummary := generateExecutiveSummary(workItem, base, riskAssessment)
	technicalSummary := generateTechnicalSummary(workItem, base)

	return &RichAnalysisResult{
		AnalysisResult:    base,
		ExecutiveSummary:  executiveSummary,
		TechnicalSummary:   technicalSummary,
		ActionItems:       actionItems,
		RiskAssessment:    riskAssessment,
		ReplayID:         replayID,
		CorrelationID:     correlationID,
		AuditTrail:       auditTrail,
	}
}

// Helper functions

func generateAnalysisID(result *contracts.AnalysisResult, timestamp time.Time) string {
	return fmt.Sprintf("analysis-%d", timestamp.Unix())
}

func inferSource(workItem *contracts.WorkItem) string {
	if workItem.Source.System != "" {
		return workItem.Source.System
	}
	if workItem.Source.IssueKey != "" {
		return "jira"
	}
	return "unknown"
}

func buildAnalysisChain(result *contracts.AnalysisResult) []*AnalysisStep {
	// TODO: Extract from stage execution when available
	// For now, return a summary chain
	return []*AnalysisStep{
		{
			Stage:         "full_analysis",
			StartedAt:     result.AnalyzedAt,
			CompletedAt:   result.AnalyzedAt.Add(1 * time.Minute), // Estimate
			DurationMs:     60000, // 1 minute estimate
			Status:         "success",
			InputSummary:   result.WorkItem.ID,
			OutputSummary:  fmt.Sprintf("%d tasks generated", len(result.BrainTaskSpecs)),
			Actor:         result.AnalyzerVersion,
			Errors:        []string{},
		},
	}
}

func buildTaskChain(result *contracts.AnalysisResult) []*TaskLinkage {
	chain := make([]*TaskLinkage, 0, len(result.BrainTaskSpecs))

	for i, spec := range result.BrainTaskSpecs {
		linkage := &TaskLinkage{
			TaskID:           spec.ID,
			SourceWorkItemID: result.WorkItem.ID,
			SourceStage:       "finalization",
			DependencyType:    "sequential",
			CreatedAt:         time.Now(),
		}

		// Link dependencies
		if i > 0 {
			linkage.DependsOnTaskIDs = []string{result.BrainTaskSpecs[i-1].ID}
		}

		chain = append(chain, linkage)
	}

	return chain
}

func buildJiraLinkage(workItem *contracts.WorkItem, result *contracts.AnalysisResult) []*JiraCorrelation {
	correlations := []*JiraCorrelation{
		{
			CorrelationType:   "parent",
			SourceID:          result.WorkItem.ID,
			TargetJiraKey:    workItem.Source.IssueKey,
			RelationshipNotes: "Analysis source work item",
			CreatedAt:         time.Now(),
			Verified:          true,
		},
	}

	return correlations
}

func assessRisks(workItem *contracts.WorkItem, result *contracts.AnalysisResult) *RiskAssessment {
	riskFactors := []*RiskFactor{}
	overallRisk := "low"

	// Check for high priority work items
	if workItem.Priority == contracts.PriorityCritical || workItem.Priority == contracts.PriorityHigh {
		riskFactors = append(riskFactors, &RiskFactor{
			ID:          "priority-risk",
			Category:    "operational",
			Severity:    "medium",
			Description: "High priority work item requires careful planning",
			Impact:     "schedule",
			Likelihood:  "possible",
		})
	}

	// Check for complex work types
	if workItem.WorkType == contracts.WorkTypeRefactor {
		riskFactors = append(riskFactors, &RiskFactor{
			ID:          "complexity-risk",
			Category:    "technical",
			Severity:    "medium",
			Description: "Refactoring has inherent complexity",
			Impact:     "quality",
			Likelihood:  "likely",
		})
	}

	// Check for low confidence
	if result.Confidence < 0.7 {
		riskFactors = append(riskFactors, &RiskFactor{
			ID:          "confidence-risk",
			Category:    "unknown",
			Severity:    "medium",
			Description: "Low analysis confidence may indicate unclear requirements",
			Impact:     "multiple",
			Likelihood:  "possible",
		})
	}

	// Determine overall risk
	criticalCount := 0
	for _, factor := range riskFactors {
		if factor.Severity == "critical" {
			criticalCount++
		}
	}

	if criticalCount > 0 {
		overallRisk = "critical"
	} else if len(riskFactors) > 2 {
		overallRisk = "high"
	} else if len(riskFactors) > 0 {
		overallRisk = "medium"
	}

	return &RiskAssessment{
		OverallRisk:     overallRisk,
		RiskFactors:     riskFactors,
		MitigationSteps: generateMitigations(riskFactors),
	}
}

func generateMitigations(riskFactors []*RiskFactor) []*MitigationStep {
	steps := []*MitigationStep{}

	for i, factor := range riskFactors {
		step := &MitigationStep{
			ID:      fmt.Sprintf("mitigate-%d", i),
			RiskID:  factor.ID,
			Step:    getMitigationStep(factor),
			Status:  "pending",
		}

		if factor.Category == "operational" {
			step.Owner = "project_manager"
		} else if factor.Category == "technical" {
			step.Owner = "tech_lead"
		}

		steps = append(steps, step)
	}

	return steps
}

func getMitigationStep(factor *RiskFactor) string {
	switch factor.Category {
	case "operational":
		return "Schedule review with stakeholders"
	case "technical":
		return "Create design document and get review"
	case "unknown":
		return "Gather more requirements or clarification"
	case "dependency":
		return "Validate dependencies with owning teams"
	default:
		return "Monitor and adjust as needed"
	}
}

func generateActionItems(workItem *contracts.WorkItem, result *contracts.AnalysisResult) []*ActionItem {
	actions := []*ActionItem{}

	// Review action
	actions = append(actions, &ActionItem{
		ID:          "review-analysis",
		Priority:    "high",
		Title:       "Review Analysis Results",
		Description:  "Review generated tasks and confirm requirements are met",
		Category:    "analysis",
		Status:      "pending",
		CreatedAt:   time.Now(),
	})

	// Approve action if needed
	if result.RequiresApproval {
		actions = append(actions, &ActionItem{
			ID:          "approve-tasks",
			Priority:    "medium",
			Title:       "Approve Task Breakdown",
			Description:  "Review and approve generated tasks for execution",
			Category:    "approval",
			EstimatedEffort: "30m",
			Status:      "pending",
			CreatedAt:   time.Now(),
			DependsOn:  []string{"review-analysis"},
		})
	}

	// Implementation actions
	for _, spec := range result.BrainTaskSpecs {
		actions = append(actions, &ActionItem{
			ID:          fmt.Sprintf("implement-%s", spec.ID),
			Priority:    getPriorityFromWorkType(spec.WorkType),
			Title:       spec.Title,
			Description:  spec.Objective,
			Category:    "code",
			Status:      "pending",
			CreatedAt:   time.Now(),
		})
	}

	return actions
}

func getPriorityFromWorkType(workType contracts.WorkType) string {
	switch workType {
	case contracts.WorkTypeImplementation:
		return "high"
	case contracts.WorkTypeDebug:
		return "high"
	case contracts.WorkTypeRefactor:
		return "medium"
	default:
		return "low"
	}
}

func generateExecutiveSummary(workItem *contracts.WorkItem, result *contracts.AnalysisResult, risk *RiskAssessment) string {
	return fmt.Sprintf(
		"Analyzed %s (%s) with %.0f%% confidence. Generated %d task(s). Risk level: %s. %s",
		workItem.Title,
		workItem.ID,
		result.Confidence*100,
		len(result.BrainTaskSpecs),
		risk.OverallRisk,
		getNextStep(workItem, result),
	)
}

func generateTechnicalSummary(workItem *contracts.WorkItem, result *contracts.AnalysisResult) string {
	lines := []string{
		fmt.Sprintf("Work Type: %s", workItem.WorkType),
		fmt.Sprintf("Work Domain: %s", workItem.WorkDomain),
		fmt.Sprintf("Priority: %s", workItem.Priority),
		fmt.Sprintf("Analyzer: %s (version %s)", result.AnalyzerVersion, result.AnalyzerVersion),
	}

	for i, spec := range result.BrainTaskSpecs {
		lines = append(lines, fmt.Sprintf(
			"Task %d (%s): %s",
			i+1, spec.ID, spec.Title,
		))
	}

	return joinLines(lines)
}

func getNextStep(workItem *contracts.WorkItem, result *contracts.AnalysisResult) string {
	if result.RequiresApproval {
		return "Requires approval before task execution."
	}
	return "Ready for task execution."
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		if result != "" {
			result += "\n"
		}
		result += line
	}
	return result
}
