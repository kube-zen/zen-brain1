# zen-sdk Integration Status

## Current State

**All zen-sdk packages are integrated (100% reuse):**

- ✅ **retry** (`pkg/retry`) – LLM provider retries, qmd retries
- ✅ **scheduler** (`pkg/scheduler`) – QMD index orchestration
- ✅ **receiptlog** (`pkg/receiptlog`) – ZenJournal foundation
- ✅ **health** (`pkg/health`) – Readiness/liveness endpoints
- ✅ **dedup** (`pkg/dedup`) – Redis message bus deduplication
- ✅ **store** (`pkg/store`) – Session persistence
- ✅ **observability** (`pkg/observability`) – OpenTelemetry tracing
- ✅ **logging** (`pkg/logging`) – Unified structured logging
- ✅ **events** (`pkg/events`) – Kubernetes event recording
- ✅ **crypto** (`pkg/crypto`) – Age encryption/decryption (via `internal/cryptoutil/`)
- ✅ **dlq** (`pkg/dlq`) – Dead Letter Queue (via `internal/dlqmgr/`)
- ✅ **leader** (`pkg/leader`) – Imported but not yet used for HA (deployment‑time concern)

## Wrapper Packages

For better integration with zen‑brain1’s configuration patterns, two wrapper packages exist:

- `internal/cryptoutil/` – Wraps `zen‑sdk/pkg/crypto` for age encryption  
- `internal/dlqmgr/` – Wraps `zen‑sdk/pkg/dlq` for dead‑letter‑queue management

These wrappers are documented in `scripts/ci/zen_sdk_allowlist.txt` and pass the ownership gate.

## CI Enforcement

The zen‑sdk ownership gate (`scripts/ci/zen_sdk_ownership_gate.py`) validates compliance and passes with the current codebase.

## Reference

See `docs/03‑DESIGN/ZEN_SDK_INTEGRATION_STATUS.md` for detailed integration status.