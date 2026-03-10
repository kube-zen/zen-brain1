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
