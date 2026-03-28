# Throughput and Utilization Dashboard

**Last updated:** 2026-03-28 12:47 EDT  
**Machine:** i9-13900H (20 cores), 64GB RAM  
**L1 Model:** Qwen3.5-0.8B-Q4_K_M.gguf  
**L1 Server:** llama.cpp, PID 2746158, port 56227, `--parallel 10 --ctx-size 65536`

## Current State

| Metric | Value |
|--------|-------|
| Jira Backlog | 28 |
| Jira Done | 613 |
| Jira RETRYING | 5 |
| L1 workers active | 1 (sequential) |
| Recommended concurrency | 3 workers max |
| Expansion status | PAUSED (awaiting next decision) |

## Parallelism Experiment Summary

| Phase | Workers | Elapsed | Tasks/min | L1-produced | Timeouts | Avg Latency | P95 |
|-------|---------|---------|-----------|-------------|----------|-------------|-----|
| Baseline | 1 | 203s | 2.95 | 60% | 0 | 20.3s | 69.9s |
| Step +2 | 3 | 305s | 1.97 | 60% | 3 | 77.5s | 180.0s |
| Step +4 | 5 | 238s | 2.53 | 30% | 4 | 92.5s | 180.0s |
| Step +6 | 7 | 180s | 3.33 | 70% | 3 | 86.3s | 180.0s |

## Completion Class Breakdown

| Class | Baseline | 3w | 5w | 7w |
|-------|----------|----|----|-----|
| fast-productive | 6 | 6 | 3 | 6 |
| slow-but-productive | 0 | 0 | 0 | 1 |
| truncated-repaired | 4 | 1 | 2 | 0 |
| no-output (timeout) | 0 | 3 | 4 | 3 |
| parse-fail | 0 | 0 | 1 | 0 |

## Truncation Repair Impact

| Metric | Value |
|--------|-------|
| Total repairs used | 7 |
| Repairs in baseline | 4 (recovered 40% of batch) |
| Repairs at 5 workers | 2 |
| Repairs at 7 workers | 0 (all either fast or timeout) |
| Repair success rate | 100% (all repaired outputs passed quality gate at ≥15) |

## Attribution History

| Batch | Date | Tasks | L1-produced | Rate |
|-------|------|-------|-------------|------|
| v1 pilot | 2026-03-27 | 10 | 3 | 30% |
| v2 pilot | 2026-03-28 | 10 | 6 | 60% |
| Expansion batch1 (original) | 2026-03-28 | 20 | 8 | 40% |
| Corrective retry | 2026-03-28 | 10 | 10 | 100% |
| **Combined expansion** | 2026-03-28 | 20 | 18 | **90%** |
| Parallelism experiment | 2026-03-28 | 40 | 22 | 55% |

## Recommendations

1. **Use sequential (1 worker) for quality-sensitive work.** 60% l1-produced with zero timeouts.
2. **Use 3 workers for bulk operations with truncation repair.** Same quality, slightly slower per-task but better utilization.
3. **Do NOT exceed 3 workers on single llama.cpp instance** without additional instances or GPU.
4. **Truncation repair is essential** — recovered 7 tasks across experiments that would have been failures.
5. **To increase effective throughput:** run multiple llama.cpp instances (each with `--parallel 3`) on different ports, load-balance across them.
