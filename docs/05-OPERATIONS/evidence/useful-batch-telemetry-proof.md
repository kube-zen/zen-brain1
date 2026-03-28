# Useful-Batch Telemetry Proof

**Date:** 2026-03-28 14:05 EDT
**Status:** PROVEN

## Proof Run

- **Binary:** `cmd/useful-batch/` with `METRICS_DIR=/var/lib/zen-brain1/metrics`
- **Task class:** dead_code
- **L1 endpoint:** http://localhost:56227/v1/chat/completions
- **Model:** Qwen3.5-0.8B-Q4_K_M.gguf
- **Workers:** 1
- **Timeout:** 60s
- **Result:** 16,446ms wall time, 759 chars output, validation=success

## Telemetry Record

Written to: `/var/lib/zen-brain1/metrics/per-task.jsonl`

```json
{
  "timestamp": "2026-03-28T14:05:02-04:00",
  "run_id": "telemetry-proof-ub-20260328-140445",
  "task_id": "telemetry-proof-ub-20260328-140445-0",
  "model": "Qwen3.5-0.8B-Q4_K_M.gguf",
  "lane": "l1-local",
  "provider": "llama-cpp",
  "prompt_size_chars": 5083,
  "output_size_chars": 759,
  "start_time": "2026-03-28T14:04:45-04:00",
  "end_time": "2026-03-28T14:05:02-04:00",
  "wall_time_ms": 16446,
  "completion_class": "fast-productive",
  "produced_by": "l1",
  "attempt_number": 1,
  "repair_used": false,
  "task_class": "dead_code",
  "final_status": "done",
  "jira_updated": false
}
```

## Schema Alignment with Remediation-Worker

Both paths emit to the same file (`per-task.jsonl`) with identical field names.
Differentiation is by:
- `task_class`: "remediation" vs "dead_code", "defects", etc.
- `run_id`: includes batch name for useful-batch, pilot/cycle prefix for remediation-worker
- `provider`: "llama-cpp" for both (same L1 endpoint)

## Skip Path

The empty-evidence skip path also emits telemetry with:
- `provider: "script-skip"`
- `produced_by: "script"`
- `completion_class: "fast-productive"`

This was not separately tested but compiles in the same code path.

## Summary

Both the remediation-worker and useful-batch (scheduler hot path) emit per-task telemetry
to the same rolling JSONL file with aligned schema.

**Next step:** Capture baseline throughput metrics before any worker-count changes.
