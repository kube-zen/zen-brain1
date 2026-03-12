# DLQ (Dead Letter Queue) Integration

**Status:** ✅ Implemented (see `internal/dlqmgr/`)

*Note: This document was originally a plan. The implementation has been completed.*

---

## Implementation

DLQ (Dead Letter Queue) is implemented via `internal/dlqmgr/`, a wrapper around `zen-sdk/pkg/dlq`:

- **Package:** `internal/dlqmgr/manager.go`
- **Usage:** `cmd/apiserver/main.go`, `cmd/controller/main.go`
- **Environment Variables:** `DLQ_CAPACITY`, `DLQ_REPLAY_INTERVAL`, `DLQ_MAX_RETRIES`
- **Features:** Failed event capture, replay worker, statistics

---

