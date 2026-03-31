> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



**Status:** Operational guidance
**Last updated:** 2026-03-26

## Purpose

This document captures **how to get reliable, useful output** from **Qwen3.5** small checkpoints served by **llama.cpp** - primarily the **certified** local class **`qwen3.5:0.8b`** (GGUF typically **Q4_K_M**). The **same inference and prompt rules** apply to **Qwen3.5-2B** (L2-style / escalation lane); expect **slower** generation and **more RAM** on CPU - see [QWEN_2B_LOCAL_EVALUATION.md](QWEN_2B_LOCAL_EVALUATION.md) for measurements and a **Go harness parity** table. It complements:

- [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md) (policy and strategy)
- [LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md](LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md) (latency / stack matrix)
- [L1_L2_LANE_RUNBOOK.md](L1_L2_LANE_RUNBOOK.md) (L1 packet shaping)
- [ZB_08B_POSITIVE_CONTROL_RUNBOOK.md](ZB_08B_POSITIVE_CONTROL_RUNBOOK.md) (cluster health check)

Zen-brain production paths may use **Ollama** or **llama.cpp** per MLQ config; the **inference knobs** below apply whenever the backend is **llama.cpp**.

## Inference stack (llama.cpp)

| Control | Recommendation |
|--------|------------------|
| Server | Start `llama-server` with **`--reasoning off`** for straight codegen (avoids reasoning token burn on Qwen-class models). |
| Request JSON | Set **`chat_template_kwargs`: `{"enable_thinking": false}`** on `/v1/chat/completions` so the chat template does not enable hidden "thinking" segments. |
| System message | Include Qwen's soft switch **`/no_think`** on the system line when you need template-compatible "no reasoning" behavior (see Phase 17C-style reports under `p17c_results/`). |
| `ignore_eos` | Prefer **`false`** (natural end-of-sequence). Forcing continuation toward **`max_tokens`** makes CPU runs slow and often yields truncated garbage. |
| `max_tokens` | Size to the task; very large caps invite **length stops** on repetitive output. |

## Prompt and task shape (what "works" for 0.8B)

The L1 / autowork line in zen-brain is explicit: **generic role prompts are insufficient**. Use a **structured work item**, same *idea* as:

- **`config/task-templates/quickwin-l1.yaml`** - GOAL, TARGET FILE, PACKAGE, EXISTING CODE, RULES, verification commands, **OUTPUT: complete file only**.
- **`internal/promptbuilder/packet.go`** - task identity, scope, phases, verification, output contract.

For **single-turn codegen smoke tests** outside the cluster, a practical pattern is a **Jira-style ticket** (ID, status, problem, files to create, success criteria, **OUTPUT CONTRACT**) plus a short **TECHNICAL SPEC**. The OUTPUT section should state that the **entire assistant message** is **raw Go** (or raw target language) with **no markdown fences** and **no tool-call XML**, unless your pipeline strips fences before compile.

### Tools in the HTTP request

OpenAI-style **`tools`** arrays are supported by llama.cpp only with **`--jinja`**. For **0.8B pure codegen** evaluation, **omitting `tools`** often works better: the model is less likely to emit **fake `<tool_call>` blocks** or chatter instead of a file body. Production Factory code may still attach tools when executing multi-step work; for **calibration** of "can it emit a compiling file in one shot," prefer **no tools** or **`tool_choice: "none"`** only after validating behavior.

### Warmup

Before timed or scored requests:

1. **Liveness**: a tiny completion (e.g. user message `say ok`, small `max_tokens`) to confirm the slot and template path.
2. **Same-shape warmup**: one or more short generations that match the **same JSON shape** and **same output contract** as the scored task (bounded code, not a single-token ping).

See **`scripts/run-08b-positive-control.sh`** for cluster-oriented warmup; for raw llama.cpp on localhost, mirror the **L1 Lane Runbook** curl examples.

## Empirical results (2026-03, external harness)

Operators ran a **standalone llama.cpp** benchmark (OpenAI API to `llama-server`, **CPU**, **Q4_K_M** base GGUF vs a **LoRA-merged** GGUF) on bounded **Go** single-file tasks with structured prompts and `go build` verification.

**Base `Qwen3.5-0.8B-Q4_K_M` (no LoRA):**

- With a **tight structured ticket** and **subtask-specific hints** (allowed stdlib imports, correct `json.Decoder.Decode(v)` usage, no invented symbols), **`go build` passed** on a representative **HTTP JSON helper** subtask (`GetJSON`-style).
- Raw output tended to be **plain Go** (no fences), which matches earlier Phase 17C observations for llama.cpp when thinking is disabled.

**LoRA-tuned GGUF (same harness, same prompt):**

- Often produced **markdown fences**, **preamble text**, and **incomplete imports** inside the fence, leading to **compile failures** under the same verifier.
- **Interpretation:** the adapter likely shifted the distribution toward **short chat-style answers** and **partial snippets**. Improving LoRA outcomes would require **training on full-file, compile-clean targets**, sufficient **sequence length**, and tokenizer/merge hygiene-not just the same inference flags as base.

**Deep dive (dataset v1/v2, label statistics, train/inference mismatch):** [QWEN_08B_LORA_TUNING_POSTMORTEM.md](QWEN_08B_LORA_TUNING_POSTMORTEM.md).

**Takeaway:** **0.8B base** remains a **strong baseline** for **well-shaped, bounded** codegen tasks when prompts follow quick-win / packet rules. **LoRA** should be judged on **end-to-end compile** metrics aligned with production output contracts, not chat scores alone.

### Qwen3.5-2B (same harness)

Under the **identical** structured Go subtask and verifier, **2B Q4_K_M** achieved **`go build` OK** with roughly **~2×** longer wall and predicted generation time than **0.8B** on a representative host (details: [QWEN_2B_LOCAL_EVALUATION.md](QWEN_2B_LOCAL_EVALUATION.md) § Go codegen harness parity). Use **2B** when escalation policy calls for higher success odds on harder bounded tasks, accepting throughput cost.

## Relation to zen-brain1 product code

- **L1 routing** and **quickwin-l1** templates encode the same constraints this guide recommends for 0.8B.
- This repository does **not** vendor the full external harness; see [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md) for **`GO_SUBTASK_HARNESS_ROOT`**, the **`scripts/run-go-subtasks-2b-cpu.sh`** wrapper, and **0.8B / 2B** run examples. The **patterns** (thinking off, structured packet, verification, avoid misleading dual instructions) are what matter for parity with production.

## Related

- [GO_SUBTASK_LLAMA_CPP_HARNESS.md](GO_SUBTASK_LLAMA_CPP_HARNESS.md)
- [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md)
- [QWEN_2B_LOCAL_EVALUATION.md](QWEN_2B_LOCAL_EVALUATION.md) — 0.8B vs 2B tok/s, RAM, Go harness parity
- [L1_L2_LANE_RUNBOOK.md](L1_L2_LANE_RUNBOOK.md) — "Packet Shaping", warmup curls
- [LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md](LLAMA_CPP_VS_OLLAMA_QWEN_0_8B_BENCHMARK.md)
