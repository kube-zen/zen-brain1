package session

import (
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
			SessionID:     req.SessionID,
			TaskID:        req.TaskID,
			ClusterID:     req.ClusterID,
			ProjectID:     req.ProjectID,
			CreatedAt:     time.Now(),
			LastAccessedAt: time.Now(),
		}
	}
	
	return &zenctx.ReMeResponse{
		SessionContext: session,
		JournalEntries: []interface{}{},
		ReconstructedAt: time.Now(),
	}, nil
}

func (m *mockZenContext) Stats(ctx context.Context) (map[zenctx.Tier]interface{}, error) {
	return map[zenctx.Tier]interface{}{
		zenctx.TierHot: map[string]interface{}{
			"type":         "mock",
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
		ID:        "TEST-ZEN-1",
		Title:     "Test ZenContext Integration",
		WorkType:  contracts.WorkTypeImplementation,
		WorkDomain: contracts.DomainCore,
		Priority:  contracts.PriorityMedium,
		Status:    contracts.StatusRequested,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:    "test",
			IssueKey:  "TEST-ZEN-1",
			Project:   "ZENPROJECT",
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