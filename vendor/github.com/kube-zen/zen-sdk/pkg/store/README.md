# pkg/store

Shared storage utilities for Zen components.

## SQLite (store.OpenSQLite)

Zen standardizes on **one** SQLite stack:

- **Driver:** `modernc.org/sqlite` (pure Go, no CGO)
- **Usage:** `github.com/kube-zen/zen-sdk/pkg/store.OpenSQLite` or `OpenSQLiteSimple`

Use this in:

- **zen-brain** – session store, profile store, knowledge store, artifacts, RAG indexer, vector indexer
- **zen-ingester** – DLQ (when using shared/edgestate or direct SQLite)
- **zen-egress** – DLQ, idempotency, nonce storage
- **zen-bridge** – any local SQLite state
- **zen-protect** – any local SQLite (runtime is zen-ingester; SaaS back may have its own)

### Why in zen-sdk

- Single place to pick driver and pragmas (WAL, busy_timeout, synchronous).
- All components can depend on `zen-sdk` and get the same CGO-free build and behavior.
- `shared/edgestate` (zen-platform) can keep its higher-level StateStore/DLQStore/NonceStore and optionally open the underlying DB via `store.OpenSQLite` for consistency, or keep using `sql.Open("sqlite", ...)` with the same pragmas; either way, driver is `modernc.org/sqlite`.

### Migration

1. **zen-brain:** Replace direct `sql.Open("sqlite", path+"?...")` and `_ "modernc.org/sqlite"` with `store.OpenSQLite(ctx, path, nil)` or `store.OpenSQLiteSimple(ctx, path)`. Add `zen-sdk` dependency and remove direct `modernc.org/sqlite` from go.mod if nothing else needs it.
2. **zen-platform shared/edgestate:** Option A: use `store.OpenSQLite` inside `SQLiteStore.Open` and add zen-sdk to shared. Option B: keep current `sql.Open("sqlite", ...)` and same pragmas; just document that driver must remain `modernc.org/sqlite` and align pragmas with `store.DefaultSQLiteOptions()`.
3. **zen-ingester / zen-egress:** Already use shared/edgestate for DLQ/nonce/idempotency; no change required unless edgestate is switched to use store.OpenSQLite.

### Example

```go
import (
    "context"
    "github.com/kube-zen/zen-sdk/pkg/store"
)

db, err := store.OpenSQLiteSimple(ctx, "/var/lib/zen/sessions.db")
if err != nil {
    return err
}
defer db.Close()
```

With options:

```go
opts := &store.SQLiteOptions{
    JournalMode: "WAL",
    BusyTimeout: 5000,
    Synchronous: "NORMAL",
}
db, err := store.OpenSQLite(ctx, path, opts)
```
