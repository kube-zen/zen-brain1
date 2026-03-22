# Local LLM escalation ladder (MLQ-oriented design)

## Status

**Draft / design intent** (2026-03-22)

This document captures **architecture and control-flow insights** for multi-queue (MLQ) execution: **tiered model escalation**, **retry semantics**, and **subtask-level recovery**. It **does not** change [SMALL_MODEL_STRATEGY.md](./SMALL_MODEL_STRATEGY.md) **ZB-023** policy by itself; operator override and product decisions are required before any new local model is “certified.”

## Goals

1. Keep **Qwen3.5 0.8B** as the **default local workhorse** (high volume, lowest marginal cost).
2. Add an **optional local escalation** to **Qwen3.5 2B** when 0.8B cannot complete a gated subtask—**without** dedicating most RAM to mid-size models.
3. Escalate further to **external** models when 2B is insufficient: e.g. **deepseek-reasoner**, **glm-5**, **Qwen3.5-397B-A17B** (or operator-configured equivalents).
4. Prefer **retries and escalation at subtask granularity**, not whole-task replay, when a long task fails partway through.

## Escalation tiers (conceptual)

| Tier | Role | Typical use |
|------|------|-------------|
| **0** | **0.8B** (certified local lane: `qwen3.5:0.8b` via host Ollama per ZB-023) | Default execution for all bounded local work. |
| **1** | **2B** (local, **strictly capped** concurrency—e.g. **1–2** concurrent instances) | Same task class when 0.8B fails **verifiable gates** (parse/tool/schema/empty output); higher success rate, higher RAM. See [QWEN_2B_LOCAL_EVALUATION.md](../05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md). |
| **2** | **External** (API / cloud) | When local 2B is exhausted or inappropriate: **deepseek-reasoner**, **glm-5**, **397B-class** MoE, etc.—reasoning depth, long context, or policy-driven escalation. |

**Ordering rationale:** Use **2B before** expensive externals to contain **cost and latency** while improving completion odds; use **externals** when **capability** dominates local throughput.

**Concurrency cap on 2B:** Only **1–2** concurrent 2B jobs is consistent with **~1 GiB higher resident memory per instance** vs 0.8B on reference hardware (evaluation doc). The remaining headroom is reserved for **0.8B parallelism** and **external** calls.

## When to escalate (signals)

Escalation should be **gated**, not “every hard-looking prompt”:

- **Hard failures:** unparseable structured output, wrong tool JSON, schema violations, empty completion after `think: false`, hard timeout.
- **Soft checks:** lightweight verifier (rules or tiny classifier) that the subtask **contract** is satisfied before accepting output.
- **Budget:** Per-tier attempt limits; optional **global cap** on fraction of traffic allowed to use 2B or externals.

## Retries (cross-queue)

Design intent: **~2 retries** available **across** the escalation path, with **clear semantics**:

- **Per subtask (preferred):** Up to **N** attempts at **0.8B**, then **one** attempt at **2B**, then **external**—without restarting completed work.
- **Per tier:** Distinct retry counts for **transient** errors (network, 429) vs **model inadequacy** (escalate tier instead of blind repeat).

Document **whether** “2 retries” means per subtask, per task, or per queue—avoid **double-charging** the same failure at every layer unless each layer addresses a different failure mode.

## Subtask-level control (checkpoint / partial replay)

**Problem:** If a task fails at **step 9 of 15**, restarting from step 1 wastes work and duplicates side effects.

**Target behavior:**

1. **Persist durable state** after each successful subtask (inputs, outputs, tool results, branch decisions).
2. On failure at step 9, **invalidate** step 9 (and any dependent work); **steps 1–8 remain committed** unless a **consistency check** requires narrow rollback.
3. **Retry** applies to **outstanding** work only—for a **linear** plan, that may be steps **9–15** (seven steps if 1-based inclusive; “six outstanding” in user language may mean remaining steps after failure—**define step numbering in implementation**).
4. **DAG / non-linear plans:** “Outstanding” is **not** always a suffix of 1..N; it is the **transitive successors** of the failed node (and nodes that must be recomputed).

**Operational requirements:**

- **Idempotency keys** per subtask to avoid double side effects (files, APIs, Jira) on retry.
- Explicit **completed / failed / skipped** status per subtask for observability and queue routing.

## Policy and compliance (ZB-023)

- The **only certified local CPU model** in production policy remains **`qwen3.5:0.8b`** via **host Docker Ollama** until explicitly changed—see [SMALL_MODEL_STRATEGY.md](./SMALL_MODEL_STRATEGY.md) and [deploy/README.md](../../deploy/README.md).
- A **local 2B** path (Ollama `create` from GGUF, llama.cpp, or other) is an **additional** capability that requires **operator configuration**, **capacity planning**, and **policy updates** if it is to become “certified.”
- **External** models are already supported by the provider-agnostic design; routing and secrets are **policy-driven**.

## Related documentation

- [SMALL_MODEL_STRATEGY.md](./SMALL_MODEL_STRATEGY.md) — canonical local 0.8B policy (ZB-023).
- [QWEN_2B_LOCAL_EVALUATION.md](../05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md) — llama.cpp 0.8B vs 2B (Q4_K_M) numbers.
- [LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md](../05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md) — stack × quantization matrix for 0.8B.
- [DEPENDENCIES.md](../01-ARCHITECTURE/DEPENDENCIES.md) — zen-sdk `retry`, routing, DLQ patterns.
