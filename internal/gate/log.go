// Package gate provides a ZenGate implementation that audits and allows all (Block 4.6).
// Use when you want an admission path that logs requests without denying; real policy enforcement can be added later.
package gate

import (
	"context"
	"log"
	"time"

	gatepkg "github.com/kube-zen/zen-brain1/pkg/gate"
	"github.com/kube-zen/zen-brain1/pkg/policy"
)

// LogGate implements ZenGate by logging every Admit/Validate and allowing all requests.
type LogGate struct{}

// NewLogGate returns a ZenGate that logs admission requests and always allows.
func NewLogGate() gatepkg.ZenGate {
	return &LogGate{}
}

// Admit logs the request and allows.
func (l *LogGate) Admit(ctx context.Context, req gatepkg.AdmissionRequest) (*gatepkg.AdmissionResponse, error) {
	log.Printf("[gate] admit request_id=%s session=%s task=%s action=%s resource=%s -> allowed",
		req.RequestID, req.SessionID, req.TaskID, req.Action, req.Resource)
	return &gatepkg.AdmissionResponse{
		RequestID:         req.RequestID,
		Allowed:           true,
		EvaluatedAt:       time.Now(),
		EvaluationDuration: 0,
	}, nil
}

// Validate logs and returns no errors.
func (l *LogGate) Validate(ctx context.Context, req gatepkg.AdmissionRequest) ([]gatepkg.ValidationError, error) {
	log.Printf("[gate] validate request_id=%s session=%s task=%s -> ok", req.RequestID, req.SessionID, req.TaskID)
	return nil, nil
}

// RegisterValidator is a no-op (no validators stored).
func (l *LogGate) RegisterValidator(ctx context.Context, validator gatepkg.Validator) error {
	return nil
}

// RegisterPolicy is a no-op (no policies evaluated).
func (l *LogGate) RegisterPolicy(ctx context.Context, _ policy.ZenPolicy) error {
	return nil
}

// Stats returns minimal stats.
func (l *LogGate) Stats(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"mode": "log", "allowed": true}, nil
}

// Close is a no-op.
func (l *LogGate) Close() error {
	return nil
}
