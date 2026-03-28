# Observability and Throughput Policy

**Date:** 2026-03-28  
**Status:** Phase A/B/C/D complete

## Core Principles

1. **Healthy-but-slow is acceptable for CPU-only 0.8b.** A task that takes 70s and produces correct output is better than one that takes 10s and produces garbage.
2. **Blanket timeout reduction is rejected.** Adaptive timeout by task shape (90-180s), never below 60s.
3. **Throughput is optimized by better parallelism + better output contracts, not shorter patience.**
4. **Observability is required for all model/lane scaling decisions.** No blind jumps.
5. **Done movement is the primary success metric.** Discovery and ticket creation are secondary.

## Terminal State Correctness (Phase A)

**Critical invariant: quality-gate-rejected tickets must not remain In Progress.**

Phase A (commit 9d0a2bf) replaced fragile stdout string matching with explicit WorkerTerminalResult JSON files.

Every remediation-worker run writes `RESULT_DIR/{JIRA_KEY}.json`:
- `terminal_class`: done | needs_review | paused | retrying | to_escalate | blocked_invalid_payload | failed
- `quality_score`: 0-25
- `quality_passed`: bool
- `jira_state`: final Jira status

Factory-fill reads these files via `handleTerminalResult`. Reconciliation remains a safety net.

### Live Proof (Phase A)

| Ticket | Path | Score | Terminal Class | Jira State |
|--------|------|-------|---------------|------------|
| ZB-1037 | success (gate passed) | 15/25 | needs_review | Done |
| ZB-1032 | success (gate passed) | 17/25 | done | Done |
| ZB-1045 | reject (gate failed) | 5/25 | blocked_invalid_payload | PAUSED |
| ZB-843 | paused (L1 blocked) | 16/25 | paused | PAUSED |

No tickets stuck In Progress after any terminal result.

## Per-Task Telemetry

Every L1 remediation call records:
- model, lane, prompt_size, output_size
- start_time, end_time, wall_time
- completion_class (fast-productive / slow-but-productive / truncated-repaired / timeout / parse-fail / validation-fail)
- produced_by (l1 / l1-partial / l1-failed / supervisor)
- quality_score (0-25)
- repair_type (if truncation repair was used)
- evidence_pack path

## Per-Schedule WORKERS Override (Phase B)

Schedule configs set `workers` field explicitly:
- `hourly-scan`: W=5 (3-task batch, W=7 showed no throughput gain)
- `quad-hourly-summary`: W=5 (6-task batch, same as hourly)
- `daily-sweep`: W=7 (10-task batch, first experiment target)

Scheduler logs effective WORKERS value per run. Env `WORKERS_OVERRIDE` takes highest priority.

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
- **5 workers:** Current default for factory-fill. Proven stable.
- **7 workers:** Approved for daily-sweep only. First experiment target.
- **10 workers:** NOT approved unless W=7 proves clean with clear benefit.

### Scaling Path (if needed)

To go beyond 5 useful workers, options are:
1. **Multiple llama.cpp instances** on different ports (each with `--parallel 3`)
2. **Reduce model size** (0.5b instead of 0.8b) for faster inference
3. **GPU inference** (not available on current machine)
4. **Smaller context window** (`--ctx-size 32768`) to reduce memory pressure

## Backlog Priority / Discovery Throttle (Phase D)

**Policy: Do not create more work faster than the factory can close it.**

When backlog has > 10 ready tickets:
- Discovery/ticketizer is throttled (skipped)
- Remediation/backlog drain gets priority
- Factory-fill continues pulling from backlog

| Work Type | % | Notes |
|-----------|---|-------|
| Remediation / backlog drain | 70% | Primary — Done movement is the metric |
| Roadmap / office execution | 20% | Office support, bounded tickets |
| Discovery refresh / dedup | 10% | Keep from flooding |

## Per-Task Telemetry (Phase 39+)

Every L1 call emits a `TaskTelemetryRecord` to `/var/lib/zen-brain1/metrics/per-task.jsonl`.

### Querying Metrics

```bash
# Human-readable summary (last 24h)
go run ./cmd/metrics-report --window last_24h --human

# JSON output for dashboards
go run ./cmd/metrics-report --window last_hour --json
```

## Evidence Paths

- Phase A proof: `docs/05-OPERATIONS/evidence/phase-a-terminal-state-proof.md`
- Runtime experiment: `docs/05-OPERATIONS/evidence/runtime-throughput-experiment/parallelism-experiment.json`
- Factory dashboard: `docs/05-OPERATIONS/evidence/factory-fill-and-backlog-utilization.md`
- Attribution scoreboard: `docs/05-OPERATIONS/evidence/l1-attribution-scoreboard.md`

## Operating Statements

- Terminal result files are the authoritative source of truth for state transitions
- Stdout heuristics are fallback only (legacy safety net)
- Quality-gate-rejected tickets must not remain In Progress
- Throughput measurements are invalid until terminal states are correct
- Worker-count changes are evidence-based, not guesswork
- Backlog drain remains the main success metric
- Discovery is throttled when backlog drain is the priority
