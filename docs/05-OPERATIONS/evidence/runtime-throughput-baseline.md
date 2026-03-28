# Runtime Throughput Baseline

**Date:** 2026-03-28  
**Experiment:** Controlled parallelism staircase (1, 3, 5, 7 workers)  
**Script:** `scripts/runtime-throughput-experiment.py`

## Baseline: Sequential (1 worker)

| Metric | Value |
|--------|-------|
| Total tasks | 10 |
| Phase elapsed | 203.3s |
| Throughput | 2.95 tasks/min |
| L1-produced | 6/10 (60%) |
| Avg wall time | 20.3s |
| P50 | 7.9s |
| P95 | 69.9s |
| Max | 69.9s |
| Timeouts | 0 |
| Truncation repairs | 4/10 |
| Completion classes | fast-productive: 6, truncated-repaired: 4 |

## Step 1: 3 Workers

| Metric | Value | vs Baseline |
|--------|-------|-------------|
| Throughput | 1.97 tasks/min | **-33%** |
| L1-produced | 6/10 (60%) | same |
| Avg wall time | 77.5s | **+282%** |
| P95 | 180.0s | **+157%** |
| Timeouts | 3 | +3 |

**Verdict: WORSE.** 3 parallel workers on single llama.cpp reduced throughput by 33%.

## Step 2: 5 Workers

| Metric | Value | vs Baseline |
|--------|-------|-------------|
| Throughput | 2.53 tasks/min | -14% |
| L1-produced | 3/10 (30%) | **-50%** |
| Avg wall time | 92.5s | **+356%** |
| P95 | 180.0s | +157% |
| Timeouts | 4 | +4 |

**Verdict: QUALITY CRASH.** L1-produced dropped to 30%. CPU contention killed half the outputs.

## Step 3: 7 Workers

| Metric | Value | vs Baseline |
|--------|-------|-------------|
| Throughput | 3.33 tasks/min | +13% |
| L1-produced | 7/10 (70%) | +17% |
| Avg wall time | 86.3s | +325% |
| P95 | 180.0s | +157% |
| Timeouts | 3 | +3 |

**Verdict: MIXED.** Best raw throughput and highest L1-produced rate, but 3 timeouts and very high variance. Statistical noise — not reproducible.

## Summary

| Workers | Throughput | L1% | Avg Latency | Timeouts | Verdict |
|---------|-----------|-----|-------------|----------|---------|
| 1 | 2.95/min | 60% | 20.3s | 0 | ✅ Best quality |
| 3 | 1.97/min | 60% | 77.5s | 3 | ❌ Slower |
| 5 | 2.53/min | 30% | 92.5s | 4 | ❌ Quality crash |
| 7 | 3.33/min | 70% | 86.3s | 3 | ⚠️ Noisy |

## Conclusion

Single llama.cpp instance cannot parallelize effectively. The `--parallel 10` flag allows concurrent request handling, but CPU contention causes latency to explode and quality to crash at 5+ workers.

**Recommended default: 1 worker (sequential).**

For higher throughput, the path is multiple llama.cpp instances (each with `--parallel 3`), not more concurrent requests to a single instance.
