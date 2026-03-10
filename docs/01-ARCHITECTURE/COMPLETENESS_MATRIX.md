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
| **Office / Jira** | Partial | Real: fetch, update, search. Partial: webhooks, attachments, JQL. | `internal/office/jira/connector.go`, `README.md` (TODOs) |
| **Session manager** | Real | Create, get, resume, evidence; SQLite/memory; ZenContext write. | `internal/session/manager.go`, `sqlite_store.go` |
| **ZenContext (Tier 1/2/3)** | Real | Redis, QMD store, S3; composite; ReConstruct. Mock: QMD falls back to mock when CLI absent. | `internal/context/composite.go`, `tier1/`, `tier2/`, `tier3/`, `internal/qmd/` |
| **QMD / KB** | Partial | Real when `qmd` CLI present; FallbackToMock when not. Block 5.1: Populate() + BLOCK5_QMD_POPULATION.md, golden-query tests. | `internal/qmd/adapter.go`, `kb_store.go`, `populate.go` |
| **Message bus** | Real | Redis when `ZEN_BRAIN_MESSAGE_BUS=redis`; optional. | `internal/messagebus/redis/` |
| **ZenLedger** | Real | CockroachDB when DSN set; stub otherwise. | `internal/ledger/cockroach.go`, `stub.go` |
| **API server** | Partial | Real: health, sessions, health-detail. Auth: optional ZEN_API_KEY (X-API-Key or Bearer); /healthz, /readyz exempt. More endpoints TBD. | `internal/apiserver/`, `auth.go`, `cmd/apiserver/` |
| **Factory execution** | Partial | Real: BoundedExecutor runs real shell, workspace allocation, proof-of-work. Templates: some steps are echo-only (simulated output); useful_templates do create real files. | `internal/factory/bounded_executor.go`, `factory.go`, `work_templates.go`, `useful_templates.go` |
| **Foreman / Worker** | Real | CRDs, reconciler, worker pool, FactoryTaskRunner, ZenGate stub, ZenGuardian stub (CheckSafety, RecordEvent), metrics, queue status. | `internal/foreman/`, `cmd/foreman/`, `pkg/guardian/`, `internal/guardian/` |
| **Evidence Vault** | Real | Interface + MemoryVault; Store/GetBySession/GetByTask. | `internal/evidence/vault.go` |
| **Funding aggregator** | Real | T661 + IRAP from Vault evidence. | `internal/funding/aggregator.go` |
| **Agent–context binding** | Real | GetForContinuation / WriteIntermediate; TaskRunnerWithContext. | `internal/agent/binding.go`, `foreman/runner.go` |
| **ReMe protocol** | Real | ReconstructSession in ZenContext; ReMeBinder wires it as agent continuation path (Worker.ContextBinder = NewReMeBinder). | `internal/context/composite.go`, `internal/agent/binding.go` (ReMeBinder) |
| **K3d / deployment** | Partial | README + make dev-up; “Deploy Zen-Brain components” still TBD (no Helm/manifests for foreman/apiserver in-cluster). | `deployments/k3d/README.md`, `Makefile` dev-up |
| **Repo polish** | Partial | Makefile: repo-sync TODO; pre-commit/repo-check exist. | `Makefile`, `scripts/ci/` |

---

## Suggested fix order (production / completeness)

1. **Factory templates** – Replace or document echo-only “simulated” steps; add one real “run tests” path (e.g. `go test ./...` in workspace when Go project) or clearly label template tiers (real vs scaffold).
2. **API surface** – Auth done (ZEN_API_KEY). Add more endpoints as needed.
3. **QMD** – Document “real vs mock” and how to run with real QMD; repo-sync for KB population optional (Block 5.1 Populate + docs done).
4. **K3d** – Add minimal deployable manifest or Helm placeholder for foreman + apiserver (or document “run binaries locally with kubeconfig” as the current path).
5. ~~**ReMe**~~ – Done: ReMeBinder wires ReConstruct as agent continuation path.
6. **Jira** – Close webhooks/attachments/JQL or document as post-1.0.
7. **repo-sync** – Implement or document; reference in COMPLETENESS_MATRIX.

---

## Doc/code drift (resolved)

- **VERTICAL_SLICE_PROGRESS.md** – Updated: Session Manager and ZenContext are marked WIRED and point at `cmd/zen-brain/main.go`.

---

*Last updated from BLOCK3_4_PROGRESS and codebase scan. Used to drive production/completeness fixes: Factory template steps, k3d README + dependencies.yaml, Makefile repo-sync note.*
