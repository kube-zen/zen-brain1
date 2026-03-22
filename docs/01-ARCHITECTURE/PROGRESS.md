# Block 3 & 4 Progress

**Date:** 2026-03-09  
**Construction Plan:** V6.1

## Block 0 & 0.5 – Complete

- **Block 0:** Repo, layout, configurable home (`ZEN_BRAIN_HOME`, `internal/config/home.go`, `paths.go`), cutover plan (`docs/05-OPERATIONS/CUTOVER.md`).
- **Block 0.5:** zen-sdk reuse 100%; all packages imported and wired: retry, scheduler, receiptlog, health, dedup, store, observability, logging, events, crypto, dlq. See `docs/03-DESIGN/ZEN_SDK_INTEGRATION_STATUS.md`.

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

## Block 3 (Nervous System) – Complete

| Item | Status | Notes |
|------|--------|------|
| **3.1 Message Bus** | Done | `pkg/messagebus`, `internal/messagebus/redis`; vertical slice publishes `session.created` / `session.completed` when `ZEN_BRAIN_MESSAGE_BUS=redis` and REDIS_URL set; zen-sdk dedup |
| **3.2 State Synchronization** | Done | ZenContext (tiered state), Session Manager (session state), ReMe (reconstruction from journal + tiers); optional message bus for events; no separate cache layer |
| **3.3 ZenJournal** | Done | `pkg/journal`, `internal/journal/receiptlog` (zen-sdk receiptlog); query API; composite uses for ReMe |
| **3.4 API Server** | Done | `internal/apiserver`: `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`, `/api/v1/version`, `/api/v1/evidence?session_id=` (optional vault); optional SessionLister and ledger ping; API key auth when `ZEN_API_KEY` set; handler tests in `handlers_test.go` |
| **3.5 KB / QMD Adapter and Index Orchestration** | Done | `internal/qmd`: adapter (CLI wrapper), kb_store, Populate, Orchestrator (zen-sdk scheduler); BLOCK5_QMD_POPULATION.md; Tier 2 uses QMD store |
| **3.6 ZenLedger** | Done | `internal/ledger/cockroach.go`; zen-brain uses CockroachLedger when `ZEN_LEDGER_DSN` or `LEDGER_DATABASE_URL` set via `ledgerClientOrStub()` |
| **3.7 CockroachDB** | Done | `make db-up`, `make db-down`, `make db-migrate`, `make db-reset`; `migrations/001_*.sql`, `migrations/002_*.sql` |

**Block 3 Fast Completeness (runtime truth):** Canonical bootstrap in `internal/runtime`: `Bootstrap(ctx, cfg)` builds ZenContext, Ledger, MessageBus from `internal/config` (and env); fills `RuntimeReport` with real/stub/mock/degraded/disabled per capability; strictness via `ZEN_BRAIN_STRICT_RUNTIME` and `ZEN_BRAIN_REQUIRE_*`. Commands: `zen-brain runtime doctor`, `zen-brain runtime report`, `zen-brain runtime ping`. Session lifecycle writes to journal and message bus when configured (`session.Config.Journal`, `EventBus`, `EventStream`); events: session.created, session.transitioned, session.evidence_added, session.checkpoint_updated. API server uses `RuntimeChecker` for `/readyz` (fails when a required capability is unhealthy); `/api/v1/health` returns `RuntimeReport` JSON. ZenContext/Ledger/MessageBus are config-driven (no hardcoded localhost in main path). Health probes: `internal/qmd/health.go`, `internal/ledger/health.go`, `internal/context/health.go` (Ping / CheckHot/CheckWarm/CheckCold). Still out of scope: full DLQ, cross-process event replay, durable consumer orchestration.

**Block 3/5 allow mock or degraded paths by design:** Unless a capability is marked *required* (config or `ZEN_BRAIN_REQUIRE_*`), the runtime may run with fallbacks. **QMD** can fall back to mock when the `qmd` CLI is missing or unavailable (Tier 2 warm store optional; `internal/qmd` adapter uses mock for search/index when not configured). **Ledger** falls back to stub when no DSN is set or Cockroach is unreachable; `RuntimeReport.Ledger` is then `mode: stub` and no token/cost records are persisted. Use `zen-brain runtime doctor` or `/api/v1/health` to see actual mode; set `ledger.required: true` or `ZEN_BRAIN_REQUIRE_LEDGER=1` to fail startup instead of stub.

**API server:** `make build-apiserver && ./bin/apiserver` — serves `/healthz`, `/readyz`, `/`, `/api/v1/sessions`, `/api/v1/health`, `/api/v1/version`, `/api/v1/evidence?session_id=` (version from `API_VERSION` env or `dev`). Readiness reflects Block 3 dependency state; `/api/v1/health` returns runtime report when bootstrapped. Handlers tested in `internal/apiserver/handlers_test.go`.

## Block 4 (Factory) – Complete

| Item | Status | Notes |
|------|--------|------|
| **4.1 Core CRDs** | BrainTask + BrainAgent | `api/v1alpha1/braintask_types.go`, `brainagent_types.go`; `deployments/crds/`; `make generate`. BrainTaskSpec aligned with `pkg/contracts.BrainTaskSpec` (WorkType/WorkDomain/Priority as contracts types, EstimatedCostUSD float64, EvidenceRequirement, SREDTags, Hypothesis); conversion in `api/v1alpha1/conversion.go`. CRDs have enum/min/max/required validations. |
| **4.2 Foreman Controller** | Added | `internal/foreman/reconciler.go`, `cmd/foreman`; reconciles BrainTask (Pending → Scheduled) |
| **4.5 Evidence Vault** | Interface + impl | `internal/evidence/vault.go`: Vault interface (Store, GetBySession, GetByTask); MemoryVault for dev |
| **4.6 ZenGate stub** | Added | `internal/gate/stub.go`: NewStubGate() implements pkg/gate.ZenGate; Admit allows all, Validate returns nil |
| **BrainQueue CRD** | Added | `api/v1alpha1/brainqueue_types.go`: priority, maxConcurrency, sessionAffinity; status depth/inFlight; CRD in deployments/crds |
| **BrainPolicy CRD** | Added | `api/v1alpha1/brainpolicy_types.go`: cluster-scoped; rules (action, requiresApproval, maxCostUSD, allowedModels); CRD in deployments/crds. **BrainPolicy → canonical policy:** `internal/policyadapter` (ValidateBrainPolicySpec, ConvertBrainPolicyRule, ConvertBrainPolicy) maps BrainPolicy to `pkg/policy.PolicyRule` for engine loading. |
| **Foreman + Gate** | Wired | Reconciler accepts optional Gate and Dispatcher; cmd/foreman uses NewStubGate() and NoOpDispatcher; Admit before Pending→Scheduled |
| **TaskDispatcher** | Interface + no-op | `internal/foreman/dispatcher.go`: TaskDispatcher.Dispatch(ctx, task); NoOpDispatcher for 4.3 placeholder |
| **4.3 Worker pool** | Implemented | `internal/foreman/worker.go`: Worker implements TaskDispatcher; queue + N goroutines; processOne: Running → TaskRunner.Run → Completed/Failed; `runner.go`: TaskRunner + PlaceholderRunner |
| **Worktree manager** | Real when configured | `internal/worktree/manager.go`: Manager interface; `internal/worktree/git_manager.go`: GitManager creates real local git worktrees (`git worktree add --detach`), cleanup via `worktree remove --force`. StubManager remains for non-worktree mode. |
| **Foreman + git worktree** | Wired | When `ZEN_FOREMAN_USE_GIT_WORKTREE=true` and `ZEN_FOREMAN_SOURCE_REPO` set, Foreman uses GitManager + GitWorkspaceManager; tasks execute in a real writable worktree. Execution mode (`workspace` \| `git-worktree`) in outcome and `zen.kube-zen.com/factory-execution-mode` annotation. |
| **Proof-of-work** | Honest artifact paths | Proof bundle records actual artifact paths (JSON, MD, log); OutputLog aggregated from execution steps (not result.Error); git evidence paths (`review/git-status.txt`, `review/git-diff-stat.txt`) when present; branch/commit in result/summary. |
| **review:real lane** | Canonical real lane | `review:real` template: workspace/git inventory, language-aware checks (Go test, Python py_compile), REVIEW.md from real observations; repo-aware when run in a git worktree. |
| **Foreman + BrainQueue** | Wired | BrainTaskSpec.QueueName optional; Foreman skips scheduling when queue exists and Phase == Paused (requeue) |
| **Foreman cmd** | Worker + Factory | cmd/foreman uses Worker with FactoryTaskRunner by default; `-workers`, `-factory-runtime-dir`, `-factory-workspace-home`, `-factory-prefer-real-templates` (env: `ZEN_FOREMAN_RUNTIME_DIR`, `ZEN_FOREMAN_WORKSPACE_HOME`, `ZEN_FOREMAN_PREFER_REAL_TEMPLATES`) |
| **FactoryTaskRunner** | Default | `internal/foreman/factory_runner.go`: NewFactoryTaskRunner(cfg) builds Factory; converts BrainTask → FactoryTaskSpec; Run returns TaskRunOutcome; Worker persists outcome to BrainTask annotations |
| **Foreman + Factory** | Default path | cmd/foreman builds FactoryTaskRunner from config (no PlaceholderRunner); runtime/workspace dirs and prefer-real-templates via flags/env |

**Foreman:** `make build-foreman && ./bin/foreman` — needs kubeconfig; apply CRDs then run. Uses ZenGate stub, Worker pool. **Default runner: FactoryTaskRunner** (runtime dir, workspace home, prefer-real-templates from env). Tasks flow Pending → Scheduled → (dispatched) → Running → Factory.ExecuteTask → Completed/Failed; outcome annotations on BrainTask.

| **Observability** | Added | `internal/foreman/metrics.go`: Prometheus counters (scheduled, admission_denied, dispatched, completed, failed), histogram (reconcile_duration_seconds), gauge (worker_queue_depth); exposed on -metrics-bind-address |
| **Session-affinity** | Added | Worker.SessionAffinity; when true, per-worker queues and sticky session→worker; `-session-affinity` / `ZEN_FOREMAN_SESSION_AFFINITY` |
| **BrainQueue status** | Added | `internal/foreman/queue_status.go`: QueueStatusReconciler watches BrainQueue + BrainTask, sets queue.Status.Depth (Pending count) and InFlight (Scheduled+Running); registered in cmd/foreman |
| **ZenContext in cluster** | Added | `deployments/zencontext-in-cluster/`: namespace, Redis Deployment+Service, MinIO Deployment+Service; README with REDIS_URL and MinIO endpoint for in-cluster ZenContext |

| **ZenGuardian** | Added | `pkg/guardian/interface.go`: ZenGuardian (RecordEvent, CheckSafety); `internal/guardian/stub.go`: StubGuardian; Foreman Reconciler optional Guardian (CheckSafety before schedule, RecordEvent after) |
| **API auth** | Added | When `ZEN_API_KEY` set, API requires X-API-Key or Authorization: Bearer; /healthz, /readyz, / exempt. `internal/apiserver/auth.go`, Server.AuthAPIKey, cmd/apiserver |

**Block 4 complete:** CRDs (BrainTask, BrainAgent, BrainQueue, BrainPolicy), Foreman with Gate + Guardian + Dispatcher, worker pool, FactoryTaskRunner, **real local git worktree manager when configured**, GitWorkspaceManager (no deferred placeholders), proof-of-work with real artifact paths and git evidence, **review:real** as canonical trustworthy lane, execution mode in outcome/annotations, observability, session-affinity, queue status, ZenContext in-cluster. **ZenGate:** PolicyGate (default) enforces BrainPolicy when present; **ZenGuardian:** LogGuardian and CircuitBreakerGuardian available. **Still out of scope for Block 4:** no remote clone/fork/PR, no distributed worktree pool, no in-cluster git cache/bare-repo manager; in-cluster Foreman/API deploy is available (see deployments/k3d/README.md).

### Block 4 completeness (optional next steps)

To raise Block 4 completeness further without changing scope:

| Action | Description | Priority |
|--------|-------------|----------|
| **ZenLedger in Foreman** | When `ZEN_LEDGER_DSN` (or `LEDGER_DATABASE_URL`) is set, optionally pass a ZenLedgerClient/TokenRecorder into Foreman so task runs or LLM usage from Factory steps can be recorded (cost visibility, SR&ED, dashboards). Today zen-brain CLI wires ledger to Planner/LLM; cmd/foreman does not. | Optional |
| **ZenLedger dashboard (4.13)** | Add Grafana dashboard or equivalent for model efficiency, cost per project, local vs API breakdown, SR&ED cost accumulator (per Construction Plan 4.13). | Optional |
| **ZenGate beyond stub** | Done | PolicyGate loads BrainPolicies, enforces maxCostUSD and allowedModels on Admit. Foreman default `-gate=policy` (modes: stub, log, policy). No policies → allow; when policies exist → non-permissive. | Done |
| **ZenGuardian beyond stub** | Done | LogGuardian (audit log), CircuitBreakerGuardian (per-session rate limit). Foreman `-guardian=stub|log|circuit-breaker`, `-guardian-circuit-max-per-session-per-min`; env `ZEN_FOREMAN_GUARDIAN`, `ZEN_FOREMAN_GUARDIAN_CIRCUIT_MAX_PER_SESSION_PER_MIN`. | Done |

### Block 4 known gaps (post-86% assessment)

| Gap | Status | Notes |
|-----|--------|-------|
| **ZenGate allow-all stub** | Addressed | PolicyGate is default; enforces BrainPolicy (maxCostUSD, allowedModels) when CRDs exist. |
| **Factory templates scaffold/echo** | Partial | Many work-type templates (operations, documentation, etc.) use `echo`/scaffold steps; **review:real** is execution-real (Go test, Python compile, git evidence). Remaining templates can be upgraded incrementally. |
| **Proof signing/verification not cryptographic** | Open | Proof chain uses hash (zen-sdk receiptlog); artifact `SignArtifact`/`verifySignature` only check digest match; full crypto (e.g. age/signify) deferred until signing infrastructure is available. |
| **Remote clone/fork/PR, distributed worktree pool, in-cluster git cache/bare-repo** | Out of scope | Documented as still out of scope for Block 4; no change. |

**Block 3 complete:** Message bus, state sync (ZenContext/Session/ReMe), ZenJournal, API server (sessions, health, version), KB/QMD adapter and orchestration, ZenLedger, CockroachDB provisioning.

## Block 5 (Intelligence) – Complete

| Item | Status | Notes |
|------|--------|------|
| **5.1 QMD Population** | Done | `internal/qmd/populate.go`: Populate(ctx, client, repoPath, paths); `docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md`: sources, scopes, golden-query validation |
| **5.2 ReMe protocol** | Done | `internal/context/composite.go`: ReconstructSession (Tier 1 → Tier 3 → Journal + KB). `internal/agent/binding.go`: ReMeBinder uses ReConstructSession for GetForContinuation; Worker uses ContextBinder so set `Worker.ContextBinder = agent.NewReMeBinder(zenContext, "default")` for ReMe continuation |
| **5.3 Agent–context binding** | Done | `internal/agent/binding.go`: AgentContextBinder (GetForContinuation, WriteIntermediate), ZenContextBinder; `foreman`: ContextBinder interface, TaskRunnerWithContext (RunWithContext); Worker uses binder + RunWithContext when set |
| **5.4 Funding evidence aggregator** | Done | `internal/funding/aggregator.go`: Aggregator from Vault; T661Narrative (Line 242/244/246), IRAPReport, FundingReport; AggregateForSession(s); T661Text(), IRAPMarkdown() |

**ReMe:** Use `agent.NewReMeBinder(zenContext, "default")` as Worker.ContextBinder to run the full ReMe protocol on continuation (reconstruct from Tier 1/3 + Journal + KB). SessionContext now includes JournalEntries (causal chain) for the agent.
**Agent-context binding:** Use `agent.NewZenContextBinder(zenContext, "default")` for Tier-1-only continuation. Use a runner that implements `TaskRunnerWithContext` to read/write session context.
**Token recording:** When ZenLedger is CockroachLedger, call `gateway.SetTokenRecorder(ledgerClient)` so Chat() records token usage (Block 5).
**Model routing:** Planner uses ModelRecommender when set (zen-brain wires `NewModelRouterRecommender(NewModelRouter(ledger, defaultModel))`); else GetModelEfficiency + RecordPlannedModelSelection; budget check via GetCostBudgetStatus before planning.
**Evidence:** Planner records hypothesis evidence when EvidenceVault is set (zen-brain uses MemoryVault).
**QMD:** See `docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md`. Validate with `go test ./internal/qmd/... -run KBQuality`.
**Funding reports:** `funding.NewAggregator(vault).AggregateForSession(ctx, sessionID, "Project Title")` returns T661 narrative and IRAP report; use `.T661.T661Text()` or `.IRAP.IRAPMarkdown()` for export.

### Block 5 Wiring Guide

**ReMe (Recursive Memory)**
- **ReConstructSession** in `internal/context/composite.go`: Tier 1 (Hot) → Tier 3 (Cold) → Journal + Tier 2 (KB). Populates `SessionContext.JournalEntries` and `RelevantKnowledge`.
- **ReMeBinder** (`internal/agent/binding.go`): Implements `AgentContextBinder` using ReConstructSession. When the Worker has a task, it can call `GetForContinuation` to get full session context (journal + KB) before running.
- **Wiring:** Set `Worker.ContextBinder = agent.NewReMeBinder(zenContext, "default")` when Foreman has ZenContext (e.g. when running with Redis and optional Journal). cmd/foreman does not set ContextBinder today; add ZenContext + ReMeBinder when in-cluster or co-located with ZenContext.

**Memory Tiers**
- **Tier 1 (Hot):** Redis; sub-ms session context. See `COMPONENT_CONTEXT.md` for full architecture.
- **Tier 2 (Warm):** QMD KB; semantic search for relevant knowledge during ReMe. See `KB_QMD_STRATEGY.md`.
- **Tier 3 (Cold):** S3/MinIO; archival.
- **SessionContext:** Carries `State`, `JournalEntries`, `RelevantKnowledge`; written back via `StoreSessionContext` / `WriteIntermediate`.

**Model Routing**
- **ModelRouter** (`internal/intelligence/model_router.go`): Uses ZenLedger `GetModelEfficiency` to recommend a model by project and task type (success rate, cost).
- **Planner:** When `Config.ModelRecommender` is set (adapter from ModelRouter), `selectOptimalModel` uses it; otherwise uses ledger directly. **zen-brain** wires `planner.NewModelRouterRecommender(intelligence.NewModelRouter(ledgerClient, defaultModel))` so the vertical slice uses cost-aware routing.

**Advanced Evidence**
- **Evidence types** (`pkg/contracts`): hypothesis, experiment, observation, measurement, analysis, conclusion, proof_of_work, execution_log.
- **Planner → hypothesis:** When `Config.EvidenceVault` is set, the planner records an `EvidenceItem` with type `hypothesis` after producing a plan (session ID, task count, confidence). **zen-brain** sets `EvidenceVault = evidence.NewMemoryVault()` so each planned session gets a hypothesis record.
- **Factory → proof_of_work:** FactoryTaskRunner stores `proof_of_work` evidence when a task succeeds and has a proof path (see REMAINING_DRAGS / Factory).
- **Funding:** `internal/funding/aggregator.go` aggregates vault evidence into T661 narrative and IRAP report.

### Block 5 Wiring Checklist

| Component        | Where to wire | Status |
|-----------------|---------------|--------|
| ModelRouter → Planner | cmd/zen-brain: ModelRecommender = NewModelRouterRecommender(NewModelRouter(ledger, defaultModel)) | ✅ Done |
| EvidenceVault → Planner | cmd/zen-brain: EvidenceVault = NewMemoryVault() | ✅ Done |
| ReMeBinder → Worker | cmd/foreman: set -zen-context-redis or ZEN_CONTEXT_REDIS_URL to enable ReMe (Worker.ContextBinder = NewReMeBinder) | ✅ Done (optional flag) |
| Token recording | cmd/zen-brain: gateway.SetTokenRecorder(ledgerClient) when ledger is CockroachLedger | ✅ Done |

## Block 6 (Developer Experience) – Complete

| Item | Status | Notes |
|------|--------|------|
| **6.1 k3d cluster setup** | Done | `make dev-up`, `deployments/k3d/README.md`, `dependencies.yaml`; ports 8080, 26257 |
| **6.2 Development scripts** | Done | `make dev-up`, `dev-down`, `dev-logs`, `dev-clean` (db-reset), `dev-build` (build-all) |
| **6.3 Local configuration** | Done | `configs/config.dev.yaml`, ZEN_BRAIN_DEV, dev defaults |
| **6.4 Debugging guide** | Done | `docs/05-OPERATIONS/DEBUGGING.md`: workers, KB/QMD, LLM, k3d patterns |

**Block 6 complete:** k3d dev cluster, make targets, local config, debugging doc. Foreman and API server can be deployed in-cluster via `deployments/k3d/foreman.yaml` and `deployments/k3d/apiserver.yaml` (see `deployments/k3d/README.md`); image build with `make dev-image`.

## Item #2: Make the Slice More Useful – Complete (2026-03-10)

Transformed the zen-brain vertical slice from MVP to genuinely useful:

- **Real Execution Templates**: 10 working templates (Go, Python, JavaScript, CI/CD, Migrations, Monitoring, Documentation, Bugfix, Refactor, Review)
- **Enhanced Proof Artifacts**: Schema v2.0.0 with SHA256 checksums, environment metadata, signature support, verification API
- **Better State Continuity**: Real file tracking, workspace locking, structured execution logs
- **Better Status Semantics**: Granular step status (pending/running/completed/failed/skipped/canceled) with detailed error information

**Test Coverage**: 22+ passing tests; vertical-slice contract gate passes

**Commit**: 9e4ec3d - "feat(Item #2): Enhanced proof artifacts with versioning, checksums, verification"

## Item #5: LLM gateway / provider fallback — strong (not MLQ)

**Naming:** This item tracks the **LLM gateway and provider lane** (Ollama, planner, fallback chain). **Multi-Level Queue (MLQ)** — L1–L4 task queues, `multi_level_queue`-style routing — is **not** implemented; see [ROADMAP.md](./ROADMAP.md) (MLQ section).

**Current State** (2026-03-11):
- Provider set: 3 providers (local-worker, planner, fallback) ✅
- Real Ollama integration: 100% success rate, 8-57s latency (Docker host networking) ✅
- Warmup & keep-alive: OllamaWarmupCoordinator, OLLAMA_KEEP_ALIVE=-1 ✅
- Health checks: LiveHealthChecker for readiness probes ✅
- Retries: zen-sdk retry on some paths; provider **fallback chain** when enabled (see `internal/llm/gateway.go`, `internal/llm/routing/fallback_chain.go`) — not subtask-level MLQ replay ✅

**Validated Performance (qwen3.5:0.8b on Docker):**
- Latency: 8-57 seconds (warm), 22-82 seconds (cold)
- Throughput: ~12 tokens/sec, 60 tasks/hour
- Success rate: 100%
- Parallel workers: 20+

**Outstanding Work** (Optional enhancements):
- **Phase 1**: Prompt system enhancement (YAML/JSON templates, role-based profiles)
- **Phase 2**: Advanced calibration (model capability registry, evaluation harness)
- **Phase 3**: Provider set optimization (evaluate reducing to 2 providers)

**Reference**: See `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` and `docs/03-DESIGN/LLM_GATEWAY.md`
