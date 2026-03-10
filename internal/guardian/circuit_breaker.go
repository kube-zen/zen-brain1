package guardian

import (
	"context"
	"sync"
	"time"

	guardianpkg "github.com/kube-zen/zen-brain1/pkg/guardian"
)

// CircuitBreakerConfig configures the circuit breaker guardian.
type CircuitBreakerConfig struct {
	// MaxTasksPerSessionPerMinute limits how many EventTaskStarted events are allowed per session per rolling minute. 0 = no limit.
	MaxTasksPerSessionPerMinute int
	// Window is the rolling window for counting (default 1 minute).
	Window time.Duration
}

// CircuitBreakerGuardian wraps another ZenGuardian and enforces a per-session task rate limit.
// When the inner guardian is set, it delegates RecordEvent and CheckSafety; otherwise only rate limiting applies.
type CircuitBreakerGuardian struct {
	inner guardianpkg.ZenGuardian
	cfg   CircuitBreakerConfig
	mu    sync.Mutex
	// sessionID -> ring of event times (we only need count in window; keep last N timestamps for pruning)
	perSession []eventAt
}

type eventAt struct {
	sessionID string
	at        time.Time
}

// NewCircuitBreakerGuardian returns a guardian that wraps inner and enforces MaxTasksPerSessionPerMinute.
// If inner is nil, only rate limiting is applied (no event logging). Window defaults to 1 minute.
func NewCircuitBreakerGuardian(inner guardianpkg.ZenGuardian, cfg CircuitBreakerConfig) guardianpkg.ZenGuardian {
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	return &CircuitBreakerGuardian{inner: inner, cfg: cfg, perSession: make([]eventAt, 0, 256)}
}

// RecordEvent forwards to inner if set, then records the event time for rate limiting (EventTaskStarted only).
func (c *CircuitBreakerGuardian) RecordEvent(ctx context.Context, ev guardianpkg.Event) error {
	if c.inner != nil {
		if err := c.inner.RecordEvent(ctx, ev); err != nil {
			return err
		}
	}
	if ev.Kind == guardianpkg.EventTaskStarted && c.cfg.MaxTasksPerSessionPerMinute > 0 {
		c.mu.Lock()
		at := ev.At
		if at.IsZero() {
			at = time.Now()
		}
		c.perSession = append(c.perSession, eventAt{ev.SessionID, at})
		// Prune old entries (older than Window)
		cut := time.Now().Add(-c.cfg.Window)
		i := 0
		for _, e := range c.perSession {
			if e.at.After(cut) {
				c.perSession[i] = e
				i++
			}
		}
		c.perSession = c.perSession[:i]
		c.mu.Unlock()
	}
	return nil
}

// CheckSafety first checks the rate limit: if this session has >= MaxTasksPerSessionPerMinute in the window, disallow.
// Then, if inner is set, delegates to inner.CheckSafety.
func (c *CircuitBreakerGuardian) CheckSafety(ctx context.Context, sessionID, taskID string, kind guardianpkg.EventKind) (guardianpkg.SafetyCheckResult, error) {
	if c.cfg.MaxTasksPerSessionPerMinute > 0 && kind == guardianpkg.EventTaskStarted {
		c.mu.Lock()
		cut := time.Now().Add(-c.cfg.Window)
		n := 0
		for _, e := range c.perSession {
			if e.sessionID == sessionID && e.at.After(cut) {
				n++
			}
		}
		c.mu.Unlock()
		if n >= c.cfg.MaxTasksPerSessionPerMinute {
			return guardianpkg.SafetyCheckResult{
				Allowed: false,
				Reason:  "circuit breaker: max tasks per session per minute exceeded",
			}, nil
		}
	}
	if c.inner != nil {
		return c.inner.CheckSafety(ctx, sessionID, taskID, kind)
	}
	return guardianpkg.SafetyCheckResult{Allowed: true}, nil
}

// Close closes the inner guardian if set.
func (c *CircuitBreakerGuardian) Close() error {
	if c.inner != nil {
		return c.inner.Close()
	}
	return nil
}
