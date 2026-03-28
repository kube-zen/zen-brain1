# Controlled Expansion Scoreboard — Batch 1

**Date:** 2026-03-28
**Contract:** v2 patch-oriented (max_tokens=2048, timeout=60s, patch_commands only)
**Batch size:** 20 tickets

## Combined Results (Original + Sequential Retry)

The original parallel run hit 30% (6/20) due to L1 server overload. A sequential retry improved the overall rate.

### By Source

| Run | Mode | Tickets | l1-produced | l1-failed | Rate |
|-----|------|---------|-------------|-----------|------|
| Original | parallel | 20 | 6 | 14 | 30% |
| Retry | sequential | 14 | 7 | 7 | 50% |
| **Combined unique** | **mixed** | **20** | **8** | **12** | **40%** |

### Full Ticket Table

| Jira Key | Initial State | Type | Lane | Time | Score | Validation | Produced By | Supervisor | Final State | Retry | Notes |
|----------|--------------|------|------|------|-------|------------|-------------|------------|-------------|-------|-------|
| ZB-883 | RETRYING | code_edit | l1 | 13.4s | 25/25 | ✅ | l1 | none | Done | yes | Retry succeeded |
| ZB-884 | RETRYING | config | l1 | 19.3s | 0/25 | ❌ | l1-needs-review | quality_gate | RETRYING | yes | Parsed but low score |
| ZB-885 | RETRYING | config | l1 | 15.2s | 25/25 | ✅ | l1 | none | Done | yes | Retry succeeded |
| ZB-886 | RETRYING | config | l1 | 12.6s | 20/25 | ✅ | l1 | none | Done | yes | Retry succeeded |
| ZB-887 | RETRYING | doc | l1 | 11.1s | 0/25 | ❌ | l1-needs-review | quality_gate | RETRYING | yes | Parsed but low score |
| ZB-888 | RETRYING | config | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | RETRYING | yes | Slow + low score |
| ZB-889 | RETRYING | doc | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | RETRYING | yes | Slow + low score |
| ZB-901 | Backlog | config | l1 | 15.2s | 0/25 | ❌ | l1-needs-review | quality_gate | PAUSED | no | |
| ZB-903 | Backlog | code_edit | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | PAUSED | no | |
| ZB-907 | Backlog | config | l1 | 8.7s | 25/25 | ✅ | l1 | none | Done | no | |
| ZB-909 | Backlog | code_edit | l1 | 10.5s | 25/25 | ✅ | l1 | none | Done | no | |
| ZB-910 | Backlog | doc | l1 | 10.7s | 0/25 | ❌ | l1-needs-review | quality_gate | PAUSED | no | |
| ZB-912 | Backlog | config | l1 | 10.2s | 20/25 | ✅ | l1 | none | Done | no | |
| ZB-916 | Backlog | code_edit | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | PAUSED | no | |
| ZB-925 | Backlog | config | l1 | 8.2s | 20/25 | ✅ | l1 | none | Done | no | |
| ZB-926 | Backlog | config | l1 | 8.4s | 25/25 | ✅ | l1 | none | Done | no | |
| ZB-927 | Backlog | config | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | PAUSED | no | |
| ZB-928 | Backlog | code_edit | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | PAUSED | no | |
| ZB-929 | Backlog | doc | l1 | 14.3s | 0/25 | ❌ | l1-needs-review | quality_gate | PAUSED | no | |
| ZB-930 | Backlog | code_edit | l1 | 45.0s | 0/25 | ❌ | l1-needs-review | timeout | PAUSED | no | |

## Attribution Summary

| Disposition | Count | % |
|-------------|-------|---|
| **l1-produced** | **8** | **40%** |
| l1-produced-needs-review | 12 | 60% |
| supervisor-written | 0 | 0% |
| script-only | 0 | 0% |
| failed | 0 | 0% |

## Pattern Analysis

### What Works (8/20 → 40%)
- **config_change tasks under 15s**: 5/7 succeeded (71%)
- **Simple code_edit tasks under 15s**: 3/6 succeeded (50%) — ZB-883, ZB-909
- Tasks completing in <15s score 20-25/25 on quality gate

### What Fails (12/20 → 60%)
- **Tasks taking 45s+**: 0/7 succeeded — all hit timeout or produced garbage
- **doc_update on large targets**: 0/4 succeeded — L1 struggles with documentation scope
- **Code_edit on complex files** (parse.go, main.go security): 0/5 at 45s+

### Root Cause
The L1 server (single llama.cpp instance, CPU) cannot reliably complete requests in under 60s for anything beyond simple config changes. The 45s+ requests all produce empty or unparseable output.

## Expansion Decision Gate

**Result: Gate C — L1-produced below 50%**

The 40% rate is below the 50% hold threshold and well below the 60% expansion threshold.

**Action required:**
1. Stop expansion
2. Investigate why ~60% of requests take 45s+ (server saturation? context too large?)
3. Consider reducing the system prompt size or task complexity for slower tasks
4. Do NOT add capacity until the rate improves

## Evidence Paths

- Original batch: `docs/05-OPERATIONS/evidence/l1-expansion-batch1/`
- Sequential retry: `docs/05-OPERATIONS/evidence/l1-expansion-batch1-retry/`
- Scoreboards: `expansion-scoreboard.json`, `expansion-retry-scoreboard.json`
