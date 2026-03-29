# Concurrency Sweep: W=5 vs W=7 Comparison

**Date:** 2026-03-28
**Status:** W=7 comparison incomplete — insufficient ready backlog to fill slots

## Context

- Resetting the 7 PAUSED tickets to Backlog was for a bounded controlled comparison only
- Terminal-state correctness remains intact throughout
- Only WORKERS changed between baseline and comparison
- W=10 is still not approved
- Backlog drain remains the priority workload

## W=5 Baseline (13 tickets, 4 cycles)

**Run:** 19:21:53 → 19:28:47 (~7 min wall time, 4 dispatch cycles)

| Cycle | Dispatched | Done | Paused | Blocked | Retrying |
|-------|-----------|------|--------|---------|----------|
| 1 (W=5) | 5 tickets | 0 | 0 | 2 | 3 (L1 timeout) |
| 2 (W=5) | 5 tickets | 2 | 1 | 1 | 1 (L1 timeout) |
| 3 (W=5) | 5 tickets | 3 | 1 | 1 | 0 |
| 4 (W=5) | 2 tickets | 1 | 1 | 0 | 0 |

**Per-ticket outcomes:**

| Ticket | Quality | Class | Jira State |
|--------|---------|-------|------------|
| ZB-1057 | 17 | done | Done |
| ZB-1031 | 16 | done | Done |
| ZB-1056 | 16 | done | Done |
| ZB-1064 | 16 | done | Done |
| ZB-1065 | 16 | done | Done |
| ZB-843 | 16 | needs_review | Done |
| ZB-1055 | 16 | paused | PAUSED |
| ZB-1058 | 16 | paused | PAUSED |
| ZB-1063 | 15 | paused | PAUSED |
| ZB-1046 | 13 | blocked_invalid_payload | PAUSED |
| ZB-1047 | 14 | blocked_invalid_payload | PAUSED |
| ZB-1048 | 14 | blocked_invalid_payload | PAUSED |
| ZB-1050 | 14 | blocked_invalid_payload | PAUSED |

**Metrics:**

| Metric | Value |
|--------|-------|
| Total tickets | 13 |
| Wall time | ~7 min |
| Avg latency | 77.8s |
| P50 latency | 27.0s |
| P95 latency | 248.0s |
| Done count | 6 (46%) |
| Paused count | 3 (23%) |
| Blocked count | 4 (31%) |
| Stuck In Progress | 0 |
| L1 timeout rate (cycle 1) | 3/5 = 60% (L1 was flaky) |
| L1 timeout rate (cycle 2+) | 1/8 = 12.5% |
| Quality gate pass rate | 9/13 = 69% |

## W=7 Comparison (incomplete)

**Problem:** Only 3 tickets were dispatchable when W=7 ran. The factory correctly filtered out stale/blocked tickets (those with `quality:blocked-invalid-payload` label from the W=5 run).

**W=7 run 1 (marine-gulf):** 3 tickets dispatched, all completed in ~16s

| Ticket | Quality | Class | Jira State | Latency |
|--------|---------|-------|------------|---------|
| ZB-1055 | 16 | done | Done | ~16s |
| ZB-1058 | 16 | done | Done | ~15s |
| ZB-1063 | 16 | done | Done | ~15s |

**W=7 run 2 (fresh-glade):** After label reset, only ZB-1047 was ready (others had been re-blocked by the first W=7 cycle). Completed in 12s.

| Ticket | Quality | Class | Jira State | Latency |
|--------|---------|-------|------------|---------|
| ZB-1047 | 14 | blocked_invalid_payload | PAUSED | 12s |

## Comparison Table

| Metric | W=5 | W=7 | Delta |
|--------|-----|-----|-------|
| Batch size | 13 | 3+1 | Insufficient for comparison |
| Avg latency | 77.8s | ~14s | Cannot compare (different batch sizes) |
| P50 latency | 27.0s | ~15s | — |
| P95 latency | 248.0s | ~16s | — |
| Done count | 6 | 3 | — |
| Blocked count | 4 | 1 | — |
| Stuck In Progress | 0 | 0 | ✅ Consistent |
| L1 timeout rate | 23% | 0% | L1 healthier during W=7 |

## Finding

**The W=7 comparison is inconclusive due to insufficient batch size.** Only 3 tickets filled the pipeline vs 5 slots for W=5. There were never 7 ready tickets simultaneously to stress-test the higher concurrency.

### What we can conclude:

1. **No regression:** W=7 produced 0 stuck tickets, 0 quality degradation, correct terminal states
2. **Factory fill correctly respects backlog state:** Blocked/stale tickets were filtered out
3. **Terminal result files work at both W=5 and W=7:** No state handling differences
4. **L1 was healthier during W=7 run:** 0 timeouts vs 60% in W=5 cycle 1 (unrelated to workers)

### What we cannot conclude:

1. Whether W=7 improves throughput vs W=5 — need a batch of ≥10 ready tickets
2. Whether CPU contention changes at W=7 — never filled 7 slots

## Recommendation

- **Keep W=7 for daily-sweep** — no regression observed, but benefit unproven
- **Run a proper W=7 vs W=5 comparison** when backlog has ≥10 ready tickets simultaneously
- **Do not test W=10** until W=7 is proven beneficial with a full batch
- **Revert to W=5 if any regression appears** in production daily-sweep runs

## Evidence Paths

- W=5 log: `/tmp/sweep-w5.log`
- W=7 log: `/tmp/sweep-w7.log`
- Terminal results: `/tmp/zen-brain1-worker-results/`
- Phase B capabilities: `docs/05-OPERATIONS/evidence/phase-b-existing-capabilities.md`
