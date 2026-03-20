---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-parallel-1
  labels:
    tranche: "ZB-024"
    simulation: "parallel-worker"
    task-class: documentation
spec:
  description: |
    Task 1 of 5-parallel simulation for ZB-024.

    Update the LOCAL_MODEL_STRATEGY.md file to add a clear note about
    qwen3.5:0.8b being the certified local model with 45-minute timeout
    expectations. This is a bounded documentation task.

  template: "documentation-update"
  context:
    files:
      - path: docs/03-DESIGN/SMALL_MODEL_STRATEGY.md
        content: |
          ## ZB-024: CPU Timeout Realities (2026-03-20)

          ### Proven CPU Performance

          - Model: qwen3.5:0.8b (certified local model)
          - Latency: 8-23 seconds per request (validated with 20+ parallel workers)
          - Long tasks: 20-45 minutes (normal for CPU inference)
          - Parallelism: 20+ workers (validated throughput)

          ### Timeout Expectations

          - Local CPU execution timeout: 45 minutes (2700 seconds)
          - This is NORMAL and EXPECTED for qwen3.5:0.8b on CPU
          - Tasks taking 20-30 minutes are VALID and WORKING AS DESIGNED
          - No premature timeout or stale re-dispatch should occur

          ### Multi-Minute Tasks Are NOT Failures

          A qwen3.5:0.8b task running for 25 minutes is NOT "stuck" or "failed" —
          it is simply a CPU inference task taking the expected time.

          The system MUST:
          - Allow tasks to run for 45+ minutes without intervention
          - Use stale threshold > 45m (e.g., 50m or 60m) to avoid racing valid tasks
          - Log clearly when tasks complete successfully with multi-minute runtimes

  allowedOutputs:
    - type: git-push
      repo: zen-brain1
      branch: main
      paths:
        - docs/03-DESIGN/SMALL_MODEL_STRATEGY.md
