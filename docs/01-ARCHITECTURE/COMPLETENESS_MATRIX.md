# Completeness Matrix

**Purpose:** Track each subsystem as **real** / **mock** / **partial** / **missing** with files and suggested fix order so the repo can move toward production-complete and self-contained.

## Executive status (single narrative)

Zen-Brain is **1.0-shaped and execution-capable**. Remaining work is hardening the real path, reducing graceful fallbacks where they matter, and upgrading weaker execution lanes — not inventing missing blocks.

| Dimension | Score | Notes |
|-----------|-------|--------|
| **Architecture completeness** | 95% | Blocks 0–6 present; contracts, CRDs, session/context plumbing solid; deployment plane live-proven. |
| **Operational / deployment completeness** | 94% | Full sandbox path proven: redeploy exits 0, Helmfile converges, foreman/apiserver/ollama-0 Ready, preload succeeds, OLLAMA_BASE_URL on apiserver. |
| **Blended overall completeness** | 94% | Canonical deploy path real; sandbox and Ollama live-proven; production-shaped with focused hardening backlog. |

**Single source of truth for status:** This matrix, [PROGRESS.md](PROGRESS.md), [RECOMMENDED_NEXT_STEPS.md](RECOMMENDED_NEXT_STEPS.md), and [DEPLOYMENT_VALIDATION.md](../04-DEVELOPMENT/DEPLOYMENT_VALIDATION.md). README Development Status reflects this narrative.

**Remaining gaps (94% → 95%+):** (1) **Real inference proof** — one request through apiserver/gateway/local-worker returning a real model response; (2) VPA path not validated in sandbox; (3) Fail-closed runtime, controller/Factory/proof depth. See [RECOMMENDED_NEXT_STEPS.md](RECOMMENDED_NEXT_STEPS.md) for full list.

**Definitions:**
- **Real:** Production code path; no mandatory fallback to mock/simulated behavior.
- **Mock:** Explicit test or dev double; real path exists elsewhere or is optional.
- **Partial:** Real path exists but degrades to mock/stub when dependency missing, or feature subset only.
- **Missing:** Not implemented or placeholder only.

---

## By subsystem

| Subsystem | Status | Notes | Key files |
|-----------|--------|-------|-----------|
| **Office / Jira** | Real | Fetch, update, search, AddAttachment, Search (JQL), Watch (webhook server + HMAC). Config/bootstrap unified (config + env); vertical-slice posts proof comment + attachments + status; `zen-brain office doctor/search/fetch/watch` CLI. Block 2 analysis: durable history store (`ZEN_BRAIN_HOME/analysis`), auditable (AnalyzedAt, AnalyzedBy, AnalyzerVersion, WorkItemSnapshot), GetAnalysisHistory/UpdateAnalysis when store configured. | `internal/office/jira/connector.go`, `internal/analyzer/store.go`, `internal/integration/office_bootstrap.go`, `cmd/zen-brain/office.go`, `README.md` |
| **Session manager** | Real | Create, get, resume, evidence; SQLite/memory; ZenContext write; structured execution checkpoint (UpdateExecutionCheckpoint, GetExecutionCheckpoint, GetExecutionCheckpointSummary) for ReMe/resume. | `internal/session/manager.go`, `internal/session/checkpoint.go`, `sqlite_store.go` |
| **ZenContext (Tier 1/2/3)** | Real | Redis, QMD store, S3; composite; ReConstruct. Mock: QMD falls back to mock when CLI absent. | `internal/context/composite.go`, `tier1/`, `tier2/`, `tier3/`, `internal/qmd/` |
| **QMD / KB** | Partial | Real when `qmd` CLI present; FallbackToMock when not. Block 5.1: Populate() + KB_QMD_STRATEGY.md, golden-query tests. | `internal/qmd/adapter.go`, `kb_store.go`, `populate.go` |
| **Nervous System (Block 3)** | Real | Canonical bootstrap (`internal/runtime.Bootstrap`), config-driven ZenContext/Ledger/MessageBus; `RuntimeReport` marks real/stub/degraded/disabled; `zen-brain runtime doctor/report/ping`; session lifecycle writes to journal + message bus when configured; /readyz reflects required capabilities. **Fail-closed in prod:** Set `ZEN_RUNTIME_PROFILE=prod` or `ZEN_BRAIN_STRICT_RUNTIME=1` to require all capabilities (no QMD mock or ledger stub). Otherwise QMD/Ledger can degrade per PROGRESS.md. | `internal/runtime/`, `internal/config` (MessageBusConfig), `internal/session/events.go`, `internal/apiserver/runtime_checker.go`, `cmd/zen-brain/runtime.go` |
| **Message bus** | Real | Redis when `ZEN_BRAIN_MESSAGE_BUS=redis`; optional. | `internal/messagebus/redis/` |
| **ZenJournal** | Real | receiptlog-backed event store; query API; ReMe reconstruction uses it. | `pkg/journal`, `internal/journal/receiptlog` |
| **ZenLedger** | Real | CockroachDB when DSN set; stub otherwise. Stub implements TokenRecorder (no-op) so zen-brain and Foreman use the same code path when ledger is unavailable. | `internal/ledger/cockroach.go`, `stub.go` |
| **API server** | Real | Block 3.4: /healthz, /readyz (runtime-aware: fails when required capability unhealthy), GET /api/v1/sessions (optional limit, state, work_item_id), GET /api/v1/sessions/:id, /api/v1/health (RuntimeReport JSON), /api/v1/version, /api/v1/evidence?session_id= (optional vault). Bootstrap from config; Auth: ZEN_API_KEY. | `internal/apiserver/`, `runtime_checker.go`, `auth.go`, `cmd/apiserver/` |
| **Factory execution** | Real | BoundedExecutor, workspace (getGitInfo: real branch/commit when git repo; empty when not, no hard failure). Real step names: format, lint, build, Run tests (gofmt, go vet, go build, go test when go.mod present). work_templates: feature/bug-fix/refactor/test use format, lint, build, Run tests; useful_templates real implementation has build+Run tests. Template selection: prefers `<workType>:real`; proof-of-work: TemplateKey, real git metadata, sorted FilesChanged. review:real is repo-aware (inventory, Go/Python checks, REVIEW.md). See [FACTORY_TEMPLATE_TIERS.md](FACTORY_TEMPLATE_TIERS.md). | `internal/factory/factory.go`, `bounded_executor.go`, `work_templates.go`, `workspace.go`, `proof.go`, `useful_templates.go` |
| **Foreman / Worker** | Real | **FactoryTaskRunner by default** (no PlaceholderRunner). cmd/foreman builds runner from `ZEN_FOREMAN_RUNTIME_DIR`, `ZEN_FOREMAN_WORKSPACE_HOME`, `ZEN_FOREMAN_PREFER_REAL_TEMPLATES`. Worker persists run outcome to BrainTask annotations. **Gate default=policy** (enforce BrainPolicy when present); stub or log via `-gate stub` / `-gate log`. **Guardian default=log** (audit); stub or circuit-breaker via `-guardian`. CRDs, reconciler, worker pool, metrics, queue status. Optional ReMe: -zen-context-redis → ReMeBinder. | `internal/foreman/runner.go`, `factory_runner.go`, `worker.go`, `internal/gate/policy_gate.go`, `cmd/foreman/main.go` |
| **Evidence Vault** | Real | Interface + MemoryVault; Store/GetBySession/GetByTask. | `internal/evidence/vault.go` |
| **Funding aggregator** | Real | T661 + IRAP from Vault evidence. | `internal/funding/aggregator.go` |
| **Agent–context binding** | Real | GetForContinuation / WriteIntermediate; TaskRunnerWithContext. | `internal/agent/binding.go`, `foreman/runner.go` |
| **ReMe protocol** | Real | ReconstructSession in ZenContext; ReMeBinder wires it as agent continuation path (Worker.ContextBinder = NewReMeBinder). | `internal/context/composite.go`, `internal/agent/binding.go` (ReMeBinder) |
| **Intelligence (Block 5)** | Real | ModelRouter wired into Planner; model-selection provenance (Source, SampleSize, Alternatives) recorded in session evidence; structured ExecutionCheckpoint (stage, proof paths, selected model, analysis summary) in ZenContext; resume consumes checkpoint and skips blind replay when proof_attached/execution_complete; miner uses template_used first (model_used fallback); compatibility-aware recommender with selection reasoning (template/config, recency, failure-aware) in proof-of-work; failure stats + diagnose; CLI: `zen-brain intelligence mine | analyze | recommend | diagnose | checkpoint`. See [INTELLIGENCE_MINING.md](../03-DESIGN/INTELLIGENCE_MINING.md). | `internal/planner/`, `internal/intelligence/`, `internal/session/checkpoint.go`, `cmd/zen-brain/main.go`, `cmd/zen-brain/intelligence.go` |
| **Human Gatekeeper** | Real | Block 2.6: Gatekeeper interface, DefaultGatekeeper (approvals, reject, delegate, escalate, notifiers, audit). | `internal/gatekeeper/`, `internal/planner` (GetPendingApprovals) |
| **K3d / deployment** | Real | Block 6: dev-up, dev-down, dev-logs, dev-build, dev-image; in-cluster Foreman + API server via `deployments/k3d/foreman.yaml`, `apiserver.yaml` (see k3d README). DEBUGGING.md. | `deployments/k3d/README.md`, `Makefile`, `Dockerfile`, `docs/05-OPERATIONS/DEBUGGING.md` |
| **Repo polish** | Partial | Makefile: repo-sync implemented (scripts/repo_sync.py, ZEN_KB_REPO_URL/DIR); pre-commit/repo-check exist. | `Makefile`, `scripts/repo_sync.py`, `scripts/ci/` |

---

## Suggested fix order (production / completeness)

1. ~~**Factory / Foreman execution**~~ – Foreman runs BrainTasks through Factory by default; outcome annotations on BrainTask; Factory prefers real templates; proof has TemplateKey and real git; review:real is repo-aware. No distributed agents, remote clone, PR creation, or in-cluster deployment in this patch.
2. **API surface** – Auth done; GET /api/v1/sessions (list with limit, state, work_item_id), GET /api/v1/sessions/:id, /api/v1/evidence?session_id= (optional vault). Add more endpoints as needed.
3. ~~**QMD**~~ – Documented: KB_QMD_STRATEGY.md "Real vs mock" + internal/qmd/README; repo-sync done (Block 5.1 Populate + docs).
4. ~~**K3d**~~ – In-cluster Foreman + API server via deployments/k3d/foreman.yaml, apiserver.yaml; make dev-image; README documents in-cluster and local run.
5. ~~**ReMe**~~ – Done: ReMeBinder wires ReConstruct as agent continuation path.
6. ~~**Jira**~~ – Done: webhooks (Watch), attachments (AddAttachment), JQL (Search); Block 2 complete.
7. ~~**repo-sync**~~ – Implemented: `make repo-sync` via `scripts/repo_sync.py` (clone/pull KB repos for QMD).

---

## Doc/code drift (resolved)

- **VERTICAL_SLICE_PROGRESS.md** – Updated: Session Manager and ZenContext are marked WIRED and point at `cmd/zen-brain/main.go`.
- **Jira** – Config/env unified (JIRA_EMAIL/JIRA_USERNAME, JIRA_API_TOKEN/JIRA_TOKEN, Project→ProjectKey). Vertical-slice uses Office for proof comment, attachments, and status (running → completed/failed/blocked). Office CLI: doctor, search, fetch, watch.

---

**Still out of scope (Block 4 patch):** No distributed agents, no remote repo clone strategy, no PR creation, no advanced scheduling or resource isolation. In-cluster Foreman/API server is available (Block 6).

*Last updated from PROGRESS.md and Block 4 completeness patch. Template tiers: FACTORY_TEMPLATE_TIERS.md; QMD real vs mock: KB_QMD_STRATEGY.md.*
