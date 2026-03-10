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
| **API server** | Real | Block 3.4: /healthz, /readyz, /api/v1/sessions, /api/v1/health, /api/v1/version. Auth: optional ZEN_API_KEY. | `internal/apiserver/`, `auth.go`, `cmd/apiserver/` |
| **Factory execution** | Partial | Real: BoundedExecutor, workspace (getGitInfo), proof-of-work; "run tests" runs go test when go.mod present; FactoryTaskRunner records PoW to Vault when set. Templates: mix of real and echo steps. | `internal/factory/bounded_executor.go`, `workspace.go`, `factory_runner.go`, `proof.go` |
| **Foreman / Worker** | Real | CRDs, reconciler, worker pool, FactoryTaskRunner, ZenGate stub, ZenGuardian stub (CheckSafety, RecordEvent), metrics, queue status. | `internal/foreman/`, `cmd/foreman/`, `pkg/guardian/`, `internal/guardian/` |
| **Evidence Vault** | Real | Interface + MemoryVault; Store/GetBySession/GetByTask. | `internal/evidence/vault.go` |
| **Funding aggregator** | Real | T661 + IRAP from Vault evidence. | `internal/funding/aggregator.go` |
| **Agent–context binding** | Real | GetForContinuation / WriteIntermediate; TaskRunnerWithContext. | `internal/agent/binding.go`, `foreman/runner.go` |
| **ReMe protocol** | Real | ReconstructSession in ZenContext; ReMeBinder wires it as agent continuation path (Worker.ContextBinder = NewReMeBinder). | `internal/context/composite.go`, `internal/agent/binding.go` (ReMeBinder) |
| **Human Gatekeeper** | Real | Block 2.6: Gatekeeper interface, DefaultGatekeeper (approvals, reject, delegate, escalate, notifiers, audit). | `internal/gatekeeper/`, `internal/planner` (GetPendingApprovals) |
| **K3d / deployment** | Partial | Block 6: dev-up, dev-down, dev-logs, dev-clean, dev-build; DEBUGGING.md. “Deploy Zen-Brain components” still TBD (no Helm/manifests for foreman/apiserver in-cluster). | `deployments/k3d/README.md`, `Makefile`, `docs/05-OPERATIONS/DEBUGGING.md` |
| **Repo polish** | Partial | Makefile: repo-sync implemented (scripts/repo_sync.py, ZEN_KB_REPO_URL/DIR); pre-commit/repo-check exist. | `Makefile`, `scripts/repo_sync.py`, `scripts/ci/` |

---

## Suggested fix order (production / completeness)

1. **Factory templates** – Replace or document echo-only “simulated” steps; add one real “run tests” path (e.g. `go test ./...` in workspace when Go project) or clearly label template tiers (real vs scaffold).
2. **API surface** – Auth done (ZEN_API_KEY). Add more endpoints as needed.
3. **QMD** – Document “real vs mock” and how to run with real QMD; repo-sync for KB population optional (Block 5.1 Populate + docs done).
4. **K3d** – Add minimal deployable manifest or Helm placeholder for foreman + apiserver (or document “run binaries locally with kubeconfig” as the current path).
5. ~~**ReMe**~~ – Done: ReMeBinder wires ReConstruct as agent continuation path.
6. ~~**Jira**~~ – Done: webhooks (Watch), attachments (AddAttachment), JQL (Search); Block 2 complete.
7. ~~**repo-sync**~~ – Implemented: `make repo-sync` via `scripts/repo_sync.py` (clone/pull KB repos for QMD).

---

## Doc/code drift (resolved)

- **VERTICAL_SLICE_PROGRESS.md** – Updated: Session Manager and ZenContext are marked WIRED and point at `cmd/zen-brain/main.go`.

---

*Last updated from BLOCK3_4_PROGRESS and codebase scan. Used to drive production/completeness fixes: Factory template steps, k3d README + dependencies.yaml, Makefile repo-sync note.*
