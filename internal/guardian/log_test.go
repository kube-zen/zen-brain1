package guardian

import (
	"context"
	"testing"
	"time"

	guardianpkg "github.com/kube-zen/zen-brain1/pkg/guardian"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogGuardian_RecordEvent(t *testing.T) {
	g := NewLogGuardian()
	ctx := context.Background()
	ev := guardianpkg.Event{
		Kind:      guardianpkg.EventTaskStarted,
		SessionID: "s1",
		TaskID:    "t1",
		Message:   "started",
		At:        time.Now(),
	}
	err := g.RecordEvent(ctx, ev)
	require.NoError(t, err)
}

func TestLogGuardian_CheckSafety_AlwaysAllows(t *testing.T) {
	g := NewLogGuardian()
	ctx := context.Background()
	res, err := g.CheckSafety(ctx, "s1", "t1", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Empty(t, res.Reason)
}

func TestLogGuardian_Close(t *testing.T) {
	g := NewLogGuardian()
	assert.NoError(t, g.Close())
}
