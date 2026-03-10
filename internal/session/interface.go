// Package session provides work session management for zen-brain.
// A work session tracks the progress of a WorkItem through the pipeline
// from creation to completion, with state persistence and SR&ED evidence collection.
package session

import (
	"context"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
	"github.com/kube-zen/zen-brain1/pkg/contracts"
	"github.com/kube-zen/zen-brain1/pkg/journal"
	"github.com/kube-zen/zen-brain1/pkg/messagebus"
)

// JournalRecorder records journal entries (optional Block 3 write-side).
type JournalRecorder interface {
	Record(ctx context.Context, entry journal.Entry) (*journal.Receipt, error)
}

// EventPublisher publishes events to the message bus (optional Block 3 nervous system).
type EventPublisher interface {
	Publish(ctx context.Context, stream string, event *messagebus.Event) error
}

// Manager manages work sessions.
type Manager interface {
	// CreateSession creates a new session for a work item.
	CreateSession(ctx context.Context, workItem *contracts.WorkItem) (*contracts.Session, error)

	// GetSession retrieves a session by ID.
	GetSession(ctx context.Context, sessionID string) (*contracts.Session, error)

	// GetSessionByWorkItem retrieves the active session for a work item.
	GetSessionByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error)

	// UpdateSession updates session content (analysis result, brain task specs).
	UpdateSession(ctx context.Context, session *contracts.Session) error

	// TransitionState transitions a session to a new state.
	TransitionState(ctx context.Context, sessionID string, newState contracts.SessionState, reason string, agent string) error

	// AddEvidence adds SR&ED evidence to a session.
	AddEvidence(ctx context.Context, sessionID string, evidence contracts.EvidenceItem) error

	// ListSessions returns sessions matching the filter.
	ListSessions(ctx context.Context, filter SessionFilter) ([]*contracts.Session, error)

	// CleanupStaleSessions cleans up sessions that have been stuck or abandoned.
	CleanupStaleSessions(ctx context.Context, maxAge time.Duration) (int, error)

	// UpdateExecutionCheckpoint writes a structured execution checkpoint into ZenContext SessionContext.State.
	UpdateExecutionCheckpoint(ctx context.Context, sessionID string, checkpoint *ExecutionCheckpoint) error

	// GetExecutionCheckpoint reads the structured execution checkpoint from ZenContext SessionContext.State.
	GetExecutionCheckpoint(ctx context.Context, sessionID string) (*ExecutionCheckpoint, error)

	// GetExecutionCheckpointSummary returns a human-readable summary of the execution checkpoint.
	GetExecutionCheckpointSummary(ctx context.Context, sessionID string) (string, error)

	// Close closes the session manager and releases resources.
	Close() error
}

// SessionFilter filters sessions for listing.
type SessionFilter struct {
	State         *contracts.SessionState `json:"state,omitempty"`
	WorkItemID    *string                 `json:"work_item_id,omitempty"`
	SourceKey     *string                 `json:"source_key,omitempty"`
	AssignedAgent *string                 `json:"assigned_agent,omitempty"`
	CreatedAfter  *time.Time              `json:"created_after,omitempty"`
	CreatedBefore *time.Time              `json:"created_before,omitempty"`
	UpdatedAfter  *time.Time              `json:"updated_after,omitempty"`
	UpdatedBefore *time.Time              `json:"updated_before,omitempty"`
	Limit         int                     `json:"limit,omitempty"`
	Offset        int                     `json:"offset,omitempty"`
}

// Store is the persistence interface for sessions.
type Store interface {
	// Create creates a new session.
	Create(ctx context.Context, session *contracts.Session) error

	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (*contracts.Session, error)

	// GetByWorkItem retrieves the active session for a work item.
	GetByWorkItem(ctx context.Context, workItemID string) (*contracts.Session, error)

	// Update updates an existing session.
	Update(ctx context.Context, session *contracts.Session) error

	// List returns sessions matching the filter.
	List(ctx context.Context, filter SessionFilter) ([]*contracts.Session, error)

	// Delete deletes a session (for cleanup).
	Delete(ctx context.Context, sessionID string) error

	// Close closes the store.
	Close() error
}

// Config holds configuration for the session manager.
type Config struct {
	// Persistence
	StoreType string `yaml:"store_type" json:"store_type"` // "memory", "sqlite", "postgres"
	DataDir   string `yaml:"data_dir" json:"data_dir"`

	// Session lifecycle
	DefaultTimeout time.Duration `yaml:"default_timeout" json:"default_timeout"`
	MaxSessionAge  time.Duration `yaml:"max_session_age" json:"max_session_age"`

	// Cleanup
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	StaleThreshold  time.Duration `yaml:"stale_threshold" json:"stale_threshold"`

	// ZenContext integration (optional)
	ZenContext zenctx.ZenContext `yaml:"-" json:"-"`

	// ClusterID is the cluster identifier for session/context lookups (default: "default").
	ClusterID string `yaml:"cluster_id" json:"cluster_id"`

	// Block 3: optional journal and message bus sinks for lifecycle events
	Journal    JournalRecorder `yaml:"-" json:"-"`
	EventBus  EventPublisher  `yaml:"-" json:"-"`
	EventStream string `yaml:"event_stream" json:"event_stream"` // default: zen-brain.events
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		StoreType:       "sqlite",
		DataDir:         "./data/sessions",
		DefaultTimeout:  24 * time.Hour,
		MaxSessionAge:   7 * 24 * time.Hour,
		CleanupInterval: 1 * time.Hour,
		StaleThreshold:  2 * time.Hour,
		EventStream:     "zen-brain.events",
		ClusterID:       "default",
	}
}
