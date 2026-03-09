// Integration test for the complete three-tier memory system.
// This test demonstrates the full ReMe protocol workflow.

package context

import (
	stdctx "context"
	"testing"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// TestThreeTierMemorySystem demonstrates the complete workflow:
// 1. Store session context in Tier 1 (Hot)
// 2. Query knowledge from Tier 2 (Warm)
// 3. Archive session to Tier 3 (Cold)
// 4. Reconstruct session using ReMe protocol
func TestThreeTierMemorySystem(t *testing.T) {
	// Create mock stores for all three tiers
	hotStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierHot: map[string]interface{}{
			"type": "mock-redis",
		},
	})
	
	warmStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierWarm: map[string]interface{}{
			"type": "mock-qmd",
		},
	})
	
	coldStore := newMockStore(map[zenctx.Tier]interface{}{
		zenctx.TierCold: map[string]interface{}{
			"type": "mock-s3",
		},
	})
	
	// Create composite with all three tiers
	config := &Config{
		Hot:   hotStore,
		Warm:  warmStore,
		Cold:  coldStore,
		Verbose: true,
	}
	
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()
	
	ctx := stdctx.Background()
	
	// Step 1: Create and store a session in Tier 1
	session := &zenctx.SessionContext{
		SessionID:     "session-integration-test",
		TaskID:        "task-analyze-requirements",
		ClusterID:     "cluster-staging",
		ProjectID:     "project-zen",
		CreatedAt:     time.Now(),
		LastAccessedAt: time.Now(),
		State:         []byte(`{"step": "analysis", "progress": 0.3}`),
	}
	
	err = composite.StoreSessionContext(ctx, session.ClusterID, session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}
	
	t.Log("✓ Step 1: Session stored in Tier 1 (Hot)")
	
	// Step 2: Query knowledge from Tier 2
	queryOpts := zenctx.QueryOptions{
		Query:   "how to analyze system requirements",
		Scopes:  []string{"company", "general"},
		Limit:   5,
		ClusterID: session.ClusterID,
		ProjectID: session.ProjectID,
	}
	
	kbChunks, err := composite.QueryKnowledge(ctx, queryOpts)
	if err != nil {
		t.Fatalf("QueryKnowledge failed: %v", err)
	}
	
	t.Logf("✓ Step 2: Retrieved %d knowledge chunks from Tier 2", len(kbChunks))
	
	// Step 3: Update session with retrieved knowledge
	session.RelevantKnowledge = kbChunks
	err = composite.StoreSessionContext(ctx, session.ClusterID, session)
	if err != nil {
		t.Fatalf("StoreSessionContext (update) failed: %v", err)
	}
	
	t.Log("✓ Step 3: Updated session with knowledge chunks")
	
	// Step 4: Archive session to Tier 3
	err = composite.ArchiveSession(ctx, session.ClusterID, session.SessionID)
	if err != nil {
		t.Fatalf("ArchiveSession failed: %v", err)
	}
	
	t.Log("✓ Step 4: Session archived to Tier 3 (Cold)")
	
	// Step 5: Delete from Tier 1 (simulating session expiration)
	err = composite.DeleteSessionContext(ctx, session.ClusterID, session.SessionID)
	if err != nil {
		t.Fatalf("DeleteSessionContext failed: %v", err)
	}
	
	t.Log("✓ Step 5: Session deleted from Tier 1 (simulating TTL expiration)")
	
	// Step 6: Reconstruct session using ReMe protocol
	req := zenctx.ReMeRequest{
		SessionID: session.SessionID,
		TaskID:    session.TaskID,
		ClusterID: session.ClusterID,
		ProjectID: session.ProjectID,
		UpToTime:  time.Now(),
	}
	
	reconstructed, err := composite.ReconstructSession(ctx, req)
	if err != nil {
		t.Fatalf("ReconstructSession failed: %v", err)
	}
	
	if reconstructed.SessionContext == nil {
		t.Fatal("ReconstructSession returned nil session")
	}
	
	t.Log("✓ Step 6: Session reconstructed using ReMe protocol")
	
	// Verify reconstructed session
	reconstructedSession := reconstructed.SessionContext
	if reconstructedSession.SessionID != session.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s",
			reconstructedSession.SessionID, session.SessionID)
	}
	
	if reconstructedSession.TaskID != session.TaskID {
		t.Errorf("TaskID mismatch: got %s, want %s",
			reconstructedSession.TaskID, session.TaskID)
	}
	
	// Session should have been retrieved from Tier 3 (Cold) and stored back in Tier 1
	// Verify it's now in Tier 1 again
	tier1Session, err := composite.GetSessionContext(ctx, session.ClusterID, session.SessionID)
	if err != nil {
		t.Fatalf("GetSessionContext after reconstruction failed: %v", err)
	}
	
	if tier1Session == nil {
		t.Error("Expected session to be in Tier 1 after reconstruction")
	} else {
		t.Log("✓ Session successfully restored to Tier 1 after reconstruction")
	}
	
	// Step 7: Check statistics
	stats, err := composite.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	
	// Should have stats from all three tiers
	for _, tier := range []zenctx.Tier{zenctx.TierHot, zenctx.TierWarm, zenctx.TierCold} {
		if _, ok := stats[tier]; !ok {
			t.Errorf("Missing stats for tier %s", tier)
		} else {
			t.Logf("✓ Stats available for tier %s", tier)
		}
	}
	
	t.Log("✓ Step 7: Statistics collected from all three tiers")
	
	t.Log("\n🎉 Three-tier memory system integration test PASSED")
}

// TestReMeProtocol_WithJournal demonstrates ReMe protocol with journal integration.
func TestReMeProtocol_WithJournal(t *testing.T) {
	// Create mock stores
	hotStore := newMockStore(nil)
	warmStore := newMockStore(nil)
	
	// Create mock journal
	mockJournal := &mockJournal{
		results: []interface{}{
			map[string]interface{}{
				"event_type": "task_started",
				"task_id":    "task-analyze",
				"timestamp":  time.Now().Add(-1 * time.Hour),
			},
			map[string]interface{}{
				"event_type": "action_executed",
				"task_id":    "task-analyze",
				"timestamp":  time.Now().Add(-30 * time.Minute),
			},
			map[string]interface{}{
				"event_type": "task_completed",
				"task_id":    "task-analyze",
				"timestamp":  time.Now().Add(-15 * time.Minute),
			},
		},
	}
	
	config := &Config{
		Hot:     hotStore,
		Warm:    warmStore,
		Journal: mockJournal,
		Verbose: true,
	}
	
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()
	
	ctx := stdctx.Background()
	
	// Reconstruct session that doesn't exist in any tier
	// Should create fresh session and query journal
	req := zenctx.ReMeRequest{
		SessionID: "session-with-journal",
		TaskID:    "task-analyze",
		ClusterID: "cluster-1",
		ProjectID: "project-zen",
	}
	
	reconstructed, err := composite.ReconstructSession(ctx, req)
	if err != nil {
		t.Fatalf("ReconstructSession failed: %v", err)
	}
	
	if reconstructed.SessionContext == nil {
		t.Fatal("ReconstructSession returned nil session")
	}
	
	// Should have journal entries
	if len(reconstructed.JournalEntries) == 0 {
		t.Error("Expected journal entries in reconstruction")
	} else {
		t.Logf("✓ Retrieved %d journal entries during reconstruction", len(reconstructed.JournalEntries))
	}
	
	// Verify session was created
	if reconstructed.SessionContext.SessionID != req.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s",
			reconstructed.SessionContext.SessionID, req.SessionID)
	}
	
	// Verify task ID matches
	if reconstructed.SessionContext.TaskID != req.TaskID {
		t.Errorf("TaskID mismatch: got %s, want %s",
			reconstructed.SessionContext.TaskID, req.TaskID)
	}
	
	t.Log("✓ ReMe protocol with journal integration PASSED")
}

// TestTierFallback demonstrates tier fallback during reconstruction.
func TestTierFallback(t *testing.T) {
	// Create composite with only Tier 1 (minimal configuration)
	hotStore := newMockStore(nil)
	
	config := &Config{
		Hot:   hotStore,
		Verbose: true,
	}
	
	composite, err := NewComposite(config)
	if err != nil {
		t.Fatalf("NewComposite failed: %v", err)
	}
	defer composite.Close()
	
	ctx := stdctx.Background()
	
	// Test 1: Session not found anywhere - should create fresh session
	req1 := zenctx.ReMeRequest{
		SessionID: "session-not-found",
		TaskID:    "task-123",
		ClusterID: "cluster-1",
	}
	
	reconstructed1, err := composite.ReconstructSession(ctx, req1)
	if err != nil {
		t.Fatalf("ReconstructSession (not found) failed: %v", err)
	}
	
	if reconstructed1.SessionContext == nil {
		t.Fatal("ReconstructSession returned nil session")
	}
	
	if reconstructed1.SessionContext.SessionID != req1.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s",
			reconstructed1.SessionContext.SessionID, req1.SessionID)
	}
	
	t.Log("✓ Tier fallback: Created fresh session when not found")
	
	// Test 2: Session exists in Tier 1 - should retrieve directly
	session := &zenctx.SessionContext{
		SessionID: "session-in-tier1",
		TaskID:    "task-456",
		ClusterID: "cluster-1",
		CreatedAt: time.Now(),
		LastAccessedAt: time.Now(),
	}
	
	err = composite.StoreSessionContext(ctx, session.ClusterID, session)
	if err != nil {
		t.Fatalf("StoreSessionContext failed: %v", err)
	}
	
	req2 := zenctx.ReMeRequest{
		SessionID: session.SessionID,
		TaskID:    session.TaskID,
		ClusterID: session.ClusterID,
	}
	
	reconstructed2, err := composite.ReconstructSession(ctx, req2)
	if err != nil {
		t.Fatalf("ReconstructSession (in Tier 1) failed: %v", err)
	}
	
	if reconstructed2.SessionContext == nil {
		t.Fatal("ReconstructSession returned nil session for Tier 1")
	}
	
	t.Log("✓ Tier fallback: Retrieved session directly from Tier 1")
	
	// Test 3: With Tier 3 (Cold) - session archived
	// (This would require a more complex test with actual archiving)
	t.Log("✓ Tier fallback tests PASSED")
}