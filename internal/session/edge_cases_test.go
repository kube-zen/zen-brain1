package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TestImplicitDefaults_DataDirEmpty tests that empty DataDir with StoreType="sqlite"
// fails gracefully with a clear error message instead of creating files in unexpected locations.
func TestImplicitDefaults_DataDirEmpty(t *testing.T) {
	config := &Config{
		StoreType:       "sqlite",
		DataDir:         "", // Empty - implicit default
		DefaultTimeout:  24 * time.Hour,
		MaxSessionAge:   7 * 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		StaleThreshold:  2 * time.Hour,
		ClusterID:       "test-cluster",
	}

	// Should fail with clear error when DataDir is empty
	manager, err := New(config, nil)
	if err == nil {
		manager.Close()
		t.Fatal("Expected error when DataDir is empty and StoreType is sqlite")
	}

	t.Logf("Got expected error: %v", err)
}

// TestImplicitDefaults_ClusterIDDefault tests that ClusterID="default" is explicit
// and doesn't cause issues in multi-cluster scenarios.
func TestImplicitDefaults_ClusterIDDefault(t *testing.T) {
	tests := []struct {
		name           string
		configClusterID string
		expectedClusterID string
	}{
		{
			name:           "empty_cluster_id_uses_default",
			configClusterID: "",
			expectedClusterID: "default",
		},
		{
			name:           "explicit_cluster_id_preserved",
			configClusterID: "production-us-east-1",
			expectedClusterID: "production-us-east-1",
		},
		{
			name:           "default_cluster_id_explicit",
			configClusterID: "default",
			expectedClusterID: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.ClusterID = tt.configClusterID
			config.StoreType = "memory" // Use memory store to avoid DataDir issues

			manager, err := New(config, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer manager.Close()

			// Verify clusterID() method returns expected value
			actualClusterID := manager.clusterID()
			if actualClusterID != tt.expectedClusterID {
				t.Errorf("Expected ClusterID=%s, got %s", tt.expectedClusterID, actualClusterID)
			}
		})
	}
}

// TestStateTransitions_TerminalStates verifies that terminal states cannot transition.
func TestStateTransitions_TerminalStates(t *testing.T) {
	config := DefaultConfig()
	config.StoreType = "memory"
	manager, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-TERMINAL",
		Title:     "Terminal State Test",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-TERMINAL",
		},
	}

	// Create session and transition to completed
	session, err := manager.CreateSession(ctx, workItem)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Transition through to completed state
	states := []contracts.SessionState{
		contracts.SessionStateAnalyzed,
		contracts.SessionStateScheduled,
		contracts.SessionStateInProgress,
		contracts.SessionStateCompleted,
	}

	for _, state := range states {
		err := manager.TransitionState(ctx, session.ID, state, "test transition", "test")
		if err != nil {
			t.Fatalf("Failed to transition to %s: %v", state, err)
		}
	}

	// Now try to transition from completed (terminal state) - should fail
	terminalTransitions := []contracts.SessionState{
		contracts.SessionStateInProgress,
		contracts.SessionStateScheduled,
		contracts.SessionStateFailed,
		contracts.SessionStateCanceled,
	}

	for _, targetState := range terminalTransitions {
		t.Run(string(targetState), func(t *testing.T) {
			err := manager.TransitionState(ctx, session.ID, targetState, "invalid transition", "test")
			if err == nil {
				t.Errorf("Expected error when transitioning from completed to %s", targetState)
			}
			t.Logf("Got expected error for completed->%s: %v", targetState, err)
		})
	}
}

// TestStateTransitions_AllTerminalStates tests failed and canceled states.
func TestStateTransitions_AllTerminalStates(t *testing.T) {
	for _, terminalState := range []contracts.SessionState{
		contracts.SessionStateCompleted,
		contracts.SessionStateFailed,
		contracts.SessionStateCanceled,
	} {
		t.Run(string(terminalState), func(t *testing.T) {
			config := DefaultConfig()
			config.StoreType = "memory"
			manager, err := New(config, nil)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer manager.Close()

			ctx := context.Background()
			workItem := &contracts.WorkItem{
				ID:        "TEST-" + string(terminalState),
				Title:     "Terminal State Test",
				WorkType:  contracts.WorkTypeImplementation,
				CreatedAt: time.Now(),
				Source: contracts.SourceMetadata{
					System:   "test",
					IssueKey: "TEST-" + string(terminalState),
				},
			}

			session, err := manager.CreateSession(ctx, workItem)
			if err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}

			// Transition to the terminal state
			if terminalState == contracts.SessionStateCompleted {
				// Need to go through full path for completed
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "test", "test")
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateScheduled, "test", "test")
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateInProgress, "test", "test")
			} else if terminalState == contracts.SessionStateFailed {
				// Need to go through in_progress first
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateAnalyzed, "test", "test")
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateScheduled, "test", "test")
				_ = manager.TransitionState(ctx, session.ID, contracts.SessionStateInProgress, "test", "test")
			}
			// For canceled, can cancel from created - no additional transitions needed

			if terminalState != contracts.SessionStateCanceled {
				err = manager.TransitionState(ctx, session.ID, terminalState, "reached terminal state", "test")
				if err != nil {
					t.Fatalf("Failed to transition to %s: %v", terminalState, err)
				}
			} else {
				// Cancel from created state
				err = manager.TransitionState(ctx, session.ID, contracts.SessionStateCanceled, "reached terminal state", "test")
				if err != nil {
					t.Fatalf("Failed to transition to canceled: %v", err)
				}
			}

			// Try to transition from terminal state - should fail
			err = manager.TransitionState(ctx, session.ID, contracts.SessionStateInProgress, "invalid", "test")
			if err == nil {
				t.Errorf("Expected error when transitioning from %s", terminalState)
			}
		})
	}
}

// TestPersistenceBoundary_DataDirValidation tests DataDir validation.
func TestPersistenceBoundary_DataDirValidation(t *testing.T) {
	tests := []struct {
		name        string
		storeType   string
		dataDir     string
		shouldError bool
	}{
		{
			name:        "memory_store_ignores_dataDir",
			storeType:   "memory",
			dataDir:     "",
			shouldError: false,
		},
		{
			name:        "sqlite_empty_dataDir_fails",
			storeType:   "sqlite",
			dataDir:     "",
			shouldError: true,
		},
		{
			name:        "sqlite_valid_dataDir_succeeds",
			storeType:   "sqlite",
			dataDir:     t.TempDir(),
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				StoreType:      tt.storeType,
				DataDir:        tt.dataDir,
				DefaultTimeout: 24 * time.Hour,
				MaxSessionAge:  7 * 24 * time.Hour,
				ClusterID:      "test",
			}

			manager, err := New(config, nil)
			if tt.shouldError {
				if err == nil {
					manager.Close()
					t.Error("Expected error but got none")
				}
				t.Logf("Got expected error: %v", err)
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				manager.Close()
			}
		})
	}
}

// TestConfigValidation_NilConfig tests that nil config is handled properly.
func TestConfigValidation_NilConfig(t *testing.T) {
	manager, err := New(nil, NewMemoryStore())
	if err != nil {
		t.Fatalf("Failed to create manager with nil config: %v", err)
	}
	defer manager.Close()

	// Verify defaults were applied
	if manager.config == nil {
		t.Fatal("Config should not be nil after New()")
	}

	if manager.config.StoreType != "sqlite" {
		t.Errorf("Expected default StoreType=sqlite, got %s", manager.config.StoreType)
	}

	if manager.config.ClusterID != "default" {
		t.Errorf("Expected default ClusterID=default, got %s", manager.config.ClusterID)
	}

	t.Logf("Nil config handled correctly with defaults: StoreType=%s, ClusterID=%s",
		manager.config.StoreType, manager.config.ClusterID)
}

// TestConcurrentSessionCreation tests race condition handling.
func TestConcurrentSessionCreation(t *testing.T) {
	config := DefaultConfig()
	config.StoreType = "memory"
	manager, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	workItem := &contracts.WorkItem{
		ID:        "TEST-CONCURRENT",
		Title:     "Concurrent Test",
		WorkType:  contracts.WorkTypeImplementation,
		CreatedAt: time.Now(),
		Source: contracts.SourceMetadata{
			System:   "test",
			IssueKey: "TEST-CONCURRENT",
		},
	}

	// Try to create multiple sessions for same work item concurrently
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := manager.CreateSession(ctx, workItem)
			results <- err
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < 5; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	// Exactly one should succeed, rest should fail
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful session creation, got %d", successCount)
	}

	if errorCount != 4 {
		t.Errorf("Expected 4 failed session creations, got %d", errorCount)
	}

	t.Logf("Concurrent creation handled correctly: %d success, %d failures", successCount, errorCount)
}

// TestStoreCreation_FallbackPath tests store creation fallback behavior.
func TestStoreCreation_FallbackPath(t *testing.T) {
	// Test that unsupported store type fails with clear error
	config := &Config{
		StoreType:      "invalid-store-type",
		DataDir:        t.TempDir(),
		DefaultTimeout: 24 * time.Hour,
		ClusterID:      "test",
	}

	manager, err := New(config, nil)
	if err == nil {
		manager.Close()
		t.Fatal("Expected error for unsupported store type")
	}

	t.Logf("Got expected error for unsupported store type: %v", err)
}

// TestSQLiteStore_FileLocation tests that SQLite store creates files in correct location.
func TestSQLiteStore_FileLocation(t *testing.T) {
	dataDir := t.TempDir()
	config := &Config{
		StoreType:      "sqlite",
		DataDir:        dataDir,
		DefaultTimeout: 24 * time.Hour,
		ClusterID:      "test",
	}

	manager, err := New(config, nil)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Verify database file was created in DataDir
	expectedDBPath := filepath.Join(dataDir, "sessions.db")
	if _, err := os.Stat(expectedDBPath); os.IsNotExist(err) {
		t.Errorf("Expected database file at %s, but it doesn't exist", expectedDBPath)
	}

	t.Logf("Database file created in correct location: %s", expectedDBPath)
}
