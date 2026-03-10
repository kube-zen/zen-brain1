# Completeness Matrix

**Purpose:** Track each subsystem as **real** / **mock** / **partial** / **missing** with files and suggested fix order so the repo can move toward production-complete and self-contained.

**Definitions:**
- **Real:** Production code path; no mandatory fallback to mock/simulated behavior.
- **Mock:** Explicit test or dev double; real path exists elsewhere or is optional.
- **Partial:** Real path exists but degrades to mock/stub when dependency missing, or feature subset only.
- **Missing:** Not implemented or placeholder only.

---

## By subsystem

| Subsystem | Status | Notes | Key files |
|-----------|--------|-------|-----------|
| **Office / Jira** | Real | Fetch, update, search, AddAttachment, Search (JQL), Watch (webhook server + HMAC). Block 2 complete. | `internal/office/jira/connector.go`, `README.md` |
| **Session manager** | Real | Create, get, resume, evidence; SQLite/memory; ZenContext write. | `internal/session/manager.go`, `sqlite_store.go` |
| **ZenContext (Tier 1/2/3)** | Real | Redis, QMD store, S3; composite; ReConstruct. Mock: QMD falls back to mock when CLI absent. | `internal/context/composite.go`, `tier1/`, `tier2/`, `tier3/`, `internal/qmd/` |
| **QMD / KB** | Partial | Real when `qmd` CLI present; FallbackToMock when not. Block 5.1: Populate() + BLOCK5_QMD_POPULATION.md, golden-query tests. | `internal/qmd/adapter.go`, `kb_store.go`, `populate.go` |
| **Nervous System (Block 3)** | Real | Connectivity layer: message bus, ZenJournal, state sync (ZenContext/Session/ReMe), API server, ZenLedger, KB/QMD. All real when deps set; bus optional, ledger stub when no DSN. Vertical slice publishes session.created, intent.analyzed, session.completed. | `pkg/messagebus`, `internal/journal`, `internal/context`, `internal/apiserver`, `internal/ledger`, `cmd/zen-brain` (events) |
| **Message bus** | Real | Redis when `ZEN_BRAIN_MESSAGE_BUS=redis`; optional. | `internal/messagebus/redis/` |
| **ZenJournal** | Real | receiptlog-backed event store; query API; ReMe reconstruction uses it. | `pkg/journal`, `internal/journal/receiptlog` |
| **ZenLedger** | Real | CockroachDB when DSN set; stub otherwise. | `internal/ledger/cockroach.go`, `stub.go` |
| **API server** | Real | Block 3.4: /healthz, /readyz, /api/v1/sessions, /api/v1/health, /api/v1/version, /api/v1/evidence?session_id= (optional vault). Auth: ZEN_API_KEY. | `internal/apiserver/`, `auth.go`, `cmd/apiserver/` |
| **Factory execution** | Real | BoundedExecutor, workspace (getGitInfo: real branch/commit when git repo; empty when not, no hard failure). Template selection: prefers `<workType>:real` when domain empty; fallback to default. Proof-of-work: TemplateKey, real git metadata, sorted FilesChanged. review:real is repo-aware (inventory, Go/Python checks, REVIEW.md). | `internal/factory/factory.go`, `bounded_executor.go`, `workspace.go`, `proof.go`, `useful_templates.go` |
| **Foreman / Worker** | Real | **FactoryTaskRunner by default** (no PlaceholderRunner). cmd/foreman builds runner from `ZEN_FOREMAN_RUNTIME_DIR`, `ZEN_FOREMAN_WORKSPACE_HOME`, `ZEN_FOREMAN_PREFER_REAL_TEMPLATES`. Worker persists run outcome to BrainTask annotations (factory-workspace, factory-proof, factory-template, factory-files-changed, factory-duration-seconds, factory-recommendation). CRDs, reconciler, worker pool, ZenGate/ZenGuardian stubs, metrics, queue status. Optional ReMe: -zen-context-redis → ReMeBinder. | `internal/foreman/runner.go`, `factory_runner.go`, `worker.go`, `cmd/foreman/main.go` |
| **Evidence Vault** | Real | Interface + MemoryVault; Store/GetBySession/GetByTask. | `internal/evidence/vault.go` |
| **Funding aggregator** | Real | T661 + IRAP from Vault evidence. | `internal/funding/aggregator.go` |
| **Agent–context binding** | Real | GetForContinuation / WriteIntermediate; TaskRunnerWithContext. | `internal/agent/binding.go`, `foreman/runner.go` |
| **ReMe protocol** | Real | ReconstructSession in ZenContext; ReMeBinder wires it as agent continuation path (Worker.ContextBinder = NewReMeBinder). | `internal/context/composite.go`, `internal/agent/binding.go` (ReMeBinder) |
| **Intelligence (Block 5)** | Real | ModelRouter wired into Planner (ModelRecommender); Planner records hypothesis evidence when EvidenceVault set; zen-brain wires both. ReMe + token recording + budget check. See BLOCK5_INTELLIGENCE_COMPLETENESS.md. | `internal/planner/`, `internal/intelligence/model_router.go`, `cmd/zen-brain` |
| **Human Gatekeeper** | Real | Block 2.6: Gatekeeper interface, DefaultGatekeeper (approvals, reject, delegate, escalate, notifiers, audit). | `internal/gatekeeper/`, `internal/planner` (GetPendingApprovals) |
| **K3d / deployment** | Partial | Block 6: dev-up, dev-down, dev-logs, dev-clean, dev-build; DEBUGGING.md. Current path: run foreman/apiserver/zen-brain locally (k3d README). In-cluster Helm/manifests TBD. | `deployments/k3d/README.md`, `Makefile`, `docs/05-OPERATIONS/DEBUGGING.md` |
| **Repo polish** | Partial | Makefile: repo-sync implemented (scripts/repo_sync.py, ZEN_KB_REPO_URL/DIR); pre-commit/repo-check exist. | `Makefile`, `scripts/repo_sync.py`, `scripts/ci/` |

---

## Suggested fix order (production / completeness)

1. ~~**Factory / Foreman execution**~~ – Foreman runs BrainTasks through Factory by default; outcome annotations on BrainTask; Factory prefers real templates; proof has TemplateKey and real git; review:real is repo-aware. No distributed agents, remote clone, PR creation, or in-cluster deployment in this patch.
2. **API surface** – Auth done; /api/v1/evidence?session_id= added (optional vault). Add more endpoints as needed.
3. ~~**QMD**~~ – Documented: BLOCK5_QMD_POPULATION "Real vs mock" + internal/qmd/README; repo-sync done (Block 5.1 Populate + docs).
4. **K3d** – Documented: k3d README "Current path" = run foreman/apiserver/zen-brain locally with kubeconfig. In-cluster TBD.
5. ~~**ReMe**~~ – Done: ReMeBinder wires ReConstruct as agent continuation path.
6. ~~**Jira**~~ – Done: webhooks (Watch), attachments (AddAttachment), JQL (Search); Block 2 complete.
7. ~~**repo-sync**~~ – Implemented: `make repo-sync` via `scripts/repo_sync.py` (clone/pull KB repos for QMD).

---

## Doc/code drift (resolved)

- **VERTICAL_SLICE_PROGRESS.md** – Updated: Session Manager and ZenContext are marked WIRED and point at `cmd/zen-brain/main.go`.

---

**Still out of scope (Block 4 patch):** No distributed agents, no remote repo clone strategy, no PR creation, no in-cluster deployment story, no advanced scheduling or resource isolation.

*Last updated from BLOCK3_4_PROGRESS and Block 4 completeness patch. Template tiers: FACTORY_TEMPLATE_TIERS.md; QMD real vs mock: BLOCK5_QMD_POPULATION.md.*
