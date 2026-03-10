// Package agent provides agent-context binding for Block 5.3.
// Agents write intermediate state to ZenContext and retrieve context for continuation.
package agent

import (
	"context"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

// AgentContextBinder provides get-for-continuation and write-intermediate state (Block 5.3).
type AgentContextBinder interface {
	// GetForContinuation retrieves session context for an agent to continue work (e.g. before Run).
	GetForContinuation(ctx context.Context, clusterID, sessionID, taskID string) (*zenctx.SessionContext, error)
	// WriteIntermediate stores updated session context (State/Scratchpad) after a step.
	WriteIntermediate(ctx context.Context, clusterID string, session *zenctx.SessionContext) error
}

// ZenContextBinder implements AgentContextBinder using ZenContext (Tier 1).
type ZenContextBinder struct {
	ZenContext zenctx.ZenContext
	ClusterID  string // default cluster when empty
}

// NewZenContextBinder returns a binder that uses the given ZenContext.
func NewZenContextBinder(zc zenctx.ZenContext, clusterID string) *ZenContextBinder {
	if clusterID == "" {
		clusterID = "default"
	}
	return &ZenContextBinder{ZenContext: zc, ClusterID: clusterID}
}

// GetForContinuation implements AgentContextBinder.
func (b *ZenContextBinder) GetForContinuation(ctx context.Context, clusterID, sessionID, taskID string) (*zenctx.SessionContext, error) {
	if clusterID == "" {
		clusterID = b.ClusterID
	}
	return b.ZenContext.GetSessionContext(ctx, clusterID, sessionID)
}

// WriteIntermediate implements AgentContextBinder.
func (b *ZenContextBinder) WriteIntermediate(ctx context.Context, clusterID string, session *zenctx.SessionContext) error {
	if clusterID == "" {
		clusterID = b.ClusterID
	}
	return b.ZenContext.StoreSessionContext(ctx, clusterID, session)
}

// ReMeBinder implements AgentContextBinder using the ReMe protocol (Block 5.2).
// GetForContinuation calls ReconstructSession so the agent gets context from Tier 1 → Tier 3 → Journal + KB.
// Use this when ZenContext is configured and you want full ReMe semantics on startup/continuation.
type ReMeBinder struct {
	ZenContext zenctx.ZenContext
	ClusterID  string
}

// NewReMeBinder returns a binder that uses ReConstructSession for continuation.
func NewReMeBinder(zc zenctx.ZenContext, clusterID string) *ReMeBinder {
	if clusterID == "" {
		clusterID = "default"
	}
	return &ReMeBinder{ZenContext: zc, ClusterID: clusterID}
}

// GetForContinuation implements AgentContextBinder by running the ReMe protocol.
func (b *ReMeBinder) GetForContinuation(ctx context.Context, clusterID, sessionID, taskID string) (*zenctx.SessionContext, error) {
	if clusterID == "" {
		clusterID = b.ClusterID
	}
	req := zenctx.ReMeRequest{
		SessionID: sessionID,
		TaskID:    taskID,
		ClusterID: clusterID,
	}
	resp, err := b.ZenContext.ReconstructSession(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.SessionContext, nil
}

// WriteIntermediate implements AgentContextBinder.
func (b *ReMeBinder) WriteIntermediate(ctx context.Context, clusterID string, session *zenctx.SessionContext) error {
	if clusterID == "" {
		clusterID = b.ClusterID
	}
	return b.ZenContext.StoreSessionContext(ctx, clusterID, session)
}
