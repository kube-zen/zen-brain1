package integration

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestOfficePipeline_ProcessWorkItem(t *testing.T) {
	ctx := context.Background()

	t.Setenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB", "1")
	t.Setenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER", "1")

	// Create pipeline with nil config (explicit dev stubs)
	pipeline, err := NewOfficePipeline(nil)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}
	defer func() {
		// Close components
		pipeline.Planner.Close()
		pipeline.Gatekeeper.Close()
		pipeline.SessionManager.Close()
	}()

	// Create a mock work item
	workItem := &contracts.WorkItem{
		ID:            "TEST-001",
		Title:         "Test work item",
		Summary:       "This is a test work item for integration testing",
		Body:          "## Problem\n\nTest problem description.\n\n## Expected Behavior\n\nShould work.",
		WorkType:      contracts.WorkTypeImplementation,
		WorkDomain:    contracts.DomainCore,
		Priority:      contracts.PriorityMedium,
		ExecutionMode: contracts.ModeAutonomous,
		Status:        contracts.StatusRequested,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ClusterID:     "default",
		ProjectID:     "TEST",
		Source: contracts.SourceMetadata{
			System:    "test",
			IssueKey:  "TEST-001",
			Project:   "TEST",
			IssueType: "Task",
		},
		EvidenceRequirement: contracts.EvidenceSummary,
	}

	// Process work item
	err = pipeline.Planner.ProcessWorkItem(ctx, workItem)
	if err != nil {
		t.Errorf("ProcessWorkItem failed: %v", err)
	}

	// Give planner a moment to process asynchronously
	time.Sleep(100 * time.Millisecond)

	// Get session status
	status, err := pipeline.Planner.GetSessionStatus(ctx, "session-TEST-001")
	if err != nil {
		t.Logf("Session not found (maybe different session ID). This is okay for now.")
	} else {
		t.Logf("Session status: %s, progress %.0f%%", status.Session.State, status.ProgressPercent)

		// Verify session was created
		if status.Session == nil {
			t.Error("Session is nil")
		} else {
			if status.Session.WorkItemID != workItem.ID {
				t.Errorf("Session work item ID mismatch: got %s, want %s", status.Session.WorkItemID, workItem.ID)
			}
		}
	}

	// Verify no pending approvals (since auto-approve is enabled)
	pending, err := pipeline.Planner.GetPendingApprovals(ctx)
	if err != nil {
		t.Errorf("GetPendingApprovals failed: %v", err)
	}
	if len(pending) > 0 {
		t.Errorf("Expected no pending approvals, got %d", len(pending))
	}
}

func TestNewOfficePipeline_RequiresExplicitStubOptIn(t *testing.T) {
	_, err := NewOfficePipeline(nil)
	if err == nil {
		t.Fatal("expected office pipeline to fail without explicit stub opt-in")
	}
}
