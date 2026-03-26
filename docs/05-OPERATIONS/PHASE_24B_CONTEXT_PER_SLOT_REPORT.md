# PHASE 24B — Context-Per-Slot Execution Report

**Date:** 2026-03-26 (updated)  
**Baseline:** HEAD 3459520

---

## Context-Per-Slot Finding

**Per-slot context budget, not provider selection, was the blocker.**
llama.cpp divides total `n_ctx` across `--parallel N` slots. The `n_ctx_seq` (per-slot) value is what matters for fitting prompts. PHASE 24 foreman failures were caused by insufficient per-slot context for the real foreman prompt + tool payload, not by MLQ misrouting or factory bypass.

### Per-slot formula

```
requested:  --ctx-size X --parallel N
observed:   n_ctx_seq = floor(X / N), rounded up by llama.cpp to alignment
```

**Key nuance**: llama.cpp rounds the per-slot value up to an alignment boundary, so the observed `n_ctx_seq` may be slightly higher than naive `X / N`.

### Observed sizing table (all tested on this host)

| `--ctx-size` | `--parallel` | Theoretical per-slot | Observed `n_ctx_seq` | First prompt (~1038 tok) | Retry prompt (~3384 tok) | Fits foreman? |
|-------------|-------------|---------------------|----------------------|-------------------------|-------------------------|--------------|
| 8192 | 10 | 819 | **1024** | ❌ (1038 > 1024) | N/A | NO |
| 32768 | 10 | 3276 | **3328** | ✅ | ❌ (3384 > 3328) | RETRY ONLY |
| 65536 | 10 | 6553 | **6656** | ✅ | ✅ | **YES** |

- Values are from llama.cpp `/slots` API (`n_ctx` field per slot)
- First prompt: ~1038 tokens (system + 5 tool definitions + task description)
- Retry prompt: ~3384 tokens (first prompt + prior generated code loaded as context)

### Llama.cpp Server Startup Command (Current)

```bash
/home/neves/git/llama.cpp/build/bin/llama-server \
  -m /home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf \
  --port 56227 --host 0.0.0.0 \
  --parallel 10 --ctx-size 65536
```

- `n_ctx`: 65536 requested → 66560 actual (6656 × 10)
- `n_ctx_seq`: 6656 tokens per slot

---

## Single Foreman Task Status

**Result: PARTIAL PASS** — L1 execution confirmed working; code quality insufficient for 0.8B.

### Evidence (f24a-single)

```
[MLQ] Selected Level 1 for task f24a-single (reason=task_class)
[MLQ] Task f24a-single starting on level 1, max_retries=2
[MLQ] Task f24a-single attempt 1: level=1 worker=mlq-level-1-w1 endpoint=http://localhost:56227
[Factory] LLM attempt: provider=llama-cpp model=qwen3.5:0.8b-q4 url=http://localhost:56227
[llama-cpp] Thinking disabled via chat_template_kwargs for useful-task request
[LLMTemplate] Generated implementation (model=qwen3.5:0.8b-q4, tokens=0)
[LLMTemplate] code_length=1424 chars
```

- TaskExecutor and no-think are **proven active** in the real foreman path
- L1 generated code (1424 chars, 22s)
- `go build` failed: 0.8B code quality insufficient (expected)

---

## Foreman Escalation Status

**Result: PASS** — L1→L2 escalation through real foreman path proven.

### Evidence

```
[MLQ] Task escalation-test-24b attempt 1: level=1 → FAIL (go build)
[MLQ] Task escalation-test-24b attempt 2: level=1 → FAIL (go build)
[MLQ] Task escalation-test-24b ESCALATING from level 1 to level 2 after 2 failures
[MLQ] Task escalation-test-24b attempt 1: level=2 worker=mlq-level-2-w1 → L2 reached ✅
```

Escalation decision is correct. L2 content failures are separate (server availability + model quality).

---

## Hidden .go Filename Bug

**Status: FIXED** — `determineTargetPath()` in `internal/factory/llm_templates.go` produced hidden files when `WorkItemID` was empty (slug = "" → filename = `.go`).

**Root cause**: `brainTaskToFactorySpec` maps `task.Spec.WorkItemID` → `spec.WorkItemID`, but when WorkItemID is unset (common in test harnesses), the slug was empty.

**Fix**: Fall back to `spec.ID` (task name) when `spec.WorkItemID` is empty.

---

## Stable L1 Config

**Chosen: `--parallel 10 --ctx-size 65536`** (6656 tokens per slot)

---

## Batch Validation Status

Pending re-run after template fix and L2 recovery. See next section.

---

## Artifacts

- Foreman single test log: `/tmp/p24a-single-test.log`
- Escalation test log: `/tmp/p24b-escalation-test.log`
- Batch test log: `/tmp/p24b-batch-test.log`
- L1 server log: `/tmp/llama-l1.log`

## Test Harnesses

- `cmd/p24-foreman-run/main.go` — 10-task batch through foreman
- `cmd/p24a-single-test/main.go` — single task through foreman
- `cmd/p24b-escalation-test/main.go` — escalation proof through foreman
- `cmd/p24b-batch-test/main.go` — 3-task parallel batch through foreman
