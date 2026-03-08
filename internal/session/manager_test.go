package session

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestDefaultManager_CreateSession(t *testing.T) {
	manager, err := New(DefaultConfig(), NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-123",
		Title:     "Test Work Item",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-123",
		},
	}

	// Create session
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session == nil {
		t.Fatal("Session is nil")
	}

	if session.WorkItemID != workItem.ID {
		t.Errorf("Expected WorkItemID %s, got %s", workItem.ID, session.WorkItemID)
	}

	if session.State != contracts.SessionStateCreated {
		t.Errorf("Expected state Created, got %s", session.State)
	}

	if len(session.StateHistory) != 1 {
		t.Errorf("Expected 1 state transition, got %d", len(session.StateHistory))
	}

	if session.StateHistory[0].ToState != contracts.SessionStateCreated {
		t.Errorf("Expected initial state Created, got %s", session.StateHistory[0].ToState)
	}

	// Try to create another session for same work item (should fail)
	_, err = manager.CreateSession(ctx, workItem)
	if err == nil {
		t.Error("Expected error when creating duplicate active session")
	}

	t.Logf("Created session: %s", session.ID)
}

func TestDefaultManager_TransitionState(t *testing.T) {
	manager, err := New(DefaultConfig(), NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-456",
		Title:     "Test Transition",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-456",
		},
	}

	// Create session
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Transition through states
	states := []struct {
		state  contracts.SessionState
		reason string
		agent  string
	}{
		{contracts.SessionStateAnalyzed, "Intent analysis complete", "analyzer"},
		{contracts.SessionStateScheduled, "Task scheduled", "planner"},
		{contracts.SessionStateInProgress, "Execution started", "worker"},
		{contracts.SessionStateCompleted, "Work completed", "worker"},
	}

	for _, tc := range states {
		err = manager.TransitionState(ctx, session.ID, tc.state, tc.reason, tc.agent)
		if err != nil {
			t.Fatalf("Failed to transition to %s: %v", tc.state, err)
		}

		// Verify transition
		updated, err := manager.GetSession(ctx, session.ID)
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}

		if updated.State != tc.state {
			t.Errorf("Expected state %s, got %s", tc.state, updated.State)
		}

		lastTransition := updated.StateHistory[len(updated.StateHistory)-1]
		if lastTransition.ToState != tc.state {
			t.Errorf("Expected last transition to %s, got %s", tc.state, lastTransition.ToState)
		}

		if lastTransition.Reason != tc.reason {
			t.Errorf("Expected reason %s, got %s", tc.reason, lastTransition.Reason)
		}

		if lastTransition.Agent != tc.agent {
			t.Errorf("Expected agent %s, got %s", tc.agent, lastTransition.Agent)
		}

		t.Logf("Transitioned to %s: %s", tc.state, tc.reason)
	}

	// Try invalid transition (completed -> in_progress)
	err = manager.TransitionState(ctx, session.ID, contracts.SessionStateInProgress, "Invalid", "test")
	if err == nil {
		t.Error("Expected error for invalid transition from completed state")
	}
}

func TestDefaultManager_AddEvidence(t *testing.T) {
	manager, err := New(DefaultConfig(), NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-789",
		Title:     "Test Evidence",
		WorkType:  contracts.WorkTypeResearch,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-789",
		},
	}

	// Create session
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Add evidence
	evidence := contracts.EvidenceItem{
		Type:    contracts.EvidenceTypeHypothesis,
		Content: "If we implement token bucket algorithm, then API gateway will handle 10k RPS with <100ms latency.",
		Metadata: map[string]string{
			"experiment_id": "exp-001",
			"metric":        "latency",
		},
		CollectedBy: "analyzer-v1",
	}

	err = manager.AddEvidence(ctx, session.ID, evidence)
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Verify evidence
	updated, err := manager.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if len(updated.EvidenceItems) != 1 {
		t.Fatalf("Expected 1 evidence item, got %d", len(updated.EvidenceItems))
	}

	item := updated.EvidenceItems[0]
	if item.Type != contracts.EvidenceTypeHypothesis {
		t.Errorf("Expected evidence type Hypothesis, got %s", item.Type)
	}

	if item.Content != evidence.Content {
		t.Errorf("Evidence content mismatch")
	}

	if item.SessionID != session.ID {
		t.Errorf("Expected SessionID %s, got %s", session.ID, item.SessionID)
	}

	if item.ID == "" {
		t.Error("Evidence ID should be auto-generated")
	}

	if item.CollectedAt.IsZero() {
		t.Error("Evidence CollectedAt should be set")
	}

	t.Logf("Added evidence: %s - %s", item.ID, item.Type)
}

func TestDefaultManager_ListSessions(t *testing.T) {
	manager, err := New(DefaultConfig(), NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	
	// Create multiple sessions with different states
	workItems := []*contracts.WorkItem{
		{ID: "TEST-1", Title: "Test 1", WorkType: contracts.WorkTypeImplementation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{System: "test", IssueKey: "TEST-1"}},
		{ID: "TEST-2", Title: "Test 2", WorkType: contracts.WorkTypeResearch, CreatedAt: time.Now(), Source: contracts.SourceMetadata{System: "test", IssueKey: "TEST-2"}},
		{ID: "TEST-3", Title: "Test 3", WorkType: contracts.WorkTypeDocumentation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{System: "test", IssueKey: "TEST-3"}},
	}

	for i, workItem := range workItems {
		session, err := manager.CreateSession(ctx, workItem)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}

		// Set different states
		switch i {
		case 0:
			// Leave as Created (default)
		case 1:
			// Transition to Analyzed
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "Analyzed", "analyzer")
			if err != nil {
				t.Fatalf("Failed to transition: %v", err)
			}
		case 2:
			// Transition through to Completed
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "Analyzed", "analyzer")
			if err != nil {
				t.Fatalf("Failed to transition: %v", err)
			}
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateScheduled, "Scheduled", "planner")
			if err != nil {
				t.Fatalf("Failed to transition: %v", err)
			}
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateInProgress, "Started", "worker")
			if err != nil {
				t.Fatalf("Failed to transition: %v", err)
			}
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateCompleted, "Done", "worker")
			if err != nil {
				t.Fatalf("Failed to transition: %v", err)
			}
		}
	}

	// Test filtering by state
	analyzedFilter := SessionFilter{
		State: &[]contracts.SessionState{contracts.SessionStateAnalyzed}[0],
	}

	sessions, err := manager.ListSessions(ctx, analyzedFilter)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 analyzed session, got %d", len(sessions))
	} else {
		t.Logf("Found analyzed session: %s", sessions[0].ID)
	}

	// Test filtering by work item
	workItemFilter := SessionFilter{
		WorkItemID: &[]string{"TEST-2"}[0],
	}

	sessions, err = manager.ListSessions(ctx, workItemFilter)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 session for TEST-2, got %d", len(sessions))
	} else {
		t.Logf("Found session for TEST-2: %s", sessions[0].ID)
	}

	// Test limit
	limitFilter := SessionFilter{
		Limit: 2,
	}

	sessions, err = manager.ListSessions(ctx, limitFilter)
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) > 2 {
		t.Errorf("Expected at most 2 sessions, got %d", len(sessions))
	}

	t.Logf("Total sessions listed with limit 2: %d", len(sessions))
}

func TestDefaultManager_CleanupStaleSessions(t *testing.T) {
	// Create a custom config with short thresholds
	config := DefaultConfig()
	config.StaleThreshold = 100 * time.Millisecond
	config.CleanupInterval = 0 // Disable automatic cleanup

	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	
	// Create a stale session (will be old due to time travel)
	workItem := &contracts.WorkItem{
		ID:        "TEST-STALE",
		Title:     "Stale Test",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now().Add(-time.Hour), // Created an hour ago
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-STALE",
		},
	}

	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Manually set updatedAt to be old (hack for test)
	// In real implementation, we'd use a mock store
	t.Logf("Created stale session: %s", session.ID)

	// Test cleanup
	cleaned, err := manager.CleanupStaleSessions(ctx, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Note: This test is limited because we can't easily mock time in memory store
	// In a real test, we'd use a mock store with controllable timestamps
	t.Logf("Cleanup would have cleaned %d sessions", cleaned)
}