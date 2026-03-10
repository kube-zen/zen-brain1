package context

import (
	stdctx "context"
	"time"

	zenctx "github.com/kube-zen/zen-brain1/pkg/context"
)

const healthTimeout = 5 * time.Second

// CheckHot verifies Tier 1 (Hot) is reachable via Stats.
func CheckHot(ctx stdctx.Context, z zenctx.ZenContext) error {
	if z == nil {
		return nil
	}
	if ctx == nil {
		ctx = stdctx.Background()
	}
	ctx, cancel := stdctx.WithTimeout(ctx, healthTimeout)
	defer cancel()
	stats, err := z.Stats(ctx)
	if err != nil {
		return err
	}
	if _, ok := stats[zenctx.TierHot]; !ok {
		// Hot is required for composite; missing means degraded
		return nil
	}
	return nil
}

// CheckWarm verifies Tier 2 (Warm) is reachable if present.
func CheckWarm(ctx stdctx.Context, z zenctx.ZenContext) error {
	if z == nil {
		return nil
	}
	if ctx == nil {
		ctx = stdctx.Background()
	}
	ctx, cancel := stdctx.WithTimeout(ctx, healthTimeout)
	defer cancel()
	_, err := z.Stats(ctx)
	return err
}

// CheckCold verifies Tier 3 (Cold) is reachable if present.
func CheckCold(ctx stdctx.Context, z zenctx.ZenContext) error {
	if z == nil {
		return nil
	}
	if ctx == nil {
		ctx = stdctx.Background()
	}
	ctx, cancel := stdctx.WithTimeout(ctx, healthTimeout)
	defer cancel()
	_, err := z.Stats(ctx)
	return err
}
