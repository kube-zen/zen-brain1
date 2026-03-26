# Small-Model Strategy

## Status

**Production** (2026-03-26, PHASE 26)

## PHASE 26 Update: Certified Runtime Locked

As of PHASE 26 (2026-03-26), the following runtime configuration is **proven and locked**:

### Certified Local CPU Lanes

| Lane | Model | Inference | Port | Role |
|------|-------|-----------|------|------|
| **L1** | Qwen3.5-0.8B Q4_K_M | llama.cpp | 56227 | Default useful lane — all regular tasks enter here |
| **L2** | Qwen3.5-2B Q4_K_M (zen-go) | llama.cpp | 60509 | Stronger bounded lane — earned by repeated L1 failure evidence |
| **L0** | qwen3.5:0.8b | Ollama | 11434 | Fallback only — NOT for regular work |

### Proven Runtime Behaviors (PHASE 22–24C)

1. **Factory routes through TaskExecutor** — all MLQ tasks use `ExecuteWithRetry()` for automatic retry/escalation
2. **L1 first, always** — regular tasks go to L1 by default; L2 is earned by repeated L1 failure evidence
3. **Thinking disabled** — llama.cpp requests include `enable_thinking: false` to prevent empty artifacts
4. **10-way L1 parallelism** — single llama.cpp server with `--parallel 10`, proven with concurrent batch
5. **Escalation proven** — L1→L2 escalation after 2 failures verified end-to-end (see P004 test)
6. **Usefulness as benchmark** — useful reporting/triage artifacts are the production go/no-go criterion

### Evidence
- 10 useful reporting tasks: 10/10 L1 success, 71s parallel wall time (PHASE 22)
- Escalation test: L1×2 failures → L2 success in 2.3s (PHASE 23)
- No-think fix: stub_hunting artifact went from 0 bytes to 1.8KB (PHASE 23)
- 7/10 usefulness tasks produced valid markdown artifacts through direct L1 path (PHASE 24C)
- Tool-definition wrapper causes empty 0.8B responses — direct HTTP path works (PHASE 24C)

### Important: Task-Shape, Not Model Capability

Standalone Go codegen failures from small models are **narrow task-shape evidence**, not broad model-family verdicts. The same 0.8B model:
- ✅ Produces valid markdown reports for dead-code, defects, tech-debt analysis
- ✅ Can parse semver, implement validation methods with existing code context
- ❌ Cannot generate standalone Go from arbitrary prompts without grounding

## 🚨 POLICY (ZB-023) - CANONICAL SOURCE OF TRUTH

**LAST UPDATED: 2026-03-26 (PHASE 26)**

### Certified Local CPU Path

- ✅ **L1 default lane:** `qwen3.5:0.8b` via **llama.cpp** (port 56227, 10 parallel slots)
- ✅ **L2 stronger lane:** `qwen3.5:2b` via **llama.cpp** (port 60509, 4 slots)
- ✅ **L0 fallback:** `qwen3.5:0.8b` via **Ollama** (port 11434, FAIL-CLOSED warning-only)
- ❌ **FORBIDDEN:** Using L0/Ollama for regular work when L1/L2 are available
- ❌ **FORBIDDEN:** Any other local model (e.g., qwen3.5:14b, llama*, mistral*) unless explicitly overridden

### Provider/Model Flexibility

- Any provider/model may serve any role if configured
- No planner/worker role restriction — `qwen3.5:0.8b` can serve any lane
- GLM is not planner-only by architecture

### Routing Policy

1. **Every regular useful task goes to L1 first** (llama.cpp 0.8B)
2. **Retries happen in L1** (up to 2 per MLQ config)
3. **Repeated failures escalate to L2** (llama.cpp 2B)
4. **L0/Ollama is fallback only** (runtime outage, not default path)

### Enforcement

- Policy: `config/policy/mlq-levels.yaml`, `config/policy/mlq-levels-local.yaml`
- Runtime: `internal/mlq/task_executor.go` (retry/escalation logic)
- Documentation: This document (canonical source of truth)

### Related Documentation

- **Operational Guide:** [L1/L2 Lane Runbook](../05-OPERATIONS/L1_L2_LANE_RUNBOOK.md)
- **Escalation design:** [LOCAL_LLM_ESCALATION_LADDER.md](./LOCAL_LLM_ESCALATION_LADDER.md)
- **2B vs 0.8B local evaluation (llama.cpp Q4_K_M):** [QWEN_2B_LOCAL_EVALUATION.md](../05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md)
- **0.8B llama.cpp vs Ollama benchmark:** [LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md](../05-OPERATIONS/LLAMA_CPP_VS_OLLAMA_QWEN_0.8B_BENCHMARK.md)
- **0.8B llama.cpp codegen guide:** [QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md](../05-OPERATIONS/QWEN_08B_LLAMA_CPP_CODEGEN_GUIDE.md)
- **Go subtask llama.cpp harness (0.8B / 2B):** [GO_SUBTASK_LLAMA_CPP_HARNESS.md](../05-OPERATIONS/GO_SUBTASK_LLAMA_CPP_HARNESS.md)
- **Local Go LoRA tuning postmortem:** [QWEN_08B_LORA_TUNING_POSTMORTEM.md](../05-OPERATIONS/QWEN_08B_LORA_TUNING_POSTMORTEM.md)

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

1. **Provider-agnostic** – System works with any LLM provider (llama.cpp, Ollama, OpenAI, etc.)
2. **llama.cpp as primary local runtime** – L1 and L2 use llama.cpp for reliable, parallel inference
3. **L1 first, always** – every task enters L1 (0.8B); escalation is earned by failure evidence
4. **Yield per token matters** – Useful throughput > raw token cost
5. **Calibration by task class** – Benchmark against real work, not general chat quality
6. **Usefulness as benchmark** – reporting/triage artifacts are the production criterion, not standalone codegen
7. **Parallelization compensates for latency** – 10 parallel L1 slots provide high aggregate throughput

### When to Use Which Lane

| Lane | Model | Inference | Use Case | Concurrency | Notes |
|------|-------|-----------|----------|-------------|-------|
| **L1** | Qwen 3.5 0.8B Q4_K_M | llama.cpp | All regular useful tasks: reporting, triage, evidence gathering, bounded edits | 10 parallel slots | Default lane — every task starts here |
| **L2** | Qwen 3.5 2B Q4_K_M | llama.cpp | L1 failures after retry: moderate adaptation, deeper analysis | 1–2 concurrent | Earned by repeated L1 failure evidence |
| **L0** | Qwen 3.5 0.8B | Ollama | Fallback only: runtime outage when L1+L2 unavailable | 1 | FAIL-CLOSED (warning-only, not default) |
| **Cloud** | GLM/OpenAI/etc. | API | High-stakes or complex tasks exceeding local capability | Pay-per-use | Reserved for when all local lanes exhausted |

**Routing rules:**
1. Every regular useful task goes to L1 first (fast, free, parallelizable)
2. Retries happen in L1 (up to 2 per MLQ config)
3. Repeated failures escalate to L2
4. L0/Ollama is fallback only — NOT for regular work
5. **DO NOT** use other local models (14b, llama*, mistral*) unless EXPLICITLY overridden by operator (ZB-023)

**Production success criterion:**
zen-brain1 is "working" when it continuously produces useful artifacts through the real runtime on regular tasks — dead-code reports, defect scans, tech-debt summaries, executive rollups. Standalone codegen failures are narrow task-shape evidence, NOT a go/no-go criterion for 24/7 operations.

### CPU-First Assumptions

- **Hardware:** 14-core i9 with 64GB RAM (common in internal deployments)
- **L1 Model:** Qwen 3.5 0.8B Q4_K_M via **llama.cpp** (port 56227, `--parallel 10 --ctx-size 65536`)
- **L2 Model:** Qwen 3.5 2B Q4_K_M via **llama.cpp** (port 60509, `--ctx-size 16384`)
- **L0 Fallback:** Qwen 3.5 0.8B via **Ollama** (port 11434, FAIL-CLOSED)
- **Warmup:** 30-60s cold start, 3-5s per subsequent request
- **Latency:** 30-90s per useful reporting task (acceptable with 10-way parallelism)
- **Parallelism:** 10 L1 slots × ~4-6 useful tasks/hour = 40-60 tasks/hour

**Non-assumptions:**
- Not committed to one provider (llama.cpp, Ollama, vLLM, OpenAI all valid)
- Any provider/model may serve any role if configured
- Not assuming GPU availability (design works on CPU-only hosts)
- Not using Ollama as primary path (llama.cpp is the certified inference runtime)

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
