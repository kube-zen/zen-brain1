package ledger

import (
	"context"
	"time"

	pkgledger "github.com/kube-zen/zen-brain1/pkg/ledger"
)

// Ping verifies the ledger is reachable. For CockroachLedger, pings the DB.
// If client is nil or not *CockroachLedger (e.g. stub), returns nil (caller should report mode stub).
func Ping(ctx context.Context, client pkgledger.ZenLedgerClient) error {
	if client == nil {
		return nil
	}
	c, ok := client.(*CockroachLedger)
	if !ok {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.Ping(ctx)
}
