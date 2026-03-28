# Runtime Throughput Baseline

**Date:** 2026-03-28  
**Status:** Baseline captured with real scheduler data  
**Next Step:** Controlled parallelism experiment

## Core Principle

**Healthy-but-slow is acceptable. Throughput is optimized by parallelism, not by shorter patience.**

## Baseline Metrics (24h window ending 2026-03-28 17:00 UTC)

| Metric | Value |
|--------|-------|
| Runs in 24h | 44 |
| Total tasks | 175 |
| L1 success | 166 |
| L1 fail | 9 |
| **L1-produced rate** | **94.9%** |
| Total wall time | 2,728s (0.8h) |
| Tasks/hour | 7.3 |
| Done/day | ~175 (if all success → Done) |

## By Schedule Type

| Schedule | Runs | Tasks | Success | Fail | L1-produced | Avg Wall |
|----------|------|-------|---------|------|-------------|----------|
| hourly-scan | 27 | 55 | 52 | 3 | 95% | 70s |
| daily-sweep | 6 | 54 | 50 | 4 | 93% | 120s |
| quad-hourly-summary | 11 | 66 | 64 | 2 | 97% | 11s |

## Key Observations

1. **hourly-scan runs are sequential (2 tasks/run, ~60s wall)** — one L1 call at a time
2. **daily-sweep runs ~10 tasks in ~120s** — already slightly parallel
3. **L1-produced rate is 94.9%** — the quality concern is solved, throughput is the bottleneck
4. **Machine is ~96% idle during scheduler runs** — wall time is 0.8h out of 24h available

## Existing Observability Stack

Before this phase:
- `/var/lib/zen-brain1/metrics/history.jsonl` — 80 run-level records (run-level only)
- `/var/lib/zen-brain1/metrics/latest-summary.json` — latest run summary
- Per-run artifacts under `/var/lib/zen-brain1/runs/{schedule}/{timestamp}/`

**Gap:** No per-task telemetry. No per-model comparison. No latency distribution.

## What This Phase Adds

### Per-Task Telemetry (NEW)

Every L1 call now records to `/var/lib/zen-brain1/metrics/per-task.jsonl`:
- model, lane, provider
- prompt_size, output_size, input_tokens, output_tokens
- start_time, end_time, wall_time_ms, first_token_ms
- completion_class (fast-productive / slow-but-productive / truncated-repaired / timeout / parse-fail / validation-fail)
- produced_by (l1 / l1-partial / l1-failed / supervisor)
- quality_score, repair_used, repair_succeeded
- task_class, remediation_type, final_status
- jira_transition, jira_updated
- evidence_pack_path

### Computed Metrics (NEW)

Available via `go run ./cmd/metrics-report`:
- L1-produced rate, timeout rate, truncation-repair rate
- Avg / P50 / P95 latency
- Tasks/hour, done/hour, done/day
- Chars/sec generation speed
- Per-model and per-lane breakdowns
- Worker utilization and queue depth

### Query Tool (NEW)

```bash
# Human-readable summary
go run ./cmd/metrics-report --window last_24h

# JSON for programmatic consumption
go run ./cmd/metrics-report --window last_hour --json

# All time
go run ./cmd/metrics-report --window all
```

## Parallelism Experiment Design

### Staircase Approach

| Step | Workers | Measure | Stop If |
|------|---------|---------|---------|
| Baseline | 1 (current) | Capture 24h of per-task telemetry | — |
| Step 1 | +2 (3 total) | Compare latency, L1-rate, throughput | Latency doubles OR L1-rate drops below 80% |
| Step 2 | +2 (5 total) | Same comparison | Same stop condition |
| Step 3 | +2 (7 total) | Same comparison | Same stop condition |

### Decision Rule

- **Keep scaling** if: done/hour improves and L1-produced rate stays above 80%
- **Stop scaling** if: timeout rate exceeds 20% OR P95 latency exceeds 3x baseline OR machine becomes unstable

### Per-Step Capture

For each step, record:
- Active L1 workers/slots
- CPU utilization (from `/proc/stat` or `top`)
- Memory utilization
- Avg latency, P95 latency
- done/hour
- L1-produced rate
- Timeout/truncation rate

## Mandatory Operating Statements

1. Healthy-but-slow is acceptable for CPU-only qwen3.5:0.8b
2. Blanket timeout reduction is rejected
3. Throughput = controlled parallelism + observability, not shorter patience
4. Done-rate and backlog drain are the primary success metrics
5. Extra capacity → real work, not idle waiting
