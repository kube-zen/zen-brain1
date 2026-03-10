package session

import (
	"context"
	"sync"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	sessions      map[string]*contracts.Session
	workItemIndex map[string]string // workItemID -> sessionID

	mutex sync.RWMutex
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions:      make(map[string]*contracts.Session),
		workItemIndex: make(map[string]string),
	}
}

// Create creates a new session.
func (s *MemoryStore) Create(ctx context.Context, session *contracts.Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sessions[session.ID]; exists {
		return ErrSessionExists
	}

	// Check for existing active session for this work item
	if existingID, exists := s.workItemIndex[session.WorkItemID]; exists {
		if existingSession, ok := s.sessions[existingID]; ok {
			// Check if existing session is still active
			if existingSession.State != contracts.SessionStateCompleted &&
				existingSession.State != contracts.SessionStateFailed &&
				existingSession.State != contracts.SessionStateCanceled {
				return ErrActiveSessionExists
			}
		}
	}

	// Store session
	s.sessions[session.ID] = session
	s.workItemIndex[session.WorkItemID] = session.ID

	return nil
}

// Get retrieves a session by ID.
func (s *MemoryStore) Get(ctx context.Context, sessionID string) (*contracts.Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	// Return a copy to prevent mutation
	return copySession(session), nil
}

// GetByWorkItem retrieves the active session for a work item.
func (s *MemoryStore) GetByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessionID, exists := s.workItemIndex[workItemID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	session, exists := s.sessions[sessionID]
	if !exists {
		delete(s.workItemIndex, workItemID)
		return nil, ErrSessionNotFound
	}

	return copySession(session), nil
}

// Update updates an existing session.
func (s *MemoryStore) Update(ctx context.Context, session *contracts.Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sessions[session.ID]; !exists {
		return ErrSessionNotFound
	}

	// Update session
	s.sessions[session.ID] = session
	s.workItemIndex[session.WorkItemID] = session.ID

	return nil
}

// List returns sessions matching the filter.
func (s *MemoryStore) List(ctx context.Context, filter SessionFilter) ([]*contracts.Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var result []*contracts.Session

	for _, session := range s.sessions {
		if !matchesFilter(session, filter) {
			continue
		}

		result = append(result, copySession(session))

		// Apply limit if specified
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result, nil
}

// Delete deletes a session.
func (s *MemoryStore) Delete(ctx context.Context, sessionID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	// Remove from index if this is the indexed session
	if indexedID, exists := s.workItemIndex[session.WorkItemID]; exists && indexedID == sessionID {
		delete(s.workItemIndex, session.WorkItemID)
	}

	delete(s.sessions, sessionID)
	return nil
}

// Close closes the store.
func (s *MemoryStore) Close() error {
	// Nothing to clean up for memory store
	return nil
}

// matchesFilter checks if a session matches the filter criteria.
func matchesFilter(session *contracts.Session, filter SessionFilter) bool {
	if filter.State != nil && session.State != *filter.State {
		return false
	}

	if filter.WorkItemID != nil && session.WorkItemID != *filter.WorkItemID {
		return false
	}

	if filter.SourceKey != nil && session.SourceKey != *filter.SourceKey {
		return false
	}

	if filter.AssignedAgent != nil && session.AssignedAgent != *filter.AssignedAgent {
		return false
	}

	if filter.CreatedAfter != nil && session.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}

	if filter.CreatedBefore != nil && session.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	if filter.UpdatedAfter != nil && session.UpdatedAt.Before(*filter.UpdatedAfter) {
		return false
	}

	if filter.UpdatedBefore != nil && session.UpdatedAt.After(*filter.UpdatedBefore) {
		return false
	}

	return true
}

// copySession creates a deep copy of a session.
func copySession(s *contracts.Session) *contracts.Session {
	if s == nil {
		return nil
	}

	copied := *s

	// Deep copy slices
	if s.StateHistory != nil {
		copied.StateHistory = make([]contracts.StateTransition, len(s.StateHistory))
		copy(copied.StateHistory, s.StateHistory)
	}

	if s.BrainTaskSpecs != nil {
		copied.BrainTaskSpecs = make([]contracts.BrainTaskSpec, len(s.BrainTaskSpecs))
		copy(copied.BrainTaskSpecs, s.BrainTaskSpecs)
	}

	if s.EvidenceItems != nil {
		copied.EvidenceItems = make([]contracts.EvidenceItem, len(s.EvidenceItems))
		copy(copied.EvidenceItems, s.EvidenceItems)
	}

	// Copy pointers
	if s.WorkItem != nil {
		workItemCopy := *s.WorkItem
		copied.WorkItem = &workItemCopy
	}

	if s.AnalysisResult != nil {
		analysisCopy := *s.AnalysisResult
		copied.AnalysisResult = &analysisCopy
	}

	if s.StartedAt != nil {
		startedCopy := *s.StartedAt
		copied.StartedAt = &startedCopy
	}

	if s.CompletedAt != nil {
		completedCopy := *s.CompletedAt
		copied.CompletedAt = &completedCopy
	}

	return &copied
}

// Error definitions.
var (
	ErrSessionNotFound     = newError("session not found")
	ErrSessionExists       = newError("session already exists")
	ErrActiveSessionExists = newError("active session already exists for work item")
)

// newError creates a simple error.
func newError(msg string) error {
	return &sessionError{msg: msg}
}

type sessionError struct {
	msg string
}

func (e *sessionError) Error() string {
	return e.msg
}
