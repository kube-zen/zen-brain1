# L1 Health and Failure Classification — Expansion Batch 1

**Date:** 2026-03-28 12:01 EDT

## Phase 1: L1 Health Status

| Check | Result |
|-------|--------|
| Process | PID 2746158, healthy, 70min CPU time |
| Endpoint | HTTP 200, responds in <1ms |
| Generation | ✅ Correct JSON in ~1s for simple prompt |
| Model | Qwen3.5-0.8B-Q4_K_M.gguf (752M params) |
| Parallel slots | 10 |
| Context size | 65536 |

**Decision: HEALTHY**

The L1 server is functional. Simple prompts complete in 1-2s. The issue is task-specific.

## Phase 2: Failure Classification — 10 Remaining Failures

After sequential retry, 10 of 20 tickets remain unresolved.

### By Failure Class

| Class | Count | Description |
|-------|-------|-------------|
| **l1-timeout** | **6** | 45-60s wall time, empty/unparseable response |
| **l1-truncated-json** | **4** | Correct JSON structure started but cut off mid-field |

### By Task Type

| Type | Total | Timeout | Truncated | Success |
|------|-------|---------|-----------|---------|
| config_change | 7 | 2 | 1 | **4** (57%) |
| code_edit | 7 | 4 | 0 | **3** (43%) |
| doc_update | 6 | 0 | 3 | **1** (17%) |

### Root Cause Analysis

**Timeout (6 tickets):** Tasks taking 45s+ produce empty responses. The L1 server becomes congested when processing complex prompts with large target files (parse.go, main.go). CPU-bound inference can't complete within 60s window.

**Truncated JSON (4 tickets):** L1 produces CORRECT output structure (edit_description, target_files, patch_commands) but the response is cut off mid-sed-command. The max_tokens budget (2048) should be sufficient — the truncation appears to be from the model hitting its effective generation limit for complex sed commands with escape sequences.

**Evidence:** ZB-887's raw output shows: `{"edit_description":"Add comment near TRANSITION_IDS...","target_files":["scripts/jira-drain.py"],"patch_commands":["sed -i '/TRANSITION_IDS/,/TR` — correctly structured JSON, just cut off.

## Decision Gate Application

**Combined results: 8/20 l1-produced (40%)**

| Gate | Threshold | Actual | Decision |
|------|-----------|--------|----------|
| A (continue) | ≥60% | 40% | ❌ |
| B (hold) | ≥50% | 40% | ❌ |
| C (stop) | <50% | 40% | **✅ TRIGGERED** |

**Gate C: Stop expansion, return to contract/runtime tuning.**

## Infrastructure Recommendations

1. **Reduce prompt complexity for doc_update tasks** — these have the worst success rate (17%). The target file content may be too large for 0.8b context.
2. **Shorten the curl timeout to 30s** — fail fast instead of waiting 60s for garbage. Tasks that can't complete in 30s won't produce useful output at 60s either.
3. **Consider `--ctx-size 32768`** instead of 65536 — smaller context = faster inference = fewer timeouts.
4. **Add JSON repair for truncated output** — since 4/10 failures produce correct JSON that's just cut off, a simple bracket-closer could recover these.
