// Package journal provides the ZenJournal interface for immutable event logging.
// ZenJournal is the event ledger that records all significant actions in zen-brain.
//
// Each entry is cryptographically linked to the previous entry via chain hashes,
// enabling tamper detection and efficient state verification.
//
// Implementation uses zen-sdk/pkg/receiptlog as the foundation.
package journal

import (
	"context"
	"time"
)

// EventType represents the type of journal event.
type EventType string

// Standard event types (see V6 Construction Plan Section 3.13 for SR&ED events)
const (
	// Intent events
	EventIntentCreated   EventType = "intent_created"
	EventIntentAnalyzed  EventType = "intent_analyzed"

	// Planning events
	EventPlanGenerated   EventType = "plan_generated"
	EventPlanApproved    EventType = "plan_approved"
	EventPlanRejected    EventType = "plan_rejected"

	// Execution events
	EventTaskQueued      EventType = "task_queued"
	EventTaskStarted     EventType = "task_started"
	EventActionExecuted  EventType = "action_executed"
	EventTaskCompleted   EventType = "task_completed"
	EventTaskFailed      EventType = "task_failed"

	// Approval events
	EventApprovalRequested EventType = "approval_requested"
	EventApprovalGranted   EventType = "approval_granted"
	EventApprovalDenied    EventType = "approval_denied"

	// Agent events
	EventAgentHeartbeat  EventType = "agent_heartbeat"
	EventSessionStarted  EventType = "session_started"
	EventSessionEnded    EventType = "session_ended"

	// Policy events
	EventPolicyViolation EventType = "policy_violation"
	EventGateEnforced    EventType = "gate_enforced"

	// SR&ED experiment events (V6)
	EventHypothesisFormulated EventType = "hypothesis_formulated"
	EventApproachAttempted    EventType = "approach_attempted"
	EventResultObserved       EventType = "result_observed"
	EventApproachAbandoned    EventType = "approach_abandoned"
	EventExperimentConcluded  EventType = "experiment_concluded"
)

// Entry represents a journal entry to be recorded.
type Entry struct {
	// EventType is the type of event
	EventType EventType `json:"event_type"`

	// Actor is who/what caused the event (e.g., "planner", "worker-123", "human:alice")
	Actor string `json:"actor"`

	// CorrelationID links related events (e.g., a session or task ID)
	CorrelationID string `json:"correlation_id"`

	// TaskID is the specific task this event relates to
	TaskID string `json:"task_id,omitempty"`

	// SessionID is the session this event relates to
	SessionID string `json:"session_id,omitempty"`

	// ClusterID for multi-cluster context
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID for project context
	ProjectID string `json:"project_id,omitempty"`

	// Payload is the event-specific data
	Payload interface{} `json:"payload,omitempty"`

	// SREDTags for SR&ED categorization
	SREDTags []string `json:"sred_tags,omitempty"`

	// Timestamp is when the event occurred (defaults to now)
	Timestamp time.Time `json:"timestamp"`
}

// Receipt is a recorded entry with chain hash and sequence number.
type Receipt struct {
	Entry

	// Sequence is the monotonically increasing sequence number
	Sequence uint64 `json:"sequence"`

	// Hash is the SHA-256 hash of this receipt
	Hash string `json:"hash"`

	// PrevHash is the hash of the previous receipt (chain link)
	PrevHash string `json:"prev_hash"`

	// RecordedAt is when the receipt was recorded
	RecordedAt time.Time `json:"recorded_at"`
}

// QueryOptions for searching journal entries.
type QueryOptions struct {
	// EventType filters by event type
	EventType EventType `json:"event_type,omitempty"`

	// CorrelationID filters by correlation ID
	CorrelationID string `json:"correlation_id,omitempty"`

	// TaskID filters by task ID
	TaskID string `json:"task_id,omitempty"`

	// SessionID filters by session ID
	SessionID string `json:"session_id,omitempty"`

	// ClusterID filters by cluster ID
	ClusterID string `json:"cluster_id,omitempty"`

	// ProjectID filters by project ID
	ProjectID string `json:"project_id,omitempty"`

	// SREDTag filters by SR&ED tag
	SREDTag string `json:"sred_tag,omitempty"`

	// Start filters events after this time
	Start time.Time `json:"start,omitempty"`

	// End filters events before this time
	End time.Time `json:"end,omitempty"`

	// Limit limits the number of results
	Limit int `json:"limit,omitempty"`

	// OrderBy specifies sort order ("asc" or "desc", default "desc")
	OrderBy string `json:"order_by,omitempty"`
}

// Stats holds journal statistics.
type Stats struct {
	TotalReceipts   uint64    `json:"total_receipts"`
	LastSequence    uint64    `json:"last_sequence"`
	LastHash        string    `json:"last_hash"`
	OldestTimestamp time.Time `json:"oldest_timestamp"`
	NewestTimestamp time.Time `json:"newest_timestamp"`
}

// ZenJournal is the interface for the immutable event ledger.
type ZenJournal interface {
	// Record records a new journal entry and returns the receipt.
	Record(ctx context.Context, entry Entry) (*Receipt, error)

	// Get retrieves a receipt by sequence number.
	Get(ctx context.Context, sequence uint64) (*Receipt, error)

	// GetByHash retrieves a receipt by its hash.
	GetByHash(ctx context.Context, hash string) (*Receipt, error)

	// Query searches for receipts matching the options.
	Query(ctx context.Context, opts QueryOptions) ([]Receipt, error)

	// QueryByCorrelation retrieves all events for a correlation ID.
	QueryByCorrelation(ctx context.Context, correlationID string) ([]Receipt, error)

	// QueryByTask retrieves all events for a task.
	QueryByTask(ctx context.Context, taskID string) ([]Receipt, error)

	// QueryBySREDTag retrieves all events with a specific SR&ED tag.
	QueryBySREDTag(ctx context.Context, tag string, start, end time.Time) ([]Receipt, error)

	// Verify verifies the chain integrity.
	Verify(ctx context.Context) (int, error)

	// Stats returns journal statistics.
	Stats() Stats

	// Close closes the journal.
	Close() error
}
