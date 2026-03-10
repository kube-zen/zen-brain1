# Block 3 & 4 Progress

**Date:** 2026-03-09  
**Construction Plan:** V6.1

## Block 3 (Nervous System) – In progress

| Item | Status | Notes |
|------|--------|------|
| **3.1 Message Bus** | Wired | `pkg/messagebus`, `internal/messagebus/redis`; vertical slice publishes `session.created` / `session.completed` when `ZEN_BRAIN_MESSAGE_BUS=redis` and REDIS_URL set |
| **3.3 ZenJournal** | Existing | `pkg/journal`, `internal/journal/receiptlog` (zen-sdk receiptlog) |
| **3.4 API Server** | Extended | `internal/apiserver`: `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`; optional SessionLister and ledger ping |
| **3.6 ZenLedger** | Implemented + wired | `internal/ledger/cockroach.go`; zen-brain uses CockroachLedger when `ZEN_LEDGER_DSN` or `LEDGER_DATABASE_URL` set via `ledgerClientOrStub()` |
| **3.7 CockroachDB** | Done | `make db-up`, `make db-down`, `make db-migrate`, `make db-reset`; `migrations/001_*.sql`, `migrations/002_*.sql` |

**API server:** `make build-apiserver && ./bin/apiserver` — serves `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`.

## Block 4 (Factory) – In progress

| Item | Status | Notes |
|------|--------|------|
| **4.1 Core CRDs** | BrainTask + BrainAgent | `api/v1alpha1/braintask_types.go`, `brainagent_types.go`; `deployments/crds/`; `make generate` |
| **4.2 Foreman Controller** | Added | `internal/foreman/reconciler.go`, `cmd/foreman`; reconciles BrainTask (Pending → Scheduled) |
| **4.5 Evidence Vault** | Interface + impl | `internal/evidence/vault.go`: Vault interface (Store, GetBySession, GetByTask); MemoryVault for dev |
| **4.6 ZenGate stub** | Added | `internal/gate/stub.go`: NewStubGate() implements pkg/gate.ZenGate; Admit allows all, Validate returns nil |
| **BrainQueue CRD** | Added | `api/v1alpha1/brainqueue_types.go`: priority, maxConcurrency, sessionAffinity; status depth/inFlight; CRD in deployments/crds |
| **BrainPolicy CRD** | Added | `api/v1alpha1/brainpolicy_types.go`: cluster-scoped; rules (action, requiresApproval, maxCostUSD, allowedModels); CRD in deployments/crds |
| **Foreman + Gate** | Wired | Reconciler accepts optional Gate and Dispatcher; cmd/foreman uses NewStubGate() and NoOpDispatcher; Admit before Pending→Scheduled |
| **TaskDispatcher** | Interface + no-op | `internal/foreman/dispatcher.go`: TaskDispatcher.Dispatch(ctx, task); NoOpDispatcher for 4.3 placeholder |
| **4.3 Worker pool** | Implemented | `internal/foreman/worker.go`: Worker implements TaskDispatcher; queue + N goroutines; processOne: Running → TaskRunner.Run → Completed/Failed; `runner.go`: TaskRunner + PlaceholderRunner |
| **Worktree manager** | Interface + stub | `internal/worktree/manager.go`: Manager.Prepare(ctx, taskID, sessionID) (dir, cleanup, err); StubManager uses os.MkdirTemp |
| **Foreman + BrainQueue** | Wired | BrainTaskSpec.QueueName optional; Foreman skips scheduling when queue exists and Phase == Paused (requeue) |
| **Foreman cmd** | Worker + flag | cmd/foreman uses Worker(PlaceholderRunner, -workers=2), Start(ctx) before mgr.Start(ctx) |
| **FactoryTaskRunner** | Added | `internal/foreman/factory_runner.go`: converts BrainTask → FactoryTaskSpec, calls Factory.ExecuteTask; use NewFactoryTaskRunner(f) when Factory available |

**Foreman:** `make build-foreman && ./bin/foreman` — needs kubeconfig; apply CRDs then run. Uses ZenGate stub, Worker pool (PlaceholderRunner), `-workers=2`. Tasks flow Pending → Scheduled → (dispatched) → Running → Completed/Failed. To run tasks via Factory, pass `foreman.NewFactoryTaskRunner(factory)` as the runner when constructing the Worker.

**Outstanding (Block 3):** KB/QMD adapter; full API surface (auth, more endpoints).  
**Outstanding (Block 4):** Wire Factory into cmd/foreman (optional, for FactoryTaskRunner); ZenContext in cluster; ZenGuardian; session-affinity dispatcher; observability; Foreman updating BrainQueue depth/inFlight (optional).
