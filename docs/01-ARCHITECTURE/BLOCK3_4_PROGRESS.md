# Block 3 & 4 Progress

**Date:** 2026-03-09  
**Construction Plan:** V6.1

## Block 3 (Nervous System) – In progress

| Item | Status | Notes |
|------|--------|------|
| **3.1 Message Bus** | Wired | `pkg/messagebus`, `internal/messagebus/redis`; vertical slice publishes `session.created` / `session.completed` when `ZEN_BRAIN_MESSAGE_BUS=redis` and REDIS_URL set |
| **3.3 ZenJournal** | Existing | `pkg/journal`, `internal/journal/receiptlog` (zen-sdk receiptlog) |
| **3.4 API Server** | Extended | `internal/apiserver`: `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`; optional SessionLister and ledger ping |
| **3.6 ZenLedger** | Implemented | `internal/ledger/cockroach.go`: ZenLedgerClient + TokenRecorder backed by CockroachDB; migrations 001 (token_records), 002 (planned_model_selections) |
| **3.7 CockroachDB** | Done | `make db-up`, `make db-down`, `make db-migrate`, `make db-reset`; `migrations/001_*.sql`, `migrations/002_*.sql` |

**API server:** `make build-apiserver && ./bin/apiserver` — serves `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`.

## Block 4 (Factory) – In progress

| Item | Status | Notes |
|------|--------|------|
| **4.1 Core CRDs** | BrainTask + BrainAgent | `api/v1alpha1/braintask_types.go`, `brainagent_types.go`; `deployments/crds/`; `make generate` |
| **4.2 Foreman Controller** | Added | `internal/foreman/reconciler.go`, `cmd/foreman`; reconciles BrainTask (Pending → Scheduled) |
| **4.5 Evidence Vault** | Interface + impl | `internal/evidence/vault.go`: Vault interface (Store, GetBySession, GetByTask); MemoryVault for dev |

**Foreman:** `make build-foreman && ./bin/foreman` — needs kubeconfig; apply CRDs then run.

**Outstanding (Block 3):** KB/QMD adapter; full API surface (auth, more endpoints); ZenLedger wired into vertical slice when DSN set.  
**Outstanding (Block 4):** BrainQueue, BrainPolicy CRDs; worker agents (4.3); ZenContext in cluster; ZenGate; ZenGuardian; worktree manager; worker pool; session-affinity dispatcher; observability.
