> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



## Date: 2026-03-25

## MLQ Gap Closed

### Before (P1 baseline)
- MLQ selection (`internal/mlq/selector.go`): ✅ Works — SelectLevel() correctly routes tasks by task_class
- MLQ policy (`config/policy/mlq-levels.yaml`): ✅ Present — L1/L2/L0 config, escalation rules, selection policy
- Runtime escalation: ❌ NOT enforced — `bounded_executor.go` has step-level retry only; no task-level retry/escalation consuming `escalation_rules`
- Real L1 concurrency: ❌ NOT implemented — `max_workers: 10` was declarative; one endpoint, no worker pool

### After (P22 implementation)
- Task-level retry/escalation: ✅ `internal/mlq/task_executor.go` — TaskExecutor.ExecuteWithRetry()
- Worker pool: ✅ `internal/mlq/task_executor.go` — WorkerPool with round-robin, busy/free
- Telemetry: ✅ `internal/mlq/task_executor.go` — TaskTelemetry per attempt, LogTelemetrySink
- Real 10-way concurrency: ✅ llama.cpp `--parallel 10` proven (10 tasks in 74s = parallel)
- Live 10-task batch: ✅ 10/10 dispatched, 10/10 succeeded on L1

## Concurrency Proof

```
All 10 tasks started within 0.3ms of each other (19:57:20.230xxx)
All 10 tasks completed within 1.7s of each other (19:58:32 - 19:58:34)
Total wall time: 74.4s (equivalent to single task, not 740s serial)
```

Standalone proof script: `/tmp/test_llama_parallel.py`
- 10 concurrent requests → 3.4s wall time (same as single request)

## Escalation Status

- Code implemented: ✅ TaskExecutor checks escalation_rules from config
- Escalation rule: L1→L2 after 2 failures (retry_count trigger)
- Fallback rule: L1→L0 on timeout/error
- Live evidence: Not triggered this run (all 10 tasks succeeded on L1 first attempt)
- Test coverage: TestTaskExecutorRetryAndEscalation verifies L1 fail ×2 → L2 success

## Known Issues

1. stub_hunting (mlq-006) produced empty artifact — model used reasoning_content instead of content
   - 0.8B thinking mode issue; model produced 439 thinking tokens but empty visible content
   - Fix: Add `chat_template_kwargs: {"enable_thinking": false}` to request or strip thinking
   - This is exactly the PHASE 18 finding about enable_thinking: false

2. duration_ms telemetry field has nanosecond precision bug (should be milliseconds)

## Commits

- `7c9d5d5` feat(mlq): add task-level retry, escalation, worker pool, and telemetry
- `cc51db7` feat(mlq): add L1 10-slot worker pool and task dispatcher
- `e2a8836` fix(mlq-dispatcher): escape quotes in stub_hunting prompt string

## Worker Endpoints

| Level | Endpoint | Model | Status |
|-------|----------|-------|--------|
| L1 | http://localhost:56227 | Qwen3.5-0.8B-Q4_K_M.gguf | ✅ Running, 10 parallel slots |
| L2 | http://localhost:60509 | zen-go-q4_k_m.gguf | ✅ Running |
| L0 | http://localhost:11434 | qwen3.5:0.8b (Ollama) | ✅ Running |

## Artifacts

All in `/tmp/zen-brain1-mlq-run/`:

| File | Size | Lines |
|------|------|-------|
| final/dead-code-report.md | 852B | 23 |
| final/defects-report.md | 1.6KB | 44 |
| final/tech-debt-report.md | 958B | 38 |
| final/roadmap-report.md | 1.2KB | 32 |
| final/bug-hunting-guide.md | 1.1KB | 56 |
| final/stub-hunting-guide.md | 0B | 0 ⚠️ empty (thinking mode) |
| final/package-hotspot-guide.md | 1.5KB | 71 |
| final/test-gap-analysis.md | 1.2KB | 51 |
| final/config-drift-guide.md | 1.4KB | 63 |
| final/executive-summary.md | 1.1KB | 47 |
| final/concurrency-report.md | 1.5KB | 26 |
| telemetry/batch-telemetry.json | — | — |
| logs/dispatch.log | — | — |
