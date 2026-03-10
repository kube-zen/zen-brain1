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
| **Foreman + Factory** | Wired | cmd/foreman: `-factory` / `ZEN_FOREMAN_FACTORY=true` uses FactoryTaskRunner; `-factory-runtime-dir` / `ZEN_FACTORY_RUNTIME_DIR` (default `/tmp/zen-foreman-factory`) |

**Foreman:** `make build-foreman && ./bin/foreman` — needs kubeconfig; apply CRDs then run. Uses ZenGate stub, Worker pool. Default runner: PlaceholderRunner. With `-factory` (or `ZEN_FOREMAN_FACTORY=true`): FactoryTaskRunner with workspace manager, BoundedExecutor, proof-of-work in `-factory-runtime-dir`. Tasks flow Pending → Scheduled → (dispatched) → Running → Factory.ExecuteTask → Completed/Failed.

| **Observability** | Added | `internal/foreman/metrics.go`: Prometheus counters (scheduled, admission_denied, dispatched, completed, failed), histogram (reconcile_duration_seconds), gauge (worker_queue_depth); exposed on -metrics-bind-address |
| **Session-affinity** | Added | Worker.SessionAffinity; when true, per-worker queues and sticky session→worker; `-session-affinity` / `ZEN_FOREMAN_SESSION_AFFINITY` |
| **BrainQueue status** | Added | `internal/foreman/queue_status.go`: QueueStatusReconciler watches BrainQueue + BrainTask, sets queue.Status.Depth (Pending count) and InFlight (Scheduled+Running); registered in cmd/foreman |
| **ZenContext in cluster** | Added | `deployments/zencontext-in-cluster/`: namespace, Redis Deployment+Service, MinIO Deployment+Service; README with REDIS_URL and MinIO endpoint for in-cluster ZenContext |

**Outstanding (Block 3):** KB/QMD adapter; full API surface (auth, more endpoints).  
**Outstanding (Block 4):** ZenGuardian (optional).

## Block 5 (Intelligence) – In progress

| Item | Status | Notes |
|------|--------|------|
| **5.3 Agent–context binding** | Done | `internal/agent/binding.go`: AgentContextBinder (GetForContinuation, WriteIntermediate), ZenContextBinder; `foreman`: ContextBinder interface, TaskRunnerWithContext (RunWithContext); Worker uses binder + RunWithContext when set |
| **5.4 Funding evidence aggregator** | Done | `internal/funding/aggregator.go`: Aggregator from Vault; T661Narrative (Line 242/244/246), IRAPReport, FundingReport; AggregateForSession(s); T661Text(), IRAPMarkdown() |

**Agent-context binding:** Set `Worker.ContextBinder = agent.NewZenContextBinder(zenContext, "default")` and use a runner that implements `TaskRunnerWithContext` to read/write session context (State/Scratchpad) for continuation.  
**Funding reports:** `funding.NewAggregator(vault).AggregateForSession(ctx, sessionID, "Project Title")` returns T661 narrative and IRAP report; use `.T661.T661Text()` or `.IRAP.IRAPMarkdown()` for export.
