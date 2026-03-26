# zen-lock / zen-flow Integration Decision

**Date:** 2026-03-26 (PHASE 30)
**Status:** Approved

## Architecture Decision

zen-brain's role in the combined platform:

- **zen-brain** = planner, router, model selection, task shaping
- **zen-flow** = execution engine for multi-step cluster jobs (Kubernetes-native)
- **zen-lock** = secret delivery, key custody, credential encryption

This decision is binding for zen-brain1's evolution. Current local useful-task runtime is NOT blocked on zen-flow integration.

## zen-lock: Use Now

**Verdict:** zen-lock should be integrated as a formal runtime dependency immediately.

### Why Now
- zen-brain already uses zen-lock patterns (see `docs/credential-rails.md`)
- Credential handling today is partially documented but not formally contracted
- zen-lock provides exactly what zen-brain needs: encrypted source-of-truth + runtime injection
- No runtime external fetching required — zen-lock's authoring-time model matches zen-brain's GitOps deployment

### Scope
- Provider credentials (LLM API keys, webhook tokens)
- Future flow/job secrets for zen-flow integration
- Worker service secrets (if any)

### Contract
- zen-brain cluster services read secrets from `/zen-lock/secrets` (ZenLock injection)
- Future zen-flow jobs read secrets from the same or equivalent injected mounts
- Source-of-truth: encrypted ZenLock CRDs in Git
- NO ad hoc credential paths in production

### Exclusions
- zen-lock does NOT fetch secrets from external providers at runtime
- zen-lock does NOT manage dynamic/rotating credentials
- zen-lock is NOT a general secrets platform

## zen-flow: Use Next (Not Now)

**Verdict:** zen-flow should be integrated as an optional cluster execution backend, NOT as a replacement for the current local runtime.

### Why Not Now
- Current local useful-task runtime is proven: 10/10, 24/7, internal scheduler
- zen-flow adds cluster orchestration power but does not improve single-node execution
- Migration cost is high; disruption risk to working 24/7 pipeline is unnecessary

### Where zen-flow Helps

| zen-brain Task Class | Current Path | Future zen-flow? | Why |
|---------------------|-------------|-------------------|-----|
| Hourly defects/bugs/stubs | Direct L1 → scheduler | No | Simple single-step, no DAG needed |
| Quad-hourly health scans | Direct L1 → scheduler | No | Parallel single-step, no dependencies |
| Daily full sweep | Direct L1 → scheduler | No | Same pattern, larger batch |
| Multi-step report pipelines | Not yet built | **Yes** | scan → summarize → validate → publish |
| Escalation workflows | Factory retry/escalation | **Yes** | L1 → L2 → manual approval → action |
| Maintenance flows | Not yet built | **Yes** | reindex → cleanup → publish |
| Approval-gated ops | Not yet built | **Yes** | patch gen → rollback prep → remediation |
| Artifact DAGs | Not yet built | **Yes** | extract → cluster → rollup → publish |
| Single-node useful tasks | Direct L1 | No | No cluster dependency needed |

### First zen-flow Candidate
Useful report batch DAG:
1. Gather raw evidence (scan all 10 task classes)
2. Summarize into executive brief
3. Validate artifact completeness
4. Publish bundle to evidence directory

This is a multi-step workflow with artifacts flowing between steps — exactly what zen-flow is built for.

## Scheduling: Native CRD, Not Raw CronJob

### Recommendation
Add a native `ScheduledJobFlow` CRD to zen-flow rather than wrapping Kubernetes CronJobs.

**Why native over CronJob bridge:**
- Scheduling stays inside the workflow product (not split between two systems)
- ConcurrencyPolicy, history limits, suspend/resume are first-class
- zen-brain submits `JobFlow` templates; zen-flow handles both execution and cadence
- No second controller to debug when schedules misfire

**Proposed CRD spec fields:**
```yaml
apiVersion: workflow.kube-zen.io/v1alpha1
kind: ScheduledJobFlow
metadata:
  name: daily-useful-sweep
spec:
  schedule: "0 6 * * *"
  timezone: "America/Toronto"
  suspend: false
  startingDeadlineSeconds: 600
  concurrencyPolicy: Forbid  # Allow | Forbid | Replace
  successfulHistoryLimit: 3
  failedHistoryLimit: 1
  jobFlowTemplate:
    # Standard JobFlow spec embedded here
    steps:
      - name: gather-evidence
        # ...
```

**Bootstrap path:** If CRD is not yet built, a temporary CronJob that creates JobFlow CRs is acceptable. But the target is native.

## 3-Phase Adoption Plan

### Phase A — Now
- Formalize zen-lock contract in zen-brain1 (`docs/05-OPERATIONS/`)
- Document this architecture decision (this doc)
- Identify first zen-flow candidate workflow (report batch DAG)
- Keep current local runtime as default

### Phase B — Next
- Implement one zen-flow-backed bounded workflow (report batch DAG)
- Keep current local runtime as default for single-node useful tasks
- Compare operational value

### Phase C — Later
- Add `ScheduledJobFlow` / `CronFlow` to zen-flow
- Allow zen-brain cluster mode to hand recurring DAG work to zen-flow
- Keep zen-brain as planner/router, not a second workflow engine
- Consider migrating multi-step workflows from local to cluster path

## Non-Goals
- Do NOT migrate current 24/7 useful tasks to zen-flow
- Do NOT add runtime external secret fetching
- Do NOT make zen-flow a prerequisite for current usefulness
- Do NOT add scheduling to zen-brain itself (it's the planner, not the executor)
