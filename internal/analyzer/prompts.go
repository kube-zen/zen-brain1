package analyzer

import (
	"context"
	"fmt"
	"strings"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// classificationStage classifies work items.
type classificationStage struct {
	llm llm.Provider
}

func (s *classificationStage) Name() Stage {
	return StageClassification
}

func (s *classificationStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	prompt := fmt.Sprintf(`Analyze the following work item and classify it:

Title: %s
Description: %s
Source: %s (Issue: %s)

Please classify this work item by answering the following questions:

1. What is the primary work type? (research, design, implementation, debug, refactor, documentation, analysis, operations, security, testing)
2. What is the work domain? (office, factory, sdk, policy, memory, observability, infrastructure, integration, core)
3. What is the appropriate priority? (critical, high, medium, low, background)
4. What knowledge base scopes are relevant? (comma-separated list)
5. Confidence in classification (0.0-1.0)

Format your response as:
WorkType: <type>
WorkDomain: <domain>
Priority: <priority>
KBScopes: <scope1, scope2, ...>
Confidence: <confidence>

Example:
WorkType: implementation
WorkDomain: core
Priority: medium
KBScopes: api-gateway, rate-limiting
Confidence: 0.85`,
		workItem.Title, workItem.Body, workItem.Source.System, workItem.Source.IssueKey)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a software engineering classifier. Be precise and consistent."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 500,
		Temperature: 0.1,
	}

	resp, err := s.llm.Chat(ctx, req)
	if err != nil {
		return StageResult{Stage: StageClassification, Confidence: 0.0}, fmt.Errorf("LLM classification failed: %w", err)
	}

	// Parse response
	result := StageResult{
		Stage:      StageClassification,
		Input:      workItem,
		Output:     make(map[string]interface{}),
		Confidence: 0.7, // Default
		Notes:      resp.Content,
	}

	// Simple parsing (in production, use structured output)
	lines := strings.Split(resp.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "WorkType:") {
			workType := strings.TrimSpace(strings.TrimPrefix(line, "WorkType:"))
			result.Output["work_type"] = workType
		} else if strings.HasPrefix(line, "WorkDomain:") {
			workDomain := strings.TrimSpace(strings.TrimPrefix(line, "WorkDomain:"))
			result.Output["work_domain"] = workDomain
		} else if strings.HasPrefix(line, "Priority:") {
			priority := strings.TrimSpace(strings.TrimPrefix(line, "Priority:"))
			result.Output["priority"] = priority
		} else if strings.HasPrefix(line, "KBScopes:") {
			scopesStr := strings.TrimSpace(strings.TrimPrefix(line, "KBScopes:"))
			scopes := strings.Split(scopesStr, ",")
			for i := range scopes {
				scopes[i] = strings.TrimSpace(scopes[i])
			}
			result.Output["kb_scopes"] = scopes
		} else if strings.HasPrefix(line, "Confidence:") {
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "Confidence:"))
			var confidence float64
			fmt.Sscanf(confStr, "%f", &confidence)
			result.Confidence = confidence
		}
	}

	return result, nil
}

// requirementsStage extracts requirements and constraints.
type requirementsStage struct {
	llm llm.Provider
}

func (s *requirementsStage) Name() Stage {
	return StageRequirements
}

func (s *requirementsStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	prompt := fmt.Sprintf(`Extract requirements and constraints from this work item:

Title: %s
Description: %s

Please extract:
1. Clear objective (what needs to be done)
2. Acceptance criteria (list of conditions for success)
3. Constraints (technical, time, resource, or other limitations)
4. Dependencies (other work items or systems this depends on)

Format your response as:
Objective: <clear objective statement>
AcceptanceCriteria: <criterion 1>; <criterion 2>; ...
Constraints: <constraint 1>; <constraint 2>; ...
Dependencies: <dependency 1>; <dependency 2>; ...`,
		workItem.Title, workItem.Body)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a requirements analyst. Extract clear, actionable requirements."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 1000,
		Temperature: 0.1,
	}

	resp, err := s.llm.Chat(ctx, req)
	if err != nil {
		return StageResult{Stage: StageRequirements, Confidence: 0.0}, fmt.Errorf("LLM requirements extraction failed: %w", err)
	}

	result := StageResult{
		Stage:      StageRequirements,
		Input:      workItem,
		Output:     make(map[string]interface{}),
		Confidence: 0.8,
		Notes:      resp.Content,
	}

	// Parse response
	lines := strings.Split(resp.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Objective:") {
			objective := strings.TrimSpace(strings.TrimPrefix(line, "Objective:"))
			result.Output["objective"] = objective
		} else if strings.HasPrefix(line, "AcceptanceCriteria:") {
			criteriaStr := strings.TrimSpace(strings.TrimPrefix(line, "AcceptanceCriteria:"))
			criteria := strings.Split(criteriaStr, ";")
			for i := range criteria {
				criteria[i] = strings.TrimSpace(criteria[i])
			}
			result.Output["acceptance_criteria"] = criteria
		} else if strings.HasPrefix(line, "Constraints:") {
			constraintsStr := strings.TrimSpace(strings.TrimPrefix(line, "Constraints:"))
			constraints := strings.Split(constraintsStr, ";")
			for i := range constraints {
				constraints[i] = strings.TrimSpace(constraints[i])
			}
			result.Output["constraints"] = constraints
		} else if strings.HasPrefix(line, "Dependencies:") {
			depsStr := strings.TrimSpace(strings.TrimPrefix(line, "Dependencies:"))
			deps := strings.Split(depsStr, ";")
			for i := range deps {
				deps[i] = strings.TrimSpace(deps[i])
			}
			result.Output["dependencies"] = deps
		}
	}

	return result, nil
}

// breakdownStage breaks down work into subtasks.
type breakdownStage struct {
	llm llm.Provider
}

func (s *breakdownStage) Name() Stage {
	return StageBreakdown
}

func (s *breakdownStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	// Skip breakdown for simple tasks
	if workItem.WorkType == contracts.WorkTypeDebug || 
	   workItem.WorkType == contracts.WorkTypeDocumentation ||
	   workItem.Priority == contracts.PriorityBackground {
		return StageResult{
			Stage:      StageBreakdown,
			Input:      workItem,
			Output:     map[string]interface{}{"subtasks": []string{"Single task"}},
			Confidence: 1.0,
			Notes:      "Simple task, no breakdown needed",
		}, nil
	}

	prompt := fmt.Sprintf(`Break down this work item into subtasks:

Title: %s
Description: %s
Work Type: %s

Break this down into 2-5 logical subtasks that could be executed independently.
For each subtask, provide a brief description.

Format your response as:
Subtasks:
1. <subtask 1 description>
2. <subtask 2 description>
...`,
		workItem.Title, workItem.Body, workItem.WorkType)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a project planner. Break down work into logical, executable subtasks."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 800,
		Temperature: 0.1,
	}

	resp, err := s.llm.Chat(ctx, req)
	if err != nil {
		return StageResult{Stage: StageBreakdown, Confidence: 0.0}, fmt.Errorf("LLM breakdown failed: %w", err)
	}

	result := StageResult{
		Stage:      StageBreakdown,
		Input:      workItem,
		Output:     make(map[string]interface{}),
		Confidence: 0.75,
		Notes:      resp.Content,
	}

	// Parse subtasks
	var subtasks []string
	lines := strings.Split(resp.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || 
		   strings.HasPrefix(line, "3.") || strings.HasPrefix(line, "4.") || 
		   strings.HasPrefix(line, "5.") || strings.HasPrefix(line, "6.") {
			// Remove number prefix
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				subtasks = append(subtasks, strings.TrimSpace(parts[1]))
			}
		}
	}

	if len(subtasks) == 0 {
		subtasks = []string{"Single task"}
	}

	result.Output["subtasks"] = subtasks
	result.Output["subtask_count"] = len(subtasks)

	return result, nil
}

// evidenceStage determines evidence requirements and SR&ED hypothesis.
type evidenceStage struct {
	llm llm.Provider
}

func (s *evidenceStage) Name() Stage {
	return StageEvidence
}

func (s *evidenceStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	// Skip SR&ED if disabled
	if workItem.SREDDisabled {
		return StageResult{
			Stage:      StageEvidence,
			Input:      workItem,
			Output:     map[string]interface{}{"sred_disabled": true},
			Confidence: 1.0,
			Notes:      "SR&ED evidence collection disabled for this work item",
		}, nil
	}

	prompt := fmt.Sprintf(`Analyze this work item for SR&ED (Scientific Research & Experimental Development) eligibility:

Title: %s
Description: %s
Work Type: %s
Work Domain: %s

Please provide:
1. SR&ED hypothesis (what technical uncertainty are we trying to resolve?)
2. Suggested SR&ED tags from: u1_dynamic_provisioning, u2_security_gates, u3_deterministic_delivery, u4_backpressure, experimental_general
3. Evidence requirements (what proof of work is needed?)

Format your response as:
Hypothesis: <SR&ED hypothesis statement>
SREDTags: <tag1, tag2, ...>
EvidenceRequirements: <requirement1; requirement2; ...>`,
		workItem.Title, workItem.Body, workItem.WorkType, workItem.WorkDomain)

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are an SR&ED analyst. Identify eligible research and development activities."},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 600,
		Temperature: 0.1,
	}

	resp, err := s.llm.Chat(ctx, req)
	if err != nil {
		return StageResult{Stage: StageEvidence, Confidence: 0.0}, fmt.Errorf("LLM evidence analysis failed: %w", err)
	}

	result := StageResult{
		Stage:      StageEvidence,
		Input:      workItem,
		Output:     make(map[string]interface{}),
		Confidence: 0.6, // SR&ED analysis is inherently uncertain
		Notes:      resp.Content,
	}

	// Parse response
	lines := strings.Split(resp.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hypothesis:") {
			hypothesis := strings.TrimSpace(strings.TrimPrefix(line, "Hypothesis:"))
			result.Output["hypothesis"] = hypothesis
		} else if strings.HasPrefix(line, "SREDTags:") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(line, "SREDTags:"))
			tags := strings.Split(tagsStr, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			result.Output["sred_tags"] = tags
		} else if strings.HasPrefix(line, "EvidenceRequirements:") {
			reqsStr := strings.TrimSpace(strings.TrimPrefix(line, "EvidenceRequirements:"))
			reqs := strings.Split(reqsStr, ";")
			for i := range reqs {
				reqs[i] = strings.TrimSpace(reqs[i])
			}
			result.Output["evidence_requirements"] = reqs
		}
	}

	return result, nil
}

// costEstimationStage estimates cost.
type costEstimationStage struct {
	llm    llm.Provider
	config *Config
}

func (s *costEstimationStage) Name() Stage {
	return StageCostEstimation
}

func (s *costEstimationStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	// Simple cost estimation based on work type and priority
	baseCosts := map[contracts.WorkType]float64{
		contracts.WorkTypeResearch:       5.0,
		contracts.WorkTypeDesign:         3.0,
		contracts.WorkTypeImplementation: 2.0,
		contracts.WorkTypeDebug:          1.0,
		contracts.WorkTypeRefactor:       2.5,
		contracts.WorkTypeDocumentation:  1.5,
		contracts.WorkTypeAnalysis:       2.0,
		contracts.WorkTypeOperations:     2.0,
		contracts.WorkTypeSecurity:       3.0,
		contracts.WorkTypeTesting:        1.5,
	}

	priorityMultipliers := map[contracts.Priority]float64{
		contracts.PriorityCritical:  2.0,
		contracts.PriorityHigh:      1.5,
		contracts.PriorityMedium:    1.0,
		contracts.PriorityLow:       0.7,
		contracts.PriorityBackground: 0.5,
	}

	baseCost := baseCosts[workItem.WorkType]
	if baseCost == 0 {
		baseCost = 2.0 // Default
	}

	multiplier := priorityMultipliers[workItem.Priority]
	if multiplier == 0 {
		multiplier = 1.0
	}

	estimatedCost := baseCost * multiplier

	// Adjust based on breakdown if available
	if breakdown, ok := prevResults[StageBreakdown]; ok {
		if count, ok := breakdown.Output["subtask_count"].(int); ok && count > 1 {
			estimatedCost *= float64(count)
		}
	}

	// Cap at max cost
	if s.config.MaxCostUSD > 0 && estimatedCost > s.config.MaxCostUSD {
		estimatedCost = s.config.MaxCostUSD
	}

	result := StageResult{
		Stage:      StageCostEstimation,
		Input:      workItem,
		Output:     map[string]interface{}{"estimated_cost_usd": estimatedCost},
		Confidence: 0.5, // Cost estimation is uncertain
		Notes:      fmt.Sprintf("Estimated cost: $%.2f (base: $%.2f * priority multiplier: %.1f)", estimatedCost, baseCost, multiplier),
	}

	return result, nil
}

// finalizationStage produces final analysis.
type finalizationStage struct {
	llm llm.Provider
}

func (s *finalizationStage) Name() Stage {
	return StageFinalization
}

func (s *finalizationStage) Process(ctx context.Context, workItem *contracts.WorkItem, prevResults map[Stage]StageResult) (StageResult, error) {
	// Summarize all stage results
	var summary strings.Builder
	summary.WriteString("Analysis Summary:\n")
	
	for stage, result := range prevResults {
		summary.WriteString(fmt.Sprintf("- %s: confidence %.2f", stage, result.Confidence))
		if len(result.Errors) > 0 {
			summary.WriteString(fmt.Sprintf(" (errors: %d)", len(result.Errors)))
		}
		summary.WriteString("\n")
	}

	result := StageResult{
		Stage:      StageFinalization,
		Input:      workItem,
		Output:     map[string]interface{}{"summary": summary.String()},
		Confidence: 1.0, // Finalization always succeeds
		Notes:      "Analysis pipeline completed",
	}

	return result, nil
}