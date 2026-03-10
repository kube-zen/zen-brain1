# Block 3 & 4 Progress

**Date:** 2026-03-09  
**Construction Plan:** V6.1

## Block 0 & 0.5 – Complete

- **Block 0:** Repo, layout, configurable home (`ZEN_BRAIN_HOME`, `internal/config/home.go`, `paths.go`), cutover plan (`docs/05-OPERATIONS/CUTOVER.md`).
- **Block 0.5:** zen-sdk reuse contract satisfied; audit and deferred packages documented. See **`docs/01-ARCHITECTURE/BLOCK0_5_ZEN_SDK.md`** for package audit, reuse contract, and deferred items (dlq, observability, leader, logging, events, crypto).

## Block 2 (Office) – Complete

| Item | Status | Notes |
|------|--------|------|
| **2.1 ZenOffice interface** | Done | `pkg/office`, `internal/office/base.go`; Fetch, FetchBySourceKey, UpdateStatus, AddComment, AddAttachment, Search, Watch |
| **2.2 Jira connector** | Done | `internal/office/jira`: fetch/update/search, AI attribution, AddAttachment (REST), Search (JQL), Watch (webhook server + HMAC validation); see `internal/office/jira/README.md` |
| **2.3 Intent Analyzer** | Done | Wired in vertical slice; `internal/analyzer` |
| **2.4 Session Manager** | Done | `internal/session/manager.go`; ZenContext integration |
| **2.5 Planner Agent** | Done | `internal/planner`; ZenContext, ZenLedgerClient, GetPendingApprovals |
| **2.6 Human Gatekeeper** | Done | `internal/gatekeeper`: Gatekeeper interface, DefaultGatekeeper (Approve, Reject, Delegate, Escalate, notifiers, audit log); Planner.GetPendingApprovals; console notifier |

**Block 2 complete:** Office abstraction, Jira connector (including webhooks, attachments, JQL), Intent Analyzer, Session Manager, Planner, Human Gatekeeper. Optional hardening (e.g. Jira webhook auth tuning, Gatekeeper HTTP UI) is post-1.0.

## Block 3 (Nervous System) – In progress

| Item | Status | Notes |
|------|--------|------|
| **3.1 Message Bus** | Wired | `pkg/messagebus`, `internal/messagebus/redis`; vertical slice publishes `session.created` / `session.completed` when `ZEN_BRAIN_MESSAGE_BUS=redis` and REDIS_URL set |
| **3.3 ZenJournal** | Existing | `pkg/journal`, `internal/journal/receiptlog` (zen-sdk receiptlog) |
| **3.4 API Server** | Extended | `internal/apiserver`: `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`; optional SessionLister and ledger ping |
| **3.6 ZenLedger** | Implemented + wired | `internal/ledger/cockroach.go`; zen-brain uses CockroachLedger when `ZEN_LEDGER_DSN` or `LEDGER_DATABASE_URL` set via `ledgerClientOrStub()` |
| **3.7 CockroachDB** | Done | `make db-up`, `make db-down`, `make db-migrate`, `make db-reset`; `migrations/001_*.sql`, `migrations/002_*.sql` |

**API server:** `make build-apiserver && ./bin/apiserver` — serves `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`.

## Block 4 (Factory) – Complete

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

| **ZenGuardian** | Added | `pkg/guardian/interface.go`: ZenGuardian (RecordEvent, CheckSafety); `internal/guardian/stub.go`: StubGuardian; Foreman Reconciler optional Guardian (CheckSafety before schedule, RecordEvent after) |
| **API auth** | Added | When `ZEN_API_KEY` set, API requires X-API-Key or Authorization: Bearer; /healthz, /readyz, / exempt. `internal/apiserver/auth.go`, Server.AuthAPIKey, cmd/apiserver |

**Block 4 complete:** CRDs (BrainTask, BrainAgent, BrainQueue, BrainPolicy), Foreman with Gate + Guardian + Dispatcher, worker pool, FactoryTaskRunner, worktree manager, observability, session-affinity, queue status, ZenContext in-cluster, ZenGate/ZenGuardian stubs. Real Guardian/Gate implementations are optional extensions.

**Outstanding (Block 3):** KB/QMD adapter; more API endpoints.

## Block 5 (Intelligence) – Complete

| Item | Status | Notes |
|------|--------|------|
| **5.1 QMD Population** | Done | `internal/qmd/populate.go`: Populate(ctx, client, repoPath, paths); `docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md`: sources, scopes, golden-query validation |
| **5.2 ReMe protocol** | Done | `internal/context/composite.go`: ReconstructSession (Tier 1 → Tier 3 → Journal + KB). `internal/agent/binding.go`: ReMeBinder uses ReConstructSession for GetForContinuation; Worker uses ContextBinder so set `Worker.ContextBinder = agent.NewReMeBinder(zenContext, "default")` for ReMe continuation |
| **5.3 Agent–context binding** | Done | `internal/agent/binding.go`: AgentContextBinder (GetForContinuation, WriteIntermediate), ZenContextBinder; `foreman`: ContextBinder interface, TaskRunnerWithContext (RunWithContext); Worker uses binder + RunWithContext when set |
| **5.4 Funding evidence aggregator** | Done | `internal/funding/aggregator.go`: Aggregator from Vault; T661Narrative (Line 242/244/246), IRAPReport, FundingReport; AggregateForSession(s); T661Text(), IRAPMarkdown() |

**ReMe:** Use `agent.NewReMeBinder(zenContext, "default")` as Worker.ContextBinder to run the full ReMe protocol on continuation (reconstruct from Tier 1/3 + Journal + KB).  
**Agent-context binding:** Use `agent.NewZenContextBinder(zenContext, "default")` for Tier-1-only continuation. Use a runner that implements `TaskRunnerWithContext` to read/write session context.  
**QMD:** See `docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md`. Validate with `go test ./internal/qmd/... -run KBQuality`.  
**Funding reports:** `funding.NewAggregator(vault).AggregateForSession(ctx, sessionID, "Project Title")` returns T661 narrative and IRAP report; use `.T661.T661Text()` or `.IRAP.IRAPMarkdown()` for export.
