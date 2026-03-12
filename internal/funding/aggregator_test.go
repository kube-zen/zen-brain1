package funding

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/evidence"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestNewAggregator(t *testing.T) {
	v := evidence.NewMemoryVault()
	a := NewAggregator(v)
	if a == nil {
		t.Fatal("NewAggregator returned nil")
	}
	if a.Vault != v {
		t.Error("Vault not set correctly")
	}
}

func TestNewAggregatorNilVault(t *testing.T) {
	a := NewAggregator(nil)
	if a == nil {
		t.Fatal("NewAggregator returned nil")
	}
}

func TestAggregateForSession_NilVault(t *testing.T) {
	a := NewAggregator(nil)
	_, err := a.AggregateForSession(context.Background(), "session-1", "Test Project")
	if err == nil {
		t.Error("expected error with nil vault")
	}
	if err.Error() != "vault is nil" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAggregateForSession_Empty(t *testing.T) {
	v := evidence.NewMemoryVault()
	a := NewAggregator(v)

	report, err := a.AggregateForSession(context.Background(), "session-empty", "Empty Project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if len(report.SessionIDs) != 1 || report.SessionIDs[0] != "session-empty" {
		t.Errorf("unexpected session IDs: %v", report.SessionIDs)
	}
	if report.T661 == nil {
		t.Error("T661 narrative is nil")
	}
	if report.IRAP == nil {
		t.Error("IRAP report is nil")
	}
}

func TestAggregateForSession_WithEvidence(t *testing.T) {
	v := evidence.NewMemoryVault()
	now := time.Now()

	// Store various evidence types
	items := []contracts.EvidenceItem{
		{
			ID:          "ev-1",
			SessionID:   "session-1",
			Type:        contracts.EvidenceTypeHypothesis,
			Content:     "We hypothesize that approach X will improve performance",
			CollectedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:          "ev-2",
			SessionID:   "session-1",
			Type:        contracts.EvidenceTypeExperiment,
			Content:     "Running experiment to test hypothesis with parameters Y",
			CollectedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:          "ev-3",
			SessionID:   "session-1",
			Type:        contracts.EvidenceTypeAnalysis,
			Content:     "Analysis shows 15% improvement in latency metrics",
			CollectedAt: now,
		},
		{
			ID:          "ev-4",
			SessionID:   "session-1",
			Type:        contracts.EvidenceTypeConclusion,
			Content:     "Hypothesis confirmed, approach X is viable for production",
			CollectedAt: now.Add(30 * time.Minute),
		},
	}

	ctx := context.Background()
	for _, item := range items {
		if err := v.Store(ctx, item); err != nil {
			t.Fatalf("failed to store evidence: %v", err)
		}
	}

	a := NewAggregator(v)
	report, err := a.AggregateForSession(ctx, "session-1", "Performance Optimization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify T661 narrative
	if report.T661.ProjectTitle != "Performance Optimization" {
		t.Errorf("unexpected project title: %s", report.T661.ProjectTitle)
	}
	if report.T661.PeriodStart.IsZero() || report.T661.PeriodEnd.IsZero() {
		t.Error("period dates should be set")
	}
	if len(report.T661.EvidenceSummary) != 4 {
		t.Errorf("expected 4 evidence refs, got %d", len(report.T661.EvidenceSummary))
	}

	// Verify evidence is categorized correctly
	if report.T661.ScientificTechnologicalUncertainties == "" {
		t.Error("uncertainties section should not be empty (hypothesis + experiment)")
	}
	if report.T661.WorkPerformedToOvercome == "" {
		t.Error("work performed section should not be empty (analysis)")
	}
	if report.T661.AdvancementsAchieved == "" {
		t.Error("advancements section should not be empty (conclusion)")
	}

	// Verify IRAP report
	if report.IRAP.Title != "Performance Optimization – Technical Report" {
		t.Errorf("unexpected IRAP title: %s", report.IRAP.Title)
	}
	if len(report.IRAP.Sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(report.IRAP.Sections))
	}
}

func TestAggregateForSessions_MultipleSessions(t *testing.T) {
	v := evidence.NewMemoryVault()
	now := time.Now()

	// Evidence from multiple sessions
	items := []contracts.EvidenceItem{
		{ID: "ev-1", SessionID: "session-1", Type: contracts.EvidenceTypeHypothesis, Content: "Hypothesis 1", CollectedAt: now.Add(-3 * time.Hour)},
		{ID: "ev-2", SessionID: "session-2", Type: contracts.EvidenceTypeHypothesis, Content: "Hypothesis 2", CollectedAt: now.Add(-2 * time.Hour)},
		{ID: "ev-3", SessionID: "session-1", Type: contracts.EvidenceTypeConclusion, Content: "Conclusion 1", CollectedAt: now.Add(-1 * time.Hour)},
	}

	ctx := context.Background()
	for _, item := range items {
		if err := v.Store(ctx, item); err != nil {
			t.Fatalf("failed to store evidence: %v", err)
		}
	}

	a := NewAggregator(v)
	report, err := a.AggregateForSessions(ctx, []string{"session-1", "session-2"}, "Multi-Session Project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.SessionIDs) != 2 {
		t.Errorf("expected 2 session IDs, got %d", len(report.SessionIDs))
	}
	if len(report.T661.EvidenceSummary) != 3 {
		t.Errorf("expected 3 evidence refs, got %d", len(report.T661.EvidenceSummary))
	}
}

func TestT661Text(t *testing.T) {
	narrative := &T661Narrative{
		ProjectTitle: "Test Project",
		PeriodStart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:    time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		ScientificTechnologicalUncertainties: "Uncertainty about X",
		WorkPerformedToOvercome:             "Work done on Y",
		AdvancementsAchieved:                "Achieved Z",
	}

	text := narrative.T661Text()
	if !contains(text, "Test Project") {
		t.Error("T661Text should contain project title")
	}
	if !contains(text, "2026-01-01") || !contains(text, "2026-03-31") {
		t.Error("T661Text should contain period dates")
	}
	if !contains(text, "Uncertainty about X") {
		t.Error("T661Text should contain uncertainties")
	}
	if !contains(text, "Line 242") {
		t.Error("T661Text should reference Line 242")
	}
}

func TestIRAPMarkdown(t *testing.T) {
	report := &IRAPReport{
		Title:   "Test Report",
		Date:    time.Date(2026, 3, 11, 0, 0, 0, 0, time.UTC),
		Summary: "Test summary",
		Sections: []Section{
			{Title: "Introduction", Content: "Intro content"},
			{Title: "Results", Content: "Results content"},
		},
		EvidenceRefs: []EvidenceRef{
			{ID: "ev-1", Type: contracts.EvidenceTypeHypothesis, SessionID: "s1", CollectedAt: time.Now()},
		},
	}

	md := report.IRAPMarkdown()
	if !contains(md, "# Test Report") {
		t.Error("IRAPMarkdown should have title heading")
	}
	if !contains(md, "## Introduction") {
		t.Error("IRAPMarkdown should have section headings")
	}
	if !contains(md, "ev-1") {
		t.Error("IRAPMarkdown should include evidence references")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"this is a very long string that should be truncated", 20, "this is a very long ..."},
		{"  whitespace  ", 20, "whitespace"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
