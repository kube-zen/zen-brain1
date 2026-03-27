# 24/7 Useful Operations Runbook

**Version:** 2.0
**Status:** Production — LIVE
**Updated:** 2026-03-26 (PHASE 27)

## Overview

zen-brain1 is now running under **real systemd supervision** with **active scheduled workloads**. The system produces reporting/triage artifacts continuously through the proven L1 runtime path.

**Production success criterion:** zen-brain1 is "working" when it continuously produces useful artifacts through the real runtime on regular tasks. Standalone Go codegen is NOT the benchmark.

## Quick Reference — Operator Controls

```bash
# Status check
./scripts/zen-ctl.sh status          # worker + timer status
./scripts/zen-ctl.sh health          # L1/L2/L0 health with slot info
./scripts/zen-ctl.sh schedule        # show active schedule

# Force an immediate batch
./scripts/zen-ctl.sh run hourly      # 3 tasks: defects, bug-hunting, stub-hunting
./scripts/zen-ctl.sh run quad        # 6 tasks: dead-code, tech-debt, etc.
./scripts/zen-ctl.sh run daily       # all 10 task classes

# Monitoring
./scripts/zen-ctl.sh latest          # show latest artifacts per batch
./scripts/zen-ctl.sh logs            # tail schedule logs
./scripts/zen-ctl.sh logs zen-brain1-l1  # tail L1 worker logs

# Recovery
./scripts/zen-ctl.sh restart         # restart L1+L2 workers
./scripts/zen-ctl.sh warmup          # warmup L1 with test request
```

## Platform Architecture

zen-brain1 operates within the zen platform:

- **zen-brain** = planner, router, model selection, task shaping
- **zen-flow** = execution engine for multi-step cluster jobs (future)
- **zen-lock** = secret delivery, key custody, credential encryption

See [ZEN_LOCK_ZEN_FLOW_INTEGRATION_DECISION.md](./ZEN_LOCK_ZEN_FLOW_INTEGRATION_DECISION.md) for the full architecture decision.
See [SECRET_CONTRACT.md](./SECRET_CONTRACT.md) for secret management rules.

| Lane | Model | Inference | Port | Slots | Context/Slot | Service | Status |
|------|-------|-----------|------|-------|-------------|---------|--------|
| L1 | Qwen3.5-0.8B Q4_K_M | llama.cpp | 56227 | 10 parallel | 6656 tokens | `zen-brain1-l1.service` | active (enabled) |
| L2 | Qwen3.5-2B Q4_K_M | llama.cpp | 60509 | 4 slots | 16384 tokens | `zen-brain1-l2.service` | active (enabled) |
| L0 | qwen3.5:0.8b | Ollama | 11434 | 1 | — | (manual) | fallback only |

### systemd Service Management

```bash
# Check service status
sudo systemctl status zen-brain1-l1 zen-brain1-l2

# Restart workers
sudo systemctl restart zen-brain1-l1 zen-brain1-l2

# View worker logs
sudo journalctl -u zen-brain1-l1 -f
sudo journalctl -u zen-brain1-l2 -f

# Log files
/var/log/zen-brain1/l1-worker.log
/var/log/zen-brain1/l2-worker.log
/var/log/zen-brain1/schedules.log   # all batch schedule output
```

### Restart Policy
- `Restart=on-failure` with 10s delay
- Burst limit: 5 restarts per 5 minutes
- Services survive shell exit and system reboots

## Health Checks

```bash
# Human-readable
./scripts/health-check.sh

# JSON (for automation)
./scripts/health-check.sh --json

# Manual health check
curl -s http://localhost:56227/health  # L1
curl -s http://localhost:60509/health  # L2
```

## Active Schedule (LIVE)

**Ownership:** zen-brain1 internal scheduler (`cmd/scheduler/scheduler`), NOT systemd timers.
**Source of truth:** `config/schedules/*.yaml`
**State:** `/run/zen-brain1/scheduler/`

| Schedule | Config | Tasks | Cadence | Proven |
|----------|--------|-------|---------|--------|
| Hourly scan | `config/schedules/hourly-scan.yaml` | defects, bug_hunting, stub_hunting | Every hour | ✅ 3/3 OK via scheduler |
| Quad-hourly summary | `config/schedules/quad-hourly-summary.yaml` | dead_code, tech_debt, package_hotspots, test_gaps, config_drift, roadmap | Every 4 hours | ✅ 6/6 OK via timer (bootstrap) |
| Daily full sweep | `config/schedules/daily-sweep.yaml` | All 10 task classes | Daily | ✅ 10/10 OK via scheduler |

### Schedule Commands

```bash
# Force immediate run through internal scheduler
./scripts/zen-ctl.sh run hourly
./scripts/zen-ctl.sh run quad
./scripts/zen-ctl.sh run daily

# View scheduler status (last run, next due, run count)
cat /run/zen-brain1/scheduler/scheduler-status.json

# View scheduler logs
sudo journalctl -u zen-brain1-scheduler -f
# or: tail -f /var/log/zen-brain1/scheduler.log
```

> **Note:** systemd timer units are DEPRECATED and disabled. They were a bootstrap-only path.
> The internal scheduler now owns all useful-task cadence.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BATCH_NAME` | adhoc | Batch identifier |
| `OUTPUT_ROOT` | /tmp/zen-brain1-runs | Artifact root directory |
| `TASKS` | all 10 | Comma-separated task class names |
| `TIMEOUT` | 300 | Per-task timeout (seconds) |
| `WORKERS` | 5 | Max concurrent L1 requests |
| `L1_ENDPOINT` | http://localhost:56227/v1/chat/completions | L1 chat API |
| `L1_MODEL` | Qwen3.5-0.8B-Q4_K_M.gguf | L1 model name |

## Artifact Storage (LIVE)

**Production artifact root:** `/var/lib/zen-brain1/runs/`

```
/var/lib/zen-brain1/runs/<batch-name>/<timestamp>/
├── final/               # Markdown report artifacts
│   ├── dead-code.md
│   ├── defects.md
│   └── ...
├── logs/
│   └── dispatch.log     # Per-task dispatch log
└── telemetry/
    └── batch-index.json # Full telemetry with per-task results
```

**Finding latest artifacts:**
```bash
./scripts/zen-ctl.sh latest
# Or: ls -td /var/lib/zen-brain1/runs/hourly-scan/*/final/*.md | head -3
```

## Telemetry

Each batch produces `batch-index.json` with:
- batch_id, batch_name, lane
- total, succeeded, failed counts
- wall_ms (total batch duration)
- per-task results: task_id, success, duration_ms, artifact_path, error

## Routing Policy

1. **Every useful task → L1 first** (llama.cpp 0.8B)
2. **Retries in L1** (up to 2 per MLQ config)
3. **Repeated failures → L2** (llama.cpp 2B)
4. **L0/Ollama = fallback only** (runtime outage)

## Operational Evidence

### Day-Zero (Manual Batch)
- **Date:** 2026-03-26 14:19
- **Result:** 10/10 OK, 4m31s wall time
- **Artifacts:** `docs/05-OPERATIONS/evidence/op-day-zero/final/`

### Hourly Scan (Unattended Timer)
- **Date:** 2026-03-26 15:08
- **Trigger:** `systemctl start zen-brain1-hourly-scan.service` (TriggeredBy: timer)
- **Result:** 3/3 OK, 1m41s wall time
- **Artifacts:** `/var/lib/zen-brain1/runs/hourly-scan/20260326-150852/final/`
- **Evidence:** `docs/05-OPERATIONS/evidence/hourly-scan-unattended/`

### Quad-Hourly Summary (Unattended Timer)
- **Date:** 2026-03-26 15:54
- **Trigger:** `systemctl start zen-brain1-quad-hourly-summary.service` (TriggeredBy: timer)
- **Result:** 6/6 OK, 2m18s wall time
- **Artifacts:** `/var/lib/zen-brain1/runs/quad-hourly-summary/20260326-155432/final/`
- **Evidence:** `docs/05-OPERATIONS/evidence/quad-hourly-unattended/`

## Failure Classification

| Symptom | Likely Cause | Action |
|---------|-------------|--------|
| `connection refused` | llama.cpp down | `./scripts/worker-ctl.sh restart` |
| Empty response | Tool-definition wrapper | Use direct HTTP path (cmd/useful-batch) |
| Timeout | 10 concurrent + slow task | Increase TIMEOUT or reduce WORKERS |
| All fail | L1 OOM/crash | Check `dmesg`, reduce parallel slots |

## Related Documents

- [Small Model Strategy](../03-DESIGN/SMALL_MODEL_STRATEGY.md) — certified runtime policy
- [Local LLM Escalation Ladder](../03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md) — escalation architecture
- [L1/L2 Lane Runbook](L1_L2_LANE_RUNBOOK.md) — operational procedure
- [Workload Schedule](../../config/supervision/workload-schedule.yaml) — cron schedule config

## Run Metrics (Phase 33)

Every scheduled batch writes three metrics artifacts:

### Per-Run (written to each run directory)

| Artifact | Path | Description |
|----------|------|-------------|
| `run-metrics.json` | `<run-root>/telemetry/run-metrics.json` | Machine-readable: task counts, wall time, Jira keys, status |
| `run-summary.md` | `<run-root>/final/run-summary.md` | Human-readable: task outcomes, artifacts, Jira links, blockers |

### Rolling Aggregates (written after every run)

| Artifact | Path | Description |
|----------|------|-------------|
| `latest-summary.json` | `/var/lib/zen-brain1/metrics/latest-summary.json` | Most recent run summary (overwritten) |
| `history.jsonl` | `/var/lib/zen-brain1/metrics/history.jsonl` | Append-only JSONL, one line per completed run |

### Operator Commands

```bash
./scripts/zen-ctl.sh latest     # Latest run metrics + artifacts
./scripts/zen-ctl.sh metrics    # Rolling history + latest summary
./scripts/zen-ctl.sh status     # Schedule status with next_due
```

### Recovery

- If rolling metrics path is not writable, scheduler logs `[METRICS] WARNING` and continues
- Fix: `sudo mkdir -p /var/lib/zen-brain1/metrics && sudo chown neves:neves /var/lib/zen-brain1/metrics`
- Per-run metrics are never skipped (written even on failed batches)

### State Directories

| Directory | Owner | Purpose |
|-----------|-------|---------|
| `/run/zen-brain1/scheduler/` | neves | **Daemon state** (last_run, next_due) — authoritative |
| `/var/lib/zen-brain1/metrics/` | neves | Rolling metrics (latest-summary, history) |
| `/var/lib/zen-brain1/runs/` | neves | Per-run artifacts + telemetry |

### next_due Interpretation

- `next_due` in `scheduler-status.json` and per-schedule state files shows when each schedule will fire next
- Zero-value (`0001-01-01`) indicates stale state from older code — safe to ignore after restart
- Daemon recomputes `next_due` correctly after each run using `cadenceDuration` (hourly/quad-hourly/daily)
