> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



**Date:** 2026-03-25
**Baseline SHA:** `2cb1faa`
**Final SHA:** See commits below

## Summary

PHASE 23 closes the last runtime gap: factory now uses TaskExecutor for real retry/escalation, llama.cpp thinking is disabled by default, and L1→L2 escalation is proven end-to-end.

## 1. Factory Integration

**Status:** ✅ Complete

Factory.executeWithLLM() now routes through TaskExecutor.ExecuteWithRetry() when MLQ is enabled.

**Code path:**
```
executeWithLLM() → executeWithLLMRetry() → TaskExecutor.ExecuteWithRetry()
  → SelectLevel() → createGeneratorForLevel() → runLLMTemplate()
  → On failure: retry (max_retries from escalation_rules)
  → After repeated failure: escalate to L2
  → On provider outage: fallback to L0
  → Telemetry logged: task_id, class, level, retries, escalated, result
```

**Files changed:**
- `internal/factory/factory.go` — added taskExecutor field, EnableMLQ() creates pools, refactored executeWithLLM()
- `internal/mlq/task_executor.go` — TaskExecutor with ExecuteWithRetry(), WorkerPool, TaskTelemetry
- `internal/mlq/selector.go` — added GetLevelOrNil() helper
- `internal/factory/intelligence_selection.go` — fixed broken import
- `internal/intelligence/empirical_router.go` — simplified, removed type conflicts

## 2. No-Think Status

**Status:** ✅ Complete

llama.cpp requests now include `chat_template_kwargs: {enable_thinking: false}` when Thinking=false.

**Evidence:**
- stub_hunting artifact: 0 bytes → 1826 bytes after fix
- All 10 regression tasks produce real content (1.3-1.9KB each)
- No empty artifacts in regression batch

**Files changed:**
- `internal/llm/openai_compatible_provider.go` — added ChatTemplateKwargs field
- `cmd/mlq-dispatcher/main.go` — added enable_thinking: false to request body

## 3. Escalation Proof

**Status:** ✅ Proven end-to-end

**Test:** Controlled failure task (IMPOSSIBLE_PREFIX_XYZ123 prompt)
- L1 attempt 1: FAILED (empty response, 714ms)
- L1 attempt 2: FAILED (empty response, 612ms)
- ESCALATION: level 1 → level 2 after 2 failures
- L2 attempt 1: SUCCEEDED (983ms)
- Total: 3 attempts, escalated=true, final=success, 2.3s

**Logs:**
```
[MLQ] Task escalation-test-001 ESCALATING from level 1 to level 2 after 2 failures
[MLQ] Task escalation-test-001 SUCCEEDED on level 2 worker mlq-level-2-w1
[MLQ-Telemetry] task_id=escalation-test-001 ... escalated=true
```

**Artifact:** `/tmp/zen-brain1-mlq-run/final/escalation-test-artifact.md`
**Logs:** `/tmp/zen-brain1-mlq-run/logs/escalation-test.log`

## 4. Regression Batch

**Status:** ✅ No regression

10 useful reporting tasks re-run after all P23 changes:
- 10/10 succeeded on L1
- 71s wall time (parallel, not serial)
- All artifacts 1.3-1.9KB (no empty files)
- stub_hunting fixed (1826 bytes, was 0)

## 5. Commits Pushed

| SHA | Message |
|-----|---------|
| `c2555be` | feat(mlq): route factory execution through task executor retry and escalation |
| `47e8ed0` | feat(llama-cpp): disable thinking by default for useful mlq tasks |
| `70a6355` | test(mlq): prove end-to-end escalation from L1 to L2 |
| (pending) | docs(mlq): lock retry escalation and no-think runtime behavior |

## 6. Worker Endpoints

| Level | Endpoint | Model | Status |
|-------|----------|-------|--------|
| L1 | http://localhost:56227 | Qwen3.5-0.8B-Q4_K_M.gguf | ✅ Running, 10 parallel slots |
| L2 | http://localhost:60509 | zen-go-q4_k_m.gguf | ✅ Running |
| L0 | http://localhost:11434 | qwen3.5:0.8b (Ollama) | ✅ Running |

## 7. Artifact Paths

- `/tmp/zen-brain1-mlq-run/final/*.md` — 10 report artifacts + escalation test artifact
- `/tmp/zen-brain1-mlq-run/logs/dispatch.log` — regression batch log
- `/tmp/zen-brain1-mlq-run/logs/escalation-test.log` — escalation test log
- `/tmp/zen-brain1-mlq-run/telemetry/batch-telemetry.json` — batch telemetry
