// Package analyzer provides Block 2 enterprise features (analyzer quality, history, breakdowns, Jira/audit traceability).
package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ============================================================================
// ENHANCED ANALYZER QUALITY
// ============================================================================

// StructuredLLMResponse provides structured JSON output from LLM responses.
type StructuredLLMResponse struct {
	WorkType      string   `json:"work_type,omitempty"`
	WorkDomain    string   `json:"work_domain,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	KBScopes      []string `json:"kb_scopes,omitempty"`
	Objective     string   `json:"objective,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Constraints   []string `json:"constraints,omitempty"`
	Dependencies  []string `json:"dependencies,omitempty"`
	Subtasks      []string `json:"subtasks,omitempty"`
	SubtaskCount  int      `json:"subtask_count,omitempty"`
	Hypothesis     string   `json:"hypothesis,omitempty"`
	SREDTags      []string `json:"sred_tags,omitempty"`
	EvidenceRequirements []string `json:"evidence_requirements,omitempty"`
	Confidence    float64  `json:"confidence"`
	Reasoning     string   `json:"reasoning,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

// ParseStructuredResponse parses JSON response from LLM.
// Falls back to text parsing if JSON fails.
func ParseStructuredResponse(llmContent string) (*StructuredLLMResponse, error) {
	// First, try to extract JSON from the response (LLMs sometimes wrap JSON in code blocks)
	jsonContent := extractJSONFromResponse(llmContent)

	// Try to parse as JSON
	var result StructuredLLMResponse
	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		// JSON parsing failed, return empty structured response
		// The caller should fall back to text parsing
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// extractJSONFromResponse extracts JSON content from LLM response.
// Handles code blocks (```json ... ```) and markdown formatting.
func extractJSONFromResponse(content string) string {
	// Remove common LLM artifacts
	content = strings.TrimSpace(content)

	// Check for JSON code block
	if strings.HasPrefix(content, "```json") {
		// Extract content between ```json and ```
		parts := strings.SplitN(content, "```json", 2)
		if len(parts) == 2 {
			inner := strings.SplitN(parts[1], "```", 2)
			if len(inner) == 2 {
				return strings.TrimSpace(inner[0])
			}
		}
	} else if strings.HasPrefix(content, "```") {
		// Generic code block
		parts := strings.SplitN(content, "```", 2)
		if len(parts) == 2 {
			inner := strings.SplitN(parts[1], "```", 2)
			if len(inner) == 2 {
				return strings.TrimSpace(inner[0])
			}
		}
	} else if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
		// Direct JSON
		return content
	}

	// If no JSON markers found, return as-is (caller will handle error)
	return content
}

// ============================================================================
// ENHANCED HISTORY & COMPARISON
// ============================================================================

// AnalysisHistoryEnhanced provides enhanced history operations.
type AnalysisHistoryEnhanced struct {
	store *FileAnalysisStore
}

// NewAnalysisHistoryEnhanced creates enhanced history operations.
func NewAnalysisHistoryEnhanced(store *FileAnalysisStore) *AnalysisHistoryEnhanced {
	return &AnalysisHistoryEnhanced{store: store}
}

// CompareAnalysis compares two analysis results and returns differences.
func (h *AnalysisHistoryEnhanced) CompareAnalysis(ctx context.Context, workItemID string, index1, index2 int) (*AnalysisComparison, error) {
	history, err := h.store.GetHistory(ctx, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	if len(history) <= index1 || len(history) <= index2 {
		return nil, fmt.Errorf("invalid indices: history has %d entries", len(history))
	}

	result1 := history[index1]
	result2 := history[index2]

	return &AnalysisComparison{
		WorkItemID:    workItemID,
		Analysis1ID:   getAnalysisID(result1, index1),
		Analysis2ID:   getAnalysisID(result2, index2),
		Analysis1Time: result1.AnalyzedAt,
		Analysis2Time: result2.AnalyzedAt,
		TaskDiff:      compareBrainTasks(result1.BrainTaskSpecs, result2.BrainTaskSpecs),
		ConfidenceDiff: result2.Confidence - result1.Confidence,
		CostDiff:      result2.EstimatedTotalCostUSD - result1.EstimatedTotalCostUSD,
		ApprovalChange: result1.RequiresApproval != result2.RequiresApproval,
	}, nil
}

// SearchHistory searches across all work items for specific criteria.
func (h *AnalysisHistoryEnhanced) SearchHistory(ctx context.Context, criteria *SearchCriteria) ([]*AnalysisSearchResult, error) {
	// List all analysis files
	storeDir := h.store.dir
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read store directory: %w", err)
	}

	var results []*AnalysisSearchResult

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		// Extract work item ID from filename
		workItemID := strings.TrimSuffix(entry.Name(), ".jsonl")

		// Get history for this work item
		history, err := h.store.GetHistory(ctx, workItemID)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Search each analysis
		for i, analysis := range history {
			if matchesCriteria(analysis, criteria) {
				results = append(results, &AnalysisSearchResult{
					WorkItemID:      workItemID,
					AnalysisIndex:   i,
					Analysis:       analysis,
					MatchScore:     calculateMatchScore(analysis, criteria),
					MatchedFields:  getMatchedFields(analysis, criteria),
				})
			}
		}
	}

	return results, nil
}

// GetConfidenceTrend returns confidence trend for a work item over time.
func (h *AnalysisHistoryEnhanced) GetConfidenceTrend(ctx context.Context, workItemID string) ([]*ConfidencePoint, error) {
	history, err := h.store.GetHistory(ctx, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	trend := make([]*ConfidencePoint, 0, len(history))
	for i, analysis := range history {
		trend = append(trend, &ConfidencePoint{
			Index:      i,
			Timestamp:  analysis.AnalyzedAt,
			Confidence: analysis.Confidence,
			TaskCount:  len(analysis.BrainTaskSpecs),
		})
	}

	return trend, nil
}

// ============================================================================
// ENHANCED BREAKDOWN WITH DEPENDENCIES
// ============================================================================

// TaskDependencyWithMetadata represents a task dependency with context.
type TaskDependencyWithMetadata struct {
	FromTaskID      string    `json:"from_task_id"`
	ToTaskID        string    `json:"to_task_id"`
	DependencyType  string    `json:"dependency_type"`  // hard/soft/conditional
	Constraint      string    `json:"constraint,omitempty"`
	Reason          string    `json:"reason"`           // Why this dependency exists
	CreatedAt       time.Time `json:"created_at"`
}

// ExecutionPath represents a parallel or sequential execution path.
type ExecutionPath struct {
	PathID          string   `json:"path_id"`
	TaskIDs         []string `json:"task_ids"`
	ExecutionType   string   `json:"execution_type"`   // sequential/parallel/conditional
	EstimatedDuration string `json:"estimated_duration"` // e.g., "2h 30m"
	ResourceUsage   *ResourceUsage `json:"resource_usage,omitempty"`
	Conditions      []string `json:"conditions,omitempty"` // For conditional paths
}

// ResourceUsage estimates resource requirements for a path.
type ResourceUsage struct {
	EstimatedCPUPct  float64 `json:"estimated_cpu_pct"`
	EstimatedMemoryMB int64   `json:"estimated_memory_mb"`
	EstimatedIO      string   `json:"estimated_io"`       // low/medium/high
}

// EnhancedTaskBreakdown provides detailed task breakdown with dependencies.
type EnhancedTaskBreakdown struct {
	PrimaryTasks      []contracts.BrainTaskSpec        `json:"primary_tasks"`
	SupportingTasks   []contracts.BrainTaskSpec        `json:"supporting_tasks"`
	Dependencies      []*TaskDependencyWithMetadata      `json:"dependencies"`
	ExecutionPaths    []*ExecutionPath                  `json:"execution_paths"`
	EstimatedTotalDuration string                       `json:"estimated_total_duration"`
	ResourceAllocation *ResourceAllocation               `json:"resource_allocation"`
	RiskPerTask       map[string]*TaskRisk             `json:"risk_per_task"`
}

// TaskRisk represents risk assessment for a specific task.
type TaskRisk struct {
	TaskID         string         `json:"task_id"`
	RiskLevel      string         `json:"risk_level"`        // low/medium/high/critical
	RiskFactors    []*RiskFactor  `json:"risk_factors"`
	MitigationPlan []*MitigationStep `json:"mitigation_plan"`
	EstimatedDelay string         `json:"estimated_delay,omitempty"` // e.g., "1-2 days"
}

// CreateEnhancedBreakdown creates an enhanced breakdown from analysis result.
func CreateEnhancedBreakdown(result *contracts.AnalysisResult) *EnhancedTaskBreakdown {
	// Separate primary and supporting tasks
	primary := []contracts.BrainTaskSpec{}
	supporting := []contracts.BrainTaskSpec{}

	for _, spec := range result.BrainTaskSpecs {
		if spec.WorkType == contracts.WorkTypeTesting ||
			spec.WorkType == contracts.WorkTypeDocumentation ||
			spec.WorkType == contracts.WorkTypeOperations {
			supporting = append(supporting, spec)
		} else {
			primary = append(primary, spec)
		}
	}

	// Create dependencies
	deps := createDependencies(result.BrainTaskSpecs)

	// Create execution paths
	paths := createExecutionPaths(result.BrainTaskSpecs)

	// Estimate total duration
	totalDuration := estimateTotalDuration(result.BrainTaskSpecs)

	// Create resource allocation
	alloc := &ResourceAllocation{
		TotalTasks:        len(result.BrainTaskSpecs),
		ParallelCapacity:   calculateParallelCapacity(paths),
		EstimatedCPUPct:   45.0, // Default estimate
		EstimatedMemoryMB: 2048,
		PriorityQueue:     false,
	}

	// Assess risk per task
	riskPerTask := assessRisksPerTask(result.BrainTaskSpecs)

	return &EnhancedTaskBreakdown{
		PrimaryTasks:          primary,
		SupportingTasks:       supporting,
		Dependencies:         deps,
		ExecutionPaths:       paths,
		EstimatedTotalDuration: totalDuration,
		ResourceAllocation:   alloc,
		RiskPerTask:          riskPerTask,
	}
}

// ============================================================================
// ENHANCED JIRA/AUDIT TRACEABILITY
// ============================================================================

// DetailedAnalysisStep represents a granular step in analysis with full audit info.
type DetailedAnalysisStep struct {
	Stage           string              `json:"stage"`
	StageID         string              `json:"stage_id"`         // Unique stage ID
	StartedAt       time.Time           `json:"started_at"`
	CompletedAt     time.Time           `json:"completed_at"`
	DurationMs      int64               `json:"duration_ms"`
	Status          string              `json:"status"`          // success/failure/skipped
	InputHash       string              `json:"input_hash"`      // Hash of input for verification
	OutputHash      string              `json:"output_hash"`     // Hash of output for verification
	LLMProvider     string              `json:"llm_provider,omitempty"`  // LLM provider used
	LLMModel        string              `json:"llm_model,omitempty"`     // LLM model used
	LLMRequestTokens int               `json:"llm_request_tokens,omitempty"`
	LLMResponseTokens int              `json:"llm_response_tokens,omitempty"`
	Actor           string              `json:"actor"`           // zen-brain/llm:provider/human
	InputSummary    string              `json:"input_summary"`
	OutputSummary   string              `json:"output_summary"`
	Errors          []string            `json:"errors,omitempty"`
	RetryCount      int                 `json:"retry_count"`     // Number of retries if failed
	CacheHit        bool                `json:"cache_hit"`       // If result was cached
	Verified        bool                `json:"verified"`        // If step output was verified
}

// DetailedJiraCorrelation represents enhanced Jira linkage.
type DetailedJiraCorrelation struct {
	CorrelationID   string    `json:"correlation_id"`   // Unique correlation ID
	CorrelationType string    `json:"correlation_type"` // parent/child/related/blocks/is_blocked_by/duplicates
	SourceID       string    `json:"source_id"`        // Analysis or task ID
	TargetJiraKey  string    `json:"target_jira_key"` // Jira issue key
	RelationshipNotes string  `json:"relationship_notes"`
	JiraSummary     string    `json:"jira_summary"`     // Jira issue summary for context
	JiraStatus      string    `json:"jira_status"`      // Current Jira status
	JiraPriority    string    `json:"jira_priority"`    // Jira priority
	JiraAssignee    string    `json:"jira_assignee"`    // Jira assignee
	JiraCreated     time.Time `json:"jira_created"`     // Jira creation time
	JiraUpdated     time.Time `json:"jira_updated"`     // Jira last update time
	CreatedAt       time.Time `json:"created_at"`
	Verified        bool      `json:"verified"`         // If correlation is verified
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
}

// AuditChainOfTrust represents the verified chain of trust for an analysis.
type AuditChainOfTrust struct {
	ChainID          string                 `json:"chain_id"`            // Unique chain ID
	AnalysisID       string                 `json:"analysis_id"`         // Analysis ID
	Actors           []*ChainActor          `json:"actors"`              // All actors in the chain
	SystemSignatures []*SystemSignature     `json:"system_signatures"`   // System verification signatures
	Timestamp        time.Time              `json:"timestamp"`           // Chain creation time
	Verified         bool                   `json:"verified"`            // If chain is fully verified
	VerificationErrors []string             `json:"verification_errors,omitempty"`
}

// ChainActor represents an actor in the chain of trust.
type ChainActor struct {
	ActorID      string    `json:"actor_id"`
	ActorType    string    `json:"actor_type"`      // system/human/llm
	ActorName    string    `json:"actor_name"`     // e.g., "zen-brain", "glm-4.7", "user@example.com"
	ActorVersion string    `json:"actor_version,omitempty"` // For systems
	RoleInChain  string    `json:"role_in_chain"`   // e.g., "analyzer", "approver", "executor"
	Timestamp    time.Time `json:"timestamp"`
	Signature    string    `json:"signature,omitempty"` // Actor signature (optional)
}

// SystemSignature represents a system-level signature for verification.
type SystemSignature struct {
	SystemID      string    `json:"system_id"`       // e.g., "zen-brain-v1.2.3"
	Signature     string    `json:"signature"`        // Cryptographic signature
	Algorithm     string    `json:"algorithm"`       // e.g., "sha256", "hmac-sha256"
	SignedAt      time.Time `json:"signed_at"`
	Verified      bool      `json:"verified"`        // If signature is valid
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// getAnalysisID generates a unique ID for an analysis.
func getAnalysisID(result *contracts.AnalysisResult, index int) string {
	return fmt.Sprintf("analysis-%s-%d", result.AnalyzedAt.Format("20060102-150405"), index)
}

// compareBrainTasks compares two sets of BrainTaskSpecs and returns differences.
func compareBrainTasks(tasks1, tasks2 []contracts.BrainTaskSpec) *TaskDiff {
	return &TaskDiff{
		AddedTasks:    findAddedTasks(tasks1, tasks2),
		RemovedTasks:  findRemovedTasks(tasks1, tasks2),
		ModifiedTasks: findModifiedTasks(tasks1, tasks2),
		TaskCountDiff: len(tasks2) - len(tasks1),
	}
}

// TaskDiff represents differences between task sets.
type TaskDiff struct {
	AddedTasks    []contracts.BrainTaskSpec `json:"added_tasks"`
	RemovedTasks  []contracts.BrainTaskSpec `json:"removed_tasks"`
	ModifiedTasks []*TaskModification      `json:"modified_tasks"`
	TaskCountDiff int                      `json:"task_count_diff"`
}

// TaskModification represents a modified task.
type TaskModification struct {
	TaskID       string              `json:"task_id"`
	TaskTitle    string              `json:"task_title"`
	ChangedFields map[string]string  `json:"changed_fields"` // field -> old_value -> new_value
}

// SearchCriteria defines search criteria for analysis history.
type SearchCriteria struct {
	WorkItemID    string    `json:"work_item_id,omitempty"`
	WorkType      string    `json:"work_type,omitempty"`
	WorkDomain    string    `json:"work_domain,omitempty"`
	Priority      string    `json:"priority,omitempty"`
	MinConfidence float64   `json:"min_confidence,omitempty"`
	MaxConfidence float64   `json:"max_confidence,omitempty"`
	FromDate      time.Time `json:"from_date,omitempty"`
	ToDate        time.Time `json:"to_date,omitempty"`
	AnalyzerVersion string  `json:"analyzer_version,omitempty"`
}

// AnalysisSearchResult represents a search result with match scoring.
type AnalysisSearchResult struct {
	WorkItemID    string                   `json:"work_item_id"`
	AnalysisIndex int                      `json:"analysis_index"`
	Analysis      *contracts.AnalysisResult `json:"analysis"`
	MatchScore    float64                  `json:"match_score"`     // 0.0-1.0
	MatchedFields []string                 `json:"matched_fields"`
}

// ConfidencePoint represents a confidence measurement at a point in time.
type ConfidencePoint struct {
	Index      int       `json:"index"`
	Timestamp  time.Time `json:"timestamp"`
	Confidence float64   `json:"confidence"`
	TaskCount  int       `json:"task_count"`
}

// AnalysisComparison represents the differences between two analyses.
type AnalysisComparison struct {
	WorkItemID     string       `json:"work_item_id"`
	Analysis1ID    string       `json:"analysis1_id"`
	Analysis2ID    string       `json:"analysis2_id"`
	Analysis1Time  time.Time    `json:"analysis1_time"`
	Analysis2Time  time.Time    `json:"analysis2_time"`
	TaskDiff       *TaskDiff    `json:"task_diff"`
	ConfidenceDiff float64      `json:"confidence_diff"`
	CostDiff       float64      `json:"cost_diff"`
	ApprovalChange bool         `json:"approval_change"`
}

// Helper functions implementation

func findAddedTasks(oldTasks, newTasks []contracts.BrainTaskSpec) []contracts.BrainTaskSpec {
	added := []contracts.BrainTaskSpec{}
	for _, newTask := range newTasks {
		found := false
		for _, oldTask := range oldTasks {
			if oldTask.ID == newTask.ID {
				found = true
				break
			}
		}
		if !found {
			added = append(added, newTask)
		}
	}
	return added
}

func findRemovedTasks(oldTasks, newTasks []contracts.BrainTaskSpec) []contracts.BrainTaskSpec {
	removed := []contracts.BrainTaskSpec{}
	for _, oldTask := range oldTasks {
		found := false
		for _, newTask := range newTasks {
			if oldTask.ID == newTask.ID {
				found = true
				break
			}
		}
		if !found {
			removed = append(removed, oldTask)
		}
	}
	return removed
}

func findModifiedTasks(oldTasks, newTasks []contracts.BrainTaskSpec) []*TaskModification {
	modified := []*TaskModification{}

	for _, newTask := range newTasks {
		for _, oldTask := range oldTasks {
			if oldTask.ID == newTask.ID {
				changes := map[string]string{}
				if oldTask.Title != newTask.Title {
					changes["title"] = fmt.Sprintf("%s -> %s", oldTask.Title, newTask.Title)
				}
				if oldTask.Objective != newTask.Objective {
					changes["objective"] = fmt.Sprintf("%s -> %s", oldTask.Objective, newTask.Objective)
				}
				if len(changes) > 0 {
					modified = append(modified, &TaskModification{
						TaskID:       newTask.ID,
						TaskTitle:    newTask.Title,
						ChangedFields: changes,
					})
				}
				break
			}
		}
	}

	return modified
}

func matchesCriteria(analysis *contracts.AnalysisResult, criteria *SearchCriteria) bool {
	if criteria.WorkItemID != "" && analysis.WorkItem.ID != criteria.WorkItemID {
		return false
	}
	if criteria.WorkType != "" && string(analysis.WorkItem.WorkType) != criteria.WorkType {
		return false
	}
	if criteria.WorkDomain != "" && string(analysis.WorkItem.WorkDomain) != criteria.WorkDomain {
		return false
	}
	if criteria.Priority != "" && string(analysis.WorkItem.Priority) != criteria.Priority {
		return false
	}
	if criteria.MinConfidence > 0 && analysis.Confidence < criteria.MinConfidence {
		return false
	}
	if criteria.MaxConfidence > 0 && analysis.Confidence > criteria.MaxConfidence {
		return false
	}
	if criteria.AnalyzerVersion != "" && analysis.AnalyzerVersion != criteria.AnalyzerVersion {
		return false
	}
	if !criteria.FromDate.IsZero() && analysis.AnalyzedAt.Before(criteria.FromDate) {
		return false
	}
	if !criteria.ToDate.IsZero() && analysis.AnalyzedAt.After(criteria.ToDate) {
		return false
	}
	return true
}

func calculateMatchScore(analysis *contracts.AnalysisResult, criteria *SearchCriteria) float64 {
	score := 0.0
	maxScore := 0.0

	if criteria.WorkType != "" {
		maxScore += 1.0
		if string(analysis.WorkItem.WorkType) == criteria.WorkType {
			score += 1.0
		}
	}
	if criteria.WorkDomain != "" {
		maxScore += 1.0
		if string(analysis.WorkItem.WorkDomain) == criteria.WorkDomain {
			score += 1.0
		}
	}
	if criteria.Priority != "" {
		maxScore += 1.0
		if string(analysis.WorkItem.Priority) == criteria.Priority {
			score += 1.0
		}
	}
	if criteria.MinConfidence > 0 {
		maxScore += 1.0
		if analysis.Confidence >= criteria.MinConfidence {
			score += 1.0
		}
	}

	if maxScore > 0 {
		return score / maxScore
	}
	return 1.0 // No criteria specified, full match
}

func getMatchedFields(analysis *contracts.AnalysisResult, criteria *SearchCriteria) []string {
	matched := []string{}
	if criteria.WorkType != "" && string(analysis.WorkItem.WorkType) == criteria.WorkType {
		matched = append(matched, "work_type")
	}
	if criteria.WorkDomain != "" && string(analysis.WorkItem.WorkDomain) == criteria.WorkDomain {
		matched = append(matched, "work_domain")
	}
	if criteria.Priority != "" && string(analysis.WorkItem.Priority) == criteria.Priority {
		matched = append(matched, "priority")
	}
	if criteria.MinConfidence > 0 && analysis.Confidence >= criteria.MinConfidence {
		matched = append(matched, "confidence")
	}
	return matched
}

func createDependencies(tasks []contracts.BrainTaskSpec) []*TaskDependencyWithMetadata {
	deps := []*TaskDependencyWithMetadata{}
	now := time.Now()

	for i := 1; i < len(tasks); i++ {
		deps = append(deps, &TaskDependencyWithMetadata{
			FromTaskID:     tasks[i-1].ID,
			ToTaskID:       tasks[i].ID,
			DependencyType: "hard",
			Constraint:     "Must complete before starting next task",
			Reason:         "Sequential execution order",
			CreatedAt:      now,
		})
	}

	return deps
}

func createExecutionPaths(tasks []contracts.BrainTaskSpec) []*ExecutionPath {
	if len(tasks) == 0 {
		return nil
	}

	// Default: single sequential path
	path := &ExecutionPath{
		PathID:         "path-1",
		TaskIDs:        make([]string, 0, len(tasks)),
		ExecutionType:  "sequential",
		EstimatedDuration: estimateTotalDuration(tasks),
		ResourceUsage: &ResourceUsage{
			EstimatedCPUPct:  45.0,
			EstimatedMemoryMB: 2048,
			EstimatedIO:      "medium",
		},
	}

	for _, task := range tasks {
		path.TaskIDs = append(path.TaskIDs, task.ID)
	}

	return []*ExecutionPath{path}
}

func estimateTotalDuration(tasks []contracts.BrainTaskSpec) string {
	if len(tasks) == 0 {
		return "0h"
	}

	// Estimate based on work type
	totalMinutes := 0
	for _, task := range tasks {
		switch task.WorkType {
		case contracts.WorkTypeDebug:
			totalMinutes += 30
		case contracts.WorkTypeTesting:
			totalMinutes += 45
		case contracts.WorkTypeDocumentation:
			totalMinutes += 60
		case contracts.WorkTypeRefactor:
			totalMinutes += 90
		case contracts.WorkTypeImplementation:
			totalMinutes += 120
		default:
			totalMinutes += 60
		}
	}

	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

func calculateParallelCapacity(paths []*ExecutionPath) int {
	// Count parallel paths
	parallelCount := 0
	for _, path := range paths {
		if path.ExecutionType == "parallel" {
			parallelCount++
		}
	}

	if parallelCount == 0 {
		return 1 // Sequential only
	}

	// Estimate parallel capacity based on resource usage
	// For simplicity, return max 4 parallel tasks
	if parallelCount > 4 {
		return 4
	}
	return parallelCount + 1
}

func assessRisksPerTask(tasks []contracts.BrainTaskSpec) map[string]*TaskRisk {
	risks := make(map[string]*TaskRisk)

	for _, task := range tasks {
		riskFactors := []*RiskFactor{}
		riskLevel := "low"

		// Assess risk based on task type
		if task.WorkType == contracts.WorkTypeRefactor {
			riskLevel = "medium"
			riskFactors = append(riskFactors, &RiskFactor{
				ID:          "refactor-risk",
				Category:    "technical",
				Severity:    "medium",
				Description: "Refactoring has inherent complexity and regression risk",
				Impact:     "quality",
				Likelihood:  "possible",
			})
		}

		if task.Priority == contracts.PriorityCritical || task.Priority == contracts.PriorityHigh {
			if riskLevel == "low" {
				riskLevel = "medium"
			}
			riskFactors = append(riskFactors, &RiskFactor{
				ID:          "priority-risk",
				Category:    "operational",
				Severity:    "medium",
				Description: "High priority task requires careful execution",
				Impact:     "schedule",
				Likelihood:  "possible",
			})
		}

		// Create mitigation steps
		mitigations := []*MitigationStep{}
		for i, factor := range riskFactors {
			mitigations = append(mitigations, &MitigationStep{
				ID:      fmt.Sprintf("mitigate-%s-%d", task.ID, i),
				RiskID:  factor.ID,
				Step:    getMitigationStep(factor),
				Status:  "pending",
			})
		}

		risks[task.ID] = &TaskRisk{
			TaskID:         task.ID,
			RiskLevel:      riskLevel,
			RiskFactors:    riskFactors,
			MitigationPlan: mitigations,
		}
	}

	return risks
}
