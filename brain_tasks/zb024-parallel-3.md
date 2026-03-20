---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-parallel-3
  labels:
    tranche: "ZB-024"
    simulation: "parallel-worker"
    task-class: config-normalization
spec:
  description: |
    Task 3 of 5-parallel simulation for ZB-024.

    Verify that all documentation references to local model timeouts
    consistently use 45-minute (2700 second) values, not
    30-second or 5-minute defaults.

    This is a bounded code review/refactor task.

  template: "structured-review"
  context:
    files:
      - path: docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md
      - path: docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md
      - path: docs/05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md
      - path: deploy/README.md
    instructions: |
      Review all documentation for consistency with 45-minute timeout policy.

      Find any instances where:
      - Timeout is set to 30 seconds or 5 minutes
      - Documentation implies multi-minute tasks are failures
      - Stale threshold is less than or equal to execution timeout

      Fix any inconsistencies found to clearly state:
      - 45-minute timeout is NORMAL for qwen3.5:0.8b CPU inference
      - Multi-minute tasks (20-45 minutes) are VALID and WORKING AS DESIGNED

  allowedOutputs:
    - type: git-push
      repo: zen-brain1
      branch: main
      paths:
        - docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md
        - docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md
        - docs/05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md
        - deploy/README.md
