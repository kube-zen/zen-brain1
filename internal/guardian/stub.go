// Package guardian provides a stub ZenGuardian implementation (Block 4.7).
package guardian

import (
	"context"

	guardianpkg "github.com/kube-zen/zen-brain1/pkg/guardian"
)

// StubGuardian implements guardian.ZenGuardian with no-op monitoring and allow-all safety.
type StubGuardian struct{}

// NewStubGuardian returns a ZenGuardian that allows all and records nothing.
func NewStubGuardian() guardianpkg.ZenGuardian {
	return &StubGuardian{}
}

// RecordEvent is a no-op.
func (StubGuardian) RecordEvent(ctx context.Context, ev guardianpkg.Event) error {
	return nil
}

// CheckSafety always allows.
func (StubGuardian) CheckSafety(ctx context.Context, sessionID, taskID string, kind guardianpkg.EventKind) (guardianpkg.SafetyCheckResult, error) {
	return guardianpkg.SafetyCheckResult{Allowed: true}, nil
}

// Close is a no-op.
func (StubGuardian) Close() error {
	return nil
}
