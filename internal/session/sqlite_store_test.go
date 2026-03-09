package session

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

func TestSQLiteStore_CreateAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	session := &contracts.Session{
		ID:         "test-session-1",
		WorkItemID: "TEST-123",
		SourceKey:  "TEST-123",
		State:      contracts.SessionStateCreated,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		StateHistory: []contracts.StateTransition{
			{
				FromState: "",
				ToState:   contracts.SessionStateCreated,
				Timestamp: time.Now(),
				Reason:    "Created",
				Agent:     "test",
			},
		},
	}

	// Create
	err = store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("Expected ID %s, got %s", session.ID, retrieved.ID)
	}
	if retrieved.WorkItemID != session.WorkItemID {
		t.Errorf("Expected WorkItemID %s, got %s", session.WorkItemID, retrieved.WorkItemID)
	}
	if retrieved.State != session.State {
		t.Errorf("Expected State %s, got %s", session.State, retrieved.State)
	}
	if len(retrieved.StateHistory) != 1 {
		t.Errorf("Expected 1 state history entry, got %d", len(retrieved.StateHistory))
	}
}

func TestSQLiteStore_GetByWorkItem(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	workItemID := "TEST-456"
	session1 := &contracts.Session{
		ID:         "session-1",
		WorkItemID: workItemID,
		SourceKey:  "TEST-456",
		State:      contracts.SessionStateCreated,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	session2 := &contracts.Session{
		ID:         "session-2",
		WorkItemID: workItemID,
		SourceKey:  "TEST-456",
		State:      contracts.SessionStateCompleted,
		CreatedAt:  time.Now().Add(time.Hour),
		UpdatedAt:  time.Now().Add(time.Hour),
	}

	// Create both sessions
	err = store.Create(ctx, session1)
	if err != nil {
		t.Fatalf("Create session1 failed: %v", err)
	}
	err = store.Create(ctx, session2)
	if err != nil {
		t.Fatalf("Create session2 failed: %v", err)
	}

	// GetByWorkItem should return the active session (not completed)
	retrieved, err := store.GetByWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatalf("GetByWorkItem failed: %v", err)
	}
	if retrieved.ID != session1.ID {
		t.Errorf("Expected active session %s, got %s", session1.ID, retrieved.ID)
	}
}

func TestSQLiteStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	session := &contracts.Session{
		ID:         "session-update",
		WorkItemID: "TEST-789",
		SourceKey:  "TEST-789",
		State:      contracts.SessionStateCreated,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err = store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	session.State = contracts.SessionStateAnalyzed
	session.UpdatedAt = time.Now()
	err = store.Update(ctx, session)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if retrieved.State != contracts.SessionStateAnalyzed {
		t.Errorf("Expected state Analyzed, got %s", retrieved.State)
	}
}

func TestSQLiteStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	// Create sessions with different states
	sessions := []*contracts.Session{
		{ID: "s1", WorkItemID: "w1", SourceKey: "TEST-1", State: contracts.SessionStateCreated, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "s2", WorkItemID: "w2", SourceKey: "TEST-2", State: contracts.SessionStateAnalyzed, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "s3", WorkItemID: "w3", SourceKey: "TEST-3", State: contracts.SessionStateCreated, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	for _, s := range sessions {
		err = store.Create(ctx, s)
		if err != nil {
			t.Fatalf("Create %s failed: %v", s.ID, err)
		}
	}

	// Filter by state
	filter := SessionFilter{
		State: &[]contracts.SessionState{contracts.SessionStateCreated}[0],
	}
	list, err := store.List(ctx, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("Expected 2 Created sessions, got %d", len(list))
	}
	for _, s := range list {
		if s.State != contracts.SessionStateCreated {
			t.Errorf("Session %s has unexpected state %s", s.ID, s.State)
		}
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	session := &contracts.Session{
		ID:         "session-delete",
		WorkItemID: "TEST-DEL",
		SourceKey:  "TEST-DEL",
		State:      contracts.SessionStateCreated,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err = store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete
	err = store.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist
	_, err = store.Get(ctx, session.ID)
	if err != ErrSessionNotFound {
		t.Errorf("Expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestSQLiteStore_ManagerIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.StoreType = "sqlite"
	config.DataDir = tmpDir

	manager, err := New(config, nil) // Let manager create the store
	if err != nil {
		t.Fatalf("Failed to create manager with SQLite store: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-MGR",
		Title:     "Manager Integration Test",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-MGR",
		},
	}

	// Create session
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Transition state
	err = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "Analyzed", "analyzer")
	if err != nil {
		t.Fatalf("TransitionState failed: %v", err)
	}

	// Verify
	updated, err := manager.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if updated.State != contracts.SessionStateAnalyzed {
		t.Errorf("Expected state Analyzed, got %s", updated.State)
	}
}

// Test that SQLiteStore handles JSON fields correctly (WorkItem, AnalysisResult, etc.)
func TestSQLiteStore_JSONFields(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLiteStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-JSON",
		Title:     "JSON Test",
		WorkType:  contracts.WorkTypeResearch,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-JSON",
		},
	}
	analysisResult := &contracts.AnalysisResult{
		WorkItem:       workItem,
		Confidence:     0.85,
		AnalysisNotes:  "Looks good",
		RequiresApproval: false,
	}
	session := &contracts.Session{
		ID:             "session-json",
		WorkItemID:     workItem.ID,
		SourceKey:      workItem.Source.IssueKey,
		State:          contracts.SessionStateAnalyzed,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		WorkItem:       workItem,
		AnalysisResult: analysisResult,
		BrainTaskSpecs: []contracts.BrainTaskSpec{
			{
				ID:          "task-1",
				Title:       "Do something",
				Description: "A task",
				WorkItemID:  workItem.ID,
				WorkType:    contracts.WorkTypeResearch,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
		EvidenceItems: []contracts.EvidenceItem{
			{
				ID:        "ev-1",
				SessionID: "session-json",
				Type:      contracts.EvidenceTypeHypothesis,
				Content:   "Hypothesis",
				CollectedAt: time.Now(),
				CollectedBy: "test",
			},
		},
	}

	err = store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create with JSON fields failed: %v", err)
	}

	retrieved, err := store.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.WorkItem == nil {
		t.Fatal("WorkItem should not be nil")
	}
	if retrieved.WorkItem.ID != workItem.ID {
		t.Errorf("WorkItem.ID mismatch: expected %s, got %s", workItem.ID, retrieved.WorkItem.ID)
	}
	if retrieved.AnalysisResult == nil {
		t.Fatal("AnalysisResult should not be nil")
	}
	if retrieved.AnalysisResult.Confidence != analysisResult.Confidence {
		t.Errorf("AnalysisResult.Confidence mismatch: expected %f, got %f", analysisResult.Confidence, retrieved.AnalysisResult.Confidence)
	}
	if len(retrieved.BrainTaskSpecs) != 1 {
		t.Errorf("Expected 1 BrainTaskSpec, got %d", len(retrieved.BrainTaskSpecs))
	}
	if len(retrieved.EvidenceItems) != 1 {
		t.Errorf("Expected 1 EvidenceItem, got %d", len(retrieved.EvidenceItems))
	}
}