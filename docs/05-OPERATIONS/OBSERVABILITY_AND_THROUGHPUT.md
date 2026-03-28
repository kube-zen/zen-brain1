# Observability and Throughput Policy

**Date:** 2026-03-28  
**Status:** Initial baseline captured

## Core Principles

1. **Healthy-but-slow is acceptable for CPU-only 0.8b.** A task that takes 70s and produces correct output is better than one that takes 10s and produces garbage.
2. **Blanket timeout reduction is rejected.** Adaptive timeout by task shape (90-180s), never below 60s.
3. **Throughput is optimized by better parallelism + better output contracts, not shorter patience.**
4. **Observability is required for all model/lane scaling decisions.** No blind jumps.
5. **Done movement is the primary success metric.** Discovery and ticket creation are secondary.

## Per-Task Telemetry

Every L1 remediation call records:
- model, lane, prompt_size, output_size
- start_time, end_time, wall_time
- completion_class (fast-productive / slow-but-productive / truncated-repaired / timeout / parse-fail / validation-fail)
- produced_by (l1 / l1-partial / l1-failed / supervisor)
- quality_score (0-25)
- repair_type (if truncation repair was used)
- evidence_pack path

## Computed Metrics

| Metric | Description |
|--------|-------------|
| tasks/min | Throughput rate |
| L1-produced % | Honest success rate |
| avg/p50/p95 wall_time | Latency distribution |
| timeout rate | Requests with empty output |
| truncation-repair rate | Responses saved by bracket repair |
| chars/sec | Generation speed |
| done/day | Jira Done movement |
| cycle time | Open→Done duration |

## Parallelism Policy

### Experiment Results (2026-03-28)

| Workers | Throughput | L1-produced | Avg Latency | P95 Latency | Timeouts | Notes |
|---------|-----------|-------------|-------------|-------------|----------|-------|
| 1 (seq) | 2.95/min | 60% | 20.3s | 69.9s | 0 | Best quality baseline |
| 3 | 1.97/min | 60% | 77.5s | 180.0s | 3 | Slower, not faster |
| 5 | 2.53/min | 30% | 92.5s | 180.0s | 4 | Quality crash |
| 7 | 3.33/min | 70% | 86.3s | 180.0s | 3 | Best throughput, noisier |

### Key Finding

Single llama.cpp `--parallel 10` does NOT improve throughput linearly with concurrent requests. CPU contention on the i9-13900H causes timeout cascades above 3 concurrent workers.

### Decision Rule

- **Sequential (1 worker):** Best for quality-sensitive batches. Use for bounded expansion.
- **3 workers:** Safe concurrency ceiling without quality loss. Use for bulk ops.
- **5+ workers:** Do NOT use with single llama.cpp instance. Quality degrades.
- **7 workers:** Best raw throughput but highest timeout variance. Acceptable only with truncation repair + retry logic.

### Scaling Path (if needed)

To go beyond 3 useful workers, options are:
1. **Multiple llama.cpp instances** on different ports (each with `--parallel 3`)
2. **Reduce model size** (0.5b instead of 0.8b) for faster inference
3. **GPU inference** (not available on current machine)
4. **Smaller context window** (`--ctx-size 32768`) to reduce memory pressure

## Per-Task Telemetry (Phase 39+)

Every L1 call now emits a `TaskTelemetryRecord` to `/var/lib/zen-brain1/metrics/per-task.jsonl`:

```json
{
  "timestamp": "2026-03-28T17:00:00Z",
  "run_id": "cycle-20260328-170000",
  "task_id": "ZB-931",
  "jira_key": "ZB-931",
  "model": "qwen3.5:0.8b",
  "lane": "l1-local",
  "provider": "llama-cpp",
  "prompt_size_chars": 1200,
  "output_size_chars": 800,
  "wall_time_ms": 45000,
  "completion_class": "slow-but-productive",
  "produced_by": "l1",
  "quality_score": 22.5,
  "task_class": "remediation",
  "final_status": "success"
}
```

### Querying Metrics

```bash
# Human-readable summary (last 24h)
go run ./cmd/metrics-report --window last_24h --human

# JSON output for dashboards
go run ./cmd/metrics-report --window last_hour --json

# All-time baseline
go run ./cmd/metrics-report --window all --human
```

### Computed Metrics Available

- Tasks/hour, Done/hour, Done/day
- L1-produced rate
- Avg / P50 / P95 latency
- Timeout rate, truncation rate, repair rate
- Chars/sec throughput
- Per-model and per-lane breakdowns
- Quality score averages

## Worker Allocation

| Work Type | % | Notes |
|-----------|---|-------|
| Remediation / backlog drain | 60% | Primary — Done movement is the metric |
| Roadmap-to-ticket / task shaping | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Keep from flooding |
| Reserved / retries / maintenance | 10% | Backstop |

## Evidence Paths

- Runtime experiment: `docs/05-OPERATIONS/evidence/runtime-throughput-experiment/parallelism-experiment.json`
- Throughput dashboard: `docs/05-OPERATIONS/evidence/throughput-and-utilization-dashboard.md`
- Corrective retry: `docs/05-OPERATIONS/evidence/l1-corrective-retry/`
- Attribution scoreboard: `docs/05-OPERATIONS/evidence/l1-attribution-scoreboard.md`
