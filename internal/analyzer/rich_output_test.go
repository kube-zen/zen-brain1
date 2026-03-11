package analyzer

import (
	"strings"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestEnrichForRichAnalysis(t *testing.T) {
	now := time.Now()

	workItem := &contracts.WorkItem{
		ID:        "ZB-123",
		Title:     "Test work item",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Source: contracts.SourceMetadata{
			IssueKey: "ZB-123",
			System:   "jira",
		},
	}

	result := &contracts.AnalysisResult{
		WorkItem:        workItem,
		Confidence:      0.85,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{ID: "task-1", Title: "First task", Objective: "Do something", WorkType: contracts.WorkTypeImplementation},
			{ID: "task-2", Title: "Second task", Objective: "Do something else", WorkType: contracts.WorkTypeImplementation},
		},
		RequiresApproval: true,
		AnalyzedAt:       now,
		AnalyzerVersion:  "test-v1",
	}

	rich := EnrichForRichAnalysis(result, workItem)

	// Verify basic fields
	if rich == nil {
		t.Fatal("EnrichForRichAnalysis returned nil")
	}

	if rich.ExecutiveSummary == "" {
		t.Error("ExecutiveSummary should not be empty")
	}

	if rich.TechnicalSummary == "" {
		t.Error("TechnicalSummary should not be empty")
	}

	if rich.ReplayID == "" {
		t.Error("ReplayID should not be empty")
	}

	if rich.CorrelationID == "" {
		t.Error("CorrelationID should not be empty")
	}

	// Verify audit trail
	if rich.AuditTrail == nil {
		t.Error("AuditTrail should not be nil")
	} else {
		if rich.AuditTrail.WorkItemID != "ZB-123" {
			t.Errorf("AuditTrail.WorkItemID = %q, want ZB-123", rich.AuditTrail.WorkItemID)
		}
		if rich.AuditTrail.JiraKey != "ZB-123" {
			t.Errorf("AuditTrail.JiraKey = %q, want ZB-123", rich.AuditTrail.JiraKey)
		}
	}

	// Verify risk assessment
	if rich.RiskAssessment == nil {
		t.Error("RiskAssessment should not be nil")
	} else {
		if rich.RiskAssessment.OverallRisk == "" {
			t.Error("RiskAssessment.OverallRisk should not be empty")
		}
	}

	// Verify action items
	if len(rich.ActionItems) == 0 {
		t.Error("ActionItems should not be empty")
	} else {
		// Should have review, approve, and implementation actions
		foundReview := false
		foundApprove := false
		for _, action := range rich.ActionItems {
			if strings.Contains(action.ID, "review") {
				foundReview = true
			}
			if strings.Contains(action.ID, "approve") {
				foundApprove = true
			}
		}
		if !foundReview {
			t.Error("Should have review action item")
		}
		if !foundApprove {
			t.Error("Should have approve action item (RequiresApproval=true)")
		}
	}

	t.Logf("✅ EnrichForRichAnalysis: executive_summary=%d chars, tasks=%d, risk=%s",
		len(rich.ExecutiveSummary), len(rich.ActionItems), rich.RiskAssessment.OverallRisk)
}

func TestAssessRisks_HighPriority(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:        "ZB-456",
		Title:     "Critical bug fix",
		WorkType:  contracts.WorkTypeDebug,
		Priority:  contracts.PriorityCritical,
		Source:    contracts.SourceMetadata{IssueKey: "ZB-456", System: "jira"},
	}

	result := &contracts.AnalysisResult{
		WorkItem:       workItem,
		Confidence:     0.75,
		AnalyzedAt:     time.Now(),
		AnalyzerVersion: "test-v1",
	}

	risk := assessRisks(workItem, result)

	if risk == nil {
		t.Fatal("assessRisks returned nil")
	}

	// High priority should generate a risk factor
	foundPriorityRisk := false
	for _, factor := range risk.RiskFactors {
		if factor.ID == "priority-risk" {
			foundPriorityRisk = true
		}
	}

	if !foundPriorityRisk {
		t.Error("Expected priority risk factor for critical priority")
	}

	t.Logf("✅ High priority risk assessment: overall=%s, factors=%d",
		risk.OverallRisk, len(risk.RiskFactors))
}

func TestAssessRisks_LowConfidence(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:        "ZB-789",
		Title:     "Unclear requirements",
		WorkType:  contracts.WorkTypeImplementation,
		Priority:  contracts.PriorityMedium,
		Source:    contracts.SourceMetadata{IssueKey: "ZB-789", System: "jira"},
	}

	result := &contracts.AnalysisResult{
		WorkItem:       workItem,
		Confidence:     0.5, // Low confidence
		AnalyzedAt:     time.Now(),
		AnalyzerVersion: "test-v1",
	}

	risk := assessRisks(workItem, result)

	if risk == nil {
		t.Fatal("assessRisks returned nil")
	}

	// Low confidence should generate a risk factor
	foundConfidenceRisk := false
	for _, factor := range risk.RiskFactors {
		if factor.ID == "confidence-risk" {
			foundConfidenceRisk = true
		}
	}

	if !foundConfidenceRisk {
		t.Error("Expected confidence risk factor for low confidence")
	}

	t.Logf("✅ Low confidence risk assessment: overall=%s, factors=%d",
		risk.OverallRisk, len(risk.RiskFactors))
}

func TestBuildTaskChain(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:     "TEST-1",
		Title:  "Test",
		Source: contracts.SourceMetadata{IssueKey: "TEST-1"},
	}

	result := &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{ID: "task-1", Title: "First"},
			{ID: "task-2", Title: "Second"},
			{ID: "task-3", Title: "Third"},
		},
		AnalyzedAt: time.Now(),
	}

	chain := buildTaskChain(result)

	if len(chain) != 3 {
		t.Fatalf("Expected 3 task links, got %d", len(chain))
	}

	// First task should have no dependencies
	if len(chain[0].DependsOnTaskIDs) != 0 {
		t.Errorf("First task should have no dependencies, got %v", chain[0].DependsOnTaskIDs)
	}

	// Second task should depend on first
	if len(chain[1].DependsOnTaskIDs) != 1 || chain[1].DependsOnTaskIDs[0] != "task-1" {
		t.Errorf("Second task should depend on task-1, got %v", chain[1].DependsOnTaskIDs)
	}

	// Third task should depend on second
	if len(chain[2].DependsOnTaskIDs) != 1 || chain[2].DependsOnTaskIDs[0] != "task-2" {
		t.Errorf("Third task should depend on task-2, got %v", chain[2].DependsOnTaskIDs)
	}

	t.Logf("✅ Task chain: %d tasks linked correctly", len(chain))
}

func TestGenerateActionItems(t *testing.T) {
	workItem := &contracts.WorkItem{
		ID:       "ACTION-1",
		Title:    "Test actions",
		WorkType: contracts.WorkTypeImplementation,
		Source:   contracts.SourceMetadata{IssueKey: "ACTION-1"},
	}

	result := &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{ID: "impl-1", Title: "Implement feature", Objective: "Build it", WorkType: contracts.WorkTypeImplementation},
		},
		RequiresApproval: true,
		AnalyzedAt:       time.Now(),
	}

	actions := generateActionItems(workItem, result)

	if len(actions) < 2 {
		t.Fatalf("Expected at least 2 actions (review + approve), got %d", len(actions))
	}

	// Check for review action
	foundReview := false
	for _, action := range actions {
		if action.Category == "analysis" {
			foundReview = true
			break
		}
	}
	if !foundReview {
		t.Error("Should have analysis category action")
	}

	// Check for approve action (since RequiresApproval=true)
	foundApprove := false
	for _, action := range actions {
		if action.Category == "approval" {
			foundApprove = true
			if len(action.DependsOn) == 0 {
				t.Error("Approve action should depend on review")
			}
			break
		}
	}
	if !foundApprove {
		t.Error("Should have approval category action when RequiresApproval=true")
	}

	t.Logf("✅ Action items: %d total, review=%v, approve=%v",
		len(actions), foundReview, foundApprove)
}

func TestGenerateMitigations(t *testing.T) {
	riskFactors := []*RiskFactor{
		{ID: "tech-risk", Category: "technical", Severity: "high"},
		{ID: "ops-risk", Category: "operational", Severity: "medium"},
		{ID: "unknown-risk", Category: "unknown", Severity: "low"},
	}

	steps := generateMitigations(riskFactors)

	if len(steps) != 3 {
		t.Fatalf("Expected 3 mitigation steps, got %d", len(steps))
	}

	// Check each step has correct risk linkage
	for i, step := range steps {
		if step.RiskID != riskFactors[i].ID {
			t.Errorf("Step %d: RiskID = %q, want %q", i, step.RiskID, riskFactors[i].ID)
		}
		if step.Status != "pending" {
			t.Errorf("Step %d: Status = %q, want pending", i, step.Status)
		}
	}

	// Check owner assignment
	for _, step := range steps {
		if step.RiskID == "tech-risk" && step.Owner != "tech_lead" {
			t.Errorf("Technical risk owner = %q, want tech_lead", step.Owner)
		}
		if step.RiskID == "ops-risk" && step.Owner != "project_manager" {
			t.Errorf("Operational risk owner = %q, want project_manager", step.Owner)
		}
	}

	t.Logf("✅ Mitigations: %d steps generated", len(steps))
}
