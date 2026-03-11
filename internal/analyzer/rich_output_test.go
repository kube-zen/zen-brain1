package analyzer

import (
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestEnrichForRichAnalysis(t *testing.T) {
	now := time.Now()

	workItem := &contracts.WorkItem{
		ID:    "ZB-123",
		Title:  "Test work item",
		Source: contracts.WorkItemSource{
			IssueKey: "ZB-123",
			Provider: "jira",
		},
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.WorkDomainCore,
		Priority:   2, // medium
	}

	base := &contracts.AnalysisResult{
		WorkItem:         workItem,
		BrainTaskSpecs:   []*contracts.BrainTaskSpec{{ID: "task-1", Title: "Task 1"}},
		Confidence:       0.85,
		AnalysisNotes:     []string{"Note 1", "Note 2"},
		RequiresApproval: true,
		AnalyzerVersion:   "zen-brain",
		AnalyzedAt:       now,
	}

	rich := EnrichForRichAnalysis(base, workItem)

	// Verify basic fields
	if rich.AnalysisResult != base {
		t.Error("AnalysisResult not embedded")
	}

	if rich.ReplayID == "" {
		t.Error("ReplayID should not be empty")
	}

	if rich.CorrelationID == "" {
		t.Error("CorrelationID should not be empty")
	}

	if len(rich.ActionItems) == 0 {
		t.Error("Expected at least one action item")
	}

	// Verify audit trail
	if rich.AuditTrail == nil {
		t.Error("Audit trail should not be nil")
	}

	if rich.AuditTrail.WorkItemID != workItem.ID {
		t.Errorf("Expected work item ID %s, got: %s", workItem.ID, rich.AuditTrail.WorkItemID)
	}

	if rich.AuditTrail.JiraKey != workItem.Source.IssueKey {
		t.Errorf("Expected Jira key %s, got: %s", workItem.Source.IssueKey, rich.AuditTrail.JiraKey)
	}

	// Verify risk assessment
	if rich.RiskAssessment == nil {
		t.Error("Risk assessment should not be nil")
	}

	// Verify summaries
	if rich.ExecutiveSummary == "" {
		t.Error("Executive summary should not be empty")
	}

	if rich.TechnicalSummary == "" {
		t.Error("Technical summary should not be empty")
	}

	t.Logf("✅ Rich analysis enrichment works correctly")
	t.Logf("  Executive: %s", rich.ExecutiveSummary)
	t.Logf("  Action items: %d", len(rich.ActionItems))
	t.Logf("  Risk level: %s", rich.RiskAssessment.OverallRisk)
}

func TestAssessRisks(t *testing.T) {
	t.Run("Low risk work item", func(t *testing.T) {
		workItem := &contracts.WorkItem{
			ID:         "ZB-123",
			Title:       "Simple task",
			WorkType:    contracts.WorkTypeImplementation,
			Priority:     0, // Priority = 0 is low
		}

		result := &contracts.AnalysisResult{
			Confidence: 0.9, // high confidence
		}

		risk := assessRisks(workItem, result)

		if risk.OverallRisk == "critical" || risk.OverallRisk == "high" {
			t.Errorf("Expected low/medium risk, got: %s", risk.OverallRisk)
		}

		if len(risk.MitigationSteps) == 0 {
			t.Error("Expected mitigation steps")
		}

		t.Logf("✅ Low risk assessment: %s", risk.OverallRisk)
	})

	t.Run("High priority risk", func(t *testing.T) {
		workItem := &contracts.WorkItem{
			ID:         "ZB-123",
			Title:       "Critical task",
			WorkType:    contracts.WorkTypeBugFix,
			Priority:     3, // Priority = 3 is high
		}

		result := &contracts.AnalysisResult{
			Confidence: 0.7,
		}

		risk := assessRisks(workItem, result)

		if risk.OverallRisk == "low" {
			t.Error("Expected higher risk for high priority")
		}

		foundPriorityRisk := false
		for _, factor := range risk.RiskFactors {
			if factor.ID == "priority-risk" {
				foundPriorityRisk = true
				break
			}
		}

		if !foundPriorityRisk {
			t.Error("Expected priority risk factor")
		}

		t.Logf("✅ High priority risk: %s", risk.OverallRisk)
	})

	t.Run("Refactor complexity risk", func(t *testing.T) {
		workItem := &contracts.WorkItem{
			ID:         "ZB-123",
			Title:       "Refactor legacy code",
			WorkType:    contracts.WorkTypeRefactor,
			Priority:     2,
		}

		result := &contracts.AnalysisResult{
			Confidence: 0.8,
		}

		risk := assessRisks(workItem, result)

		foundComplexityRisk := false
		for _, factor := range risk.RiskFactors {
			if factor.ID == "complexity-risk" {
				foundComplexityRisk = true
				break
			}
		}

		if !foundComplexityRisk {
			t.Error("Expected complexity risk factor")
		}

		t.Logf("✅ Refactor complexity risk: %s", risk.OverallRisk)
	})

	t.Run("Low confidence risk", func(t *testing.T) {
		workItem := &contracts.WorkItem{
			ID:         "ZB-123",
			Title:       "Unclear task",
			WorkType:    contracts.WorkTypeImplementation,
			Priority:     1,
		}

		result := &contracts.AnalysisResult{
			Confidence: 0.5, // low
		}

		risk := assessRisks(workItem, result)

		foundConfidenceRisk := false
		for _, factor := range risk.RiskFactors {
			if factor.ID == "confidence-risk" {
				foundConfidenceRisk = true
				break
			}
		}

		if !foundConfidenceRisk {
			t.Error("Expected confidence risk factor")
		}

		t.Logf("✅ Low confidence risk: %s", risk.OverallRisk)
	})
}

func TestGenerateActionItems(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:    "ZB-123",
		Title:  "Test work item",
		Source: contracts.WorkItemSource{
			IssueKey: "ZB-123",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:        workItem,
		BrainTaskSpecs: []*contracts.BrainTaskSpec{
			{ID: "task-1", Title: "Task 1", WorkType: contracts.WorkTypeImplementation},
			{ID: "task-2", Title: "Task 2", WorkType: contracts.WorkTypeBugFix},
		},
		RequiresApproval: true,
	}

	actions := generateActionItems(workItem, result)

	if len(actions) < 3 { // review + approve + tasks
		t.Errorf("Expected at least 3 action items, got: %d", len(actions))
	}

	// Check for review action
	hasReviewAction := false
	for _, action := range actions {
		if action.ID == "review-analysis" {
			hasReviewAction = true
			if action.Priority != "high" {
				t.Errorf("Review action should be high priority, got: %s", action.Priority)
			}
			break
		}
	}

	if !hasReviewAction {
		t.Error("Expected review action")
	}

	// Check for approve action
	hasApproveAction := false
	for _, action := range actions {
		if action.ID == "approve-tasks" {
			hasApproveAction = true
			if action.Priority != "medium" {
				t.Errorf("Approve action should be medium priority, got: %s", action.Priority)
			}
			if len(action.DependsOn) != 1 {
				t.Errorf("Approve action should depend on review, got: %v", action.DependsOn)
			}
			break
		}
	}

	if !hasApproveAction {
		t.Error("Expected approve action")
	}

	t.Logf("✅ Generated %d action items", len(actions))
}

func TestGenerateExecutiveSummary(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:         "ZB-123",
		Title:       "Fix authentication bug",
		Source: contracts.WorkItemSource{
			IssueKey: "ZB-123",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:         workItem,
		BrainTaskSpecs:   []*contracts.BrainTaskSpec{{ID: "task-1"}},
		Confidence:       0.85,
		AnalyzerVersion:  "zen-brain",
		RequiresApproval: false,
	}

	risk := &RiskAssessment{
		OverallRisk: "medium",
	}

	summary := generateExecutiveSummary(workItem, result, risk)

	// Check for key elements
	contains := func(s, substr string) bool {
		return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
	}

	indexOf := func(s, substr string) int {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return i
			}
		}
		return -1
	}

	if !contains(summary, "ZB-123") {
		t.Error("Summary should contain work item ID")
	}

	if !contains(summary, "85%") {
		t.Error("Summary should contain confidence percentage")
	}

	if !contains(summary, "1 task") {
		t.Error("Summary should contain task count")
	}

	if !contains(summary, "medium") {
		t.Error("Summary should contain risk level")
	}

	t.Logf("✅ Executive summary: %s", summary)
}

func TestBuildJiraLinkage(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:    "ZB-123",
		Title:  "Parent task",
		Source: contracts.SourceMetadata{
			IssueKey: "ZB-123",
			System: "jira",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:      workItem,
		BrainTaskSpecs: []*contracts.BrainTaskSpec{
			{ID: "task-1"},
		},
	}

	linkage := buildJiraLinkage(workItem, result)

	if len(linkage) < 1 {
		t.Error("Expected at least one Jira correlation")
	}

	// Check for parent correlation
	foundParent := false
	for _, corr := range linkage {
		if corr.CorrelationType == "parent" && corr.TargetJiraKey == "ZB-123" {
			foundParent = true
			if !corr.Verified {
				t.Error("Parent correlation should be verified")
			}
			break
		}
	}

	if !foundParent {
		t.Error("Expected parent correlation to work item")
	}

	t.Logf("✅ Built %d Jira correlations", len(linkage))
}

func TestBuildTaskChain(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID: "ZB-123",
	}

	result := &contracts.AnalysisResult{
		WorkItem:      workItem,
		BrainTaskSpecs: []*contracts.BrainTaskSpec{
			{ID: "task-1"},
			{ID: "task-2"},
			{ID: "task-3"},
		},
	}

	chain := buildTaskChain(result)

	if len(chain) != 3 {
		t.Errorf("Expected 3 task linkages, got: %d", len(chain))
	}

	// Check first task has no dependencies
	if len(chain[0].DependsOnTaskIDs) != 0 {
		t.Errorf("First task should have no dependencies, got: %v", chain[0].DependsOnTaskIDs)
	}

	// Check second task depends on first
	if len(chain[1].DependsOnTaskIDs) != 1 || chain[1].DependsOnTaskIDs[0] != "task-1" {
		t.Errorf("Second task should depend on task-1, got: %v", chain[1].DependsOnTaskIDs)
	}

	// Check third task depends on second
	if len(chain[2].DependsOnTaskIDs) != 1 || chain[2].DependsOnTaskIDs[0] != "task-2" {
		t.Errorf("Third task should depend on task-2, got: %v", chain[2].DependsOnTaskIDs)
	}

	t.Logf("✅ Built task chain with %d dependencies", len(chain))
}

func TestAnalysisAuditTrail(t *testing.T) {
	now := time.Now()

	workItem := &contracts.WorkItem{
		ID:    "ZB-123",
		Title:  "Test work item",
		Source: contracts.SourceMetadata{
			IssueKey: "ZB-123",
			System: "jira",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:      workItem,
		BrainTaskSpecs: []*contracts.BrainTaskSpec{{ID: "task-1"}},
		AnalyzedAt:    now,
		AnalyzerVersion: "zen-brain",
	}

	trail := &AnalysisAuditTrail{
		AnalysisID:   generateAnalysisID(result, now),
		WorkItemID:  workItem.ID,
		JiraKey:      workItem.Source.IssueKey,
		WorkItemSource: inferSource(workItem),
		AnalysisChain: buildAnalysisChain(result),
		TaskChain:     buildTaskChain(result),
		JiraLinkage:  buildJiraLinkage(workItem, result),
		CustodyStart:  now,
		CustodyEnd:    now.Add(time.Minute),
		ChainOfTrust:  []string{"zen-brain", "1.0", "jira"},
		Verified:       false,
	}

	if trail.AnalysisID == "" {
		t.Error("Analysis ID should not be empty")
	}

	if trail.WorkItemID != workItem.ID {
		t.Errorf("Expected work item ID %s, got: %s", workItem.ID, trail.WorkItemID)
	}

	if trail.JiraKey != workItem.Source.IssueKey {
		t.Errorf("Expected Jira key %s, got: %s", workItem.Source.IssueKey, trail.JiraKey)
	}

	if len(trail.ChainOfTrust) == 0 {
		t.Error("Chain of trust should not be empty")
	}

	if len(trail.AnalysisChain) == 0 {
		t.Error("Analysis chain should not be empty")
	}

	if len(trail.TaskChain) == 0 {
		t.Error("Task chain should not be empty")
	}

	t.Logf("✅ Audit trail: %s", trail.AnalysisID)
	t.Logf("  Chain of trust: %v", trail.ChainOfTrust)
}

func TestReplayability(t *testing.T) {
	// Test replayability structure
	replay := &Replayability{
		ReplayID:        "replay-1234567890",
		CanReplay:        true,
		ReplayParameters: &ReplayParameters{
			LLMProvider:    "openai",
			LLMModel:      "gpt-4",
			Temperature:    0.7,
			MaxTokens:      2000,
			EnabledStages: []string{"classification", "requirements"},
		},
		ReplayCount:     0,
	}

	if replay.ReplayID == "" {
		t.Error("Replay ID should not be empty")
	}

	if !replay.CanReplay {
		t.Error("Should be able to replay")
	}

	if replay.ReplayParameters == nil {
		t.Error("Replay parameters should not be nil")
	}

	t.Logf("✅ Replayability: %s", replay.ReplayID)
}

func TestGetPriorityFromWorkType(t *testing.T) {
	tests := []struct {
		workType   contracts.WorkType
		expected   string
	}{
		{contracts.WorkTypeImplementation, "high"},
		{contracts.WorkTypeBugFix, "high"},
		{contracts.WorkTypeRefactor, "medium"},
		{contracts.WorkTypeTesting, "low"},
		{contracts.WorkTypeDocumentation, "low"},
		{contracts.WorkTypeDesign, "low"},
	}

	for _, tt := range tests {
		t.Run(string(tt.workType), func(t *testing.T) {
			priority := getPriorityFromWorkType(tt.workType)
			if priority != tt.expected {
				t.Errorf("Expected priority '%s', got: %s", tt.expected, priority)
			}
		})
	}

	t.Logf("✅ All work types mapped to priorities correctly")
}
