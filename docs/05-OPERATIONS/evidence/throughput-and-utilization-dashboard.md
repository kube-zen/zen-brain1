# Throughput and Utilization Dashboard

**Updated:** 2026-03-28 13:25 EDT  
**Machine:** i9-13900H 20-core, 64GB RAM  
**L1 Model:** Qwen3.5-0.8B-Q4_K_M.gguf (single llama.cpp, port 56227, --parallel 10)

## Current State

| Metric | Value |
|--------|-------|
| L1 workers active | 1 (sequential proven best) |
| L1-produced rate (corrective retry) | **100%** (10/10) |
| L1-produced rate (full expansion) | **90%** (18/20) |
| L1-produced rate (scheduler 24h) | **94.9%** (166/175) |
| Jira Done count | 613+ |
| Jira Backlog | 28 |
| Done/day (24h run data) | ~175 |
| Tasks/hour (scheduler) | 7.3 |
| Machine utilization during runs | ~4% (0.8h wall time / 24h) |
| Truncation repair rate | 40-50% of slow tasks |

## Per-Task Telemetry Status

| Feature | Status |
|---------|--------|
| Per-task JSONL recording | ✅ Implemented (per-task.jsonl) |
| Completion classification | ✅ fast-productive / slow-but-productive / timeout / parse-fail / validation-fail |
| Attribution tracking | ✅ l1 / l1-partial / l1-failed / supervisor |
| Quality gate integration | ✅ Quality score recorded per task |
| Repair tracking | ✅ repair_used + repair_succeeded |
| Metrics CLI tool | ✅ cmd/metrics-report |
| Per-model comparison | ⏳ Needs multi-model data |
| Worker utilization tracking | ⏳ Next: parallelism experiment |
| Real-time dashboard | ⏳ Planned |

## Latency Distribution (Existing Data)

### By Schedule Type (24h)

| Schedule | Avg Wall | Tasks/Run | Success Rate |
|----------|----------|-----------|-------------|
| hourly-scan | 70s | 2 | 95% |
| daily-sweep | 120s | 9 | 93% |
| quad-hourly-summary | 11s | 6 | 97% |

### By Completion Class (Benchmark)

| Class | Description |
|-------|-------------|
| fast-productive | <30s, parseable, good quality |
| slow-but-productive | >30s, parseable, good quality |
| truncated-repaired | Output truncated, bracket-repair saved it |
| timeout | No usable output |
| parse-fail | Output not parseable as JSON |
| validation-fail | Parsed but quality gate rejected |

## Hourly Scan Trend (Last 12h)

| Time (UTC) | Tasks | Success | Fail | Rate | Wall |
|------------|-------|---------|------|------|------|
| 05:44 | 2 | 2 | 0 | 100% | 60s |
| 06:45 | 2 | 2 | 0 | 100% | 62s |
| 07:46 | 2 | 2 | 0 | 100% | 59s |
| 08:48 | 2 | 2 | 0 | 100% | 65s |
| 09:48 | 2 | 2 | 0 | 100% | 59s |
| 10:49 | 2 | 2 | 0 | 100% | 41s |
| 11:50 | 2 | 2 | 0 | 100% | 62s |
| 12:51 | 2 | 2 | 0 | 100% | 55s |
| 13:52 | 2 | 2 | 0 | 100% | 46s |
| 14:54 | 2 | 2 | 0 | 100% | 74s |
| 15:46 | 2 | 2 | 0 | 100% | 54s |
| 16:49 | 2 | 2 | 0 | 100% | 144s |

**Note:** Hourly scans run sequentially (1 worker), 2 tasks per run at ~30-60s each. Machine is idle ~55 minutes of every hour.

## Scaling Opportunity

The machine is running at **~4% utilization** during scheduler operations:
- 44 runs in 24h = 2,728s total wall time
- Remaining 83,672s (23.2h) = idle

**This means there is massive headroom for parallel L1 work.**

### Parallelism Experiment Plan

| Step | Workers | Expected Impact | Risk |
|------|---------|----------------|------|
| Baseline | 1 | Current state | None |
| Step 1 | 3 | 2-3x throughput | Moderate latency increase |
| Step 2 | 5 | 3-4x throughput | Possible quality drop |
| Step 3 | 7 | Max throughput | Timeout risk, quality degradation |

**Stop condition:** L1-produced rate drops below 80% OR timeout rate exceeds 20% OR P95 latency > 3x baseline.

## Worker Allocation (Target)

| Work Type | % | Notes |
|-----------|---|-------|
| Remediation / backlog drain | 60% | Primary — Done movement is the metric |
| Roadmap-to-ticket / task shaping | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Keep from flooding |
| Reserved / retries / maintenance | 10% | Backstop |

## Evidence Paths

- Per-task telemetry: `/var/lib/zen-brain1/metrics/per-task.jsonl`
- Run-level history: `/var/lib/zen-brain1/metrics/history.jsonl`
- Runtime baseline: `docs/05-OPERATIONS/evidence/runtime-throughput-baseline.md`
- Attribution scoreboard: `docs/05-OPERATIONS/evidence/l1-attribution-scoreboard.md`
- Parallelism experiment: `docs/05-OPERATIONS/evidence/runtime-throughput-experiment/`
