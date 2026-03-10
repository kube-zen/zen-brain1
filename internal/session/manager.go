// Package session provides work session management.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// DefaultManager is the default implementation of Manager.
type DefaultManager struct {
	config *Config
	store  Store
	zenctx zenctx.ZenContext // Optional ZenContext integration

	// For session creation
	sessionCounter uint64
	mutex          sync.RWMutex

	// Cleanup
	cleanupTicker *time.Ticker
	cleanupDone   chan bool
}

// New creates a new DefaultManager.
func New(config *Config, store Store) (*DefaultManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if store == nil {
		// Create default store based on config
		var err error
		store, err = createStore(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create store: %w", err)
		}
	}

	manager := &DefaultManager{
		config:      config,
		store:       store,
		zenctx:      config.ZenContext,
		cleanupDone: make(chan bool),
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		manager.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go manager.cleanupRoutine()
	}

	return manager, nil
}

// CreateSession creates a new session for a work item.
func (m *DefaultManager) CreateSession(ctx context.Context, workItem *contracts.WorkItem) (*contracts.Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if there's already an active session for this work item
	existing, err := m.store.GetByWorkItem(ctx, workItem.ID)
	if err == nil && existing != nil {
		// Check if session is still active (not completed/failed/canceled)
		if existing.State != contracts.SessionStateCompleted &&
			existing.State != contracts.SessionStateFailed &&
			existing.State != contracts.SessionStateCanceled {
			return nil, fmt.Errorf("active session %s already exists for work item %s", existing.ID, workItem.ID)
		}
	}

	// Generate session ID (in production, use UUID)
	sessionID := fmt.Sprintf("session-%d-%d", time.Now().Unix(), m.sessionCounter)
	m.sessionCounter++

	now := time.Now()
	session := &contracts.Session{
		ID:         sessionID,
		WorkItemID: workItem.ID,
		SourceKey:  workItem.Source.IssueKey,
		State:      contracts.SessionStateCreated,
		WorkItem:   workItem,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Record initial state transition
	session.StateHistory = []contracts.StateTransition{{
		FromState: "", // No previous state
		ToState:   contracts.SessionStateCreated,
		Timestamp: now,
		Reason:    "Session created",
		Agent:     "session-manager",
	}}

	if err := m.store.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	log.Printf("Created session %s for work item %s", session.ID, workItem.ID)

	// Block 3: emit session.created to journal and message bus when configured
	EmitSessionCreated(ctx, m.config, session, workItem.ID)

	// Create corresponding ZenContext SessionContext if ZenContext is configured
	if m.zenctx != nil {
		zenSession := &zenctx.SessionContext{
			SessionID:         session.ID,
			TaskID:            workItem.ID,
			ClusterID:         "default", // TODO: make configurable
			ProjectID:         workItem.Source.Project,
			CreatedAt:         now,
			LastAccessedAt:    now,
			State:             nil, // Agent state will be populated later
			RelevantKnowledge: nil,
			Scratchpad:        nil,
		}
		err := m.zenctx.StoreSessionContext(ctx, zenSession.ClusterID, zenSession)
		if err != nil {
			// Log error but don't fail session creation
			log.Printf("Warning: failed to create ZenContext for session %s: %v", session.ID, err)
		} else {
			log.Printf("Created ZenContext SessionContext for session %s", session.ID)
		}
	}

	return session, nil
}

// UpdateExecutionCheckpoint writes a structured execution checkpoint into ZenContext SessionContext.State.
func (m *DefaultManager) UpdateExecutionCheckpoint(ctx context.Context, sessionID string, checkpoint *ExecutionCheckpoint) error {
	if m.zenctx == nil {
		return fmt.Errorf("ZenContext not configured")
	}
	if checkpoint == nil {
		return fmt.Errorf("checkpoint cannot be nil")
	}
	clusterID := "default"
	sc, err := m.zenctx.GetSessionContext(ctx, clusterID, sessionID)
	if err != nil || sc == nil {
		log.Printf("Warning: SessionContext not found for session %s (cluster %s)", sessionID, clusterID)
		return fmt.Errorf("session context not found: %s", sessionID)
	}
	checkpoint.UpdatedAt = time.Now()
	stateBytes, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	sc.State = stateBytes
	sc.LastAccessedAt = time.Now()
	if err := m.zenctx.StoreSessionContext(ctx, sc.ClusterID, sc); err != nil {
		return fmt.Errorf("store session context: %w", err)
	}
	// Block 3: emit session.checkpoint_updated to journal and message bus when configured
	EmitSessionCheckpointUpdated(ctx, m.config, sessionID, checkpoint.WorkItemID, checkpoint.Stage)
	return nil
}

// GetExecutionCheckpoint reads the structured execution checkpoint from ZenContext SessionContext.State.
func (m *DefaultManager) GetExecutionCheckpoint(ctx context.Context, sessionID string) (*ExecutionCheckpoint, error) {
	if m.zenctx == nil {
		return nil, fmt.Errorf("ZenContext not configured")
	}
	clusterID := "default"
	sc, err := m.zenctx.GetSessionContext(ctx, clusterID, sessionID)
	if err != nil || sc == nil {
		return nil, fmt.Errorf("session context not found: %w", err)
	}
	if len(sc.State) == 0 {
		return nil, nil
	}
	var cp ExecutionCheckpoint
	if err := json.Unmarshal(sc.State, &cp); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}
	return &cp, nil
}

// GetExecutionCheckpointSummary returns a human-readable summary of the execution checkpoint.
func (m *DefaultManager) GetExecutionCheckpointSummary(ctx context.Context, sessionID string) (string, error) {
	checkpoint, err := m.GetExecutionCheckpoint(ctx, sessionID)
	if err != nil {
		return "", err
	}

	if checkpoint == nil {
		return fmt.Sprintf("Session %s: No execution checkpoint found", sessionID), nil
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Session %s Execution Checkpoint Summary\n", sessionID))
	summary.WriteString(fmt.Sprintf("  Stage: %s\n", checkpoint.Stage))
	summary.WriteString(fmt.Sprintf("  Work Item ID: %s\n", checkpoint.WorkItemID))
	summary.WriteString(fmt.Sprintf("  Tasks: %d\n", len(checkpoint.BrainTaskIDs)))
	summary.WriteString(fmt.Sprintf("  Proof Artifacts: %d\n", len(checkpoint.ProofPaths)))
	summary.WriteString(fmt.Sprintf("  Knowledge Chunks: %d\n", len(checkpoint.KnowledgeChunkIDs)))

	if checkpoint.LastRecommendation != "" {
		summary.WriteString(fmt.Sprintf("  Last Recommendation: %s\n", checkpoint.LastRecommendation))
	}

	if !checkpoint.UpdatedAt.IsZero() {
		summary.WriteString(fmt.Sprintf("  Updated: %s\n", checkpoint.UpdatedAt.Format(time.RFC3339)))
	}

	// Check if this looks like a terminal state
	if checkpoint.Stage == "proof_attached" && len(checkpoint.ProofPaths) > 0 {
		summary.WriteString("  Status: Terminal state with proof artifacts attached\n")
	}

	return summary.String(), nil
}

// updateZenContextLastAccessed updates the LastAccessedAt timestamp in ZenContext.
func (m *DefaultManager) updateZenContextLastAccessed(ctx context.Context, sessionID string) {
	if m.zenctx == nil {
		return
	}

	clusterID := "default"
	sessionCtx, err := m.zenctx.GetSessionContext(ctx, clusterID, sessionID)
	if err != nil || sessionCtx == nil {
		// SessionContext may not exist (e.g., created before ZenContext integration)
		log.Printf("Warning: ZenContext SessionContext not found for session %s (cluster: %s)", sessionID, clusterID)
		return
	}

	sessionCtx.LastAccessedAt = time.Now()
	if err := m.zenctx.StoreSessionContext(ctx, clusterID, sessionCtx); err != nil {
		log.Printf("Warning: failed to update ZenContext LastAccessedAt for session %s: %v", sessionID, err)
	}
}

// GetSession retrieves a session by ID.
func (m *DefaultManager) GetSession(ctx context.Context, sessionID string) (*contracts.Session, error) {
	session, err := m.store.Get(ctx, sessionID)
	if err == nil && session != nil {
		m.updateZenContextLastAccessed(ctx, sessionID)
	}
	return session, err
}

// GetSessionByWorkItem retrieves the active session for a work item.
func (m *DefaultManager) GetSessionByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error) {
	session, err := m.store.GetByWorkItem(ctx, workItemID)
	if err == nil && session != nil {
		m.updateZenContextLastAccessed(ctx, session.ID)
	}
	return session, err
}

// UpdateSession updates session content.
func (m *DefaultManager) UpdateSession(ctx context.Context, session *contracts.Session) error {
	session.UpdatedAt = time.Now()
	err := m.store.Update(ctx, session)
	if err == nil {
		m.updateZenContextLastAccessed(ctx, session.ID)
	}
	return err
}

// TransitionState transitions a session to a new state.
func (m *DefaultManager) TransitionState(ctx context.Context, sessionID string, newState contracts.SessionState, reason string, agent string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Validate state transition
	if !isValidTransition(session.State, newState) {
		return fmt.Errorf("invalid state transition: %s -> %s", session.State, newState)
	}

	// Record transition
	transition := contracts.StateTransition{
		FromState: session.State,
		ToState:   newState,
		Timestamp: time.Now(),
		Reason:    reason,
		Agent:     agent,
	}

	oldState := session.State
	session.State = newState
	session.StateHistory = append(session.StateHistory, transition)
	session.UpdatedAt = time.Now()

	// Update timestamps based on state
	now := time.Now()
	switch newState {
	case contracts.SessionStateInProgress:
		if session.StartedAt == nil {
			session.StartedAt = &now
		}
	case contracts.SessionStateCompleted, contracts.SessionStateFailed, contracts.SessionStateCanceled:
		if session.CompletedAt == nil {
			session.CompletedAt = &now
		}
	}

	if err := m.store.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Update ZenContext LastAccessedAt
	m.updateZenContextLastAccessed(ctx, sessionID)

	// Block 3: emit session.transitioned to journal and message bus when configured
	EmitSessionTransitioned(ctx, m.config, sessionID, session.WorkItemID, string(oldState), string(newState), reason, agent)

	log.Printf("Session %s transitioned: %s -> %s (reason: %s, agent: %s)",
		sessionID, oldState, newState, reason, agent)
	return nil
}

// AddEvidence adds SR&ED evidence to a session.
func (m *DefaultManager) AddEvidence(ctx context.Context, sessionID string, evidence contracts.EvidenceItem) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, err := m.store.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Set evidence metadata
	evidence.ID = fmt.Sprintf("evidence-%d", len(session.EvidenceItems)+1)
	evidence.SessionID = sessionID
	if evidence.CollectedAt.IsZero() {
		evidence.CollectedAt = time.Now()
	}

	session.EvidenceItems = append(session.EvidenceItems, evidence)
	session.UpdatedAt = time.Now()

	if err := m.store.Update(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Update ZenContext LastAccessedAt
	m.updateZenContextLastAccessed(ctx, sessionID)

	// Block 3: emit session.evidence_added to journal and message bus when configured
	EmitSessionEvidenceAdded(ctx, m.config, sessionID, session.WorkItemID, evidence)

	log.Printf("Added evidence %s to session %s (type: %s)",
		evidence.ID, sessionID, evidence.Type)
	return nil
}

// ListSessions returns sessions matching the filter.
func (m *DefaultManager) ListSessions(ctx context.Context, filter SessionFilter) ([]*contracts.Session, error) {
	return m.store.List(ctx, filter)
}

// CleanupStaleSessions cleans up sessions that have been stuck or abandoned.
func (m *DefaultManager) CleanupStaleSessions(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		maxAge = m.config.StaleThreshold
	}

	cutoff := time.Now().Add(-maxAge)
	filter := SessionFilter{
		UpdatedBefore: &cutoff,
		State: &[]contracts.SessionState{
			contracts.SessionStateCreated,
			contracts.SessionStateAnalyzed,
			contracts.SessionStateScheduled,
			contracts.SessionStateInProgress,
		}[0],
		Limit: 100, // Batch cleanup
	}

	sessions, err := m.store.List(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to list stale sessions: %w", err)
	}

	cleaned := 0
	for _, session := range sessions {
		// Transition to failed with cleanup reason
		if err := m.TransitionState(ctx, session.ID, contracts.SessionStateFailed,
			fmt.Sprintf("Session stale (no update for %v)", maxAge), "cleanup"); err != nil {
			log.Printf("Failed to clean up session %s: %v", session.ID, err)
			continue
		}
		cleaned++
	}

	if cleaned > 0 {
		log.Printf("Cleaned up %d stale sessions (older than %v)", cleaned, maxAge)
	}

	return cleaned, nil
}

// Close closes the session manager.
func (m *DefaultManager) Close() error {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
		m.cleanupDone <- true
	}

	if m.store != nil {
		return m.store.Close()
	}

	return nil
}

// cleanupRoutine runs periodic cleanup.
func (m *DefaultManager) cleanupRoutine() {
	for {
		select {
		case <-m.cleanupTicker.C:
			ctx := context.Background()
			if _, err := m.CleanupStaleSessions(ctx, m.config.StaleThreshold); err != nil {
				log.Printf("Cleanup failed: %v", err)
			}
		case <-m.cleanupDone:
			return
		}
	}
}

// isValidTransition validates state transitions.
func isValidTransition(from, to contracts.SessionState) bool {
	// Define valid transitions
	validTransitions := map[contracts.SessionState][]contracts.SessionState{
		contracts.SessionStateCreated: {
			contracts.SessionStateAnalyzed,
			contracts.SessionStateBlocked,
			contracts.SessionStateCanceled,
		},
		contracts.SessionStateAnalyzed: {
			contracts.SessionStateScheduled,
			contracts.SessionStateBlocked,
			contracts.SessionStateCanceled,
		},
		contracts.SessionStateScheduled: {
			contracts.SessionStateInProgress,
			contracts.SessionStateBlocked,
			contracts.SessionStateCanceled,
		},
		contracts.SessionStateInProgress: {
			contracts.SessionStateCompleted,
			contracts.SessionStateFailed,
			contracts.SessionStateBlocked,
			contracts.SessionStateCanceled,
		},
		contracts.SessionStateBlocked: {
			contracts.SessionStateInProgress,
			contracts.SessionStateCanceled,
		},
		// Terminal states can't transition
		contracts.SessionStateCompleted: {},
		contracts.SessionStateFailed:    {},
		contracts.SessionStateCanceled:  {},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false // Unknown from state
	}

	for _, state := range allowed {
		if state == to {
			return true
		}
	}

	return false
}

// createStore creates a store based on configuration.
func createStore(config *Config) (Store, error) {
	switch config.StoreType {
	case "memory":
		return NewMemoryStore(), nil
	case "sqlite":
		// Ensure data directory exists
		if err := os.MkdirAll(config.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
		dbPath := filepath.Join(config.DataDir, "sessions.db")
		return NewSQLiteStore(dbPath)
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.StoreType)
	}
}
