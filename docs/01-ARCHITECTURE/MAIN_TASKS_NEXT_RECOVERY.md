# Main Tasks Next — Recovery / Resume Point

**Purpose:** Single place to resume work on the zen-brain vertical slice. These are the **five main tasks** to tackle next (as of the last planning session).

**Progress (2026-03-09):** Task 1 proven — `zen-brain vertical-slice --mock` runs E2E (Office → Analyze → Plan → Session → Factory → Proof-of-work → Status). Single proof-of-work source: main uses Factory’s artifact via `GetProofOfWork` when present; no duplicate creation. Task 2 (Item #2): deterministic proof (sorted slices), failure summary in proof markdown, factory errors pkg; committed. Task 3: persistent session store (SQLite via ZEN_BRAIN_SESSION_STORE=sqlite or when using --resume); vertical-slice --resume <session-id> continues from stored session; ZenContext session state updated after proof attachment. Task 4 (rescue batch): proof/evidence types (EvidenceTypeProofOfWork, EvidenceTypeExecutionLog), provider fallback calibration (LocalWorkerMaxTokens), vertical-slice watchdog (timeout + session failed on timeout); Item #5 doc and prompts package added.

---

## 1) Run and prove the vertical slice in a real local environment

**Priority:** Highest.

**Goal:** This flow working for real, not just structurally present:

- Office intake  
- Analyze  
- Plan  
- Session  
- Factory execution  
- Proof-of-work  
- Status update  

**Current state:** Pipeline is wired in `internal/runner/runner.go` (`processWorkItem` → vertical slice). Factory, templates, proof-of-work, session, and Office Manager are integrated. Some steps are still simulated or mock-dependent.

**Recovery / next steps:**

- Run the vertical slice end-to-end locally: `vertical-slice` (and/or `vertical-slice --mock`) from `cmd/zen-brain`.
- Replace any remaining mocks/simulations with real behavior (e.g. real workspace commands in Factory, real Jira when env is set).
- Use existing docs: [VERTICAL_SLICE_PROGRESS.md](./VERTICAL_SLICE_PROGRESS.md), [TRUSTWORTHY_VERTICAL_SLICE_COMPLETE.md](../04-DEVELOPMENT/TRUSTWORTHY_VERTICAL_SLICE_COMPLETE.md), [FACTORY_INTEGRATION_COMPLETE.md](../04-DEVELOPMENT/FACTORY_INTEGRATION_COMPLETE.md).

---

## 2) Convert the slice from “thin connected path” to “trusted useful path”

**Focus:**

- Deterministic outputs  
- Better failure handling  
- Cleaner proof generation  
- State continuity  

**Current state:** Item #2 largely done. Deterministic proof (all string slices sorted in proof), failure summary block in proof markdown when result != completed, factory errors pkg, test fix (NewFactory).

**Recovery / next steps:**

- See [COMPLETENESS_MATRIX.md](./COMPLETENESS_MATRIX.md) and [REMAINING_DRAGS.md](./REMAINING_DRAGS.md). Optional: schema version in proof markdown, extra state continuity (e.g. workspace file list in proof).

---

## 3) Tighten session/context as real runtime glue

**Focus:** Session/context is the key to making the slice trustworthy rather than one-shot.

**Current state:** Session manager + ZenContext (three-tier: Redis / QMD / S3) are wired. Session creation and ZenContext storage exist in `internal/session/manager.go` and `pkg/context`. Integration tests cover session + ZenContext.

**Recovery / next steps:**

- Session manager already stores/updates ZenContext on create and transition; `--resume <session-id>` uses persisted session (SQLite when `ZEN_BRAIN_SESSION_STORE=sqlite` or when resuming). ZenContext SessionContext.State is updated after proof-of-work attachment (vertical slice in main.go).
- Ensure remaining steps read session context where useful (e.g. planner already has ZenContext; factory could accept session ID for logging).
- Use [ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md) and session/zencontext integration tests as reference.
- Goal: resume/reconstruct sessions and carry state across runs; use `zen-brain vertical-slice --resume <id>` with persistent store.

---

## 4) Start the first real 0.1 rescue batch

**Best rescue targets:**

- Jira/ticket handling knowledge  
- Provider fallback/calibration  
- Proof/evidence templates  
- Watchdog behavior  
- Task/work templates  

**Current state:** First rescue batch shipped. **Proof/evidence:** `EvidenceTypeProofOfWork` and `EvidenceTypeExecutionLog` in contracts; vertical slice uses `EvidenceTypeProofOfWork` for proof artifacts. **Provider fallback/calibration:** `LocalWorkerMaxTokens` in fallback chain and gateway config—when set, skip local worker for large prompts (use planner first). **Watchdog:** vertical slice runs under a global timeout (default 15 min; `ZEN_BRAIN_VERTICAL_SLICE_TIMEOUT_SECONDS`); on timeout session transitions to failed and run exits. **Task/work templates:** `review`/`real` template added (review checklist + REVIEW.md). **Task 5 wire:** analyzer uses PromptManager and `work_item_analysis` template. Office/Jira and intelligence/mining remain as further rescue targets.

**Recovery / next steps:**

- Shipped: review work type template (`review`/`real`) in factory; analyzer uses prompt manager and `work_item_analysis` template (Task 5 wire). Optional: Jira/ticket knowledge rescue.
- Reuse: Office Manager, Factory templates, proof-of-work artifacts, and [COMPLETENESS_MATRIX.md](./COMPLETENESS_MATRIX.md) (Intelligence row) for patterns.

---

## 5) Keep the LLM / provider lane practical, not broad

**Focus:** Use the intended model strategy to improve usefulness; avoid adding more provider complexity. **MLQ (L1–L4 queues)** is out of scope here until implemented — see [ROADMAP.md](./ROADMAP.md).

**Current state:** LLM Gateway, fallback chain, and routing live in `internal/llm` and `pkg/llm`. Session-context-aware routing exists (`ProviderOrderForContext` in `internal/llm/routing/fallback_chain.go`). This is **not** multi-level task queuing.

**Recovery / next steps:**

- Analyzer now uses PromptManager and `work_item_analysis` template from [internal/llm/prompts.go](../../internal/llm/prompts.go); fallback to hardcoded prompt if template missing. Keep provider set small; tune for reliability and cost.
- Improvements should be in prompt/usage and calibration, not in adding more providers or options.

---

## Quick reference

| Task | Doc / code |
|------|------------|
| 1 – Prove slice locally | [VERTICAL_SLICE_PROGRESS.md](./VERTICAL_SLICE_PROGRESS.md), [runner.go](../../internal/runner/runner.go), `vertical-slice` command |
| 2 – Trusted useful path | [COMPLETENESS_MATRIX.md](./COMPLETENESS_MATRIX.md), [FACTORY_TEMPLATE_TIERS.md](./FACTORY_TEMPLATE_TIERS.md), factory templates |
| 3 – Session/context glue | [ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md), [session/manager.go](../../internal/session/manager.go), [pkg/context](../../pkg/context/) |
| 4 – 0.1 rescue batch | Office/Jira, [REMAINING_DRAGS.md](./REMAINING_DRAGS.md) (Intelligence/Factory), proof templates, watchdog |
| 5 – LLM/provider practical | [internal/llm](../../internal/llm/), [fallback_chain.go](../../internal/llm/routing/fallback_chain.go) |

**Build:** `go build ./...` from repo root. Fix any regressions in `internal/runner` or contracts/llm types before adding new behavior.
