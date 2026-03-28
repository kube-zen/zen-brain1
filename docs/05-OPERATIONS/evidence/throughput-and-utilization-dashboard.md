# Throughput and Utilization Dashboard

**Updated:** 2026-03-28 12:49 EDT  
**Machine:** i9-13900H 20-core, 64GB RAM  
**L1 Model:** Qwen3.5-0.8B-Q4_K_M.gguf (single llama.cpp, port 56227, --parallel 10)

## Current State

| Metric | Value |
|--------|-------|
| L1 workers active | 1 (sequential proven best) |
| L1-produced rate (corrective retry) | **100%** (10/10) |
| L1-produced rate (full expansion) | **90%** (18/20) |
| L1-produced rate (parallelism benchmark) | 60% (1w), 30% (5w), 70% (7w) |
| Jira Done count | 613 |
| Jira Backlog | 28 |
| Jira RETRYING | 5 |
| Jira PAUSED | 0 |
| Done today | +51 (from 562 baseline) |
| Throughput (sequential) | 2.95 tasks/min |
| Throughput (7 workers) | 3.33 tasks/min |
| Avg latency (sequential) | 20.3s |
| P95 latency (sequential) | 69.9s |
| Truncation repair rate | 40-50% of slow tasks |

## Latency Distribution

### By Task Type (sequential baseline)

| Task Type | Avg | P50 | P95 | Success Rate |
|-----------|-----|-----|-----|-------------|
| config_change | ~8s | 7s | 12s | ~80% |
| doc_update | ~12s | 10s | 25s | ~70% |
| code_edit | ~35s | 20s | 70s | ~50% |

### By Completion Class

| Class | Count (baseline) | Description |
|-------|-----------------|-------------|
| fast-productive | 6/10 | <60s, parseable, good quality |
| truncated-repaired | 4/10 | Cut off but repairable via bracket fix |
| slow-but-productive | varies | 60-80s, productive after repair |
| no-output | varies | Timeout, empty response |
| parse-fail | rare | Unparseable even after repair |

## Backlog Trend

| Date | Backlog | Done | Delta |
|------|---------|------|-------|
| 2026-03-27 | 524 | 0 | Baseline |
| 2026-03-28 10:00 | 8 | 529 | Bulk drain |
| 2026-03-28 12:00 | 28 | 603 | Expansion batch |
| 2026-03-28 12:49 | 28 | 613 | Corrective retry |

## Top Bottlenecks

1. **Single llama.cpp CPU instance** — cannot parallelize effectively beyond 3 concurrent requests
2. **JSON truncation on complex sed commands** — 40-50% of slow outputs need bracket repair
3. **28 remaining Backlog tickets** — mix of real findings and new scanner output, need triage
4. **5 RETRYING tickets** — need re-evaluation after corrective retry success

## Worker Utilization

- L1 (port 56227): Active, healthy, sequential mode recommended
- 15 other llama instances on various ports (other models/workloads)
- Machine CPU: ~50% utilized during sequential runs, pegged during 5+ worker runs
- Machine RAM: 51/62GB used (llama instances consume majority)

## Recommended Actions

1. **Keep sequential for quality batches** (expansion, attribution-critical work)
2. **Use 3 workers max for bulk ops** (discovery dedup, config batch changes)
3. **Fix the 5 RETRYING tickets** — likely recoverable with corrective retry
4. **Triage the 28 Backlog** — separate real findings from scanner noise
5. **Consider multi-instance** if throughput >3.5 tasks/min is needed
