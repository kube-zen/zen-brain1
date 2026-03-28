# L1 Worker Scaling Experiment: WORKERS 5 → 7

**Date:** 2026-03-28
**Status:** EXPERIMENT COMPLETE

## Baseline Summary

See `runtime-throughput-baseline.md` for full baseline.

| Item | Value |
|------|-------|
| Machine | i9-13900H, 20 cores, 62GB RAM |
| Main L1 server | port 56227, `--parallel 10`, Qwen3.5-0.8B |
| Previous WORKERS | 5 |
| New WORKERS | 7 |

## Change Applied

Single change: `cmd/scheduler/main.go` line 224: `WORKERS=5` → `WORKERS=7`.

Scheduler binary rebuilt and restarted via systemd. No other changes.

## Observation Window

Two controlled runs of 3 identical tasks (dead_code, tech_debt, defects):

| Run | WORKERS | Start | End |
|-----|---------|-------|-----|
| baseline-5 | 5 | 15:04:01 | 15:05:20 |
| step-7 | 7 | 15:05:32 | 15:07:44 |

## Metric Comparison

| Metric | Baseline (W=5) | Step (W=7) |
|--------|---------------|------------|
| Tasks | 3 | 3 |
| Success | 3/3 | 3/3 |
| L1 produced | 3 (100%) | 3 (100%) |
| Timeouts | 0 | 0 |
| Avg latency | 41s | 99s |
| P50 latency | 25s | 125s |
| P95 latency | 79s | 132s |
| Batch wall | 79.2s | 132.0s |
| Validation | 2 success, 1 context-fail | 3 success |

**Batch wall delta:** +52.7s (+67% slower)

## Per-Task Breakdown

### Baseline (W=5) — tasks ran sequentially-ish
| Task | Wall | Class |
|------|------|-------|
| dead_code | 18.1s | fast-productive |
| defects | 24.6s | fast-productive |
| tech_debt | 79.2s | slow-but-productive |

### Step (W=7) — 3 concurrent on same server
| Task | Wall | Class |
|------|------|-------|
| defects | 39.2s | slow-but-productive |
| tech_debt | 124.9s | slow-but-productive |
| dead_code | 132.0s | slow-but-productive |

## Quality / Attribution Impact

- **L1-produced rate:** 100% in both runs — unchanged
- **Timeout rate:** 0% in both runs — unchanged
- **Truncation-repair:** 0 in both — no truncation
- **Validation:** Step W=7 actually better (3/3 vs 2/3 baseline — tech_debt had repetition in baseline)

## Crash During Experiment

The first WORKERS=7 attempt crashed the L1 server (EOF on all 3 connections). Cause: the machine had 56/62GB memory in use from 6+ idle llama.cpp instances. After the crash freed memory, the retry succeeded cleanly.

**Lesson:** Memory pressure from idle background servers is a real risk. Not a parallelism issue per se.

## Key Finding

**3 concurrent tasks on a single llama.cpp server cause individual latency to increase ~143% due to CPU contention.** The `--parallel 10` flag means the server accepts 10 connections, not that it processes them 10x faster. CPU-only inference shares the same cores.

Batch wall time went from 79.2s → 132.0s. Individual tasks went from 18-79s to 39-132s.

**However:**
- Quality held (100% L1 produced, 0 timeouts)
- Validation actually improved (3/3 vs 2/3)
- The latency increase is from CPU contention, not from broken output

## Recommendation

**Keep WORKERS=5 for 3-task batches. Consider 7 for larger batches only.**

Rationale:
1. For 3-task hourly-scans, W=5 is already sufficient (tasks complete in 55-144s total)
2. W=7 offers no throughput gain for batches ≤ 3 tasks — latency increases with no wall-time benefit
3. For daily-sweep with 10 tasks, W=7 *might* help by keeping the pipeline fuller. Worth testing separately.
4. The crash risk from memory pressure is real — don't increase parallelism blindly.

**Specific recommendation:**
- Revert scheduler to WORKERS=5 for now
- Run one daily-sweep (10 tasks) at WORKERS=7 to see if throughput improves with larger batches
- If daily-sweep at W=7 is faster, keep 7 only for daily-sweep schedules
- Do not jump to 8 until the daily-sweep test confirms headroom

## Follow-up Actions

1. Revert WORKERS to 5 in scheduler (safe default)
2. Add per-schedule WORKERS override (hourly-scan=5, daily-sweep=7)
3. Monitor memory pressure from idle llama instances
4. Consider shutting down unused llama servers to free memory
