package planner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/internal/office"
	"github.com/kube-zen/zen-brain1/internal/session"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/ledger"
)

// mockAnalyzer is a mock Intent Analyzer.
type mockAnalyzer struct{}

func (m *mockAnalyzer) Analyze(ctx context.Context, workItem *contracts.WorkItem) (*contracts.AnalysisResult, error) {
	return &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{
				ID:          "task-1",
				Title:       "Test Task",
				Description: "Test description",
				WorkItemID:  workItem.ID,
				SourceKey:   workItem.Source.IssueKey,
				WorkType:    workItem.WorkType,
				WorkDomain:  workItem.WorkDomain,
				Priority:    workItem.Priority,
				Objective:   "Complete test task",
				EstimatedCostUSD: 1.50,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		Confidence:     0.85,
		AnalysisNotes:  "Test analysis complete",
		RequiresApproval: false,
		EstimatedTotalCostUSD: 1.50,
	}, nil
}

func (m *mockAnalyzer) AnalyzeBatch(ctx context.Context, workItems []*contracts.WorkItem) ([]*contracts.AnalysisResult, error) {
	return nil, nil
}

func (m *mockAnalyzer) GetAnalysisHistory(ctx context.Context, workItemID string) ([]*contracts.AnalysisResult, error) {
	return nil, nil
}

func (m *mockAnalyzer) UpdateAnalysis(ctx context.Context, result *contracts.AnalysisResult) error {
	return nil
}

// mockSessionManager is a mock Session manager.
type mockSessionManager struct {
	sessions map[string]*contracts.Session
	transitions []string
}

func newMockSessionManager() *mockSessionManager {
	return &mockSessionManager{
		sessions: make(map[string]*contracts.Session),
		transitions: make([]string, 0),
	}
}

func (m *mockSessionManager) CreateSession(ctx context.Context, workItem *contracts.WorkItem) (*contracts.Session, error) {
	sessionID := "session-" + workItem.ID
	session := &contracts.Session{
		ID:         sessionID,
		WorkItemID: workItem.ID,
		SourceKey:  workItem.Source.IssueKey,
		State:      contracts.SessionStateCreated,
		WorkItem:   workItem,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		StateHistory: []contracts.StateTransition{{
			ToState:   contracts.SessionStateCreated,
			Timestamp: time.Now(),
			Reason:    "Session created",
			Agent:     "session-manager",
		}},
	}
	m.sessions[sessionID] = session
	return session, nil
}

func (m *mockSessionManager) GetSession(ctx context.Context, sessionID string) (*contracts.Session, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, &sessionError{msg: "session not found"}
	}
	return session, nil
}

func (m *mockSessionManager) GetSessionByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error) {
	for _, session := range m.sessions {
		if session.WorkItemID == workItemID {
			return session, nil
		}
	}
	return nil, &sessionError{msg: "session not found"}
}

func (m *mockSessionManager) UpdateSession(ctx context.Context, session *contracts.Session) error {
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionManager) TransitionState(ctx context.Context, sessionID string, newState contracts.SessionState, reason string, agent string) error {
	session, ok := m.sessions[sessionID]
	if !ok {
		return &sessionError{msg: "session not found"}
	}
	
	m.transitions = append(m.transitions, fmt.Sprintf("%s -> %s: %s", session.State, newState, reason))
	session.State = newState
	session.StateHistory = append(session.StateHistory, contracts.StateTransition{
		FromState:  session.State,
		ToState:    newState,
		Timestamp:  time.Now(),
		Reason:     reason,
		Agent:      agent,
	})
	return nil
}

func (m *mockSessionManager) AddEvidence(ctx context.Context, sessionID string, evidence contracts.EvidenceItem) error {
	session, ok := m.sessions[sessionID]
	if !ok {
		return &sessionError{msg: "session not found"}
	}
	
	evidence.ID = fmt.Sprintf("evidence-%d", len(session.EvidenceItems)+1)
	session.EvidenceItems = append(session.EvidenceItems, evidence)
	return nil
}

func (m *mockSessionManager) ListSessions(ctx context.Context, filter session.SessionFilter) ([]*contracts.Session, error) {
	var result []*contracts.Session
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSessionManager) CleanupStaleSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	return 0, nil
}

func (m *mockSessionManager) Close() error {
	return nil
}

type sessionError struct {
	msg string
}

func (e *sessionError) Error() string {
	return e.msg
}

// mockLedgerClient is a mock Ledger client.
type mockLedgerClient struct {
	efficiencies []ledger.ModelEfficiency
}

func (m *mockLedgerClient) GetModelEfficiency(ctx context.Context, projectID string, taskType string) ([]ledger.ModelEfficiency, error) {
	return m.efficiencies, nil
}

func (m *mockLedgerClient) GetCostBudgetStatus(ctx context.Context, projectID string) (*ledger.BudgetStatus, error) {
	return &ledger.BudgetStatus{
		ProjectID:      projectID,
		PeriodStart:    time.Now().Add(-30 * 24 * time.Hour),
		PeriodEnd:      time.Now().Add(30 * 24 * time.Hour),
		SpentUSD:       50.0,
		BudgetLimitUSD: 1000.0,
		RemainingUSD:   950.0,
		PercentUsed:    5.0,
	}, nil
}

func (m *mockLedgerClient) RecordPlannedModelSelection(ctx context.Context, sessionID, taskID, modelID, reason string) error {
	return nil
}

func TestDefaultPlanner_ProcessWorkItem(t *testing.T) {
	// Create mocks
	officeManager := office.NewManager()
	analyzer := &mockAnalyzer{}
	sessionManager := newMockSessionManager()
	ledgerClient := &mockLedgerClient{
		efficiencies: []ledger.ModelEfficiency{
			{
				ModelID:        "glm-4.7",
				AvgCostPerTask: 0.80,
				AvgTokensPerTask: 2000,
				SuccessRate:    0.92,
				AvgCorrections: 0.3,
				AvgLatencyMs:   5000,
				SampleSize:     100,
			},
			{
				ModelID:        "claude-sonnet-4-6",
				AvgCostPerTask: 1.20,
				AvgTokensPerTask: 2500,
				SuccessRate:    0.95,
				AvgCorrections: 0.2,
				AvgLatencyMs:   3000,
				SampleSize:     80,
			},
		},
	}
	
	// Create config
	config := &Config{
		OfficeManager:  officeManager,
		Analyzer:       analyzer,
		SessionManager: sessionManager,
		LedgerClient:   ledgerClient,
		DefaultModel:   "glm-4.7",
		FallbackModel:  "glm-4.7",
		MaxCostUSD:     10.0,
		RequireApproval: false, // Disable approval for test
		AutoApproveCost: 2.0,
		AnalysisTimeout: 300,
		ExecutionTimeout: 3600,
		MetricsEnabled:  true,
	}
	
	// Create planner
	planner, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
	defer planner.Close()
	
	// Create test work item
	workItem := &contracts.WorkItem{
		ID:        "TEST-123",
		Title:     "Test Work Item",
		Summary:   "Test summary",
		Body:      "This is a test work item description.",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:    "test",
			IssueKey:  "TEST-123",
			IssueType: "Task",
		},
		EvidenceRequirement: contracts.EvidenceSummary,
	}
	
	// Process work item
	ctx := context.Background()
	err = planner.ProcessWorkItem(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to process work item: %v", err)
	}
	
	// Give async processing time
	time.Sleep(100 * time.Millisecond)
	
	// Check that session was created
	session, err := sessionManager.GetSessionByWorkItem(ctx, workItem.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	if session == nil {
		t.Fatal("Session was not created")
	}
	
	if session.WorkItemID != workItem.ID {
		t.Errorf("Expected WorkItemID %s, got %s", workItem.ID, session.WorkItemID)
	}
	
	// Check that session has analysis results
	if session.AnalysisResult == nil {
		t.Error("Session should have analysis results")
	}
	
	if len(session.BrainTaskSpecs) == 0 {
		t.Error("Session should have brain task specs")
	}
	
	// Check state transitions
	if len(sessionManager.transitions) == 0 {
		t.Error("Expected state transitions")
	} else {
		t.Logf("State transitions: %v", sessionManager.transitions)
	}
	
	t.Logf("Planner processed work item %s, session %s created", workItem.ID, session.ID)
}

func TestDefaultPlanner_GetSessionStatus(t *testing.T) {
	// Create mocks
	sessionManager := newMockSessionManager()
	
	// Create a session
	workItem := &contracts.WorkItem{
		ID:        "TEST-456",
		Title:     "Test Status",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-456",
		},
	}
	
	ctx := context.Background()
	session, err := sessionManager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Add analysis results
	session.AnalysisResult = &contracts.AnalysisResult{
		WorkItem: workItem,
		BrainTaskSpecs: []contracts.BrainTaskSpec{{
			ID:          "task-1",
			Title:       "Test Task",
			Description: "Test description",
			WorkItemID:  workItem.ID,
			SourceKey:   workItem.Source.IssueKey,
			WorkType:    workItem.WorkType,
			WorkDomain:  workItem.WorkDomain,
			Priority:    workItem.Priority,
			Objective:   "Complete test task",
			EstimatedCostUSD: 2.50,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}},
		Confidence:     0.85,
		AnalysisNotes:  "Test analysis",
		RequiresApproval: false,
		EstimatedTotalCostUSD: 2.50,
	}
	
	sessionManager.sessions[session.ID] = session
	
	// Create config with minimal components
	config := &Config{
		OfficeManager:  office.NewManager(),
		Analyzer:       &mockAnalyzer{},
		SessionManager: sessionManager,
		LedgerClient:   &mockLedgerClient{},
	}
	
	planner, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
	defer planner.Close()
	
	// Get session status
	status, err := planner.GetSessionStatus(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session status: %v", err)
	}
	
	if status == nil {
		t.Fatal("Status is nil")
	}
	
	if status.Session.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, status.Session.ID)
	}
	
	if status.WorkItem.ID != workItem.ID {
		t.Errorf("Expected work item ID %s, got %s", workItem.ID, status.WorkItem.ID)
	}
	
	if status.Analysis == nil {
		t.Error("Expected analysis in status")
	}
	
	if status.EstimatedCostUSD != 2.50 {
		t.Errorf("Expected estimated cost $2.50, got $%.2f", status.EstimatedCostUSD)
	}
	
	t.Logf("Session status: state=%s, progress=%.0f%%, cost=$%.2f", 
		status.Session.State, status.ProgressPercent, status.EstimatedCostUSD)
}

func TestDefaultPlanner_ApproveRejectSession(t *testing.T) {
	// Create mocks
	sessionManager := newMockSessionManager()
	
	// Create a session in blocked state
	workItem := &contracts.WorkItem{
		ID:        "TEST-789",
		Title:     "Test Approval",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-789",
		},
	}
	
	ctx := context.Background()
	session, err := sessionManager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Manually set to blocked (simulating planner)
	session.State = contracts.SessionStateBlocked
	sessionManager.sessions[session.ID] = session
	
	// Create planner
	config := &Config{
		OfficeManager:  office.NewManager(),
		Analyzer:       &mockAnalyzer{},
		SessionManager: sessionManager,
		LedgerClient:   &mockLedgerClient{},
	}
	
	planner, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
	defer planner.Close()
	
	// Test approval
	err = planner.ApproveSession(ctx, session.ID, "test-user", "Looks good")
	if err != nil {
		t.Fatalf("Failed to approve session: %v", err)
	}
	
	// Check session state
	updated, err := sessionManager.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	// Should be in_progress (since we auto-transition after approval)
	if updated.State != contracts.SessionStateInProgress {
		t.Errorf("Expected state in_progress after approval, got %s", updated.State)
	}
	
	t.Logf("Session approved and transitioned to %s", updated.State)
	
	// Create another session for rejection test
	session2, err := sessionManager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	session2.State = contracts.SessionStateBlocked
	sessionManager.sessions[session2.ID] = session2
	
	// Test rejection
	err = planner.RejectSession(ctx, session2.ID, "test-user", "Not aligned with goals")
	if err != nil {
		t.Fatalf("Failed to reject session: %v", err)
	}
	
	updated2, err := sessionManager.GetSession(ctx, session2.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	if updated2.State != contracts.SessionStateCanceled {
		t.Errorf("Expected state canceled after rejection, got %s", updated2.State)
	}
	
	t.Logf("Session rejected and transitioned to %s", updated2.State)
}