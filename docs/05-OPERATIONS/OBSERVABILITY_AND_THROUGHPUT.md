# Observability and Throughput Policy

**Date:** 2026-03-28  
**Status:** Phase A–D complete

## Core Principles

1. **Healthy-but-slow is acceptable for CPU-only 0.8b.** A task that takes 70s and produces correct output is better than one that takes 10s and produces garbage.
2. **Blanket timeout reduction is rejected.** Adaptive timeout by task shape (90-180s), never below 60s.
3. **Throughput is optimized by better parallelism + better output contracts, not shorter patience.**
4. **Observability is required for all model/lane scaling decisions.** No blind jumps.
5. **Done movement is the primary success metric.** Discovery and ticket creation are secondary.

## Terminal Result Files (Phase A)

Every remediation-worker run produces a `RESULT_DIR/{JIRA_KEY}.json` terminal result file:

```json
{
  "jira_key": "ZB-1037",
  "terminal_class": "needs_review",
  "quality_score": 15,
  "quality_passed": true,
  "l1_status": "needs_review",
  "jira_state": "Done",
  "evidence_path": "/var/lib/zen-brain1/evidence/...",
  "gate_log_path": "/var/lib/zen-brain1/quality-gate-logs/ZB-1037-passed.json",
  "timestamp": "2026-03-28T17:33:42-04:00"
}
```

### Terminal Classifications

| Class | Meaning | Jira State |
|-------|---------|------------|
| `done` | Quality gate passed, L1 success | Done |
| `needs_review` | Quality gate passed, L1 says review needed | Done |
| `paused` | Quality gate rejected (score < 15) | PAUSED |
| `blocked_invalid_payload` | Quality gate rejected, payload not usable | PAUSED |
| `retrying` | L1 call failed entirely | RETRYING |
| `to_escalate` | L1 says human must handle | TO_ESCALATE |
| `failed` | Unrecoverable error | RETRYING |

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

## Per-Schedule Workers Override (Phase B)

| Schedule | Workers | Rationale |
|----------|---------|-----------|
| hourly-scan | 5 | 3-task batch, W=5 proven sufficient |
| quad-hourly-summary | 5 | 6-task batch, same as hourly until evidence supports higher |
| daily-sweep | 7 | First experiment — 10-task batch, evidence-based |

### Decision Rule
- Keep highest worker level that improves done/hour without de quality/state degradation
- **W=10 not approved unless W=7 is clean**

## Parallelism Experiment Results (2026-03-28 baseline)

| Workers | Throughput | L1-produced | Avg Latency | P95 Latency | Timeouts | Notes |
|---------|-----------|-------------|-------------|-------------|----------|-------|
| 1 (seq) | 2.95/min | 60% | 20.3s | 69.9s | 0 | Best quality baseline |
| 3 | 1.97/min | 60% | 77.5s | 180.0s | 3 | Slower, not faster |
| 5 | 2.53/min | 30% | 92.5s | 180.0s | 4 | Quality crash |
| 7 | 3.33/min | 70% | 86.3s | 180.0s | 3 | Best throughput, noisier |

### Key Finding

Single llama.cpp `--parallel 10` does NOT improve throughput linearly. CPU contention on i9-13900H causes timeout cascades above 3 concurrent workers.

## Discovery Throttle (Phase C)

**Policy:** if ready backlog > 10 tickets, skip discovery.

| Work Type | Allocation | Notes |
|-----------|------------|-------|
| Remediation / backlog drain | 70% | Primary — Done movement is the metric |
| Roadmap / office execution | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Throttled when backlog is large |

## Evidence Paths

- Phase A proof: `docs/05-OPERATIONS/evidence/phase-a-terminal-state-proof.md`
- Runtime experiment: `docs/05-OPERATIONS/evidence/runtime-throughput-experiment/parallelism-experiment.json`
- Throughput dashboard: `docs/05-OPERATIONS/evidence/throughput-and-utilization-dashboard.md`
- Corrective retry: `docs/05-OPERATIONS/evidence/l1-corrective-retry/`
- Attribution scoreboard: `docs/05-OPERATIONS/evidence/l1-attribution-scoreboard.md`
- Quality gate logs: `/var/lib/zen-brain1/quality-gate-logs/`
- Terminal result files: `/tmp/zen-brain1-worker-results/` (per-run)

- Per-task telemetry: `/var/lib/zen-brain1/metrics/per-task.jsonl`

## Worker Allocation
| Work Type | % | Notes |
|-----------|---|-------|
| Remediation / backlog drain | 60% | Primary — Done movement is the metric |
| Roadmap-to-ticket / task shaping | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Keep from flooding |
| Reserved / retries / maintenance | 10% | Backstop |

## Operating Statements
- quality-gate-rejected tickets must not remain in In Progress
- throughput measurements are invalid until terminal states are correct
- discovery must be throttled when backlog drain is the priority
- worker-count changes are evidence-based, not guesswork
