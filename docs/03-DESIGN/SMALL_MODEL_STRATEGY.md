# Small-Model Strategy

## Status

**Draft** (2026-03-08)

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

**Zen-Brain 1.0 optimizes first for CPU-first local small-model operation**, with provider-agnostic design allowing future paths to larger models or paid APIs when justified.

### Core Principles

1. **Provider-agnostic** – System works with any LLM provider (Ollama, OpenAI, Anthropic, etc.)
2. **Small model as baseline, not hard dependency** – Qwen 0.8B is important reference, but design must support alternatives
3. **Yield per token matters** – Useful throughput > raw token cost
4. **Calibration by task class** – Benchmark against real work, not general chat quality
5. **Fallback/escalation path** – Route to larger models or paid APIs when small model cannot complete task
6. **Parallelization compensates for slowness** – 24 workers × 4-6 tasks/hour > 1 large model × 100 tasks/hour at lower cost

### When to Use Which Model

| Model Class | Use Case | Throughput (CPU 14-core) | Cost | Notes |
|------------|----------|---------------------------|------|-------|
| **Local small (0.8B)** | Bounded tasks: code completion, tool calling, doc updates, simple refactors | 96-144 tasks/hour | $0 | Requires warmup, calibration, role tuning |
| **Local medium (2-4B)** | Medium tasks: multi-step planning, code generation, design | 4-6 tasks/hour | $0 | May require GPU for acceptable latency |
| **Paid API (cloud)** | Complex/ambiguous tasks, high-stakes work, when small model fails | 60-100 tasks/hour | $2-5/hour | High reliability, low latency, higher cost |

**Routing rules:**
1. Try local small first (fast, cheap, parallelizable)
2. Escalate to local medium if small fails calibration thresholds
3. Escalate to paid API if medium fails or task is high-stakes

### CPU-First Assumptions

- **Hardware:** 14-core i9 with 64GB RAM (common in internal deployments)
- **Model:** Qwen 3.5 0.8B as reference implementation (OpenAI-compatible API)
- **Warmup:** 30-60s cold start, 3-5s per subsequent request
- **Latency:** 10-15 minutes for complex multi-step tasks (acceptable with parallelization)
- **Parallelism:** 24 workers × 4-6 tasks/hour = 96-144 tasks/hour total throughput

**Non-assumptions:**
- Not committed to one provider (Ollama, vLLM, OpenAI, etc. all valid)
- Not committed to one model family (Qwen, Llama, etc. all valid)
- Not assuming GPU availability (design works on CPU-only hosts)

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
- [Bounded Orchestrator Loop](../../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – Escalation path integrates with orchestrator retry logic

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

## References

- Qwen 3.5 Model Cards: [https://huggingface.co/Qwen](https://huggingface.co/Qwen)
- Ollama Model Library: [https://ollama.com/library](https://ollama.com/library)
- ZenLedger Design: [ZEN_LEDGER.md](ZEN_LEDGER.md)
- Bounded Orchestrator Loop: [BOUNDED_ORCHESTRATOR_LOOP.md](BOUNDED_ORCHESTRATOR_LOOP.md)