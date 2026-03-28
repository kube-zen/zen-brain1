# Remediation Worker Telemetry Proof

**Date:** 2026-03-28 13:59 EDT
**Status:** PROVEN

## Proof Run

- **Tool:** `cmd/telemetry-proof/`
- **L1 endpoint:** http://localhost:56227/v1/chat/completions
- **Model:** Qwen3.5-0.8B-Q4_K_M.gguf
- **Result:** 3515ms, 40 chars output, parse succeeded, classified fast-productive

## Telemetry Record

Written to: `/var/lib/zen-brain1/metrics/per-task.jsonl`

```json
{
  "timestamp": "2026-03-28T13:59:50-04:00",
  "run_id": "telemetry-proof",
  "task_id": "proof-task-001",
  "jira_key": "PROOF-001",
  "schedule_name": "telemetry-proof",
  "model": "Qwen3.5-0.8B-Q4_K_M.gguf",
  "lane": "l1-local",
  "provider": "llama-cpp",
  "prompt_size_chars": 209,
  "output_size_chars": 40,
  "start_time": "2026-03-28T13:59:46-04:00",
  "end_time": "2026-03-28T13:59:50-04:00",
  "wall_time_ms": 3515,
  "completion_class": "fast-productive",
  "produced_by": "l1",
  "attempt_number": 1,
  "repair_used": false,
  "task_class": "proof",
  "final_status": "success",
  "jira_updated": false
}
```

## Fields Verified

| Field | Present | Value |
|-------|---------|-------|
| model | ✅ | Qwen3.5-0.8B-Q4_K_M.gguf |
| lane | ✅ | l1-local |
| provider | ✅ | llama-cpp |
| prompt_size_chars | ✅ | 209 |
| output_size_chars | ✅ | 40 |
| start_time | ✅ | 13:59:46 |
| end_time | ✅ | 13:59:50 |
| wall_time_ms | ✅ | 3515 |
| completion_class | ✅ | fast-productive |
| produced_by | ✅ | l1 |
| attempt_number | ✅ | 1 |
| repair_used | ✅ | false |
| task_class | ✅ | proof |
| final_status | ✅ | success |

## Summary

Per-task telemetry emission is **proven** for the metrics collector path.
The same collector and schema are used by the remediation-worker telemetry wrapper.

**Next step:** Instrument useful-batch (the actual scheduler hot path) with the same schema.
