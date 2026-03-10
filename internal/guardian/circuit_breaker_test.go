package guardian

import (
	"context"
	"testing"
	"time"

	guardianpkg "github.com/kube-zen/zen-brain1/pkg/guardian"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreakerGuardian_NoLimit_AllowsAll(t *testing.T) {
	cfg := CircuitBreakerConfig{MaxTasksPerSessionPerMinute: 0}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	res, err := g.CheckSafety(ctx, "s1", "t1", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestCircuitBreakerGuardian_UnderLimit_Allows(t *testing.T) {
	cfg := CircuitBreakerConfig{
		MaxTasksPerSessionPerMinute: 3,
		Window:                      time.Minute,
	}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t1", At: time.Now()})
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t2", At: time.Now()})
	res, err := g.CheckSafety(ctx, "s1", "t3", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestCircuitBreakerGuardian_AtLimit_Disallows(t *testing.T) {
	cfg := CircuitBreakerConfig{
		MaxTasksPerSessionPerMinute: 3,
		Window:                      time.Minute,
	}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		err := g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t1", At: time.Now()})
		require.NoError(t, err)
	}
	res, err := g.CheckSafety(ctx, "s1", "t2", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Contains(t, res.Reason, "circuit breaker")
}

func TestCircuitBreakerGuardian_OverLimit_Disallows(t *testing.T) {
	cfg := CircuitBreakerConfig{
		MaxTasksPerSessionPerMinute: 2,
		Window:                      time.Minute,
	}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t1", At: time.Now()})
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t2", At: time.Now()})
	res, err := g.CheckSafety(ctx, "s1", "t3", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Contains(t, res.Reason, "max tasks")
}

func TestCircuitBreakerGuardian_OtherSession_Allowed(t *testing.T) {
	cfg := CircuitBreakerConfig{
		MaxTasksPerSessionPerMinute: 1,
		Window:                      time.Minute,
	}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t1", At: time.Now()})
	res, err := g.CheckSafety(ctx, "s2", "t1", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestCircuitBreakerGuardian_NonTaskStarted_IgnoredForLimit(t *testing.T) {
	cfg := CircuitBreakerConfig{
		MaxTasksPerSessionPerMinute: 1,
		Window:                      time.Minute,
	}
	g := NewCircuitBreakerGuardian(nil, cfg)
	ctx := context.Background()
	g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskCompleted, SessionID: "s1", TaskID: "t1", At: time.Now()})
	res, err := g.CheckSafety(ctx, "s1", "t2", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
}

func TestCircuitBreakerGuardian_DelegatesToInner(t *testing.T) {
	inner := NewLogGuardian()
	cfg := CircuitBreakerConfig{MaxTasksPerSessionPerMinute: 0}
	g := NewCircuitBreakerGuardian(inner, cfg)
	ctx := context.Background()
	err := g.RecordEvent(ctx, guardianpkg.Event{Kind: guardianpkg.EventTaskStarted, SessionID: "s1", TaskID: "t1", At: time.Now()})
	require.NoError(t, err)
	res, err := g.CheckSafety(ctx, "s1", "t1", guardianpkg.EventTaskStarted)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.NoError(t, g.Close())
}
