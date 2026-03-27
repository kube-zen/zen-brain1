# Overnight Operations Runbook

> **NOTE:** The primary inference runtime is **llama.cpp** (L1/L2). Ollama (L0) is fallback only. Ollama-related procedures in this runbook are for the fallback lane.

**ZB-024C: 5-Worker CPU-Only Operations**

This runbook describes how to operate zen-brain1 in unattended overnight mode with 5 parallel workers using qwen3.5:0.8b on CPU.

## System Configuration

**Proven configuration (ZB-024B):**
- Workers: 5 parallel goroutines
- Model: qwen3.5:0.8b ONLY
- Ollama endpoint: http://host.k3d.internal:11434
- Timeout: 2700s (45 minutes)
- Stale threshold: 50 minutes (> timeout)
- In-cluster Ollama: FORBIDDEN

## Healthy System Indicators

### Queue Health
```bash
# Check for stuck tasks
kubectl get braintasks -n zen-brain

# Expected:
# - All tasks in terminal state (Completed or Failed)
# - No tasks stuck in Running for > 50 minutes
# - No tasks stuck in Scheduled/Pending indefinitely
```

### Worker Logs - Normal Patterns
```bash
kubectl logs -n zen-brain deploy/foreman --tail=100 | grep "Worker.processOne"
```

**Healthy patterns:**
- `claimed successfully (was Scheduled, now Running)` - atomic claim working
- `completion update conflict (attempt 1/5), retrying...` - acceptable retry
- `status updated to Completed successfully` - task finished
- `not claimable (phase=Completed), skipping` - duplicate claim prevention

**Normal retry noise:**
```
[Worker.processOne] task zen-brain/task-123 claim conflict (attempt 1), retrying...
[Worker.processOne] task zen-brain/task-123 completion update conflict (attempt 1/5), retrying...
```
These are EXPECTED during parallel execution and indicate the conflict resolution is working.

### Degraded System Indicators

**Signs of problems:**
1. **Tasks stuck in Running > 50 minutes**
   - Indicates timeout not working or LLM hang
   - Check: `kubectl logs -n zen-brain deploy/foreman --since=1h | grep "zb024"`
   - Action: Check Ollama connectivity, consider restarting foreman

2. **Tasks stuck in Scheduled indefinitely**
   - Indicates dispatch/claim race condition
   - Check: `kubectl logs -n zen-brain deploy/foreman --since=30m | grep "not claimable"`
   - Action: This should NOT happen after ZB-024B fix. Investigate if seen.

3. **No logs from workers**
   - Indicates worker pool not running
   - Check: `kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman`
   - Action: Check pod status, restart deployment if needed

4. **High conflict rate (>50% of tasks)**
   - Indicates severe resource contention or reconciler issue
   - Check: `kubectl logs -n zen-brain deploy/foreman --since=1h | grep "conflict" | wc -l`
   - Action: Scale down to 2 workers temporarily, investigate reconciler

## Monitoring Commands

### Quick Health Check
```bash
# 1. Check task states
kubectl get braintasks -n zen-brain

# 2. Check recent completions
kubectl logs -n zen-brain deploy/foreman --since=1h | \
  grep "status updated to Completed successfully" | tail -20

# 3. Check for failures
kubectl logs -n zen-brain deploy/foreman --since=1h | \
  grep "execution failed\|status updated to Failed"

# 4. Check worker pool
kubectl logs -n zen-brain deploy/foreman --tail=50 | \
  grep "Worker.Start.*numWorkers"
```

### Overnight Monitoring
```bash
# Run every 4-6 hours
watch -n 14400 'kubectl get braintasks -n zen-brain && \
  kubectl logs -n zen-brain deploy/foreman --since=4h | \
  grep -E "status updated to (Completed|Failed)" | tail -10'
```

## Troubleshooting

### Issue: Tasks Stuck in Running

**Symptoms:**
- Task in Running phase for > 50 minutes
- No log activity for the task

**Investigation:**
```bash
# Get task details
kubectl describe braintask <task-name> -n zen-brain

# Check foreman logs for this task
kubectl logs -n zen-brain deploy/foreman | grep <task-name>

# Check Ollama connectivity
kubectl exec -n zen-brain deploy/foreman -- curl -s http://host.k3d.internal:11434/api/tags
```

**Resolution:**
1. Check if Ollama is responsive
2. Check if model is loaded: `ollama list` on host
3. If hung, delete the task to allow queue to drain
4. If persistent, restart foreman deployment

### Issue: High Conflict Rate

**Symptoms:**
- Logs show many "conflict, retrying" messages
- Tasks complete but with high retry counts

**Investigation:**
```bash
# Count conflicts in last hour
kubectl logs -n zen-brain deploy/foreman --since=1h | \
  grep "conflict" | wc -l

# Check reconciler activity
kubectl logs -n zen-brain deploy/foreman --since=1h | \
  grep "BrainTask.*scheduled" | wc -l
```

**Resolution:**
1. If conflict rate > 50%, scale to 2 workers temporarily:
   ```bash
   kubectl set image deploy/foreman foreman=zen-registry:5000/zen-brain:dev -n zen-brain
   kubectl set env deploy/foreman WORKERS=2 -n zen-brain  # if env var supported
   ```
2. Monitor for 30 minutes
3. If conflicts drop, gradually increase workers back to 5
4. If conflicts persist, check for reconciler bug

### Issue: Failed Tasks

**Symptoms:**
- Tasks in Failed state with error messages

**Investigation:**
```bash
# Get failure details
kubectl describe braintask <task-name> -n zen-brain | grep -A10 "Message:"

# Common failure: timeout
# Message: "LLM generation failed: context deadline exceeded"
# Solution: Task took too long, check if model is qwen3.5:0.8b

# Common failure: Ollama connectivity
# Message: "ollama client: Post ... connection refused"
# Solution: Check Ollama is running, check network path
```

**Resolution:**
1. Check if failure is transient (network, resource) vs permanent (bad task)
2. For transient failures, submit task again
3. For permanent failures, investigate task objective/constraints
4. Failed tasks do NOT block queue - other tasks continue

## Overnight Checklist

**Before leaving system unattended:**

- [ ] Ollama is running on host: `curl http://host.k3d.internal:11434/api/tags`
- [ ] Model qwen3.5:0.8b is loaded: `ollama list | grep qwen3.5:0.8b`
- [ ] Foreman pod is Running: `kubectl get pods -n zen-brain | grep foreman`
- [ ] No stuck tasks: `kubectl get braintasks -n zen-brain`
- [ ] Recent completions visible in logs: `kubectl logs -n zen-brain deploy/foreman --since=10m | grep "status updated to Completed"`
- [ ] Worker count is 5: `kubectl logs -n zen-brain deploy/foreman --tail=100 | grep "numWorkers=5"`

**Expected overnight behavior:**
- 5-10 tasks per hour (varies by task complexity)
- Some conflict retries are NORMAL (10-30%)
- All tasks reach terminal state (Completed or Failed)
- Queue depth decreases over time
- No tasks stuck in Running > 50 minutes

**In the morning:**
- Check for any Failed tasks
- Verify queue drained or processing normally
- Review logs for unexpected patterns
- Check disk space in workspaces: `kubectl exec -n zen-brain deploy/foreman -- df -h /tmp/zen-brain-factory`

## Scaling Guidance

**Current status:** Ready for unattended 5-worker run

**Next stage (10 workers):**
- Pre-requisite: 24-hour successful 5-worker run with <20% conflict rate
- Monitor: Queue throughput, completion rate, failure rate
- Validate: No resource exhaustion (CPU, memory, disk)

**DO NOT scale to 20 workers without:**
- Measured throughput at 10 workers
- Conflict rate < 10% at 10 workers
- Resource headroom confirmed (CPU < 80%, memory < 80%)
- 48+ hours successful operation at 10 workers

## Related Documentation

- [OLLAMA_08B_OPERATIONS_GUIDE.md](./OLLAMA_08B_OPERATIONS_GUIDE.md) - Ollama operations
- [OLLAMA_WARMUP_RUNBOOK.md](./OLLAMA_WARMUP_RUNBOOK.md) - Model warmup procedures
- [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md) - Local model policy

## Change History

- 2026-03-20: Initial version for ZB-024C 5-worker overnight operations
- Proven configuration: 5 workers, qwen3.5:0.8b, 45m timeout, atomic claim/completion
