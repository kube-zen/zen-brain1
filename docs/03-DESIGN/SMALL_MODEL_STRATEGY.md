# Small-Model Strategy

## Status

**Production** (2026-03-25)

## PHASE 23 Update: Runtime Behavior Locked

As of PHASE 23 (2026-03-25), the following runtime behaviors are now **proven and locked**:

1. **Factory routes through TaskExecutor** — all MLQ tasks use `ExecuteWithRetry()` for automatic retry/escalation
2. **L1 first, always** — regular tasks go to L1 by default; L2 is earned by repeated L1 failure evidence
3. **Thinking disabled** — llama.cpp requests include `enable_thinking: false` to prevent empty artifacts
4. **10-way L1 parallelism** — single llama.cpp server with `--parallel 10`, proven with concurrent batch
5. **Escalation proven** — L1→L2 escalation after 2 failures verified end-to-end (see P004 test)

### Evidence
- 10 useful reporting tasks: 10/10 L1 success, 71s parallel wall time
- Escalation test: L1×2 failures → L2 success in 2.3s
- No-think fix: stub_hunting artifact went from 0 bytes to 1.8KB

## 🚨 CRITICAL POLICY (ZB-023) - CANONICAL SOURCE OF TRUTH

**UNTIL EXPLICITLY OVERRIDDEN BY THE OPERATOR:**

### Certified Local CPU Path

- ✅ **ONLY allowed local model:** `qwen3.5:0.8b`
- ✅ **ONLY supported local inference path:** Host Docker Ollama (http://host.k3d.internal:11434)
- ❌ **FORBIDDEN:** In-cluster Ollama for active local CPU path
- ❌ **FORBIDDEN:** Any other local model (e.g., qwen3.5:14b, llama*, mistral*)

### Provider/Model Flexibility

- Any provider/model may serve any role if configured
- The outdated "planner=GLM, worker=0.8b" split is **REMOVED**
- `qwen3.5:0.8b` is NOT worker-only by architecture
- GLM is NOT planner-only by architecture
- **However:** The ONLY certified LOCAL CPU lane is `qwen3.5:0.8b` via host Docker Ollama

### Enforcement (FAIL-CLOSED)

- Policy: `config/policy/providers.yaml`, `config/policy/routing.yaml`
- Runtime: `internal/llm/ollama_provider.go`, `internal/llm/gateway.go`
- CI: `scripts/ci/local_model_policy_gate.py` (blocks PRs that drift)
- Documentation: This document (canonical source of truth)

### Related Documentation

- **Operational Guide:** [OLLAMA_08B_OPERATIONS_GUIDE.md](../05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md)
- **Warmup Runbook:** [OLLAMA_WARMUP_RUNBOOK.md](../05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md)
- **Deployment:** [deploy/README.md](../../deploy/README.md)
- **Runbook:** [ZB_023_LOCAL_CPU_INFERENCE_RULE.md](../05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md)
- **Escalation design (0.8B → 2B → external, subtask retries):** [LOCAL_LLM_ESCALATION_LADDER.md](./LOCAL_LLM_ESCALATION_LADDER.md)
- **2B vs 0.8B local evaluation (llama.cpp Q4_K_M, incl. Go harness parity):** [QWEN_2B_LOCAL_EVALUATION.md](../05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md)
- **0.8B llama.cpp vs Ollama (2×2 matrix):** [LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md](../05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md)
- **0.8B llama.cpp codegen, testing, LoRA vs base notes:** [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](../05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md)
- **Go subtask harness (llama.cpp, 0.8B / 2B, operator checkout):** [GO_SUBTASK_LLAMA_CPP_HARNESS.md](../05-OPERATIONS/GO_SUBTASK_LLAMA_CPP_HARNESS.md)

---

## Context

Zen-Brain 1.0 is designed as an **internal force multiplier** first, not a market product. To be useful as a trusted operator, the system must:

1. Run reliably on modest hardware (CPU-only hosts are common in internal deployments)
2. Provide predictable throughput and latency for planning and execution tasks
3. Maintain cost efficiency while being open to any provider/model
4. Calibrate and benchmark against real work, not generic chat quality

Small models (Qwen 0.8B class and similar) represent a sweet spot:
- Extremely cheap to run (free local inference)
- Highly parallelizable (many workers, many tasks)
- Capable of bounded, well-context-shaped tasks
- Require careful calibration to be useful beyond toy chat

## Decision

**Zen-Brain 1.0 optimizes for CPU-first local small-model operation**, with provider-agnostic design allowing future paths to larger models or paid APIs when justified.

### Core Principles

1. **Provider-agnostic** – System works with any LLM provider (Ollama, OpenAI, Anthropic, etc.)
2. **Certified local model as baseline** – qwen3.5:0.8b is ONLY certified local model (ZB-023)
3. **Yield per token matters** – Useful throughput > raw token cost
4. **Calibration by task class** – Benchmark against real work, not general chat quality
5. **Fallback/escalation path** – Route to larger cloud models when certified local model cannot complete task
6. **Parallelization compensates for slowness** – 20+ workers ×4-6 tasks/hour > 1 large model ×100 tasks/hour at lower cost

### When to Use Which Model

| Model Class | Use Case | Throughput (CPU 14-core) | Cost | Notes |
|------------|----------|---------------------------|------|-------|
| **Local certified (0.8B)** | Bounded tasks: code completion, tool calling, doc updates, simple refactors, planning, review, summarization | 96-144 tasks/hour | $0 | Certified for CPU, validated with 20+ parallel workers |
| **Cloud (GLM/OpenAI/etc.)** | Complex/ambiguous tasks, high-stakes work, when certified model fails calibration | 60-100 tasks/hour | $2-5/hour | High reliability, low latency, higher cost |

**Routing rules:**
1. Try local certified model first (fast, free, parallelizable)
2. Escalate to cloud provider if certified model fails or task is high-stakes
3. **DO NOT** use other local models (14b, llama*, mistral*) unless EXPLICITLY overridden by operator (ZB-023)

**Important:** The table above reflects **certified local model (qwen3.5:0.8b)** and cloud providers. "Local medium (2-4B)" models are **NOT CERTIFIED** for local CPU inference (ZB-023).

### CPU-First Assumptions

- **Hardware:** 14-core i9 with 64GB RAM (common in internal deployments)
- **Model:** Qwen 3.5 0.8B as **ONLY certified local model** (ZB-023)
- **Deployment:** Host Docker Ollama (http://host.k3d.internal:11434) - **ONLY supported local path**
- **Warmup:** 30-60s cold start, 3-5s per subsequent request
- **Latency:** 10-15 minutes for complex multi-step tasks (acceptable with parallelization)
- **Parallelism:** 24 workers × 4-6 tasks/hour = 96-144 tasks/hour total throughput

**Non-assumptions:**
- Not committed to one provider (Ollama, vLLM, OpenAI, etc. all valid)
- Any provider/model may serve any role if configured (outdated planner/worker split removed)
- Not assuming GPU availability (design works on CPU-only hosts)
- **ZB-023:** No assumption that other local models (14b, llama*, mistral*) are supported

### Calibration and Evaluation

#### Warmup Strategy
- Pre-load model before first task (30-60s one-time cost)
- Keep workers warm between tasks (maintain process state)
- Pool size scaled to match hardware (14 cores → 24 workers max)

#### Evaluation Harness
- **Task classes:** Planning, implementation, testing, documentation, review
- **Metrics:** Completion rate, tool-call accuracy, code correctness, time-to-completion
- **Baselines:** Human baseline for each task class (established via small sample set)
- **Thresholds:** Minimum accuracy/time-to-completion for model to stay in rotation

#### Provider/Model Capability Registry
- Store per-model calibration data (latency, throughput, accuracy by task class)
- Use for routing decisions (pick best model for task type)
- Update over time as models improve or new options emerge

### Prompt/Profile Tuning

#### Role-Based Profiles
Different roles require different prompt tuning:
- **Planner:** Emphasize step-by-step reasoning, tool selection
- **Implementer:** Emphasize code correctness, test coverage
- **Reviewer:** Emphasize clarity, bug detection, style consistency
- **Ops:** Emphasize safety, approval gates, rollback considerations

#### Temperature and Sampling
- Low temperature (0.1-0.3) for code generation and testing
- Medium temperature (0.5-0.7) for planning and documentation
- High temperature (0.8-1.0) for creative or exploratory tasks (rare in 1.0)

### Token/Yield Tracking

- **ZenLedger tracks:** Tokens used, cost, yield (tasks completed / tokens)
- **Metrics:** Tokens per task, tokens per hour, yield by task class
- **Goals:** Maximize useful tokens per hour, not minimize absolute token cost
- **Optimization:** Shift work to model class with best yield for task type

### Benchmark Suite

#### Planner Benchmarks
- Task: "Break down this Jira ticket into steps"
- Baseline: Human planner time and quality
- Metric: Steps correct, tools appropriate, no missing edge cases

#### Implementer Benchmarks
- Task: "Implement this feature with tests"
- Baseline: Human implementer time and code quality
- Metric: Code compiles, tests pass, no regressions, style compliant

#### Ops Benchmarks
- Task: "Approve this deployment change"
- Baseline: Human ops time and safety check quality
- Metric: Approved safely, no production issues, rollback plan documented

### Fallback and Escalation

#### Escalation Triggers
- Small model fails calibration thresholds (accuracy, time-to-completion)
- Task explicitly marked as "high-stakes" or "human approval required"
- Task involves external dependencies not in local model training data

#### Escalation Path
1. Try local small model (default)
2. Escalate to local medium model if small fails thresholds
3. Escalate to paid API if medium fails or task is high-stakes
4. Human intervention if all automated paths fail

#### Rollback
- Paid API results are cached and tagged for comparison
- Learn from paid API usage to improve small model prompts/calibration

## Consequences

### Positive
- **Low operational cost** – Local small models are effectively free (hardware already owned)
- **High reliability** – Parallelization compensates for individual slowness
- **Data privacy** – All inference happens on-premises
- **Provider flexibility** – Can switch models/providers without architecture changes
- **Continuous improvement** – Calibration data improves routing over time

### Negative
- **Higher latency per task** – Small models take longer (10-15 minutes for complex tasks)
- **Requires calibration** – Not "plug and play"; needs benchmarking and tuning
- **Bounded scope** – Small models cannot handle all task types well
- **Warmup cost** – 30-60s one-time cost per worker pool

### Neutral
- **Design remains open** – Can add GPU, paid APIs, or different models without breaking architecture
- **Parallelization is key** – CPU-only hosts rely on many workers, not one fast worker
- **Token yield matters** – Optimization target is useful tokens per hour, not raw token cost

## Alternatives Considered

### Alternative 1: Commit to paid API-first
- **Pros:** Low latency, high quality, no calibration needed
- **Cons:** High operational cost, data leaves premises, provider lock-in
- **Rejected:** 1.0 is internal force multiplier; paid APIs should be fallback

### Alternative 2: Commit to GPU-first with large models
- **Pros:** Low latency, high quality per request
- **Cons:** High hardware cost, not available everywhere, overkill for bounded tasks
- **Rejected:** CPU-only hosts are common; optimize for reality first

### Alternative 3: No calibration, just use best available model
- **Pros:** Simple, no overhead
- **Cons:** Unpredictable quality, cannot trust system blindly
- **Rejected:** Trusted operator requires predictable behavior and measurable improvement

## Related Decisions

- [ADR-0001](../01-ARCHITECTURE/ADR/0001_STRUCTURED_TAGS.md) – Structured tags define task classes for calibration
- [ADR-0007](../01-ARCHITECTURE/ADR/0007_QMD_FOR_KNOWLEDGE_BASE.md) – QMD for KB retrieval provides context for prompts
- [Bounded Orchestrator Loop](BOUNDED_ORCHESTRATOR_LOOP.md) – Escalation path integrates with orchestrator retry logic

## Future Work

### 1.1+
- Investigate fine-tuning/distillation for task-specific optimization
- Add more sophisticated routing (multi-model ensembles, early exit detection)
- Expand benchmark suite to cover more task types
- Auto-calibration based on historical results

### Research Lane
- Lightweight adapters for small models (LoRA, prefix tuning)
- Synthetic example generation for training data
- Role-specific dataset creation and curation
- ROI evaluation for fine-tuning vs paid API escalation

## 2026-03-26: llama.cpp codegen validation (external harness)

Independent **llama.cpp** runs (OpenAI-compatible API, **Qwen3.5-0.8B Q4_K_M** base GGUF) reproduced what L1 packet design assumes: **structured** autowork / quick-win style prompts (GOAL, files, success criteria, explicit OUTPUT contract) plus **thinking disabled** at the server and template level yield **compiling single-file Go** on bounded tasks when hints are specific (stdlib imports, correct JSON decode usage). A **LoRA-merged** GGUF evaluated under the same harness **did not** beat base on strict `go build` gates—output skewed toward fenced snippets and missing imports—indicating **adapter training data and objectives** must match **full-file, compile-clean** production outputs before LoRA can replace base locally.

**Details and operator knobs:** [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](../05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md).

## References

- Qwen 3.5 Model Cards: [https://huggingface.co/Qwen](https://huggingface.co/Qwen)
- Ollama Model Library: [https://ollama.com/library](https://ollama.com/library)
- ZenLedger Design: [ZEN_LEDGER.md](ZEN_LEDGER.md)
- Bounded Orchestrator Loop: [BOUNDED_ORCHESTRATOR_LOOP.md](BOUNDED_ORCHESTRATOR_LOOP.md)

## Phase 15 Updates (2026-03-25)

See [L1/L2 Lane Runbook](../05-OPERATIONS/L1_L2_LANE_RUNBOOK.md) for the operational procedure.
See [08B Positive Control Runbook](../05-OPERATIONS/08B_POSITIVE_CONTROL_RUNBOOK.md) for the test script.
See [MLQ Lane Routing Matrix](../05-OPERATIONS/MLQ_LANE_ROUTING_MATRIX.md) for routing decisions.

Key changes:
- L1 = 0.8B workhorse (bounded single-file tasks)
- L2 = 2B bounded (1-3 files, moderate adaptation)
- Default target-file context injection from ZEN_SOURCE_REPO for all tasks
- quickwin-l1.yaml template replaces heavy 4-phase rescue packets for L1
- MLQ rescue is NOT the default proof vehicle for 0.8B capability
- Run `scripts/run-08b-positive-control.sh` for preflight health check
