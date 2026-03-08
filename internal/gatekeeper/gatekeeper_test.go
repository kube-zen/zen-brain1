package gatekeeper

import (
	"context"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/planner"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// mockPlanner is a mock Planner for testing.
type mockPlanner struct {
	pendingSessions []*contracts.Session
	approvedSessions map[string]bool
	rejectedSessions map[string]bool
}

func newMockPlanner() *mockPlanner {
	return &mockPlanner{
		pendingSessions:  make([]*contracts.Session, 0),
		approvedSessions: make(map[string]bool),
		rejectedSessions: make(map[string]bool),
	}
}

func (m *mockPlanner) ProcessWorkItem(ctx context.Context, workItem *contracts.WorkItem) error {
	return nil
}

func (m *mockPlanner) ProcessBatch(ctx context.Context, workItems []*contracts.WorkItem) error {
	return nil
}

func (m *mockPlanner) GetSessionStatus(ctx context.Context, sessionID string) (*planner.SessionStatus, error) {
	return nil, nil
}

func (m *mockPlanner) ApproveSession(ctx context.Context, sessionID string, approver string, notes string) error {
	m.approvedSessions[sessionID] = true
	// Remove from pending
	for i, session := range m.pendingSessions {
		if session.ID == sessionID {
			m.pendingSessions = append(m.pendingSessions[:i], m.pendingSessions[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockPlanner) RejectSession(ctx context.Context, sessionID string, rejector string, reason string) error {
	m.rejectedSessions[sessionID] = true
	// Remove from pending
	for i, session := range m.pendingSessions {
		if session.ID == sessionID {
			m.pendingSessions = append(m.pendingSessions[:i], m.pendingSessions[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockPlanner) CancelSession(ctx context.Context, sessionID string, canceller string, reason string) error {
	return nil
}

func (m *mockPlanner) GetPendingApprovals(ctx context.Context) ([]*contracts.Session, error) {
	return m.pendingSessions, nil
}

func (m *mockPlanner) Close() error {
	return nil
}

func TestDefaultGatekeeper_GetPendingApprovals(t *testing.T) {
	// Create mock planner with pending sessions
	mockPlanner := newMockPlanner()
	
	// Add a pending session
	session := &contracts.Session{
		ID:         "session-test-123",
		WorkItemID: "TEST-123",
		SourceKey:  "TEST-123",
		State:      contracts.SessionStateBlocked,
		WorkItem: &contracts.WorkItem{
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
		},
		AnalysisResult: &contracts.AnalysisResult{
			WorkItem: &contracts.WorkItem{
				ID: "TEST-123",
			},
			BrainTaskSpecs: []contracts.BrainTaskSpec{},
			Confidence:     0.85,
			AnalysisNotes:  "Test analysis",
			RequiresApproval: true,
			EstimatedTotalCostUSD: 3.50,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockPlanner.pendingSessions = append(mockPlanner.pendingSessions, session)
	
	// Create gatekeeper config
	config := &Config{
		Planner: mockPlanner,
		DefaultChannels: []NotificationChannel{ChannelConsole},
		DefaultApprovalLevel: "team_lead",
		DefaultDeadlineHours: 24,
		ReminderInterval:     4 * time.Hour,
		EscalationInterval:   8 * time.Hour,
		AuditLogEnabled:      false, // Disable for test
		HTTPEnabled:         false,
	}
	
	// Create gatekeeper
	gatekeeper, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gatekeeper: %v", err)
	}
	defer gatekeeper.Close()
	
	// Get pending approvals
	ctx := context.Background()
	approvals, err := gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	if len(approvals) != 1 {
		t.Fatalf("Expected 1 approval, got %d", len(approvals))
	}
	
	approval := approvals[0]
	if approval.SessionID != "session-test-123" {
		t.Errorf("Expected session ID session-test-123, got %s", approval.SessionID)
	}
	
	if approval.WorkItem.Title != "Test Work Item" {
		t.Errorf("Expected work item title 'Test Work Item', got %s", approval.WorkItem.Title)
	}
	
	if approval.EstimatedCostUSD != 3.50 {
		t.Errorf("Expected estimated cost $3.50, got $%.2f", approval.EstimatedCostUSD)
	}
	
	if approval.Status != ApprovalStatusPending {
		t.Errorf("Expected status pending, got %s", approval.Status)
	}
	
	t.Logf("Got pending approval: %s - %s ($%.2f)", approval.ID, approval.WorkItem.Title, approval.EstimatedCostUSD)
}

func TestDefaultGatekeeper_Approve(t *testing.T) {
	// Create mock planner with pending session
	mockPlanner := newMockPlanner()
	
	session := &contracts.Session{
		ID:         "session-test-456",
		WorkItemID: "TEST-456",
		SourceKey:  "TEST-456",
		State:      contracts.SessionStateBlocked,
		WorkItem: &contracts.WorkItem{
			ID:        "TEST-456",
			Title:     "Approve Test",
			WorkType:  contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:  contracts.PriorityMedium,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-456",
			},
		},
		AnalysisResult: &contracts.AnalysisResult{
			WorkItem: &contracts.WorkItem{
				ID: "TEST-456",
			},
			EstimatedTotalCostUSD: 2.50,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockPlanner.pendingSessions = append(mockPlanner.pendingSessions, session)
	
	// Create gatekeeper
	config := &Config{
		Planner: mockPlanner,
		AuditLogEnabled: false,
		HTTPEnabled: false,
	}
	
	gatekeeper, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gatekeeper: %v", err)
	}
	defer gatekeeper.Close()
	
	// First, get the approval ID
	ctx := context.Background()
	approvals, err := gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	if len(approvals) != 1 {
		t.Fatalf("Expected 1 approval, got %d", len(approvals))
	}
	
	approvalID := approvals[0].ID
	
	// Approve the session
	decision := ApprovalDecision{
		Decision:  "approved",
		DecidedBy: "test-user",
		DecidedAt: time.Now(),
		Reason:    "Looks good, proceed",
	}
	
	err = gatekeeper.Approve(ctx, approvalID, decision)
	if err != nil {
		t.Fatalf("Failed to approve: %v", err)
	}
	
	// Check that planner's ApproveSession was called
	if !mockPlanner.approvedSessions["session-test-456"] {
		t.Error("Planner's ApproveSession was not called")
	}
	
	// Check that approval is no longer pending
	approvals, err = gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	// Should be 0 now since we approved it
	if len(approvals) != 0 {
		t.Errorf("Expected 0 pending approvals after approval, got %d", len(approvals))
	}
	
	// Get approval history
	history, err := gatekeeper.GetApprovalHistory(ctx, "session-test-456")
	if err != nil {
		t.Fatalf("Failed to get approval history: %v", err)
	}
	
	if len(history) == 0 {
		t.Error("Expected approval history events")
	} else {
		lastEvent := history[len(history)-1]
		if lastEvent.EventType != "approved" {
			t.Errorf("Expected last event type 'approved', got %s", lastEvent.EventType)
		}
		t.Logf("Approval recorded: %s by %s", lastEvent.EventType, lastEvent.Actor)
	}
}

func TestDefaultGatekeeper_Reject(t *testing.T) {
	// Create mock planner with pending session
	mockPlanner := newMockPlanner()
	
	session := &contracts.Session{
		ID:         "session-test-789",
		WorkItemID: "TEST-789",
		SourceKey:  "TEST-789",
		State:      contracts.SessionStateBlocked,
		WorkItem: &contracts.WorkItem{
			ID:        "TEST-789",
			Title:     "Reject Test",
			WorkType:  contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:  contracts.PriorityMedium,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-789",
			},
		},
		AnalysisResult: &contracts.AnalysisResult{
			WorkItem: &contracts.WorkItem{
				ID: "TEST-789",
			},
			EstimatedTotalCostUSD: 5.00,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockPlanner.pendingSessions = append(mockPlanner.pendingSessions, session)
	
	// Create gatekeeper
	config := &Config{
		Planner: mockPlanner,
		AuditLogEnabled: false,
		HTTPEnabled: false,
	}
	
	gatekeeper, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gatekeeper: %v", err)
	}
	defer gatekeeper.Close()
	
	// Get approval ID
	ctx := context.Background()
	approvals, err := gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	approvalID := approvals[0].ID
	
	// Reject the session
	decision := ApprovalDecision{
		Decision:  "rejected",
		DecidedBy: "test-user",
		DecidedAt: time.Now(),
		Reason:    "Not aligned with current priorities",
	}
	
	err = gatekeeper.Reject(ctx, approvalID, decision)
	if err != nil {
		t.Fatalf("Failed to reject: %v", err)
	}
	
	// Check that planner's RejectSession was called
	if !mockPlanner.rejectedSessions["session-test-789"] {
		t.Error("Planner's RejectSession was not called")
	}
	
	t.Logf("Session rejected successfully")
}

func TestDefaultGatekeeper_Delegate(t *testing.T) {
	// Create mock planner with pending session
	mockPlanner := newMockPlanner()
	
	session := &contracts.Session{
		ID:         "session-test-999",
		WorkItemID: "TEST-999",
		SourceKey:  "TEST-999",
		State:      contracts.SessionStateBlocked,
		WorkItem: &contracts.WorkItem{
			ID:        "TEST-999",
			Title:     "Delegate Test",
			WorkType:  contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:  contracts.PriorityMedium,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-999",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockPlanner.pendingSessions = append(mockPlanner.pendingSessions, session)
	
	// Create gatekeeper
	config := &Config{
		Planner: mockPlanner,
		AuditLogEnabled: false,
		HTTPEnabled: false,
	}
	
	gatekeeper, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gatekeeper: %v", err)
	}
	defer gatekeeper.Close()
	
	// Get approval ID
	ctx := context.Background()
	approvals, err := gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	approvalID := approvals[0].ID
	
	// Test delegation
	err = gatekeeper.DelegateApproval(ctx, approvalID, "delegate-user", "On vacation")
	if err != nil {
		t.Fatalf("Failed to delegate: %v", err)
	}
	
	// Get approval to check status
	approval, err := gatekeeper.GetApproval(ctx, approvalID)
	if err != nil {
		t.Fatalf("Failed to get approval: %v", err)
	}
	
	if approval.Status != ApprovalStatusDelegated {
		t.Errorf("Expected status delegated, got %s", approval.Status)
	}
	
	if len(approval.AssignedTo) != 1 || approval.AssignedTo[0] != "delegate-user" {
		t.Errorf("Expected assigned to delegate-user, got %v", approval.AssignedTo)
	}
	
	t.Logf("Delegation tested successfully")
}

func TestDefaultGatekeeper_Escalate(t *testing.T) {
	// Create mock planner with pending session
	mockPlanner := newMockPlanner()
	
	session := &contracts.Session{
		ID:         "session-test-888",
		WorkItemID: "TEST-888",
		SourceKey:  "TEST-888",
		State:      contracts.SessionStateBlocked,
		WorkItem: &contracts.WorkItem{
			ID:        "TEST-888",
			Title:     "Escalate Test",
			WorkType:  contracts.WorkTypeImplementation,
			WorkDomain: contracts.DomainCore,
			Priority:  contracts.PriorityMedium,
			Status:    contracts.StatusRequested,
			CreatedAt: time.Now(),
			Source: contracts.SourceMetadata{
				System:   "test",
				IssueKey: "TEST-888",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	mockPlanner.pendingSessions = append(mockPlanner.pendingSessions, session)
	
	// Create gatekeeper
	config := &Config{
		Planner: mockPlanner,
		AuditLogEnabled: false,
		HTTPEnabled: false,
	}
	
	gatekeeper, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create gatekeeper: %v", err)
	}
	defer gatekeeper.Close()
	
	// Get approval ID
	ctx := context.Background()
	approvals, err := gatekeeper.GetPendingApprovals(ctx, ApprovalFilter{})
	if err != nil {
		t.Fatalf("Failed to get pending approvals: %v", err)
	}
	
	approvalID := approvals[0].ID
	
	// Test escalation
	err = gatekeeper.EscalateApproval(ctx, approvalID, "Urgent business need")
	if err != nil {
		t.Fatalf("Failed to escalate: %v", err)
	}
	
	// Get approval again
	approval, err := gatekeeper.GetApproval(ctx, approvalID)
	if err != nil {
		t.Fatalf("Failed to get approval: %v", err)
	}
	
	if approval.Status != ApprovalStatusEscalated {
		t.Errorf("Expected status escalated, got %s", approval.Status)
	}
	
	// Should be escalated to manager (from team_lead default)
	if approval.ApprovalLevel != "manager" {
		t.Errorf("Expected approval level manager, got %s", approval.ApprovalLevel)
	}
	
	t.Logf("Escalation tested successfully")
}