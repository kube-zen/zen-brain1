// Package guardian provides the ZenGuardian interface for proactive monitoring (Block 4.7).
// ZenGuardian watches running agents and can intervene; implements circuit breaking,
// anomaly detection, PII filtering, and safety boundaries.
package guardian

import (
	"context"
	"time"
)

// EventKind is the kind of event the Guardian can record or react to.
type EventKind string

const (
	EventTaskStarted   EventKind = "task_started"
	EventTaskCompleted EventKind = "task_completed"
	EventTaskFailed    EventKind = "task_failed"
	EventSessionStart  EventKind = "session_start"
	EventSessionEnd    EventKind = "session_end"
	EventAnomaly       EventKind = "anomaly"
)

// Event is an event the Guardian may record or use for safety checks.
type Event struct {
	Kind      EventKind
	SessionID string
	TaskID    string
	Message   string
	Payload   map[string]interface{}
	At        time.Time
}

// SafetyCheckResult is the result of a safety check (e.g. before or during execution).
type SafetyCheckResult struct {
	Allowed bool   // Whether to allow the operation
	Reason  string // Reason or intervention message
}

// ZenGuardian is the interface for proactive monitoring and safety (Block 4.7).
type ZenGuardian interface {
	// RecordEvent records an event for audit or anomaly detection.
	RecordEvent(ctx context.Context, ev Event) error
	// CheckSafety runs a safety check (e.g. before running a task); returns whether to allow and reason.
	CheckSafety(ctx context.Context, sessionID, taskID string, kind EventKind) (SafetyCheckResult, error)
	// Close releases resources.
	Close() error
}
