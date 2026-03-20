---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-parallel-2
  labels:
    tranche: "ZB-024"
    simulation: "parallel-worker"
    task-class: documentation
spec:
  description: |
    Task 2 of 5-parallel simulation for ZB-024.

    Add a "Long-Timeout Quick Reference" section to
    deploy/README.md that clearly documents that
    45-minute timeouts are normal and expected for qwen3.5:0.8b
    CPU inference. This is a bounded documentation task.

  template: "documentation-update"
  context:
    files:
      - path: deploy/README.md
        content: |
          ### 45-Minute Timeout Quick Reference (ZB-024)

          | Timeout Setting | Value | Purpose |
          |--------------|-------|---------|
          | Local Worker Timeout | 45m (2700s) | Time for qwen3.5:0.8b to complete CPU inference request |
          | Factory LLM Timeout | 45m (2700s) | Time for Factory LLM generation |
          | Task Execution Timeout | 45m (2700s) | Time for task to complete |
          | Ollama Keep Alive | 45m | Keep model resident in memory |
          | Stale Detection Threshold | 50m | Re-dispatch tasks Running > 50m |

          **IMPORTANT: Multi-minute tasks (20-45 minutes) are NORMAL for qwen3.5:0.8b CPU inference.**

          These timeouts are intentionally long to avoid:
          - Premature failure of valid long-running tasks
          - Unnecessary retry loops
          - False positive stale re-dispatch
          - Timeout churn that masks real issues

          When a task runs for 20-30 minutes, the system should:
          - Allow it to complete successfully
          - Log the completion time clearly
          - NOT treat it as a timeout failure
          - NOT re-dispatch it as stale

  allowedOutputs:
    - type: git-push
      repo: zen-brain1
      branch: main
      paths:
        - deploy/README.md
