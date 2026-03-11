// Package analyzer provides the Intent Analyzer for zen-brain.
package analyzer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	internalllm "github.com/kube-zen/zen-brain1/internal/llm"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/kb"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// DefaultAnalyzer is the default implementation of IntentAnalyzer.
type DefaultAnalyzer struct {
	config        *Config
	llm           llm.Provider
	kbStore       kb.Store
	promptManager *internalllm.PromptManager
	pipeline      []StageProcessor
	// HistoryStore optional: when set, analysis results are persisted and GetAnalysisHistory/UpdateAnalysis use it (Block 2 enterprise).
	HistoryStore AnalysisHistoryStore
}

// StageProcessor processes a single stage of analysis.
type StageProcessor interface {
	Name() Stage
	Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error)
}

// New creates a new DefaultAnalyzer.
func New(config *Config, llmProvider llm.Provider, kbStore kb.Store) (*DefaultAnalyzer, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if llmProvider == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}

	analyzer := &DefaultAnalyzer{
		config:        config,
		llm:           llmProvider,
		kbStore:       kbStore,
		promptManager: internalllm.InitializeDefaultManager(),
	}

	// Build pipeline based on enabled stages
	analyzer.buildPipeline()

	return analyzer, nil
}

// buildPipeline builds the stage processors based on configuration.
func (a *DefaultAnalyzer) buildPipeline() {
	a.pipeline = make([]StageProcessor, 0, len(a.config.EnabledStages))

	for _, stage := range a.config.EnabledStages {
		var processor StageProcessor

		switch stage {
		case StageClassification:
			processor = &classificationStage{llm: a.llm, promptManager: a.promptManager}
		case StageRequirements:
			processor = &requirementsStage{llm: a.llm, promptManager: a.promptManager}
		case StageBreakdown:
			processor = &breakdownStage{llm: a.llm, promptManager: a.promptManager}
		case StageEvidence:
			processor = &evidenceStage{llm: a.llm, promptManager: a.promptManager}
		case StageCostEstimation:
			processor = &costEstimationStage{llm: a.llm, config: a.config, promptManager: a.promptManager}
		case StageFinalization:
			processor = &finalizationStage{llm: a.llm, promptManager: a.promptManager}
		default:
			log.Printf("Warning: Unknown stage %s, skipping", stage)
			continue
		}

		a.pipeline = append(a.pipeline, processor)
	}
}

// Analyze analyzes a work item and produces BrainTask specifications.
func (a *DefaultAnalyzer) Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error) {
	startTime := time.Now()

	// Execute pipeline stages
	stageResults := make(map[Stage]StageResult)
	var errors []string

	for _, processor := range a.pipeline {
		stageStart := time.Now()

		result, err := processor.Process(ctx, workItem, stageResults)
		result.DurationMs = time.Since(stageStart).Milliseconds()

		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			errors = append(errors, fmt.Sprintf("Stage %s failed: %v", processor.Name(), err))

			// Continue with next stage if possible
			if a.shouldContinueAfterError(processor.Name()) {
				log.Printf("Stage %s failed but continuing: %v", processor.Name(), err)
			} else {
				break
			}
		}

		stageResults[processor.Name()] = result
	}

	// Combine stage results into final BrainTaskSpecs
	brainTaskSpecs, err := a.combineStageResults(ctx, workItem, stageResults)
	if err != nil {
		return nil, fmt.Errorf("failed to combine stage results: %w", err)
	}

	// Calculate overall confidence (average of stage confidences)
	var totalConfidence float64
	stageCount := 0
	for _, result := range stageResults {
		if len(result.Errors) == 0 {
			totalConfidence += result.Confidence
			stageCount++
		}
	}

	overallConfidence := 0.0
	if stageCount > 0 {
		overallConfidence = totalConfidence / float64(stageCount)
	}

	// Build final result
	result := &contracts.AnalysisResult{
		WorkItem:              workItem,
		BrainTaskSpecs:        brainTaskSpecs,
		Confidence:            overallConfidence,
		AnalysisNotes:         a.buildAnalysisNotes(stageResults, errors),
		RequiresApproval:      a.config.RequireApproval,
		EstimatedTotalCostUSD: a.estimateTotalCost(brainTaskSpecs),
	}
	
	// Enrich with audit, rich analysis, and other enhancements
	a.enrichResult(result, workItem)

	if a.HistoryStore != nil {
		if err := a.HistoryStore.Store(ctx, workItem.ID, result); err != nil {
			log.Printf("Failed to store analysis history for %s: %v", workItem.ID, err)
		}
	}

	log.Printf("Analyzed work item %s in %v: %d tasks, confidence %.2f",
		workItem.ID, time.Since(startTime), len(brainTaskSpecs), overallConfidence)

	return result, nil
}

// enrichResult sets audit and snapshot fields for durability and auditability.
func (a *DefaultAnalyzer) enrichResult(result *contracts.AnalysisResult, workItem *contracts.WorkItem) {
	EnrichForAudit(result, workItem, a.config.AnalyzedBy, a.config.AnalyzerVersion)
}

// AnalyzeRich performs analysis and returns enhanced rich result with operator-facing content.
// This is the recommended method for CLI and API consumers that want rich output.
func (a *DefaultAnalyzer) AnalyzeRich(ctx context.Context, workItem *contracts.WorkItem) (*RichAnalysisResult, error) {
	// Get base analysis
	base, err := a.Analyze(ctx, workItem)
	if err != nil {
		return nil, err
	}

	// Enrich with rich operator-facing content
	return EnrichForRichAnalysis(base, workItem), nil
}

// EnrichForAudit sets AnalyzedAt, AnalyzedBy, AnalyzerVersion, and WorkItemSnapshot on result (Block 2 enterprise).
// Use from DefaultAnalyzer or simpleAnalyzer before persisting. analyzedBy/version default to "zen-brain"/"1.0" if empty.
func EnrichForAudit(result *contracts.AnalysisResult, workItem *contracts.WorkItem, analyzedBy, analyzerVersion string) {
	result.AnalyzedAt = time.Now().UTC()
	result.AnalyzedBy = analyzedBy
	if result.AnalyzedBy == "" {
		result.AnalyzedBy = "zen-brain"
	}
	result.AnalyzerVersion = analyzerVersion
	if result.AnalyzerVersion == "" {
		result.AnalyzerVersion = "1.0"
	}
	if workItem != nil {
		result.WorkItemSnapshot = &contracts.WorkItemSnapshot{
			ID:         workItem.ID,
			SourceKey:  workItem.Source.IssueKey,
			Title:      workItem.Title,
			WorkType:   string(workItem.WorkType),
			WorkDomain: string(workItem.WorkDomain),
		}
	}
}

// AnalyzeBatch analyzes multiple work items in batch.
func (a *DefaultAnalyzer) AnalyzeBatch(ctx context.Context, workItems []*contracts.WorkItem) ([]*contracts.AnalysisResult, error) {
	results := make([]*contracts.AnalysisResult, 0, len(workItems))

	for _, workItem := range workItems {
		result, err := a.Analyze(ctx, workItem)
		if err != nil {
			log.Printf("Failed to analyze work item %s: %v", workItem.ID, err)
			// Continue with remaining items
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// GetAnalysisHistory returns analysis history for a work item (from HistoryStore when set).
func (a *DefaultAnalyzer) GetAnalysisHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error) {
	if a.HistoryStore != nil {
		return a.HistoryStore.GetHistory(ctx, workItemID)
	}
	return nil, fmt.Errorf("analysis history not available: HistoryStore not configured")
}

// UpdateAnalysis appends an updated analysis to history (Store when HistoryStore is set).
func (a *DefaultAnalyzer) UpdateAnalysis(ctx context.Context, result *contracts.AnalysisResult) error {
	if a.HistoryStore == nil {
		return fmt.Errorf("analysis history not available: HistoryStore not configured")
	}
	if result == nil || result.WorkItem == nil {
		return fmt.Errorf("result and result.WorkItem are required")
	}
	a.enrichResult(result, result.WorkItem)
	return a.HistoryStore.Store(ctx, result.WorkItem.ID, result)
}

// shouldContinueAfterError determines if pipeline should continue after a stage error.
func (a *DefaultAnalyzer) shouldContinueAfterError(stage Stage) bool {
	// Critical stages that should stop the pipeline
	criticalStages := map[Stage]bool{
		StageClassification: true,
		StageFinalization:   true,
	}

	return !criticalStages[stage]
}

// combineStageResults combines stage results into BrainTaskSpecs.
func (a *DefaultAnalyzer) combineStageResults(ctx context.Context, workItem *contracts.WorkItem, stageResults map[Stage]StageResult) ([]contracts.BrainTaskSpec, error) {
	// Extract classification
	classification, ok := stageResults[StageClassification]
	if !ok {
		return nil, fmt.Errorf("classification stage required")
	}

	// Extract requirements
	requirements, _ := stageResults[StageRequirements]

	// Extract breakdown
	breakdown, _ := stageResults[StageBreakdown]

	// Extract evidence
	evidence, _ := stageResults[StageEvidence]

	// Use classification results if available, otherwise use work item defaults
	workType := workItem.WorkType
	workDomain := workItem.WorkDomain
	priority := workItem.Priority
	kbScopes := workItem.KBScopes

	if classification.Output != nil {
		if wt, ok := classification.Output["work_type"].(string); ok && wt != "" {
			workType = contracts.WorkType(wt)
		}
		if wd, ok := classification.Output["work_domain"].(string); ok && wd != "" {
			workDomain = contracts.WorkDomain(wd)
		}
		if p, ok := classification.Output["priority"].(string); ok && p != "" {
			priority = contracts.Priority(p)
		}
		if scopes, ok := classification.Output["kb_scopes"].([]string); ok && len(scopes) > 0 {
			kbScopes = scopes
		}
	}

	baseSpec := contracts.BrainTaskSpec{
		ID:                  fmt.Sprintf("%s-%d", workItem.ID, time.Now().Unix()),
		Title:               workItem.Title,
		Description:         workItem.Body,
		WorkItemID:          workItem.ID,
		SourceKey:           workItem.Source.IssueKey,
		WorkType:            workType,
		WorkDomain:          workDomain,
		Priority:            priority,
		Objective:           a.extractObjective(workItem, requirements),
		AcceptanceCriteria:  a.extractAcceptanceCriteria(requirements),
		Constraints:         a.extractConstraints(requirements),
		EvidenceRequirement: workItem.EvidenceRequirement,
		SREDTags:            workItem.Tags.SRED,
		Hypothesis:          a.extractHypothesis(evidence),
		EstimatedCostUSD:    a.extractEstimatedCost(stageResults),
		TimeoutSeconds:      3600, // Default 1 hour
		MaxRetries:          3,
		KBScopes:            kbScopes,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	// Multi-task breakdown: when breakdown stage produced multiple subtasks, create one BrainTaskSpec per subtask.
	subtasks := a.extractSubtasksFromBreakdown(breakdown)
	if len(subtasks) > 1 {
		specs := make([]contracts.BrainTaskSpec, 0, len(subtasks))
		for i, title := range subtasks {
			spec := baseSpec
			spec.ID = fmt.Sprintf("%s-%d-%d", workItem.ID, time.Now().Unix(), i+1)
			spec.Title = title
			spec.Objective = title
			specs = append(specs, spec)
		}
		return specs, nil
	}

	return []contracts.BrainTaskSpec{baseSpec}, nil
}

// extractSubtasksFromBreakdown returns subtask titles from breakdown stage output ([]string or []interface{}).
func (a *DefaultAnalyzer) extractSubtasksFromBreakdown(breakdown StageResult) []string {
	if breakdown.Output == nil {
		return nil
	}
	// Direct []string
	if s, ok := breakdown.Output["subtasks"].([]string); ok && len(s) > 0 {
		return s
	}
	// From JSON/LLM: []interface{}
	if raw, ok := breakdown.Output["subtasks"].([]interface{}); ok {
		var out []string
		for _, v := range raw {
			if str, ok := v.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

// Helper methods for extracting information from stage results
func (a *DefaultAnalyzer) extractObjective(workItem *contracts.WorkItem, requirements StageResult) string {
	if obj, ok := requirements.Output["objective"].(string); ok && obj != "" {
		return obj
	}
	return workItem.Body
}

func (a *DefaultAnalyzer) extractAcceptanceCriteria(requirements StageResult) []string {
	if criteria, ok := requirements.Output["acceptance_criteria"].([]string); ok {
		return criteria
	}
	return nil
}

func (a *DefaultAnalyzer) extractConstraints(requirements StageResult) []string {
	if constraints, ok := requirements.Output["constraints"].([]string); ok {
		return constraints
	}
	return nil
}

func (a *DefaultAnalyzer) extractHypothesis(evidence StageResult) string {
	if hypothesis, ok := evidence.Output["hypothesis"].(string); ok {
		return hypothesis
	}
	return ""
}

func (a *DefaultAnalyzer) extractEstimatedCost(stageResults map[Stage]StageResult) float64 {
	if costEstimation, ok := stageResults[StageCostEstimation]; ok {
		if cost, ok := costEstimation.Output["estimated_cost_usd"].(float64); ok {
			return cost
		}
	}
	return 0.0
}

func (a *DefaultAnalyzer) estimateTotalCost(specs []contracts.BrainTaskSpec) float64 {
	var total float64
	for _, spec := range specs {
		total += spec.EstimatedCostUSD
	}
	return total
}

func (a *DefaultAnalyzer) buildAnalysisNotes(stageResults map[Stage]StageResult, errors []string) string {
	var notes strings.Builder

	notes.WriteString("Analysis completed with the following stages:\n")
	for stage, result := range stageResults {
		notes.WriteString(fmt.Sprintf("- %s: confidence %.2f", stage, result.Confidence))
		if len(result.Errors) > 0 {
			notes.WriteString(fmt.Sprintf(" (errors: %s)", strings.Join(result.Errors, ", ")))
		}
		notes.WriteString("\n")
	}

	if len(errors) > 0 {
		notes.WriteString("\nPipeline errors:\n")
		for _, err := range errors {
			notes.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return notes.String()
}
