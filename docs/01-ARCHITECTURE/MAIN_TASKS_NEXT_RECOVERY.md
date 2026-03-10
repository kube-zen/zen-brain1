# Main Tasks Next — Recovery / Resume Point

**Purpose:** Single place to resume work on the zen-brain vertical slice. These are the **five main tasks** to tackle next (as of the last planning session).

**Progress (2026-03-09):** Task 1 proven — `zen-brain vertical-slice --mock` runs E2E (Office → Analyze → Plan → Session → Factory → Proof-of-work → Status). Single proof-of-work source: main uses Factory’s artifact via `GetProofOfWork` when present; no duplicate creation.

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

**Current state:** Item #2 is in progress (~75%). Templates were upgraded from echo-only to real file-creating steps; proof artifacts and state continuity are partially improved.

**Recovery / next steps:**

- See [ITEM2_MAKE_SLICE_MORE_USEFUL.md](./ITEM2_MAKE_SLICE_MORE_USEFUL.md).
- Harden: deterministic proof-of-work format, clear failure paths, workspace/session state carried across steps.

---

## 3) Tighten session/context as real runtime glue

**Focus:** Session/context is the key to making the slice trustworthy rather than one-shot.

**Current state:** Session manager + ZenContext (three-tier: Redis / QMD / S3) are wired. Session creation and ZenContext storage exist in `internal/session/manager.go` and `pkg/context`. Integration tests cover session + ZenContext.

**Recovery / next steps:**

- Ensure every step of the vertical slice reads/writes session context where appropriate (e.g. planner, factory, proof-of-work attachment).
- Use [ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md) and session/zencontext integration tests as reference.
- Goal: resume/reconstruct sessions and carry state across runs, not just in-memory for one run.

---

## 4) Start the first real 0.1 rescue batch

**Best rescue targets:**

- Jira/ticket handling knowledge  
- Provider fallback/calibration  
- Proof/evidence templates  
- Watchdog behavior  
- Task/work templates  

**Current state:** Office/Jira integration exists (`internal/office`, `internal/integration/office.go`). Factory has work templates and proof generation (`internal/factory/templates`, `work_templates.go`, `useful_templates.go`). Intelligence/mining (Item #3) is implemented (miner, pattern store, recommender).

**Recovery / next steps:**

- Pick one rescue area (e.g. Jira/ticket knowledge or proof templates) and implement a small, shippable batch.
- Reuse: Office Manager, Factory templates, proof-of-work artifacts, and [ITEM3_INTELLIGENCE_MINING.md](./ITEM3_INTELLIGENCE_MINING.md) for patterns.

---

## 5) Keep MLQ/provider lane practical, not broad

**Focus:** Use the intended model strategy to improve usefulness; avoid adding more provider complexity.

**Current state:** LLM Gateway, fallback chain, and routing live in `internal/llm` and `pkg/llm`. Session-context-aware routing exists (`ProviderOrderForContext` in `internal/llm/routing/fallback_chain.go`).

**Recovery / next steps:**

- Keep provider set small; tune for reliability and cost (e.g. local worker + planner escalation).
- Improvements should be in prompt/usage and calibration, not in adding more providers or options.

---

## Quick reference

| Task | Doc / code |
|------|------------|
| 1 – Prove slice locally | [VERTICAL_SLICE_PROGRESS.md](./VERTICAL_SLICE_PROGRESS.md), [runner.go](../../internal/runner/runner.go), `vertical-slice` command |
| 2 – Trusted useful path | [ITEM2_MAKE_SLICE_MORE_USEFUL.md](./ITEM2_MAKE_SLICE_MORE_USEFUL.md), factory templates |
| 3 – Session/context glue | [ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md), [session/manager.go](../../internal/session/manager.go), [pkg/context](../../pkg/context/) |
| 4 – 0.1 rescue batch | Office/Jira, [ITEM3_INTELLIGENCE_MINING.md](./ITEM3_INTELLIGENCE_MINING.md), proof templates, watchdog |
| 5 – MLQ/provider practical | [internal/llm](../../internal/llm/), [fallback_chain.go](../../internal/llm/routing/fallback_chain.go) |

**Build:** `go build ./...` from repo root. Fix any regressions in `internal/runner` or contracts/llm types before adding new behavior.
