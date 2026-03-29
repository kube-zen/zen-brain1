> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only.

# ZB-022: Canonical Flow Design and Gap Inventory

**Status:** Architecture Complete
**Date:** 2026-03-19
**Executor:** AI with full codebase access

## Executive Summary

zen-brain1 has real CRD/control-plane substrate, real worker plane, and real API server. The problem is **fragmentation**, not missing architecture. This note defines the canonical dogfood path and identifies gaps.

## What Already Exists in Code

### ✅ CRD/Control Plane
- **BrainTask CRD** (`api/v1alpha1/braintask_types.go`)
  - Fields: WorkItemID, SessionID, SourceKey (Jira link)
  - Fields: WorkType, WorkDomain, Priority
  - Fields: Objective, AcceptanceCriteria, Constraints
  - Fields: EvidenceRequirement, SREDTags
  - Fields: QueueName (BrainQueue integration)
  - Phase: Pending, Scheduled, Running, Completed, Failed, Canceled
  - AssignedAgent: worker assignment

- **BrainQueue CRD** (`api/v1alpha1/brainqueue_types.go`)
  - Priority, MaxConcurrency, SessionAffinity
  - Phase: Ready, Paused, Draining
  - Depth (pending tasks), InFlight (running tasks)

- **BrainAgent CRD** (`api/v1alpha1/brainagent_types.go`)
  - Worker registration and capability tracking

### ✅ Worker Plane (Foreman)
- **Foreman Controller** (`cmd/foreman/main.go`)
  - Reconciles BrainTask resources
  - Configurable worker count (default: 2)
  - Runtime directory and workspace home (fail-closed)
  - Git worktree support
  - Session affinity
  - Gate and Guardian modes

- **Reconciler** (`internal/foreman/reconciler.go`)
  - Pending -> Scheduled -> Running -> Completed/Failed
  - Queue pause check
  - Guardian safety check
  - Gate admission
  - Dispatcher for worker assignment

- **Worker Pool** (`internal/foreman/worker.go`)
  - Session affinity (same session → same worker)
  - Round-robin mode
  - Least-loaded worker assignment
  - Queue depth tracking
  - Metrics: TasksDispatchedTotal, WorkerQueueDepth

- **FactoryTaskRunner** (`internal/foreman/factory_runner.go`)
  - Converts BrainTask to FactoryTaskSpec
  - Executes via Factory.ExecuteTask
  - Workspace and git worktree modes
  - Proof-of-work evidence recording

- **Factory** (`internal/factory/factory.go`)
  - Workspace management
  - Git worktree management
  - Bounded executor
  - Proof-of-work manager
  - LLM integration (not fully wired)

### ✅ LLM Integration
- **LLMGenerator** (`internal/factory/llm_generator.go`)
  - Provider configuration
  - Model override support
  - Temperature control
  - MaxTokens limit
  - Thinking mode support
  - Prompt templates for different work types

### ✅ Jira Integration
- **Jira Connector** (`internal/office/jira/connector.go`)
  - Jira API integration
  - Webhook support
  - Status, WorkType, Priority mappings
  - API token authentication
  - ZenOffice interface implementation

### ✅ Policy System
- **Policy Configuration** (`config/policy/`)
  - roles.yaml - 4 AI agent roles
  - tasks.yaml - 11 task classes
  - providers.yaml - 3 providers, 6 models
  - routing.yaml - 4 routing strategies
  - prompts.yaml - system prompts
  - chains.yaml - 5 task execution chains

- **Policy Enforcement** (`internal/factory/policy_enforcement.go`)
  - Fail-closed validation
  - No synthetic defaults
  - Must create real repo files
  - Target path validation

## Fragmented Path

### Jira -> BrainTask Ingestion (ZB-023)
**Status:** ❌ MISSING

**What exists:**
- Jira connector can fetch issues
- self_improvement.go has a loop for Jira tasks
- BrainTask CRD has SourceKey field for Jira key

**What's missing:**
- No code path that:
  1. Fetches Jira issues
  2. Converts to BrainTask
  3. Creates BrainTask CR
  4. Ensures idempotency (no duplicate BrainTasks)

**Gap:** Jira issues stay in self_improvement.go loop and never enter BrainTask/Foreman execution plane.

### BrainTask -> Foreman Execution (ZB-024)
**Status:** ✅ WIRED (but LLM not fully integrated)

**What exists:**
- BrainTask CRD
- Foreman reconciler
- Worker pool with session affinity
- FactoryTaskRunner
- Factory workspace/git worktree execution
- LLMGenerator (not wired to policy YAML)

**What's missing:**
- LLMGenerator is not wired to policy YAML configuration
- No provider/model selection based on task_class/role from policy
- No enforcement that local Ollama uses qwen3.5:0.8b only

**Gap:** Factory execution works but doesn't use policy-driven provider/model selection.

### Result -> Jira Feedback (ZB-025)
**Status:** ❌ MISSING

**What exists:**
- FactoryTaskRunner captures result
- BrainTask has status fields (Phase, Message)
- Jira connector can update issues

**What's missing:**
- No code path that:
  1. Captures FactoryTaskRunner result
  2. Updates BrainTask status
  3. Converts result to Jira comment/update
  4. Reports back to Jira issue

**Gap:** Results stay in BrainTask CR and never flow back to Jira.

### Observability (ZB-026)
**Status:** ⚠️ PARTIAL

**What exists:**
- Foreman metrics: TasksDispatchedTotal, WorkerQueueDepth, ReconcileDurationSeconds
- BrainTask status fields
- BrainQueue depth/inFlight

**What's missing:**
- No operator runbook for "what are workers doing right now?"
- No way to see active tasks quickly
- No failure mode distinction (ingestion vs queue vs execution vs model)
- No provider/model usage breakdown
- No local Ollama latency tracking

**Gap:** Metrics exist but no operator-friendly visibility.

## Canonical Dogfood Path

### Flow Definition

```
Jira Issue (ZB-022)
   ↓
[Jira Connector] Fetch issue
   ↓
[Ingestion Service] Convert to BrainTask CR
   ↓
[BrainTask CR] Pending phase
   ↓
[Foreman Reconciler] Schedule task
   ↓
[Worker Pool] Dispatch to worker
   ↓
[FactoryTaskRunner] Execute task
   ↓
[Factory + LLMGenerator] Use policy YAML for provider/model
   ↓
[Result Capture] Capture outcome
   ↓
[BrainTask CR] Update status (Completed/Failed)
   ↓
[Jira Connector] Update Jira issue with result
   ↓
Jira Issue (resolved/commented)
```

### Components in Canonical Path

1. **Jira Connector** (exists) - Fetch issues
2. **Ingestion Service** (MISSING) - Create BrainTask CR from Jira issue
3. **BrainTask CR** (exists) - Task definition
4. **Foreman Reconciler** (exists) - Schedule and dispatch
5. **Worker Pool** (exists) - Execute tasks
6. **FactoryTaskRunner** (exists) - Run task via Factory
7. **Factory + LLMGenerator** (exists, needs wiring) - Execute with policy
8. **Result Capture** (exists) - Capture outcome
9. **Jira Feedback** (MISSING) - Update Jira with result

## Gap List with Severity

### CRITICAL (Blocks Dogfood)
1. **Jira -> BrainTask Ingestion** (ZB-023) - MISSING
   - Severity: CRITICAL
   - Files needed: `internal/ingestion/jira_to_braintask.go`
   - Effort: 2-4 hours

2. **Result -> Jira Feedback** (ZB-025) - MISSING
   - Severity: CRITICAL
   - Files needed: `internal/feedback/braintask_to_jira.go`
   - Effort: 2-4 hours

### HIGH (Blocks Policy Integration)
3. **Policy YAML -> LLMGenerator Integration** (ZB-024) - PARTIAL
   - Severity: HIGH
   - Files needed: Update `internal/factory/llm_generator.go`
   - Effort: 4-6 hours

4. **Local Ollama Clamp Enforcement** (ZB-024) - MISSING
   - Severity: HIGH
   - Files needed: Update `internal/factory/llm_generator.go`
   - Effort: 1-2 hours

### MEDIUM (Blocks Observability)
5. **Operator Observability Runbook** (ZB-026) - MISSING
   - Severity: MEDIUM
   - Files needed: `docs/05-OPERATIONS/worker_observability.md`
   - Effort: 2-3 hours

6. **Queue/Task Status CLI** (ZB-026) - MISSING
   - Severity: MEDIUM
   - Files needed: `cmd/zen-brain/worker_status.go`
   - Effort: 3-4 hours

### LOW (Nice to Have)
7. **Provider/Model Usage Metrics** (ZB-026) - MISSING
   - Severity: LOW
   - Files needed: Update `internal/foreman/metrics.go`
   - Effort: 2-3 hours

8. **Failure Mode Classification** (ZB-026) - MISSING
   - Severity: LOW
   - Files needed: Update BrainTask status fields
   - Effort: 1-2 hours

## Minimum Code Changes to Unify

### Phase 1: Wire the Canonical Path (8-12 hours)
1. Create `internal/ingestion/jira_to_braintask.go` (ZB-023)
2. Create `internal/feedback/braintask_to_jira.go` (ZB-025)
3. Update `internal/factory/llm_generator.go` to use policy YAML (ZB-024)
4. Add Ollama clamp enforcement (ZB-024)

### Phase 2: Observability (4-6 hours)
5. Create `docs/05-OPERATIONS/worker_observability.md` (ZB-026)
6. Create `cmd/zen-brain/worker_status.go` CLI (ZB-026)

### Phase 3: Scale Testing (4-8 hours)
7. Test with 1 worker (ZB-027 Stage 1)
8. Test with 3 workers (ZB-027 Stage 2)
9. Test with 5 workers (ZB-027 Stage 3)

## Execution Map: Jira to Result

```
1. Jira Issue Created (PROJ-123)
   - Type: Task, Story, Bug
   - Labels: zen-brain-self-work
   - Status: To Do

2. Ingestion Service (NEW)
   - Poll Jira for issues with label
   - Convert to BrainTask:
     * SourceKey: PROJ-123
     * WorkType: map from Jira type
     * WorkDomain: map from Jira component
     * Objective: Jira summary + description
     * AcceptanceCriteria: Jira acceptance criteria
   - Create BrainTask CR (idempotent by SourceKey)
   - Set phase: Pending

3. Foreman Reconciler
   - Watch BrainTask CRs
   - Pending -> Scheduled
   - Check BrainQueue (if QueueName set)
   - Check Guardian (safety)
   - Check Gate (admission)
   - Dispatch to Worker Pool

4. Worker Pool
   - Receive task (session affinity)
   - Update BrainTask: Running
   - Call FactoryTaskRunner.Run()

5. FactoryTaskRunner
   - Convert BrainTask to FactoryTaskSpec
   - Call Factory.ExecuteTask()
   - Capture result
   - Return TaskRunOutcome

6. Factory + LLMGenerator
   - Load policy YAML
   - Select provider/model by task_class/role
   - Clamp to qwen3.5:0.8b if local Ollama
   - Execute LLM generation
   - Create files in workspace/git worktree
   - Generate proof-of-work

7. Worker Pool
   - Update BrainTask: Completed/Failed
   - Record metrics

8. Feedback Service (NEW)
   - Watch BrainTask status changes
   - Completed/Failed -> Jira update:
     * Add comment with result summary
     * Attach proof-of-work link
     * Update status (Done/Failed)
     * Link to commit/PR if available

9. Jira Issue Updated
   - Status: Done/Failed
   - Comment: Result summary
   - Attachment: Proof-of-work
   - Link: Commit/PR
```

## Success Criteria

- ✅ Canonical flow is explicit
- ✅ No ambiguity about which queue/task path is authoritative (BrainTask/Foreman)
- ✅ Gap list with severity and effort estimates
- ✅ Execution map from Jira to result
- ✅ Minimum code changes identified

## Next Steps

1. **ZB-023:** Implement Jira -> BrainTask ingestion service
2. **ZB-024:** Wire policy YAML to LLMGenerator + Ollama clamp
3. **ZB-025:** Implement BrainTask -> Jira feedback service
4. **ZB-026:** Create operator observability runbook and CLI
5. **ZB-027:** Controlled scale-up (1 -> 3 -> 5 workers)
6. **ZB-028:** Execute real dogfood tasks through canonical path
