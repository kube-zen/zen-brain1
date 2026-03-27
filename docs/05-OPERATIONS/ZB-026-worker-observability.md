> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

> **NOTE:** This document references Ollama as it existed during development. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.



**Status:** Operator Runbook
**Date:** 2026-03-19
**Version:** 1.0

## Overview

This runbook provides operators with visibility into zen-brain1 worker plane. You can answer "what are workers doing right now?" and "why is a task blocked?" without reading raw pod logs.

## Quick Status Check

### Check Worker Pool Status

```bash
# Get overall worker status
kubectl get braintasks -n zen-brain

# Get queue depth
kubectl get brainqueue -n zen-brain

# Get active workers
kubectl get brainagents -n zen-brain
```

### Check Task Status

```bash
# List all tasks by phase
kubectl get braintasks -n zen-brain -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,SESSION:.spec.sessionID,AGE:.metadata.creationTimestamp

# Watch task status in real-time
watch kubectl get braintasks -n zen-brain

# Get failed tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Failed
```

## Queue Visibility

### Queue Depth and In-Flight Tasks

```bash
# Get queue metrics
kubectl get brainqueue dogfood -n zen-brain -o yaml

# Check queue depth (pending tasks)
kubectl get brainqueue dogfood -n zen-brain -o jsonpath='{.status.depth}'

# Check in-flight tasks (running tasks)
kubectl get brainqueue dogfood -n zen-brain -o jsonpath='{.status.inFlight}'

# Check queue phase (Ready/Paused/Draining)
kubectl get brainqueue dogfood -n zen-brain -o jsonpath='{.status.phase}'
```

### Queue Management

```bash
# Pause queue (stops new task dispatch)
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Paused"}}'

# Resume queue
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Ready"}}'

# Drain queue (complete in-flight, no new dispatch)
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Draining"}}'
```

## Task Lifecycle Visibility

### Pending Tasks

```bash
# List pending tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Pending

# Get pending task details
kubectl describe braintask <task-name> -n zen-brain

# Check why task is pending (queue paused, guardian blocked, gate denied)
kubectl get braintask <task-name> -n zen-brain -o jsonpath='{.status.conditions[?(@.type=="Scheduled")].message}'
```

### Running Tasks

```bash
# List running tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Running

# Get worker assignment
kubectl get braintask <task-name> -n zen-brain -o jsonpath='{.status.assignedAgent}'

# Check task age (stale tasks)
kubectl get braintasks -n zen-brain --field-selector status.phase=Running -o custom-columns=NAME:.metadata.name,AGE:.metadata.creationTimestamp,ASSIGNED:.status.assignedAgent
```

### Completed Tasks

```bash
# List completed tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Completed

# Get result summary
kubectl get braintask <task-name> -n zen-brain -o jsonpath='{.status.message}'

# Check completion time
kubectl get braintask <task-name> -n zen-brain -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,AGE:.metadata.creationTimestamp
```

### Failed Tasks

```bash
# List failed tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Failed

# Get failure reason
kubectl describe braintask <task-name> -n zen-brain

# Check error message
kubectl get braintask <task-name> -n zen-brain -o jsonpath='{.status.message}'
```

## Failure Mode Classification

### Ingestion Failure

**Symptoms:**
- Task stuck in Pending phase
- No SourceKey assigned
- Queue depth increasing

**Diagnosis:**
```bash
# Check Jira connector logs
kubectl logs -n zen-brain deployment/zen-brain-apiserver | grep -i "jira\|ingestion"

# Check BrainTask creation
kubectl get braintasks -n zen-brain --field-selector status.phase=Pending -o yaml | grep -A5 "metadata:"
```

**Resolution:**
- Check Jira API token
- Check Jira label filter
- Check ingestion service logs

### Queue/Scheduler Failure

**Symptoms:**
- Tasks stuck in Pending
- Queue phase is Ready
- No worker assignment

**Diagnosis:**
```bash
# Check queue status
kubectl get brainqueue -n zen-brain -o yaml

# Check Foreman logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "reconcile\|schedule"

# Check reconciler metrics
kubectl exec -n zen-brain deployment/zen-brain-foreman -- curl localhost:8080/metrics | grep reconcile_duration
```

**Resolution:**
- Check Foreman deployment status
- Check reconciler errors in logs
- Check Guardian/Gate configuration

### Execution Failure

**Symptoms:**
- Tasks in Running phase for too long
- Worker crashes
- No completion

**Diagnosis:**
```bash
# Check worker logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "worker\|execute"

# Check Factory logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "factory\|llm"

# Check task execution time
kubectl get braintasks -n zen-brain --field-selector status.phase=Running -o custom-columns=NAME:.metadata.name,AGE:.metadata.creationTimestamp
```

**Resolution:**
- Check LLM provider configuration
- Check Ollama service (if local)
- Check workspace/git worktree configuration
- Check timeout settings

### Model/Provider Failure

**Symptoms:**
- LLM generation errors
- Timeout errors
- Invalid model errors

**Diagnosis:**
```bash
# Check LLM logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "llm\|provider\|model"

# Check Ollama status (if local)
kubectl exec -n zen-brain deployment/zen-brain-foreman -- curl http://host.docker.internal:11434/api/tags

# Check provider configuration
kubectl get configmap zen-brain-policy -n zen-brain -o yaml | grep -A10 "providers:"
```

**Resolution:**
- Check Ollama is running on host
- Check model is available: `ollama list`
- Check policy YAML provider configuration
- Enforce Ollama clamp: verify model is qwen3.5:0.8b

## Metrics and Dashboards

### Prometheus Metrics

```bash
# Get Foreman metrics
kubectl exec -n zen-brain deployment/zen-brain-foreman -- curl localhost:8080/metrics

# Key metrics:
# - tasks_dispatched_total
# - worker_queue_depth
# - reconcile_duration_seconds
```

### Grafana Dashboard (if configured)

```bash
# Port-forward Grafana
kubectl port-forward -n monitoring svc/grafana 3000:80

# Open http://localhost:3000
# Dashboard: Zen-Brain Worker Pool
```

## Worker Pool Management

### Scale Workers Up/Down

```bash
# Scale Foreman deployment (adjust worker count)
kubectl scale deployment zen-brain-foreman -n zen-brain --replicas=3

# Update worker count in deployment
kubectl patch deployment zen-brain-foreman -n zen-brain --type json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":["--workers=5"]}]'
```

### Check Worker Utilization

```bash
# Get worker pool status
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "worker.*started\|dispatched"

# Check worker load distribution (session affinity)
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep "session-affinity\|worker.*channel"
```

## Jira Integration Visibility

### Check Ingestion Status

```bash
# Check Jira ingestion logs
kubectl logs -n zen-brain deployment/zen-brain-apiserver | grep -i "ingestion\|jira"

# Check BrainTask creation from Jira
kubectl get braintasks -n zen-brain -l zen.kube-zen.com/source=jira
```

### Check Feedback Status

```bash
# Check Jira feedback logs
kubectl logs -n zen-brain deployment/zen-brain-apiserver | grep -i "feedback\|jira"

# Check reported tasks
kubectl get braintasks -n zen-brain -l zen.kube-zen.com/reported-to-jira=true
```

## Common Issues

### Issue: Tasks Stuck in Pending

**Symptoms:**
- Many tasks in Pending phase
- Queue depth increasing
- No tasks being dispatched

**Diagnosis:**
```bash
# Check queue status
kubectl get brainqueue -n zen-brain

# Check Foreman deployment
kubectl get deployment zen-brain-foreman -n zen-brain

# Check reconciler logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | tail -50
```

**Resolution:**
1. Check if queue is paused
2. Check if Foreman is running
3. Check reconciler errors
4. Check Guardian/Gate configuration

### Issue: Workers Not Picking Up Tasks

**Symptoms:**
- Tasks in Pending
- No Running tasks
- Queue depth increasing

**Diagnosis:**
```bash
# Check worker pool
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep "worker.*started"

# Check dispatcher
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep "dispatch"

# Check queue depth
kubectl get brainqueue -n zen-brain -o jsonpath='{.status.depth}'
```

**Resolution:**
1. Verify workers are started
2. Check dispatcher errors
3. Scale up workers if needed
4. Check session affinity configuration

### Issue: Tasks Failing Quickly

**Symptoms:**
- Many Failed tasks
- No Completed tasks
- Error messages in logs

**Diagnosis:**
```bash
# Get failed task details
kubectl get braintasks -n zen-brain --field-selector status.phase=Failed -o yaml

# Check Factory logs
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "error\|fail"

# Check LLM provider
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep -i "llm\|provider"
```

**Resolution:**
1. Check LLM provider configuration
2. Check Ollama service (if local)
3. Check workspace configuration
4. Check timeout settings
5. Review task constraints

## Operational Procedures

### Pause Worker Pool

```bash
# Pause queue (stops new dispatch)
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Paused"}}'

# Wait for in-flight tasks to complete
kubectl get braintasks -n zen-brain --field-selector status.phase=Running -w
```

### Resume Worker Pool

```bash
# Resume queue
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Ready"}}'

# Verify tasks are being dispatched
kubectl logs -n zen-brain deployment/zen-brain-foreman -f | grep "dispatch"
```

### Drain Worker Pool

```bash
# Set queue to draining
kubectl patch brainqueue dogfood -n zen-brain --type merge -p '{"status":{"phase":"Draining"}}'

# Monitor in-flight tasks
kubectl get braintasks -n zen-brain --field-selector status.phase=Running -w
```

### Scale Workers

```bash
# Scale to 5 workers
kubectl patch deployment zen-brain-foreman -n zen-brain --type json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/args","value":["--workers=5"]}]'

# Verify workers started
kubectl logs -n zen-brain deployment/zen-brain-foreman | grep "worker.*started"
```

## Summary

**What you can see:**
- ✅ Queue depth (pending tasks)
- ✅ In-flight tasks (running tasks)
- ✅ Task status (Pending/Running/Completed/Failed)
- ✅ Worker assignment
- ✅ Failure reasons
- ✅ Jira ingestion status
- ✅ Jira feedback status

**What you can do:**
- ✅ Pause/Resume queue
- ✅ Drain queue
- ✅ Scale workers up/down
- ✅ Diagnose failure modes
- ✅ Check provider/model configuration

**Failure modes distinguishable:**
- ✅ Ingestion failure (Jira/API)
- ✅ Queue/Scheduler failure (Foreman/Reconciler)
- ✅ Execution failure (Factory/Worker)
- ✅ Model/Provider failure (LLM/Ollama)

## Next Steps

1. **ZB-027:** Test controlled scale-up (1 -> 3 -> 5 workers)
2. **ZB-028:** Execute real dogfood tasks through canonical path
3. **Monitor:** Watch metrics and adjust configuration as needed
