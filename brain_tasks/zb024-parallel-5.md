> **NOTE:** This task references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-parallel-5
  labels:
    tranche: "ZB-024"
    simulation: "parallel-worker"
    task-class: documentation
spec:
  description: |
    Task 5 of 5-parallel simulation for ZB-024.

    Add a "Long-Timeout Quick Reference" section to
    docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md that documents
    the relationship between warmup time, keep-alive, and 45-minute
    execution timeout. This is a bounded documentation task.

  template: "documentation-update"
  context:
    files:
      - path: docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md
        content: |
          ## 45-Minute Timeout Quick Reference (ZB-024)

          | Setting | Value | Purpose |
          |---------|-------|---------|
          | Warmup Time | 45-60 seconds | One-time cold start cost on startup |
          | Keep Alive | 45m | Keep model resident between tasks |
          | First Request Latency | 3-5 seconds | Time for first request after warmup |
          | Subsequent Request Latency | 8-23 seconds | Time for warmed requests |
          | Task Execution Timeout | 45m | Time for full task completion |
          | Stale Threshold | 50m | Re-dispatch if Running > 50m |

          **Key Relationship:**

          Keep alive (45m) >> Task timeout (45m) > First request latency (3-5s)

          This means:
          - After first warmup, model stays resident for 45 minutes
          - Tasks can take up to 45 minutes to complete
          - Multiple tasks can run within the same 45-minute keep-alive window
          - System doesn't need to reload model between tasks

          **Why 45-Minute Timeout?**

          qwen3.5:0.8b is a small model running on CPU:
          - Per-request latency: 8-23 seconds (validated)
          - Complex planning: 10-15 minutes of LLM calls
          - Total: 20-30+ minutes is NORMAL

          45-minute timeout is intentionally long to:
          - Cover worst-case complex planning tasks
          - Avoid false positive timeouts
          - Allow time for retries if needed
          - Prevent unnecessary re-dispatch as "stale"

          **Troubleshooting:**

          If tasks consistently hit 45-minute timeout:
          1. Check logs for actual LLM call latencies
          2. Verify model is warmed (look for "warmup done" log)
          3. Check if queue is backing up (many tasks waiting)
          4. Check if workers are actually busy (not idle)
          5. Consider if task complexity exceeds qwen3.5:0.8b capability

  allowedOutputs:
    - type: git-push
      repo: zen-brain1
      branch: main
      paths:
        - docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md
