# Local LLM escalation ladder (MLQ-oriented design)

## Status

**Production** (2026-03-26, PHASE 26)

This document defines the **tiered model escalation** architecture for zen-brain1 multi-queue (MLQ) execution. Updated to reflect the certified runtime: llama.cpp L1/L2 as primary, Ollama L0 as fallback only.

### Certified Runtime (PHASE 26)

| Tier | Model | Inference | Port | Concurrency | Role |
|------|-------|-----------|------|-------------|------|
| **L1** | Qwen3.5-0.8B Q4_K_M | llama.cpp | 56227 | 10 parallel slots | Default - all regular useful tasks |
| **L2** | Qwen3.5-2B Q4_K_M (zen-go) | llama.cpp | 60509 | 4 slots | Earned by repeated L1 failure |
| **L0** | qwen3.5:0.8b | Ollama | 11434 | 1 | Fallback only (FAIL-CLOSED) |
| **External** | deepseek/GLM/etc. | API | - | Pay-per-use | When local lanes exhausted |

### Implementation status (zen-brain1)

| Topic | In code today | Status |
|--------|----------------|--------|
| MLQ level selection, retry/escalation | `internal/mlq/task_executor.go` | ✅ Production |
| L1→L2 escalation after 2 failures | `config/policy/mlq-levels.yaml` | ✅ Proven (PHASE 23) |
| 10-way L1 parallelism | `--parallel 10` on llama.cpp | ✅ Proven (PHASE 22) |
| No-think for llama.cpp | `enable_thinking: false` | ✅ Proven (PHASE 23) |
| L0 Ollama FAIL-CLOSED | `internal/llm/ollama_provider.go` | ✅ Warning-only log |

## Goals

1. Keep **Qwen3.5 0.8B** as the **default local workhorse** (high volume, lowest marginal cost).
2. Add an **optional local escalation** to **Qwen3.5 2B** when 0.8B cannot complete a gated subtask-**without** dedicating most RAM to mid-size models.
3. Escalate further to **external** models when 2B is insufficient: e.g. **deepseek-reasoner**, **glm-5**, **Qwen3.5-397B-A17B** (or operator-configured equivalents).
4. Prefer **retries and escalation at subtask granularity**, not whole-task replay, when a long task fails partway through.

## Escalation tiers (production)

| Tier | Role | Inference | Typical use |
|------|------|-----------|-------------|
| **L1** | **0.8B workhorse** - all useful reporting, triage, evidence tasks | llama.cpp (10 slots, 6656 ctx/slot) | Default execution for all bounded useful work. |
| **L2** | **2B stronger** - when L1 fails after retries | llama.cpp (4 slots, 16384 ctx/slot) | Same task class when L1 fails; higher success rate. |
| **L0** | **Ollama fallback** - runtime outage only | Ollama (FAIL-CLOSED) | NOT for regular work. Only when L1+L2 unavailable. |
| **External** | **Cloud/API** - complex or high-stakes | deepseek-reasoner, GLM-5, etc. | When local lanes exhausted or task exceeds local capability. |

**Routing rules:**
1. Every useful task → L1 first (llama.cpp 0.8B, 10 parallel slots)
2. L1 retry (up to 2) → still fails → escalate to L2 (llama.cpp 2B)
3. L2 retry (up to 2) → still fails → escalate to L0/External
4. L2 concurrency capped at 1-2 concurrent jobs (higher RAM per instance)

## When to escalate (signals)

Escalation should be **gated**, not "every hard-looking prompt":

- **Hard failures:** unparseable structured output, wrong tool JSON, schema violations, empty completion after `think: false`, hard timeout.
- **Soft checks:** lightweight verifier (rules or tiny classifier) that the subtask **contract** is satisfied before accepting output.
- **Budget:** Per-tier attempt limits; optional **global cap** on fraction of traffic allowed to use 2B or externals.

## Retries (cross-queue)

Design intent: **~2 retries** available **across** the escalation path, with **clear semantics**:

- **Per subtask (preferred):** Up to **N** attempts at **0.8B**, then **one** attempt at **2B**, then **external**-without restarting completed work.
- **Per tier:** Distinct retry counts for **transient** errors (network, 429) vs **model inadequacy** (escalate tier instead of blind repeat).

Document **whether** "2 retries" means per subtask, per task, or per queue-avoid **double-charging** the same failure at every layer unless each layer addresses a different failure mode.

## Subtask-level control (checkpoint / partial replay)

**Problem:** If a task fails at **step 9 of 15**, restarting from step 1 wastes work and duplicates side effects.

**Target behavior:**

1. **Persist durable state** after each successful subtask (inputs, outputs, tool results, branch decisions).
2. On failure at step 9, **invalidate** step 9 (and any dependent work); **steps 1-8 remain committed** unless a **consistency check** requires narrow rollback.
3. **Retry** applies to **outstanding** work only-for a **linear** plan, that may be steps **9-15** (seven steps if 1-based inclusive; "six outstanding" in user language may mean remaining steps after failure-**define step numbering in implementation**).
4. **DAG / non-linear plans:** "Outstanding" is **not** always a suffix of 1..N; it is the **transitive successors** of the failed node (and nodes that must be recomputed).

**Operational requirements:**

- **Idempotency keys** per subtask to avoid double side effects (files, APIs, Jira) on retry.
- Explicit **completed / failed / skipped** status per subtask for observability and queue routing.

## Policy and compliance (ZB-023)

- **Primary local runtime:** llama.cpp (L1: 0.8B port 56227, L2: 2B port 60509)
- **Fallback only:** Ollama (L0: 0.8B port 11434, FAIL-CLOSED warning-only log)
- No assumption that other local models (14b, llama*, mistral*) are supported
- External models are already supported by the provider-agnostic design; routing is policy-driven

## Related documentation

- [SMALL_MODEL_STRATEGY.md](./SMALL_MODEL_STRATEGY.md) — canonical local lane policy (ZB-023).
- [L1/L2 Lane Runbook](../05-OPERATIONS/L1_L2_LANE_RUNBOOK.md) — operational procedure for L1/L2 tasks.
- [QWEN_2B_LOCAL_EVALUATION.md](../05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md) — llama.cpp 0.8B vs 2B (Q4_K_M) numbers.
