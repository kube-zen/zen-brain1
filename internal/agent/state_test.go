package agent

import (
	"context"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// mockZenContextForTest is a simplified mock for testing.
type mockZenContextForTest struct {
	sessions map[string]*zenctx.SessionContext
}

func newMockZenContextForTest() *mockZenContextForTest {
	return &mockZenContextForTest{
		sessions: make(map[string]*zenctx.SessionContext),
	}
}

func (m *mockZenContextForTest) GetSessionContext(ctx context.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	key := clusterID + ":" + sessionID
	return m.sessions[key], nil
}

func (m *mockZenContextForTest) StoreSessionContext(ctx context.Context, clusterID string, session *zenctx.SessionContext) error {
	key := clusterID + ":" + session.SessionID
	m.sessions[key] = session
	return nil
}

func (m *mockZenContextForTest) DeleteSessionContext(ctx context.Context, clusterID, sessionID string) error {
	key := clusterID + ":" + sessionID
	delete(m.sessions, key)
	return nil
}

func (m *mockZenContextForTest) QueryKnowledge(ctx context.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return nil, nil
}

func (m *mockZenContextForTest) StoreKnowledge(ctx context.Context, chunks []zenctx.KnowledgeChunk) error {
	return nil
}

func (m *mockZenContextForTest) ArchiveSession(ctx context.Context, clusterID, sessionID string) error {
	return nil
}

func (m *mockZenContextForTest) ReconstructSession(ctx context.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	// Simple reconstruction
	session, _ := m.GetSessionContext(ctx, req.ClusterID, req.SessionID)
	if session == nil {
		session = &zenctx.SessionContext{
			SessionID:      req.SessionID,
			TaskID:         req.TaskID,
			ClusterID:      req.ClusterID,
			ProjectID:      req.ProjectID,
			CreatedAt:      time.Now(),
			LastAccessedAt: time.Now(),
		}
	}
	// Ensure RelevantKnowledge is not nil
	if session.RelevantKnowledge == nil {
		session.RelevantKnowledge = []zenctx.KnowledgeChunk{}
	}
	return &zenctx.ReMeResponse{
		SessionContext:  session,
		JournalEntries:  []interface{}{},
		ReconstructedAt: time.Now(),
	}, nil
}

func (m *mockZenContextForTest) Stats(ctx context.Context) (map[zenctx.Tier]interface{}, error) {
	return nil, nil
}

func (m *mockZenContextForTest) Close() error {
	return nil
}

func TestNewAgentState(t *testing.T) {
	state := NewAgentState("agent-123", RolePlanner, "session-456", "task-789")

	if state.AgentID != "agent-123" {
		t.Errorf("AgentID mismatch: got %s, want %s", state.AgentID, "agent-123")
	}

	if state.AgentRole != RolePlanner {
		t.Errorf("AgentRole mismatch: got %s, want %s", state.AgentRole, RolePlanner)
	}

	if state.SessionID != "session-456" {
		t.Errorf("SessionID mismatch: got %s, want %s", state.SessionID, "session-456")
	}

	if state.TaskID != "task-789" {
		t.Errorf("TaskID mismatch: got %s, want %s", state.TaskID, "task-789")
	}

	if state.IsComplete {
		t.Error("New agent state should not be complete")
	}

	if state.StepsCompleted != 0 {
		t.Errorf("StepsCompleted should be 0, got %d", state.StepsCompleted)
	}

	t.Log("✓ NewAgentState creates correct initial state")
}

func TestAgentState_SerializeDeserialize(t *testing.T) {
	state := NewAgentState("agent-123", RoleWorker, "session-1", "task-1")
	state.UpdateStep("analysis")
	state.UpdateProgress(0.5)
	state.AddDecision("Use model X", "Better for this task", 0.8, []string{"model Y", "model Z"})
	state.AddObservation("System load is high", "system", 0.6)
	state.AddError("Timeout occurred", "analysis", true, "Retried with longer timeout")

	// Serialize
	data, err := state.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialized data is empty")
	}

	// Deserialize
	deserialized, err := DeserializeAgentState(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify fields
	if deserialized.AgentID != state.AgentID {
		t.Errorf("AgentID mismatch after deserialize: got %s, want %s", deserialized.AgentID, state.AgentID)
	}

	if deserialized.AgentRole != state.AgentRole {
		t.Errorf("AgentRole mismatch: got %s, want %s", deserialized.AgentRole, state.AgentRole)
	}

	if deserialized.CurrentStep != "analysis" {
		t.Errorf("CurrentStep mismatch: got %s, want %s", deserialized.CurrentStep, "analysis")
	}

	if deserialized.StepProgress != 0.5 {
		t.Errorf("StepProgress mismatch: got %f, want %f", deserialized.StepProgress, 0.5)
	}

	if len(deserialized.Decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(deserialized.Decisions))
	} else {
		if deserialized.Decisions[0].Decision != "Use model X" {
			t.Errorf("Decision mismatch: got %s, want %s", deserialized.Decisions[0].Decision, "Use model X")
		}
	}

	if len(deserialized.Observations) != 1 {
		t.Errorf("Expected 1 observation, got %d", len(deserialized.Observations))
	}

	if len(deserialized.ErrorsEncountered) != 1 {
		t.Errorf("Expected 1 error, got %d", len(deserialized.ErrorsEncountered))
	}

	t.Log("✓ AgentState serialize/deserialize works correctly")
}

func TestStateManager_StoreAndLoadAgentState(t *testing.T) {
	mockZC := newMockZenContextForTest()
	stateManager := NewStateManager(mockZC, "test-cluster")

	ctx := context.Background()

	// Create agent state
	agentState := NewAgentState("test-agent", RolePlanner, "test-session", "test-task")
	agentState.UpdateStep("planning")
	agentState.AddDecision("Test decision", "Test reason", 0.9, nil)

	// Store agent state
	err := stateManager.StoreAgentState(ctx, agentState)
	if err != nil {
		t.Fatalf("StoreAgentState failed: %v", err)
	}

	// Load agent state
	loadedState, err := stateManager.LoadAgentState(ctx, "test-session")
	if err != nil {
		t.Fatalf("LoadAgentState failed: %v", err)
	}

	if loadedState == nil {
		t.Fatal("Loaded state is nil")
	}

	// Verify loaded state
	if loadedState.AgentID != agentState.AgentID {
		t.Errorf("AgentID mismatch: got %s, want %s", loadedState.AgentID, agentState.AgentID)
	}

	if loadedState.SessionID != agentState.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", loadedState.SessionID, agentState.SessionID)
	}

	if loadedState.CurrentStep != "planning" {
		t.Errorf("CurrentStep mismatch: got %s, want %s", loadedState.CurrentStep, "planning")
	}

	if len(loadedState.Decisions) != 1 {
		t.Errorf("Expected 1 decision, got %d", len(loadedState.Decisions))
	}

	t.Log("✓ StateManager store/load works correctly")
}

func TestStateManager_LoadNonExistentAgentState(t *testing.T) {
	mockZC := newMockZenContextForTest()
	stateManager := NewStateManager(mockZC, "test-cluster")

	ctx := context.Background()

	// Try to load non-existent state
	state, err := stateManager.LoadAgentState(ctx, "non-existent-session")
	if err != nil {
		t.Fatalf("LoadAgentState should not error for non-existent session: %v", err)
	}

	// Should return nil, not error
	if state != nil {
		t.Error("Expected nil for non-existent agent state")
	}

	t.Log("✓ StateManager handles non-existent state correctly")
}

func TestStateManager_ReconstructAgent(t *testing.T) {
	mockZC := newMockZenContextForTest()
	stateManager := NewStateManager(mockZC, "test-cluster")

	ctx := context.Background()

	// Reconstruct agent (no existing state)
	agentState, knowledge, err := stateManager.ReconstructAgent(ctx, "new-session", "new-task")
	if err != nil {
		t.Fatalf("ReconstructAgent failed: %v", err)
	}

	if agentState == nil {
		t.Fatal("ReconstructAgent should create new agent state")
	}

	if agentState.SessionID != "new-session" {
		t.Errorf("SessionID mismatch: got %s, want %s", agentState.SessionID, "new-session")
	}

	if agentState.TaskID != "new-task" {
		t.Errorf("TaskID mismatch: got %s, want %s", agentState.TaskID, "new-task")
	}

	if knowledge == nil {
		t.Error("Knowledge should not be nil (empty slice)")
	}

	t.Log("✓ StateManager.ReconstructAgent works correctly")
}

func TestAgentState_Methods(t *testing.T) {
	state := NewAgentState("test-agent", RoleAnalyzer, "session-1", "task-1")

	// Test UpdateStep
	state.UpdateStep("analysis")
	if state.CurrentStep != "analysis" {
		t.Errorf("UpdateStep failed: got %s, want %s", state.CurrentStep, "analysis")
	}

	// Test UpdateProgress
	state.UpdateProgress(0.75)
	if state.StepProgress != 0.75 {
		t.Errorf("UpdateProgress failed: got %f, want %f", state.StepProgress, 0.75)
	}

	// Test CompleteStep
	initialSteps := state.StepsCompleted
	state.CompleteStep()
	if state.StepsCompleted != initialSteps+1 {
		t.Errorf("CompleteStep failed: got %d steps, want %d", state.StepsCompleted, initialSteps+1)
	}
	if state.CurrentStep != "" {
		t.Errorf("CompleteStep should clear CurrentStep, got %s", state.CurrentStep)
	}

	// Test AddDecision
	state.AddDecision("Test decision", "Test reason", 0.8, []string{"alt1", "alt2"})
	if len(state.Decisions) != 1 {
		t.Errorf("AddDecision failed: got %d decisions, want 1", len(state.Decisions))
	}

	// Test AddObservation
	state.AddObservation("Test observation", "system", 0.5)
	if len(state.Observations) != 1 {
		t.Errorf("AddObservation failed: got %d observations, want 1", len(state.Observations))
	}

	// Test Complete
	state.Complete("success", "Task completed successfully")
	if !state.IsComplete {
		t.Error("Complete failed: IsComplete should be true")
	}
	if state.Result != "success" {
		t.Errorf("Complete failed: got result %s, want %s", state.Result, "success")
	}

	t.Log("✓ All AgentState methods work correctly")
}
