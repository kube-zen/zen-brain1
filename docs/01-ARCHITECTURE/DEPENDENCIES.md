# Dependencies & External Reuse

**Purpose:** zen-brain 1.0 consumes zen-sdk and other external dependencies for generic cross-cutting runtime capabilities. No local reimplementation of these concerns without an approved ADR.

## zen-sdk Reuse Contract

### Status: ~95% Complete

The reuse contract is in good shape: all mandatory reuse points (receiptlog, dedup, retry, health, store, scheduler) are satisfied and in use. Some items are **explicitly deferred**: DLQ, observability, leader, logging, events, and crypto adoption. This is **low risk** for current scope (no blocking gaps), but those deferred items remain **backlog — not done-done**. When adding DLQ, standardized tracing, HA leader election, structured logging, K8s event recording, or crypto helpers, wire the corresponding zen-sdk packages per this doc.

---

### Audit: zen-sdk Packages in Use

| zen-sdk package | Use in zen-brain | Location |
|-----------------|------------------|----------|
| `pkg/receiptlog` | ZenJournal foundation | `internal/journal/receiptlog/journal.go`; `pkg/journal` interface |
| `pkg/dedup` | Message bus deduplication | `internal/messagebus/redis/dedup.go` |
| `pkg/retry` | LLM provider retries, qmd retries | `internal/llm/gateway.go`, `internal/llm/routing/fallback_chain.go`, `internal/qmd/adapter.go` |
| `pkg/health` | Readiness/liveness endpoints | `internal/apiserver/server.go` |
| `pkg/store` | Session persistence | `internal/session/sqlite_store.go` |
| `pkg/scheduler` | QMD index orchestration | `internal/qmd/orchestrator.go` |

**go.mod:** `github.com/kube-zen/zen-sdk v0.3.0`. No dependency on zen-lock.

---

### zen-sdk Packages Not Yet Used (Deferred)

The following are plan-required but **not yet imported** in zen-brain; use when the corresponding feature is implemented.

| zen-sdk package | Plan requirement | Status |
|-----------------|------------------|--------|
| `pkg/dlq` | Failed task/message handling | Deferred; add when implementing DLQ for message bus or task failures |
| `pkg/observability` | Tracing/metrics wiring | Deferred; Foreman uses Prometheus directly; wire zen-sdk observability when standardizing |
| `pkg/leader` | Leader election for HA | Deferred; use when running multiple control-plane replicas |
| `pkg/logging` | Structured logging | Deferred; use when standardizing log format across components |
| `pkg/events` | Kubernetes event recording | Deferred; use when recording K8s events from controllers |
| `pkg/crypto` | Encryption and secret-protection | Deferred; use when adding secret encryption or HMAC helpers |

**Migration note (0.5.2):** "Migrate zen-lock/pkg/crypto to zen-sdk" is **N/A** for zen-brain — this repo does not depend on zen-lock. If a future change introduces crypto, use zen-sdk/pkg/crypto (or add it to zen-sdk first per the reuse rule).

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
| **OpenTelemetry** (future) | Distributed tracing | Not yet implemented |

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
