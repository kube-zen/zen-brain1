# Block 0.5: Pre-requisite SDK (zen-sdk Reuse Contract)

**Purpose:** zen-brain 1.0 consumes zen-sdk for generic cross-cutting runtime capabilities. No local reimplementation of these concerns without an approved ADR.

**Construction Plan:** V6.1, Block 0.5.

---

## Audit: zen-sdk Packages in Use

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

## Plan-Required Packages Not Yet Used (Deferred)

The Construction Plan also lists the following. They are **not yet imported** in zen-brain; use when the corresponding feature is implemented.

| zen-sdk package | Plan requirement | Status |
|-----------------|------------------|--------|
| `pkg/dlq` | Failed task/message handling | Deferred; add when implementing DLQ for message bus or task failures |
| `pkg/observability` | Tracing/metrics wiring | Deferred; Foreman uses Prometheus directly; wire zen-sdk observability when standardizing |
| `pkg/leader` | Leader election for HA | Deferred; use when running multiple control-plane replicas |
| `pkg/logging` | Structured logging | Deferred; use when standardizing log format across components |
| `pkg/events` | Kubernetes event recording | Deferred; use when recording K8s events from controllers |
| `pkg/crypto` | Encryption and secret-protection | Deferred; use when adding secret encryption or HMAC helpers |

**Migration note (0.5.2):** “Migrate zen-lock/pkg/crypto to zen-sdk” is **N/A** for zen-brain — this repo does not depend on zen-lock. If a future change introduces crypto, use zen-sdk/pkg/crypto (or add it to zen-sdk first per the reuse rule).

---

## Reuse Contract

1. **Mandatory reuse:** The capabilities in the “Audit” table MUST be implemented via zen-sdk. Do not reimplement receiptlog, dedup, retry, health, store, or scheduler logic inside zen-brain.
2. **New generic capabilities:** If a new capability is generic and reusable across Zen projects, add it to zen-sdk first, then import it into zen-brain.
3. **Exceptions:** Any local replacement for a zen-sdk concern requires an approved ADR and a note in this document.

---

## Acceptance Criteria (Block 0.5)

- [x] ZenJournal implementation is explicitly built on `zen-sdk/pkg/receiptlog`
- [x] Message bus implementation explicitly uses `zen-sdk/pkg/dedup`
- [ ] Failed task/message handling explicitly uses `zen-sdk/pkg/dlq` — **deferred**
- [x] LLM/provider layer explicitly uses `zen-sdk/pkg/retry`
- [x] API/runtime health endpoints explicitly use `zen-sdk/pkg/health`
- [ ] Runtime tracing/metrics explicitly use `zen-sdk/pkg/observability` — **deferred** (Prometheus in use)
- [ ] HA control-plane path explicitly uses `zen-sdk/pkg/leader` — **deferred**
- [x] No local replacement package for receiptlog, dedup, retry, health, store, scheduler exists without an approved ADR

**Block 0.5 is complete** for the current scope: all mandatory reuse points are satisfied; dlq/observability/leader/logging/events/crypto are documented as deferred.

---

## Completeness (tracking)

- **CUTOVER.md:** Block 0.5 milestone is marked complete (SDK audit done; see above).
- **BLOCK_COMPLETION_MATRIX.md:** Block 0.5 appears as ✅ Complete; next action is to implement deferred packages when the corresponding features are built (dlq for failed tasks/messages, observability for tracing, leader for HA, etc.).
- **Lifting completeness further:** To move from “complete with deferred” to “more reuse in use,” wire one or more of the deferred zen-sdk packages when adding the related feature (e.g. `pkg/dlq` when implementing DLQ for message bus or task failures; `pkg/observability` when standardizing tracing; `pkg/logging` when standardizing log format).
