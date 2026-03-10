# Block 3 & 4 Progress

**Date:** 2026-03-09  
**Construction Plan:** V6.1

## Block 3 (Nervous System) – Started

| Item | Status | Notes |
|------|--------|------|
| **3.1 Message Bus** | Existing | `pkg/messagebus`, `internal/messagebus/redis` (Redis Streams + DedupMessageBus) |
| **3.3 ZenJournal** | Existing | `pkg/journal`, `internal/journal/receiptlog` (zen-sdk receiptlog) |
| **3.4 API Server** | Added | `internal/apiserver` (health/readiness via zen-sdk health), `cmd/apiserver` |
| **3.6 ZenLedger** | Stub + schema | `pkg/ledger`, `internal/ledger/stub`; migrations for `token_records` table |
| **3.7 CockroachDB** | Added | `make db-up`, `make db-down`, `make db-migrate` (migrate CLI), `make db-reset`; `migrations/001_zen_ledger_tokens.*.sql` |

**API server:** `make build-apiserver && ./bin/apiserver` — serves `/healthz`, `/readyz`, `/`.

## Block 4 (Factory) – Started

| Item | Status | Notes |
|------|--------|------|
| **4.1 Core CRDs** | BrainTask added | `api/v1alpha1/braintask_types.go`, `deployments/crds/zen.kube-zen.com_braintasks.yaml`; `make generate` |
| **4.2 Foreman Controller** | Added | `internal/foreman/reconciler.go`, `cmd/foreman`; reconciles BrainTask (Pending → Scheduled) |

**Foreman:** `make build-foreman && ./bin/foreman` — needs kubeconfig; applies CRD then run.

**Outstanding (Block 3):** Message bus wiring into vertical slice; ZenLedger implementation (write to CockroachDB); KB/QMD adapter; full API surface.  
**Outstanding (Block 4):** BrainAgent, BrainQueue, BrainPolicy CRDs; worker agents (4.3); ZenContext in cluster; Evidence Vault; ZenGate; ZenGuardian; worktree manager; worker pool; session-affinity dispatcher; observability.
