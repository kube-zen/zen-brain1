package session

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// mockZenContext is a mock implementation of zenctx.ZenContext for testing.
type mockZenContext struct {
	mu       sync.RWMutex
	sessions map[string]*zenctx.SessionContext // key: clusterID:sessionID
}

func newMockZenContext() *mockZenContext {
	return &mockZenContext{
		sessions: make(map[string]*zenctx.SessionContext),
	}
}

func (m *mockZenContext) GetSessionContext(ctx context.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := clusterID + ":" + sessionID
	return m.sessions[key], nil
}

func (m *mockZenContext) StoreSessionContext(ctx context.Context, clusterID string, session *zenctx.SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := clusterID + ":" + session.SessionID
	m.sessions[key] = session
	return nil
}

func (m *mockZenContext) DeleteSessionContext(ctx context.Context, clusterID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := clusterID + ":" + sessionID
	delete(m.sessions, key)
	return nil
}

func (m *mockZenContext) QueryKnowledge(ctx context.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	return nil, nil
}

func (m *mockZenContext) StoreKnowledge(ctx context.Context, chunks []zenctx.KnowledgeChunk) error {
	return nil
}

func (m *mockZenContext) ArchiveSession(ctx context.Context, clusterID, sessionID string) error {
	return nil
}

func (m *mockZenContext) ReconstructSession(ctx context.Context, req zenctx.ReMeRequest) (*zenctx.ReMeResponse, error) {
	// Simple reconstruction that returns session if exists
	session, err := m.GetSessionContext(ctx, req.ClusterID, req.SessionID)
	if err != nil || session == nil {
		// Create fresh session
		session = &zenctx.SessionContext{
			SessionID:      req.SessionID,
			TaskID:         req.TaskID,
			ClusterID:      req.ClusterID,
			ProjectID:      req.ProjectID,
			CreatedAt:      time.Now(),
			LastAccessedAt: time.Now(),
		}
	}

	return &zenctx.ReMeResponse{
		SessionContext:  session,
		JournalEntries:  []interface{}{},
		ReconstructedAt: time.Now(),
	}, nil
}

func (m *mockZenContext) Stats(ctx context.Context) (map[zenctx.Tier]interface{}, error) {
	return map[zenctx.Tier]interface{}{
		zenctx.TierHot: map[string]interface{}{
			"type":          "mock",
			"session_count": len(m.sessions),
		},
	}, nil
}

func (m *mockZenContext) Close() error {
	return nil
}

// countSessions returns the number of sessions stored in the mock.
func (m *mockZenContext) countSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// hasSession checks if a session exists.
func (m *mockZenContext) hasSession(clusterID, sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := clusterID + ":" + sessionID
	_, exists := m.sessions[key]
	return exists
}

// getSession retrieves a session (for verification).
func (m *mockZenContext) getSession(clusterID, sessionID string) *zenctx.SessionContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := clusterID + ":" + sessionID
	return m.sessions[key]
}

func TestSessionManager_WithZenContext(t *testing.T) {
	// Create mock ZenContext
	mockZC := newMockZenContext()

	// Create session manager with ZenContext integration
	config := DefaultConfig()
	config.ZenContext = mockZC

	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create a work item
	workItem := &contracts.WorkItem{
		ID:         "TEST-ZEN-1",
		Title:      "Test ZenContext Integration",
		WorkType:   contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:   contracts.PriorityMedium,
		Status:     contracts.StatusRequested,
		CreatedAt:  time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-ZEN-1",
			Project:  "ZENPROJECT",
		},
	}

	// Create session
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session was created
	if session == nil {
		t.Fatal("Session is nil")
	}

	// Verify ZenContext has a corresponding SessionContext
	if !mockZC.hasSession("default", session.ID) {
		t.Error("ZenContext does not have SessionContext after CreateSession")
	}

	// Verify SessionContext fields
	zenSession := mockZC.getSession("default", session.ID)
	if zenSession == nil {
		t.Fatal("ZenContext session is nil")
	}

	if zenSession.SessionID != session.ID {
		t.Errorf("SessionID mismatch: got %s, want %s", zenSession.SessionID, session.ID)
	}

	if zenSession.TaskID != workItem.ID {
		t.Errorf("TaskID mismatch: got %s, want %s", zenSession.TaskID, workItem.ID)
	}

	if zenSession.ProjectID != workItem.Source.Project {
		t.Errorf("ProjectID mismatch: got %s, want %s", zenSession.ProjectID, workItem.Source.Project)
	}

	if zenSession.ClusterID != "default" {
		t.Errorf("ClusterID mismatch: got %s, want %s", zenSession.ClusterID, "default")
	}

	t.Logf("✓ Session %s created with ZenContext integration", session.ID)

	// Test GetSession updates LastAccessedAt
	initialAccess := zenSession.LastAccessedAt
	time.Sleep(time.Millisecond) // Ensure time advances

	_, err = manager.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	zenSession2 := mockZC.getSession("default", session.ID)
	if zenSession2.LastAccessedAt.Equal(initialAccess) {
		t.Error("LastAccessedAt not updated after GetSession")
	} else {
		t.Logf("✓ GetSession updated LastAccessedAt: %v -> %v", initialAccess, zenSession2.LastAccessedAt)
	}

	// Test UpdateSession works (may update LastAccessedAt)
	session.WorkItem.Title = "Updated Title"
	err = manager.UpdateSession(ctx, session)
	if err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}
	t.Log("✓ UpdateSession succeeded")

	// Test TransitionState works (may update LastAccessedAt)
	err = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "Analysis complete", "analyzer")
	if err != nil {
		t.Fatalf("TransitionState failed: %v", err)
	}
	t.Log("✓ TransitionState succeeded")

	// Verify session count
	if count := mockZC.countSessions(); count != 1 {
		t.Errorf("Expected 1 session in ZenContext, got %d", count)
	}

	t.Log("✓ All ZenContext integration tests passed")
}

func TestSessionManager_WithoutZenContext(t *testing.T) {
	// Ensure session manager works without ZenContext (backward compatibility)
	config := DefaultConfig()
	config.ZenContext = nil // Explicitly nil

	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-NOZEN",
		Title:     "Test No ZenContext",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-NOZEN",
		},
	}

	// Should work without ZenContext
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession failed without ZenContext: %v", err)
	}

	if session == nil {
		t.Fatal("Session is nil")
	}

	t.Log("✓ Session manager works without ZenContext (backward compatibility)")
}

func TestUpdateExecutionCheckpoint_WritesStructuredJSON(t *testing.T) {
	mockZC := newMockZenContext()
	config := DefaultConfig()
	config.ZenContext = mockZC
	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer manager.Close()
	ctx := context.Background()

	workItem := &contracts.WorkItem{ID: "W1", Title: "T", WorkType: contracts.WorkTypeImplementation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{IssueKey: "W1"}}
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	checkpoint := &ExecutionCheckpoint{
		Stage:                "proof_attached",
		SessionID:            session.ID,
		WorkItemID:           "W1",
		BrainTaskIDs:         []string{"task-1", "task-2"},
		ProofPaths:           []string{"/path/to/pow.json"},
		LastRecommendation:   "merge",
		KnowledgeChunkIDs:    []string{"kc-1"},
		KnowledgeSourcePaths: []string{"/docs/source.md"},
		UpdatedAt:            time.Now(),
	}
	err = manager.UpdateExecutionCheckpoint(ctx, session.ID, checkpoint)
	if err != nil {
		t.Fatalf("UpdateExecutionCheckpoint: %v", err)
	}

	// Verify State in ZenContext is JSON
	sc := mockZC.getSession("default", session.ID)
	if sc == nil || len(sc.State) == 0 {
		t.Fatal("SessionContext.State should be set with checkpoint JSON")
	}
	// Quick sanity: state should contain stage and session_id
	if !bytes.Contains(sc.State, []byte("proof_attached")) || !bytes.Contains(sc.State, []byte(session.ID)) {
		t.Error("State JSON should contain stage and session_id")
	}
}

func TestGetExecutionCheckpoint_ReadsBack(t *testing.T) {
	mockZC := newMockZenContext()
	config := DefaultConfig()
	config.ZenContext = mockZC
	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer manager.Close()
	ctx := context.Background()

	workItem := &contracts.WorkItem{ID: "W2", Title: "T", WorkType: contracts.WorkTypeImplementation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{IssueKey: "W2"}}
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	checkpoint := &ExecutionCheckpoint{
		Stage:        "proof_attached",
		SessionID:    session.ID,
		WorkItemID:   "W2",
		BrainTaskIDs: []string{"task-1"},
		ProofPaths:   []string{"/pow.json"},
		UpdatedAt:    time.Now(),
	}
	if err := manager.UpdateExecutionCheckpoint(ctx, session.ID, checkpoint); err != nil {
		t.Fatalf("UpdateExecutionCheckpoint: %v", err)
	}

	read, err := manager.GetExecutionCheckpoint(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetExecutionCheckpoint: %v", err)
	}
	if read == nil {
		t.Fatal("GetExecutionCheckpoint returned nil")
	}
	if read.Stage != "proof_attached" || read.SessionID != session.ID || read.WorkItemID != "W2" {
		t.Errorf("checkpoint mismatch: stage=%s session=%s work=%s", read.Stage, read.SessionID, read.WorkItemID)
	}
	if len(read.BrainTaskIDs) != 1 || read.BrainTaskIDs[0] != "task-1" {
		t.Errorf("BrainTaskIDs mismatch: %v", read.BrainTaskIDs)
	}
	if len(read.ProofPaths) != 1 || read.ProofPaths[0] != "/pow.json" {
		t.Errorf("ProofPaths mismatch: %v", read.ProofPaths)
	}
}

func TestCheckpointPreservesRelevantKnowledge(t *testing.T) {
	mockZC := newMockZenContext()
	config := DefaultConfig()
	config.ZenContext = mockZC
	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer manager.Close()
	ctx := context.Background()

	workItem := &contracts.WorkItem{ID: "W3", Title: "T", WorkType: contracts.WorkTypeImplementation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{IssueKey: "W3"}}
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	checkpoint := &ExecutionCheckpoint{
		Stage:                "proof_attached",
		SessionID:            session.ID,
		WorkItemID:           "W3",
		KnowledgeChunkIDs:     []string{"chunk-1", "chunk-2"},
		KnowledgeSourcePaths: []string{"/kb/doc1.md", "/kb/doc2.md"},
		UpdatedAt:            time.Now(),
	}
	if err := manager.UpdateExecutionCheckpoint(ctx, session.ID, checkpoint); err != nil {
		t.Fatalf("UpdateExecutionCheckpoint: %v", err)
	}

	read, err := manager.GetExecutionCheckpoint(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetExecutionCheckpoint: %v", err)
	}
	if len(read.KnowledgeChunkIDs) != 2 || read.KnowledgeChunkIDs[0] != "chunk-1" || read.KnowledgeChunkIDs[1] != "chunk-2" {
		t.Errorf("KnowledgeChunkIDs not preserved: %v", read.KnowledgeChunkIDs)
	}
	if len(read.KnowledgeSourcePaths) != 2 || read.KnowledgeSourcePaths[0] != "/kb/doc1.md" || read.KnowledgeSourcePaths[1] != "/kb/doc2.md" {
		t.Errorf("KnowledgeSourcePaths not preserved: %v", read.KnowledgeSourcePaths)
	}
}

func TestGetExecutionCheckpointSummary_RendersStableText(t *testing.T) {
	mockZC := newMockZenContext()
	config := DefaultConfig()
	config.ZenContext = mockZC
	manager, err := New(config, NewMemoryStore())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer manager.Close()
	ctx := context.Background()

	workItem := &contracts.WorkItem{ID: "W4", Title: "T", WorkType: contracts.WorkTypeImplementation, CreatedAt: time.Now(), Source: contracts.SourceMetadata{IssueKey: "W4"}}
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	checkpoint := &ExecutionCheckpoint{
		Stage:              "proof_attached",
		SessionID:          session.ID,
		WorkItemID:         "W4",
		BrainTaskIDs:       []string{"t1"},
		ProofPaths:         []string{"/p.json"},
		SelectedModel:      "glm-4.7",
		LastRecommendation: "merge",
		UpdatedAt:          time.Now(),
	}
	if err := manager.UpdateExecutionCheckpoint(ctx, session.ID, checkpoint); err != nil {
		t.Fatalf("UpdateExecutionCheckpoint: %v", err)
	}

	summary, err := manager.GetExecutionCheckpointSummary(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetExecutionCheckpointSummary: %v", err)
	}
	if summary == "" {
		t.Fatal("summary should be non-empty")
	}
	if !bytes.Contains([]byte(summary), []byte("proof_attached")) || !bytes.Contains([]byte(summary), []byte(session.ID)) {
		t.Errorf("summary should contain stage and session id: %s", summary)
	}
	if !bytes.Contains([]byte(summary), []byte("merge")) {
		t.Errorf("summary should contain last recommendation: %s", summary)
	}
	// SelectedModel and AnalysisSummary are included when present on the checkpoint
	if !bytes.Contains([]byte(summary), []byte("Tasks:")) || !bytes.Contains([]byte(summary), []byte("Proof Artifacts:")) {
		t.Errorf("summary should contain task and proof counts: %s", summary)
	}
}

func TestShouldSkipReplayForResume(t *testing.T) {
	tests := []struct {
		name     string
		cp       *ExecutionCheckpoint
		wantSkip bool
	}{
		{"nil", nil, false},
		{"no proofs", &ExecutionCheckpoint{Stage: "proof_attached", ProofPaths: nil}, false},
		{"proof_attached with paths", &ExecutionCheckpoint{Stage: "proof_attached", ProofPaths: []string{"/p.json"}}, true},
		{"execution_complete with paths", &ExecutionCheckpoint{Stage: "execution_complete", ProofPaths: []string{"/p.json"}}, true},
		{"in_progress with paths", &ExecutionCheckpoint{Stage: "in_progress", ProofPaths: []string{"/p.json"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSkipReplayForResume(tt.cp)
			if got != tt.wantSkip {
				t.Errorf("ShouldSkipReplayForResume() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}
