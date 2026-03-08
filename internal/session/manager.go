// Package session provides work session management.
package session

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// DefaultManager is the default implementation of Manager.
type DefaultManager struct {
	config *Config
	store  Store
	
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
		config:  config,
		store:   store,
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
		ID:          sessionID,
		WorkItemID:  workItem.ID,
		SourceKey:   workItem.Source.IssueKey,
		State:       contracts.SessionStateCreated,
		WorkItem:    workItem,
		CreatedAt:   now,
		UpdatedAt:   now,
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
	return session, nil
}

// GetSession retrieves a session by ID.
func (m *DefaultManager) GetSession(ctx context.Context, sessionID string) (*contracts.Session, error) {
	return m.store.Get(ctx, sessionID)
}

// GetSessionByWorkItem retrieves the active session for a work item.
func (m *DefaultManager) GetSessionByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error) {
	return m.store.GetByWorkItem(ctx, workItemID)
}

// UpdateSession updates session content.
func (m *DefaultManager) UpdateSession(ctx context.Context, session *contracts.Session) error {
	session.UpdatedAt = time.Now()
	return m.store.Update(ctx, session)
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
		FromState:  session.State,
		ToState:    newState,
		Timestamp:  time.Now(),
		Reason:     reason,
		Agent:      agent,
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
		// TODO: Implement SQLite store
		return NewMemoryStore(), nil // Fallback for now
	default:
		return nil, fmt.Errorf("unsupported store type: %s", config.StoreType)
	}
}