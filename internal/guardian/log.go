// Package guardian provides real ZenGuardian implementations (Block 4.7).
package guardian

import (
	"context"
	"log"
	"sync"

	guardianpkg "github.com/kube-zen/zen-brain1/pkg/guardian"
)

// LogGuardian records events to the standard logger and allows all operations (no safety blocking).
// Use for audit trail when no circuit breaking or anomaly detection is needed.
type LogGuardian struct {
	mu sync.Mutex
}

// NewLogGuardian returns a ZenGuardian that logs every event and always allows CheckSafety.
func NewLogGuardian() guardianpkg.ZenGuardian {
	return &LogGuardian{}
}

// RecordEvent logs the event (kind, session, task, message, payload) for audit.
func (g *LogGuardian) RecordEvent(ctx context.Context, ev guardianpkg.Event) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	log.Printf("[guardian] event=%s session=%s task=%s msg=%s payload=%v",
		ev.Kind, ev.SessionID, ev.TaskID, ev.Message, ev.Payload)
	return nil
}

// CheckSafety always allows (no blocking).
func (g *LogGuardian) CheckSafety(ctx context.Context, sessionID, taskID string, kind guardianpkg.EventKind) (guardianpkg.SafetyCheckResult, error) {
	return guardianpkg.SafetyCheckResult{Allowed: true}, nil
}

// Close is a no-op.
func (g *LogGuardian) Close() error {
	return nil
}
