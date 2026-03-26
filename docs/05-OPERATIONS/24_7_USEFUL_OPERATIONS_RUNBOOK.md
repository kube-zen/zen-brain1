# 24/7 Useful Operations Runbook

**Version:** 1.0
**Status:** Production
**Updated:** 2026-03-26 (PHASE 26)

## Overview

zen-brain1 is now capable of continuous 24/7 useful operations. The system produces reporting/triage artifacts (dead-code scans, defect reports, tech-debt summaries, etc.) through the proven L1 runtime path.

**Production success criterion:** zen-brain1 is "working" when it continuously produces useful artifacts through the real runtime on regular tasks. Standalone Go codegen is NOT the benchmark.

## Service Topology

| Lane | Model | Inference | Port | Slots | Context/Slot | Role |
|------|-------|-----------|------|-------|-------------|------|
| L1 | Qwen3.5-0.8B Q4_K_M | llama.cpp | 56227 | 10 parallel | 6656 tokens | Default — all regular useful tasks |
| L2 | Qwen3.5-2B Q4_K_M | llama.cpp | 60509 | 4 slots | 16384 tokens | Earned by repeated L1 failure |
| L0 | qwen3.5:0.8b | Ollama | 11434 | 1 | — | Fallback only (FAIL-CLOSED) |

## Worker Management

### Quick Start

```bash
# Start all workers
./scripts/worker-ctl.sh start

# Check health
./scripts/health-check.sh

# Warmup L1
./scripts/worker-ctl.sh warmup

# Stop all
./scripts/worker-ctl.sh stop
```

### systemd Services (for 24/7)

```bash
# Install services
sudo cp config/supervision/zen-brain1-l1.service /etc/systemd/system/
sudo cp config/supervision/zen-brain1-l2.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now zen-brain1-l1 zen-brain1-l2

# Check status
sudo systemctl status zen-brain1-l1 zen-brain1-l2

# View logs
journalctl -u zen-brain1-l1 -f
```

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

## Workload Scheduling

### Manual Batch Run

```bash
# All 10 task classes
BATCH_NAME=adhoc OUTPUT_ROOT=/tmp/zen-brain1-runs /tmp/useful-batch

# Specific tasks
BATCH_NAME=quick-scan TASKS=dead_code,defects,stub_hunting WORKERS=3 /tmp/useful-batch

# Custom output location
BATCH_NAME=nightly OUTPUT_ROOT=/var/lib/zen-brain1-runs /tmp/useful-batch
```

### Schedule Configuration

See `config/supervision/workload-schedule.yaml` for cron-based schedule:

| Schedule | Tasks | Frequency |
|----------|-------|-----------|
| hourly-scan | dead_code, defects, stub_hunting | Every hour |
| quad-hourly-summary | tech_debt, roadmap, config_drift, package_hotspots | Every 4 hours |
| daily-full-sweep | All 10 task classes | Daily at 6 AM |

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

## Artifact Storage

```
<OUTPUT_ROOT>/<BATCH_NAME>/<timestamp>/
├── final/               # Markdown report artifacts
│   ├── dead-code.md
│   ├── defects.md
│   ├── tech-debt.md
│   └── ...
├── logs/
│   └── dispatch.log     # Per-task dispatch log
└── telemetry/
    └── batch-index.json # Full telemetry with per-task results
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

## Day-Zero Evidence

**Date:** 2026-03-26
**Batch:** op-day-zero
**Result:** 10/10 OK, 4m31s wall time

| Report | Lines | Headings |
|--------|-------|----------|
| bug-hunting.md | 105 | 16 |
| config-policy-drift.md | 87 | 14 |
| dead-code.md | 91 | 13 |
| defects.md | 93 | 8 |
| executive-summary.md | 42 | 9 |
| package-hotspots.md | 79 | 8 |
| roadmap.md | 42 | 9 |
| stub-hunting.md | 60 | 2 |
| tech-debt.md | 74 | 4 |
| test-gaps.md | 63 | 12 |

Full artifacts: `docs/05-OPERATIONS/evidence/op-day-zero/final/`

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
