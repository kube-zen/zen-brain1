// Package gate provides a stub ZenGate implementation for development and testing.
// Block 4.6: Stub admits all requests and validates none (no-op).
package gate

import (
	"context"
	"time"

	gatepkg "github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/policy"
)

// StubGate implements gate.ZenGate by allowing all requests and returning no validation errors.
type StubGate struct{}

// NewStubGate returns a ZenGate stub that admits all and validates to nil.
func NewStubGate() gatepkg.ZenGate {
	return &StubGate{}
}

// Admit allows all requests.
func (s *StubGate) Admit(ctx context.Context, req gatepkg.AdmissionRequest) (*gatepkg.AdmissionResponse, error) {
	return &gatepkg.AdmissionResponse{
		RequestID:   req.RequestID,
		Allowed:     true,
		EvaluatedAt: time.Now(),
	}, nil
}

// Validate returns no validation errors.
func (s *StubGate) Validate(ctx context.Context, req gatepkg.AdmissionRequest) ([]gatepkg.ValidationError, error) {
	return nil, nil
}

// RegisterValidator is a no-op.
func (s *StubGate) RegisterValidator(ctx context.Context, validator gatepkg.Validator) error {
	return nil
}

// RegisterPolicy is a no-op.
func (s *StubGate) RegisterPolicy(ctx context.Context, _ policy.ZenPolicy) error {
	return nil
}

// Stats returns empty stats.
func (s *StubGate) Stats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// Close is a no-op.
func (s *StubGate) Close() error {
	return nil
}
