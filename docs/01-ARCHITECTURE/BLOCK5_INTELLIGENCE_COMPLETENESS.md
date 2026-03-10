# Block 5: Intelligence completeness

**Purpose:** Summarize ReMe, memory, model routing, and advanced evidence—and how to wire them for higher completeness.

## ReMe (Recursive Memory)

- **ReConstructSession** in `internal/context/composite.go`: Tier 1 (Hot) → Tier 3 (Cold) → Journal + Tier 2 (KB). Populates `SessionContext.JournalEntries` and `RelevantKnowledge`.
- **ReMeBinder** (`internal/agent/binding.go`): Implements `AgentContextBinder` using ReConstructSession. When the Worker has a task, it can call `GetForContinuation` to get full session context (journal + KB) before running.
- **Wiring:** Set `Worker.ContextBinder = agent.NewReMeBinder(zenContext, "default")` when Foreman has ZenContext (e.g. when running with Redis and optional Journal). cmd/foreman does not set ContextBinder today; add ZenContext + ReMeBinder when in-cluster or co-located with ZenContext.

## Memory

- **Tier 1 (Hot):** Redis; sub-ms session context.
- **Tier 2 (Warm):** QMD KB; semantic search for relevant knowledge during ReMe.
- **Tier 3 (Cold):** S3/MinIO; archival.
- **SessionContext:** Carries `State`, `JournalEntries`, `RelevantKnowledge`; written back via `StoreSessionContext` / `WriteIntermediate`.

## Model routing

- **ModelRouter** (`internal/intelligence/model_router.go`): Uses ZenLedger `GetModelEfficiency` to recommend a model by project and task type (success rate, cost).
- **Planner:** When `Config.ModelRecommender` is set (adapter from ModelRouter), `selectOptimalModel` uses it; otherwise uses ledger directly. **zen-brain** wires `planner.NewModelRouterRecommender(intelligence.NewModelRouter(ledgerClient, defaultModel))` so the vertical slice uses cost-aware routing.

## Advanced evidence

- **Evidence types** (`pkg/contracts`): hypothesis, experiment, observation, measurement, analysis, conclusion, proof_of_work, execution_log.
- **Planner → hypothesis:** When `Config.EvidenceVault` is set, the planner records an `EvidenceItem` with type `hypothesis` after producing a plan (session ID, task count, confidence). **zen-brain** sets `EvidenceVault = evidence.NewMemoryVault()` so each planned session gets a hypothesis record.
- **Factory → proof_of_work:** FactoryTaskRunner stores `proof_of_work` evidence when a task succeeds and has a proof path (see REMAINING_DRAGS / Factory).
- **Funding:** `internal/funding/aggregator.go` aggregates vault evidence into T661 narrative and IRAP report.

## Wiring checklist

| Component        | Where to wire | Status |
|-----------------|---------------|--------|
| ModelRouter → Planner | cmd/zen-brain: ModelRecommender = NewModelRouterRecommender(NewModelRouter(ledger, defaultModel)) | Done |
| EvidenceVault → Planner | cmd/zen-brain: EvidenceVault = NewMemoryVault() | Done |
| ReMeBinder → Worker | cmd/foreman: when ZenContext available, Worker.ContextBinder = NewReMeBinder(zenContext, "default") | Optional (Foreman has no ZenContext today) |
| Token recording | cmd/zen-brain: gateway.SetTokenRecorder(ledgerClient) when ledger is CockroachLedger | Done |

## References

- BLOCK3_4_PROGRESS.md — Block 5 table
- REMAINING_DRAGS.md — Intelligence bullet
- internal/planner/interface.go — ModelRecommender, EvidenceVault
- internal/agent/binding.go — ReMeBinder
