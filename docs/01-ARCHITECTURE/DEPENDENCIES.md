# Dependencies & External Reuse

**Purpose:** zen-brain 1.0 consumes zen-sdk and other external dependencies for generic cross-cutting runtime capabilities. No local reimplementation of these concerns without an approved ADR.

## zen-sdk Reuse Contract

### Status: 100% Complete

All zen-sdk packages are imported and wired: retry, scheduler, receiptlog, health, dedup, store, observability, logging, events, crypto, and dlq.

---

### Audit: zen-sdk Packages in Use

| zen-sdk package | Use in zen-brain | Location |
|-----------------|------------------|----------|
| `pkg/receiptlog` | ZenJournal foundation | `internal/journal/receiptlog/journal.go`; `pkg/journal` interface |
| `pkg/dedup` | Message bus deduplication | `internal/messagebus/redis/dedup.go` |
| `pkg/retry` | LLM provider retries, qmd retries | `internal/llm/gateway.go`, `internal/llm/routing/fallback_chain.go`, `internal/qmd/adapter.go` |
| `pkg/health` | Readiness/liveness endpoints | `internal/apiserver/server.go`, `internal/apiserver/runtime_checker.go` |
| `pkg/store` | Session persistence | `internal/session/sqlite_store.go` |
| `pkg/scheduler` | QMD index orchestration | `internal/qmd/orchestrator.go` |
| `pkg/observability` | OpenTelemetry tracing | `cmd/controller/main.go`, `cmd/apiserver/main.go` |
| `pkg/logging` | Structured logging | `cmd/controller/main.go`, `cmd/apiserver/main.go`, multiple reconcilers |
| `pkg/events` | Kubernetes event recording | `internal/zencontroller/project_reconciler.go`, `internal/zencontroller/cluster_reconciler.go` |
| `pkg/crypto` | Age encryption/decryption | `internal/cryptoutil/crypto.go` (wrapper) |
| `pkg/dlq` | Dead Letter Queue | `internal/dlqmgr/manager.go` (wrapper) |

**go.mod:** `github.com/kube-zen/zen-sdk v0.3.0`. No dependency on zen-lock.

**All zen-sdk packages integrated** - No deferred packages remaining.

---

### Reuse Rules

1. **Mandatory reuse:** The capabilities in the "Audit" table MUST be implemented via zen-sdk. Do not reimplement receiptlog, dedup, retry, health, store, or scheduler logic inside zen-brain.
2. **New generic capabilities:** If a new capability is generic and reusable across Zen projects, add it to zen-sdk first, then import it into zen-brain.
3. **Exceptions:** Any local replacement for a zen-sdk concern requires an approved ADR and a note in this document.

### Allowlist (zen_sdk_ownership_gate)

The gate `scripts/ci/zen_sdk_ownership_gate.py` flags directories that match SDK package names and .go files containing SDK-like keywords. Entries in `scripts/ci/zen_sdk_allowlist.txt` are **allowed** because they are either:

- **Domain usage:** The file uses the same vocabulary (e.g. "Retry", "Schedule", "Health") for domain concepts (e.g. factory step retry, session state, API health detail), and **imports** zen-sdk for the actual implementation where applicable; or
- **Approved exception:** The path is explicitly documented (e.g. `internal/journal/receiptlog` uses zen-sdk receiptlog; journal is the only approved local receiptlog wrapper).

Adding a new allowlist entry requires a comment in the allowlist file and, if it is a true local reimplementation, an ADR. See [REPO_RULES.md](../04-DEVELOPMENT/REPO_RULES.md) and [RECOMMENDED_NEXT_STEPS.md](RECOMMENDED_NEXT_STEPS.md).

---

## Other External Dependencies

### Storage & Data

| Dependency | Use | Location |
|------------|-----|----------|
| **Redis** (go-redis/v9) | Tier 1 hot storage for ZenContext | `internal/context/tier1/redis_client.go` |
| **CockroachDB** | ZenLedger (token/cost accounting), structured data | `internal/ledger/cockroach.go` |
| **S3/MinIO** (AWS SDK v2) | Tier 3 cold storage for ZenContext | `internal/context/tier3/s3_client.go` |
| **SQLite** | Session persistence | `internal/session/sqlite_store.go` |

### Kubernetes

| Dependency | Use | Location |
|------------|-----|----------|
| **controller-runtime** | CRD controllers (Foreman, QueueStatus) | `internal/foreman/`, `cmd/foreman/` |
| **client-go** | Kubernetes API client | `internal/worktree/`, `internal/gatekeeper/` |
| **apimachinery** | API types and meta | Multiple components |

### AI/LLM

| Dependency | Use | Location |
|------------|-----|----------|
| **Ollama** (future) | Local LLM provider | Not yet implemented |
| **OpenAI API** (future) | Cloud LLM provider | Not yet implemented |
| **qmd CLI** | Knowledge base indexing and search | `internal/qmd/adapter.go` |

### Monitoring & Observability

| Dependency | Use | Location |
|------------|-----|----------|
| **Prometheus** | Metrics collection | `internal/foreman/metrics.go` |
| **OpenTelemetry** | Distributed tracing | `cmd/controller/main.go`, `cmd/apiserver/main.go` |

---

## Acceptance Criteria

- [x] ZenJournal implementation is explicitly built on `zen-sdk/pkg/receiptlog`
- [x] Message bus implementation explicitly uses `zen-sdk/pkg/dedup`
- [ ] Failed task/message handling explicitly uses `zen-sdk/pkg/dlq` — **deferred**
- [x] LLM/provider layer explicitly uses `zen-sdk/pkg/retry`
- [x] API/runtime health endpoints explicitly use `zen-sdk/pkg/health`
- [ ] Runtime tracing/metrics explicitly use `zen-sdk/pkg/observability` — **deferred** (Prometheus in use)
- [ ] HA control-plane path explicitly uses `zen-sdk/pkg/leader` — **deferred**
- [x] No local replacement package for receiptlog, dedup, retry, health, store, scheduler exists without an approved ADR

---

## Lifting to Done-Done

Wire the deferred zen-sdk packages when adding the related feature:

- **`pkg/dlq`** - DLQ for message bus or task failures
- **`pkg/observability`** - Tracing/metrics standardization
- **`pkg/leader`** - HA control-plane leader election
- **`pkg/logging`** - Structured logging across components
- **`pkg/events`** - K8s event recording from controllers
- **`pkg/crypto`** - Secret encryption or HMAC helpers

---

## References

- zen-sdk repository: `github.com/kube-zen/zen-sdk`
- zen-sdk documentation: See internal package docs
- Block 0.5: Original zen-sdk reuse audit
- ADR process: `docs/01-ARCHITECTURE/ADR/`
