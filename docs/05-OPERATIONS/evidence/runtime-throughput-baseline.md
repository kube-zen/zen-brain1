# Runtime Throughput Baseline — Local L1 Capacity

**Date:** 2026-03-28 14:44 EDT
**Status:** BASELINE CAPTURED
**Purpose:** Record measured baseline before controlled WORKERS increase from 5 to 7.

## Machine Specifications

| Item | Value |
|------|-------|
| CPU | Intel i9-13900H, 20 cores (8P + 12E) |
| RAM | 62 GB DDR5 |
| Disk | 1.6 TB NVMe, 39% used |
| OS | Linux 6.17.0-19-generic |
| Uptime | 7 days |
| Load avg | 1.80 / 1.93 / 2.20 |

## Main L1 Server Configuration

| Item | Value |
|------|-------|
| Binary | llama.cpp llama-server |
| Model | Qwen3.5-0.8B-Q4_K_M.gguf |
| Port | 56227 (0.0.0.0) |
| `--parallel` | **10** (supports up to 10 concurrent requests) |
| `--ctx-size` | 65536 |
| Threads | default |
| Reasoning | off |
| PID | 2746158 |
| CPU usage | ~139% (idle-ish, bursts during inference) |
| Memory | ~12% (7.4 GB of 62 GB) |

**The main L1 endpoint already has more concurrency capacity than the scheduler is currently using.** The server supports 10 parallel slots but only 5 concurrent requests are being dispatched.

## Additional Llama Servers (Background)

| Port | Model | PID | Notes |
|------|-------|-----|-------|
| 46693 | Qwen3.5-0.8B-Q4_K_M | 130099 | Background, idle |
| 39229 | zen-go-tuned-Q4_K_M | 132402 | Background, idle |
| 56963 | Qwen3.5-0.8B-Q4_K_M | 1694158 | Background, idle |
| 55423 | zen-go-q4_k_m-latest | 1696451 | Background, idle |
| 47315 | Qwen3.5-0.8B-Q4_K_M | 1698979 | Background, idle |

These consume ~5% memory combined but negligible CPU when idle.

## Current Scheduler Configuration

| Item | Value |
|------|-------|
| Scheduler binary | `cmd/scheduler/scheduler` (systemd supervised) |
| Batch binary | `cmd/useful-batch/useful-batch` |
| **WORKERS** | **5** (hardcoded in cmd/scheduler/main.go line 224) |
| TIMEOUT | 300s |
| L1_ENDPOINT | http://localhost:56227 (inherited from env) |
| Schedules | hourly-scan (3 tasks), quad-hourly-summary (6 tasks), daily-sweep (10 tasks) |

## Historical Run Metrics (81 runs)

| Schedule | Runs | Tasks | L1 Success | Avg Wall |
|----------|------|-------|------------|----------|
| hourly-scan | 47 | 109 | 106 (97%) | 88s |
| quad-hourly-summary | 20 | 114 | 98 (86%) | 114s |
| daily-sweep | 14 | 127 | 104 (82%) | 262s |
| **Total** | **81** | **350** | **308 (88%)** | **124s avg** |

**Latency distribution:** p50=74s, p95=430s, max=605s

## Per-Task Telemetry (new — 2 records from proof runs)

| Metric | Value |
|--------|-------|
| Avg latency | 9,980ms |
| P50 latency | 16,446ms |
| Completion class | fast-productive (both) |
| Produced by | l1 (both) |
| L1-produced rate | 100% |

Note: Per-task telemetry was just instrumented. Production data will accumulate from scheduled runs.

## Baseline Summary Table

| Metric | Current Value |
|--------|---------------|
| Machine | i9-13900H, 20 cores, 62GB RAM |
| Main L1 port | 56227 |
| Server `--parallel` | 10 |
| Scheduler WORKERS | **5** |
| CPU util (main server) | ~139% / ~7% of total 20 cores |
| Memory util | 12% (7.4 GB / 62 GB) |
| Historical L1 success | 88% (308/350) |
| Recent hourly wall time | 55-144s |
| Recent hourly L1 success | ~100% |
| Avg wall time (all runs) | 124s |
| P50 wall time | 74s |
| P95 wall time | 430s |
| Total tasks completed | 350 across 81 runs |

## Current Bottleneck Hypothesis

The scheduler is **under-driving** available server parallelism. The llama.cpp server supports 10 concurrent slots but only 5 workers are configured. CPU utilization during runs shows headroom — the machine is not saturated.

Evidence for this:
- Load average ~2 on a 20-core machine
- Server CPU ~139% on a single process (not 1600%+)
- Daily-sweep with 10 tasks at WORKERS=5 takes 262s wall time — half the tasks wait for a slot

## Recommended First Step

Increase scheduler WORKERS from **5 → 7**.

This is a single controlled change. Do not modify:
- Server `--parallel` (already 10, sufficient)
- Timeout policy (300s is appropriate)
- Discovery scope
- Remediation contract

Success criteria:
- Done/hour improves
- L1-produced rate stays ≥80%
- Timeout rate does not materially increase
- P95 latency does not degrade more than 20%
