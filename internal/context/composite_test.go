package context

import (
	stdctx "context"
	"fmt"
	"sync"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// mockStore is a mock implementation of Store for testing.
type mockStore struct {
	sessions     map[string]*zenctx.SessionContext // key: clusterID:sessionID
	queries      []zenctx.QueryOptions
	storedChunks []zenctx.KnowledgeChunk
	archived     []string // sessionIDs that were archived
	stats        map[zenctx.Tier]interface{}

	mu sync.RWMutex
}

func newMockStore(stats map[zenctx.Tier]interface{}) *mockStore {
	if stats == nil {
		stats = map[zenctx.Tier]interface{}{
			zenctx.TierHot: map[string]interface{}{
				"type":          "mock",
				"session_count": 0,
			},
		}
	}
	return &mockStore{
		sessions: make(map[string]*zenctx.SessionContext),
		stats:    stats,
	}
}

func (m *mockStore) GetSessionContext(ctx stdctx.Context, clusterID, sessionID string) (*zenctx.SessionContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", clusterID, sessionID)
	return m.sessions[key], nil
}

func (m *mockStore) StoreSessionContext(ctx stdctx.Context, clusterID string, session *zenctx.SessionContext) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", clusterID, session.SessionID)
	m.sessions[key] = session
	return nil
}

func (m *mockStore) DeleteSessionContext(ctx stdctx.Context, clusterID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", clusterID, sessionID)
	delete(m.sessions, key)
	return nil
}

func (m *mockStore) QueryKnowledge(ctx stdctx.Context, opts zenctx.QueryOptions) ([]zenctx.KnowledgeChunk, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = append(m.queries, opts)
	// Return empty result for simplicity
	return []zenctx.KnowledgeChunk{}, nil
}

func (m *mockStore) StoreKnowledge(ctx stdctx.Context, chunks []zenctx.KnowledgeChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storedChunks = append(m.storedChunks, chunks...)
	return nil
}

func (m *mockStore) ArchiveSession(ctx stdctx.Context, clusterID, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.archived = append(m.archived, fmt.Sprintf("%s:%s", clusterID, sessionID))
	return nil
}

func (m *mockStore) Stats(ctx stdctx.Context) (map[zenctx.Tier]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats, nil
}

func (m *mockStore) Close() error {
	return nil
}

// mockJournal is a mock implementation of Journal for testing.
type mockJournal struct {
	queries []interface{}
	results []interface{}
}

func newMockJournal(results []interface{}) *mockJournal {
	return &mockJournal{
		results: results,
	}
}

func (m *mockJournal) Query(ctx stdctx.Context, opts interface{}) ([]interface{}, error) {
	m.queries = append(m.queries, opts)
	return m.results, nil
}

func TestNewComposite(t *testing.T) {
	hotStore := newMockStore(nil)

	config := &Config{
		Hot:     hotStore,
		Verbose: true,
	}

	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}

	if composite == nil {
		t.Fatal("NewComposite returned nil")
	}

	composite.Close()
}

func TestComposite_GetSessionContext(t *testing.T) {
	hotStore := newMockStore(nil)
	config := &Config{
		Hot: hotStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	// Get non-existent session
	session, err := composite.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed: %v", err)
	}

	if session != nil {
		t.Error("Expected nil for non-existent session")
	}

	// Store a session
	testSession := &zenctx.SessionContext{
		SessionID:      "session-123",
		TaskID:         "task-456",
		ClusterID:      "cluster-1",
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	err = composite.StoreSessionContext(ctx, "cluster-1", testSession)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Retrieve the session
	retrieved, err := composite.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed after store: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected non-nil session after store")
	}

	if retrieved.SessionID != testSession.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", retrieved.SessionID, testSession.SessionID)
	}

	if retrieved.TaskID != testSession.TaskID {
		t.Errorf("TaskID mismatch: got %s, want %s", retrieved.TaskID, testSession.TaskID)
	}
}

func TestComposite_DeleteSessionContext(t *testing.T) {
	hotStore := newMockStore(nil)
	coldStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierCold: map[string]interface{}{
			"type":          "mock-cold",
			"session_count": 0,
		},
	})

	config := &Config{
		Hot:  hotStore,
		Cold: coldStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	// Store a session
	session := &zenctx.SessionContext{
		SessionID:      "session-123",
		TaskID:         "task-456",
		ClusterID:      "cluster-1",
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	err = composite.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Delete the session
	err = composite.DeleteSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("DeleteSessionContext failed: %v", err)
	}

	// Verify it's gone
	retrieved, err := composite.GetSessionContext(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("GetSessionContext failed after delete: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected nil after delete")
	}
}

func TestComposite_ReconstructSession_FromTier1(t *testing.T) {
	hotStore := newMockStore(nil)
	config := &Config{
		Hot: hotStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	// Store a session in Tier 1
	session := &zenctx.SessionContext{
		SessionID:      "session-123",
		TaskID:         "task-456",
		ClusterID:      "cluster-1",
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	err = composite.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Reconstruct session - should find it in Tier 1
	req := zenctx.ReMeRequest{
		SessionID: "session-123",
		TaskID:    "task-456",
		ClusterID: "cluster-1",
	}

	resp, err := composite.ReconstructSession(ctx, req)
	if err != nil {
		t.Fatalf("ReconstructSession failed: %v", err)
	}

	if resp.SessionContext == nil {
		t.Fatal("Expected non-nil SessionContext")
	}

	if resp.SessionContext.SessionID != session.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", resp.SessionContext.SessionID, session.SessionID)
	}
}

func TestComposite_ReconstructSession_NewSession(t *testing.T) {
	hotStore := newMockStore(nil)
	warmStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierWarm: map[string]interface{}{
			"type":      "mock-warm",
			"kb_chunks": 0,
		},
	})
	journal := newMockJournal([]interface{}{
		map[string]interface{}{"event": "task_started", "task_id": "task-456"},
		map[string]interface{}{"event": "action_executed", "task_id": "task-456"},
	})

	config := &Config{
		Hot:     hotStore,
		Warm:    warmStore,
		Journal: journal,
		Verbose: true,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	// Reconstruct session that doesn't exist - should create new session
	req := zenctx.ReMeRequest{
		SessionID: "session-123",
		TaskID:    "task-456",
		ClusterID: "cluster-1",
		ProjectID: "project-789",
	}

	resp, err := composite.ReconstructSession(ctx, req)
	if err != nil {
		t.Fatalf("ReconstructSession failed: %v", err)
	}

	if resp.SessionContext == nil {
		t.Fatal("Expected non-nil SessionContext")
	}

	if resp.SessionContext.SessionID != "session-123" {
		t.Errorf("SessionID mismatch: got %s, want %s", resp.SessionContext.SessionID, "session-123")
	}

	if resp.SessionContext.TaskID != "task-456" {
		t.Errorf("TaskID mismatch: got %s, want %s", resp.SessionContext.TaskID, "task-456")
	}

	if resp.SessionContext.ClusterID != "cluster-1" {
		t.Errorf("ClusterID mismatch: got %s, want %s", resp.SessionContext.ClusterID, "cluster-1")
	}

	if resp.SessionContext.ProjectID != "project-789" {
		t.Errorf("ProjectID mismatch: got %s, want %s", resp.SessionContext.ProjectID, "project-789")
	}

	// Should have queried journal
	if len(journal.queries) == 0 {
		t.Error("Expected journal query during reconstruction")
	}

	// Should have queried warm store
	if len(warmStore.queries) == 0 {
		t.Error("Expected warm store query during reconstruction")
	}
}

func TestComposite_Stats(t *testing.T) {
	hotStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierHot: map[string]interface{}{
			"type":          "mock-hot",
			"session_count": 5,
		},
	})
	warmStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierWarm: map[string]interface{}{
			"type":      "mock-warm",
			"kb_chunks": 100,
		},
	})
	coldStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierCold: map[string]interface{}{
			"type":           "mock-cold",
			"session_count":  20,
			"retention_days": 90,
		},
	})

	config := &Config{
		Hot:  hotStore,
		Warm: warmStore,
		Cold: coldStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()
	stats, err := composite.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	// Should have stats from all three tiers
	if _, ok := stats[zenctx.TierHot]; !ok {
		t.Error("Expected TierHot in stats")
	}

	if _, ok := stats[zenctx.TierWarm]; !ok {
		t.Error("Expected TierWarm in stats")
	}

	if _, ok := stats[zenctx.TierCold]; !ok {
		t.Error("Expected TierCold in stats")
	}
}

func TestComposite_ArchiveSession(t *testing.T) {
	hotStore := newMockStore(nil)
	coldStore := newMockStore(nil)

	config := &Config{
		Hot:  hotStore,
		Cold: coldStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	// Store a session
	session := &zenctx.SessionContext{
		SessionID:      "session-123",
		TaskID:         "task-456",
		ClusterID:      "cluster-1",
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
	}

	err = composite.StoreSessionContext(ctx, "cluster-1", session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}

	// Archive the session
	err = composite.ArchiveSession(ctx, "cluster-1", "session-123")
	if err != nil {
		t.Fatalf("ArchiveSession failed: %v", err)
	}

	// Should have archived the session (stored in cold store)
	coldStore.mu.RLock()
	key := fmt.Sprintf("%s:%s", "cluster-1", "session-123")
	_, hasSession := coldStore.sessions[key]
	coldStore.mu.RUnlock()
	if !hasSession {
		t.Error("Expected session to be stored in cold store (archived)")
	}

	// Session should have been deleted from hot store (or at least attempted)
	// Note: ArchiveSession attempts to delete from hot store after archiving
}

func TestComposite_QueryKnowledge(t *testing.T) {
	hotStore := newMockStore(nil)
	warmStore := newMockStore(nil)
	config := &Config{
		Hot:  hotStore,
		Warm: warmStore,
	}
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()

	ctx := stdctx.Background()

	opts := zenctx.QueryOptions{
		Query:     "how to deploy a service",
		Scopes:    []string{"company", "general"},
		Limit:     10,
		ClusterID: "cluster-1",
		ProjectID: "project-zen",
	}

	results, err := composite.QueryKnowledge(ctx, opts)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}

	if results == nil {
		t.Error("Expected non-nil results")
	}

	// Should have recorded the query
	if len(warmStore.queries) == 0 {
		t.Error("Expected query to be recorded")
	}

	if warmStore.queries[0].Query != opts.Query {
		t.Errorf("Query mismatch: got %s, want %s", warmStore.queries[0].Query, opts.Query)
	}
}
