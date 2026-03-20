# ZB-025: 24/7 Operator Controls

**Status:** Design Complete - Awaiting valid Jira API token

---

## Overview

Operator commands for unattended 5-worker dogfood operations on Jira-sourced tasks.

All commands use `zen-brain office` CLI and require proper Jira connectivity.

## Prerequisites

```bash
# 1. Valid Jira API token in secrets file
cat ~/.zen-brain/secrets/jira.yaml
# Should contain JIRA_API_TOKEN with ATATT3... format

# 2. Verify connectivity
./zen-brain office doctor
# Should show: "API reachability: ok"

# 3. Check foreman is running
kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman
# Should be Running with 5 workers
```

## Commands

### 1. Start Unattended Run

**Command:**
```bash
# Start unattended dogfood ingestion and execution
zen-brain office start-dogfood
```

**Behavior:**
- Ingests Jira issues matching dogfood query
- Creates BrainTask objects for allowed task classes
- Foreman executes tasks with 5 workers
- Runs until stop condition or manual stop
- Logs all activities to foreman pod

**Example:**
```bash
# Start overnight run
zen-brain office start-dogfood \
  --jql 'project = ZB AND labels in ("zen-brain-dogfood")' \
  --max-queue-depth 10 \
  --max-runtime 8h
```

### 2. Status Check

**Command:**
```bash
# Check current operational status
zen-brain office status
```

**Shows:**
- Jira connectivity status
- Active workers count
- Queue depth
- In-flight tasks
- Recent completions (last 10)
- Recent failures (last 5)
- Stuck task count
- Queue health (OK/degraded/blocked)

**Example Output:**
```
=== Operator Status ===

Jira Connectivity: ✅ OK
  - URL: https://zen-mesh.atlassian.net
  - Project: ZB
  - Last check: 30s ago

Workers: ✅ 5 active
  - Pool size: 5
  - In-flight: 3

Queue: ✅ OK
  - Depth: 7
  - Max depth: 10
  - Avg wait time: 2m 15s

Tasks:
  - Active: 3 (Running)
  - Completed (last hour): 12
  - Failed (last hour): 1
  - Stuck (>50m): 0

Recent Completions:
  ✅ jira-ZB-456 (docs_update) - 12m 34s
  ✅ jira-ZB-457 (runbook_update) - 8m 12s
  ✅ jira-ZB-458 (config_cleanup) - 15m 45s

Recent Failures:
  ❌ jira-ZB-459 (docs_update) - 3m 22s - STEP_EXECUTION_FAILED
```

### 3. Stop / Pause

**Command:**
```bash
# Stop unattended run gracefully
zen-brain office stop-dogfood
```

**Behavior:**
- Stops new task ingestion
- Allows in-flight tasks to complete
- Waits up to 15 minutes for graceful shutdown
- Exits when queue drains or timeout
- Reports final statistics

**Force Stop (Immediate):**
```bash
# Stop immediately (may leave tasks in Running state)
zen-brain office stop-dogfood --force
```

### 4. Recovery from Degraded State

**Command:**
```bash
# Recover from degraded/blocked queue
zen-brain office recover
```

**Behavior:**
- Identifies stuck tasks (>50m in Running)
- Identifies tasks with excessive retries (>5)
- Takes action:
  - Delete stuck tasks (they'll be retried)
  - Scale workers to 2 if conflict rate >50%
  - Force-refresh Jira connection
  - Drain queue to clean state
- Reports recovery actions taken

**Example Output:**
```
=== Recovery Actions ===

Found 1 stuck task (>50m):
  - jira-ZB-501 (stuck 67m) -> DELETED

High conflict rate (65%) detected:
  - Scaling workers: 5 -> 2

Refreshing Jira connection...
  ✅ Connection refreshed

Queue draining:
  ✅ 3 tasks cancelled
  ✅ Queue clean

Recommendation:
  Monitor for 30 minutes at 2 workers
  If conflict rate <20%, scale back to 5
```

### 5. Queue Query

**Command:**
```bash
# Query queue state in detail
zen-brain office queue-query
```

**Shows:**
- All BrainTasks with ZB-025 source
- Grouped by status (Running, Completed, Failed)
- Sort by age (oldest first)
- Shows retry counts
- Shows execution time

**Example Output:**
```
=== Queue Query ===

Running (3):
  jira-ZB-502 (docs_update) - Age: 12m - Retries: 0
  jira-ZB-503 (runbook_update) - Age: 8m - Retries: 1
  jira-ZB-504 (config_cleanup) - Age: 5m - Retries: 0

Completed (12):
  jira-ZB-495 (docs_update) - Duration: 14m 22s - Retries: 1
  jira-ZB-496 (runbook_update) - Duration: 9m 34s - Retries: 0
  jira-ZB-497 (config_cleanup) - Duration: 11m 12s - Retries: 0

Failed (1):
  jira-ZB-490 (docs_update) - Duration: 3m 12s - Retries: 2
  - Error: STEP_EXECUTION_FAILED (exit code 2)
```

## Monitoring During Run

### Quick Health Check

```bash
# Run every 15-30 minutes during overnight
watch -n 900 'zen-brain office status'
```

### Detailed Queue Monitoring

```bash
# Monitor queue depth and task ages
watch -n 600 'zen-brain office queue-query | grep -A20 "Running"'
```

### Check for Degradation

```bash
# Check for stuck tasks or high conflict rate
zen-brain office recover --check-only
```

## Stop Conditions

### Automatic Stop

Run stops automatically when:

1. **Max runtime reached** (--max-runtime parameter)
2. **Queue depth exceeds limit** (--max-queue-depth exceeded)
3. **Stuck task threshold exceeded** (>3 tasks >50m)
4. **Failure rate too high** (>30% of tasks failing)
5. **Jira connectivity lost** (3 consecutive failed checks)

### Manual Stop

```bash
# Graceful stop
zen-brain office stop-dogfood

# Force stop
zen-brain office stop-dogfood --force
```

## Escalation Paths

### When to Escalate

Escalate to human operator when:

1. **Queue blocked** for >30 minutes
2. **Stuck tasks** >5 for >50 minutes
3. **Failure rate** >50% for >30 minutes
4. **Jira connectivity** lost for >10 minutes
5. **Forbidden task classes** detected in ingestion

### Escalation Actions

1. **Stop new ingestion**
2. **Send notification** (via configured channel)
3. **Report status** with:
   - Current queue state
   - Recent failures
   - Conflict rate
   - Worker count
4. **Wait for operator action**

## Operator Commands Summary

| Command | Purpose | Frequency |
|---------|---------|-----------|
| `zen-brain office start-dogfood` | Start unattended run | Daily (evening) |
| `zen-brain office status` | Quick health check | Every 15-30 min |
| `zen-brain office stop-dogfood` | Graceful shutdown | Manual or auto |
| `zen-brain office recover` | Fix degraded state | When needed |
| `zen-brain office queue-query` | Detailed queue state | When investigating |

## Troubleshooting

### Issue: Won't Start

```bash
# Check prerequisites
zen-brain office doctor

# Check foreman
kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman

# Check for stuck tasks
kubectl get braintasks -n zen-brain | grep -E "Running|Pending"
```

### Issue: No Tasks Processing

```bash
# Check queue depth
zen-brain office queue-query

# Check workers
kubectl logs -n zen-brain deploy/foreman --tail=50 | grep "Worker.Start"

# Check Jira ingestion
zen-brain office status | grep "Ingestion"
```

### Issue: Many Failures

```bash
# Check recent failures
zen-brain office queue-query | grep -A5 "Failed"

# Check foreman logs for errors
kubectl logs -n zen-brain deploy/foreman --since=30m | grep -E "execution failed|STEP_EXECUTION_FAILED"

# Check Jira issue quality
zen-brain office search "status = Failed AND labels in (zen-brain-dogfood)" | head -10
```

## Change History

- 2026-03-20: Initial version for ZB-025 24/7 operator controls
