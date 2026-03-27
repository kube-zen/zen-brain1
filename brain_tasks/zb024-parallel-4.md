> **NOTE:** This task references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

---
apiVersion: v1alpha1
kind: BrainTask
metadata:
  name: zb024-parallel-4
  labels:
    tranche: "ZB-024"
    simulation: "parallel-worker"
    task-class: documentation
spec:
  description: |
    Task 4 of 5-parallel simulation for ZB-024.

    Create an "Overnight Dogfood Checklist" runbook section in
    docs/05-OPERATIONS/ that provides operators with:
    - Pre-flight checks before starting unattended run
    - Live monitoring commands during unattended run
    - Stop/recovery commands if system becomes unhealthy
    - Clear criteria for healthy vs degraded state

    This is a bounded documentation task.

  template: "documentation-update"
  context:
    files:
      - path: docs/05-OPERATIONS/OVERNIGHT_DOGFOOD_CHECKLIST.md
        content: |
          # Overnight Dogfood Checklist (ZB-024)

          ## Pre-Flight Checks

          ### 1. System Health Check

          ```bash
          # Check all components are healthy
          kubectl get pods -n zen-brain
          ```

          Expected:
          - apiserver: Running
          - foreman: Running
          - Ollama pods: NONE (in-cluster is forbidden)
          - All pods in Running state

          ### 2. Local CPU Profile Check

          ```bash
          # Check local CPU profile is active
          kubectl logs -n zen-brain deploy/apiserver | grep "ZB-024: Local CPU inference profile active"
          ```

          Expected:
          - "local_cpu_profile=enabled" in logs
          - "provider=ollama" in logs
          - "model=qwen3.5:0.8b" in logs
          - "thinking=false" in logs

          ### 3. Timeout Configuration Check

          ```bash
          # Check timeout values are configured for 45m
          kubectl logs -n zen-brain deploy/apiserver | grep -E "timeout|keep_alive|stale"
          kubectl logs -n zen-brain deploy/foreman | grep -E "timeout|thinking"
          ```

          Expected:
          - "local_worker_timeout=2700" or "45m"
          - "llm_timeout=2700s" or "45m"
          - "keep_alive=45m"
          - "stale_threshold=50m" or "> 2700s"

          ### 4. Queue Check

          ```bash
          # Check queue is empty (no stuck tasks)
          kubectl logs -n zen-brain deploy/foreman | grep -E "queue depth|pending tasks"
          ```

          Expected:
          - Queue depth is low (0-5 tasks)
          - No tasks in "Pending" or "Running" state for > 50 minutes

          ## Live Monitoring Commands

          ### 1. Real-Time Task Status

          ```bash
          # Watch task status in real-time
          kubectl get braintasks -n zen-brain -w
          ```

          ### 2. Worker Activity

          ```bash
          # Check worker logs for activity
          kubectl logs -n zen-brain deploy/foreman -f | tail -50
          ```

          ### 3. LLM Gateway Activity

          ```bash
          # Check LLM gateway for qwen3.5:0.8b activity
          kubectl logs -n zen-brain deploy/apiserver -f | grep -E "Ollama|qwen3.5:0.8b|latency"
          ```

          ### 4. Queue Depth

          ```bash
          # Check queue depth over time
          watch "kubectl logs -n zen-brain deploy/foreman | grep queue depth"
          ```

          ## Stop/Recovery Commands

          ### Emergency Stop

          ```bash
          # Stop all zen-brain components
          kubectl scale deploy -n zen-brain --replicas=0 apiserver
          kubectl scale deploy -n zen-brain --replicas=0 foreman
          ```

          ### Graceful Stop

          ```bash
          # Stop gracefully (let tasks complete)
          # Mark existing tasks as failed with clear reason
          ```

          ### Recovery Procedure

          ```bash
          # Restart components after issue resolution
          kubectl rollout restart deploy -n zen-brain apiserver
          kubectl rollout restart deploy -n zen-brain foreman
          ```

          ## Health vs Degraded Criteria

          ### Healthy

          - All pods Running
          - Local CPU profile active
          - Queue depth < 10
          - No tasks in "Running" state > 50 minutes
          - LLM gateway responding with qwen3.5:0.8b
          - No errors or crashes in logs

          ### Degraded

          - Any pod NotReady or Crashing
          - Queue depth > 20 (backlog building)
          - Tasks in "Running" state > 60 minutes (stale)
          - LLM gateway timeout rate > 10%
          - Frequent retries (> 3 per task)
          - Escalation rate > 20% (many tasks escalating)

          ### Action Required

          - **Degraded:** Investigate logs, check resource usage, consider scaling
          - **Healthy:** Safe to leave unattended, check in 1-2 hours

          ## Quick Health Check Script

          ```bash
          #!/bin/bash
          # Quick health check for overnight runs

          echo "=== Overnight Health Check ==="
          echo

          # Check pods
          echo "1. Pod Status:"
          kubectl get pods -n zen-brain
          echo

          # Check local CPU profile
          echo "2. Local CPU Profile:"
          kubectl logs -n zen-brain deploy/apiserver | grep "ZB-024" || echo "  WARNING: No ZB-024 logs found"
          echo

          # Check queue
          echo "3. Queue Status:"
          kubectl logs -n zen-brain deploy/foreman | grep -E "queue depth|pending" | tail -5
          echo

          # Check for errors
          echo "4. Recent Errors:"
          kubectl logs -n zen-brain deploy/apiserver --tail=50 | grep -i error || echo "  No recent errors"
          kubectl logs -n zen-brain deploy/foreman --tail=50 | grep -i error || echo "  No recent errors"
          echo

          # Summary
          echo "=== Summary ==="
          if kubectl get pods -n zen-brain | grep -q Running; then
            echo "✓ System appears healthy"
          else
            echo "✗ System may be degraded - check logs"
          fi
          ```

  allowedOutputs:
    - type: git-push
      repo: zen-brain1
      branch: main
      paths:
        - docs/05-OPERATIONS/OVERNIGHT_DOGFOOD_CHECKLIST.md
