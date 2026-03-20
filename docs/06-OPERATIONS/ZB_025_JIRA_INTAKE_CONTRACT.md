# ZB-025: Jira Intake Contract

**Status:** Design Complete - Awaiting valid Jira API token

---

## Purpose

Defines the bounded, safe contract for ingesting Jira issues into Zen-Brain BrainTasks for unattended 5-worker dogfood operations.

## Allowed Task Classes

### Safe Task Classes (Initial Scope)

| Class | Description | Safe Because |
|-------|-------------|-------------|
| `docs_update` | Update README, runbook, or guide documentation | Files are read-only, scoped to specific docs/ paths |
| `runbook_update` | Add or update operational procedures | Bounded to docs/05-OPERATIONS/, no infra changes |
| `config_cleanup` | Update config files with clear intent | Well-defined paths, reviewable changes |
| `review_summarization` | Summarize code changes or design docs | Read-only analysis, output is comment/summary |
| `bounded_refactor` | Refactor within single package with explicit scope | No API changes, no architecture mutations |

### Forbidden Task Classes (NOT Allowed Initially)

| Class | Why Forbidden | When to Allow |
|-------|----------------|---------------|
| `architecture_work` | Broad infra changes, undefined scope | After proven architecture patterns |
| `infra_mutation` | Risky infrastructure changes, hard to roll back | After proven infra safety |
| `dangerous_automation` | Auto-deploys, DB migrations, etc. | Never - manual approval only |
| `unbounded_codegen` | Changes across many files/packages | After proven file scope safety |

## Jira Query / Filter

### Initial Filter (Dogfood Only)

```yaml
# JQL query for safe dogfood tasks
project = "ZB"
AND (
  labels in ("zen-brain-dogfood", "dogfood")
  OR component in ("docs", "operations", "documentation")
)
AND (
  issuetype in ("Task", "Sub-task")
  OR component = "documentation"
)
AND status in ("To Do", "Backlog", "In Progress")
```

### Label-Based Intake

| Label | Meaning | Allowed Classes |
|-------|---------|----------------|
| `zen-brain-dogfood` | Safe for unattended dogfood | All safe classes |
| `zen-brain-review` | Requires human review after AI work | review_summarization only |
| `zen-brain-test-only` | Test integration, no real changes | docs_update, config_cleanup |

## Output Scope Limits

### Allowed File Paths

All changes must be scoped under:

```yaml
allowed_paths:
  - "docs/**"                    # Documentation only
  - "config/policy/**"            # Policy definitions
  - "config/profiles/**"           # Runtime profiles
  - "internal/foreman/**"          # Foreman execution plane only
  - "scripts/ci/**"               # CI gates only
```

### Forbidden Paths

Never modify without explicit approval:

```yaml
forbidden_paths:
  - "api/v1alpha1/**"           # CRD definitions
  - "internal/llm/**"              # LLM providers
  - "internal/integration/**"       # External integrations
  - "deploy/ollama-in-cluster/**" # In-cluster Ollama (FORBIDDEN)
  - "charts/**"                    # Helm charts
```

## File Change Limits

| Operation | Limit | Rationale |
|-----------|-------|-----------|
| Files added | ≤ 5 | Small, focused changes |
| Files modified | ≤ 10 | Bounded scope |
| Files deleted | 0 | No deletions in dogfood |
| Lines changed | ≤ 500 | Reasonable for one session |
| Test files | Unlimited | Tests always allowed |

## Task Metadata Requirements

### Required Labels

All dogfood issues MUST have:

- `zen-brain-dogfood` - enables auto-ingestion
- Task class label: `docs_update`, `runbook_update`, etc.

### Required Fields

| Field | Requirement | Example |
|-------|-------------|---------|
| Summary | Must describe intent clearly | "Update runbook with 45m timeout notes" |
| Description | Must include objective and constraints | "Update OLLAMA_WARMUP_RUNBOOK.md to document 45m timeout expectations. DO NOT modify any other files." |
| Component | Must be one of allowed types | "documentation", "operations", "foreman" |
| Priority | Must be Medium or Low | Prevents high-priority risky work |

### Forbidden Fields

| Field | Why Forbidden |
|-------|----------------|
| ` Epic Link` | Epic work is out of scope |
| ` Sprint` | Sprint planning is manual |
| Custom fields for | Unvalidated schema fields |

## BrainTask Mapping

### Jira -> BrainTask

```yaml
source_key: "ZB-123"                # Jira issue key
metadata:
  labels:
    tranche: "ZB-025"
    source: "jira"
    source_key: "ZB-123"
    task_class: "docs_update"
    dogfood: "true"
spec:
  title: <Jira Summary>
  objective: <Jira Description>
  sessionID: "jira-ZB-123"       # Link back to Jira
  workDomain: "infrastructure"        # Fixed for all dogfood
  workType: "documentation"           # Mapped from class
  workItemID: "ZB-123"
  queueName: "default"
  estimatedCostUSD: 0.01
```

### BrainTask -> Jira Feedback

#### Success Feedback

```yaml
jira_comment: |
  [zen-brain | agent:factory | model:qwen3.5:0.8b | session:jira-ZB-123 | task:task-xyz | 2026-03-20T18:45:00Z]

  # Proof-of-Work: ZB-123

  ## Execution Summary
  - **Status**: Success
  - **Duration**: 12m 34s
  - **Files Changed**: 3
  - **Tests Added**: 2

  ## Work Done
  - Updated docs/05-OPERATIONS/OVERNIGHT_RUNBOOK.md with 45m timeout guidance
  - Added conflict retry examples
  - Updated troubleshooting section

  ## Evidence
  - Commit hash: abc123def456
  - Diff: https://github.com/kube-zen/zen-brain1/commit/abc123

  ## Next Action
  Ready for review and merge.
```

#### Failure Feedback

```yaml
jira_comment: |
  [zen-brain | agent:factory | model:qwen3.5:0.8b | session:jira-ZB-123 | task:task-xyz | 2026-03-20T18:45:00Z]

  # Execution Failed: ZB-123

  ## Failure Summary
  - **Status**: Failed
  - **Duration**: 3m 12s
  - **Error**: Step execution failed (exit code 2)

  ## Error Details
  ```
  [STEP_EXECUTION_FAILED] command execution failed (step: step-1) (exit code: 2) (cause: exit status 2)
  ```

  ## Retry Information
  - **Attempt**: 1 of 2
  - **Next Action**: Retrying automatically

  ## No Files Modified
  No changes were committed due to execution failure.
```

## Safety Gates

### Pre-Ingestion Gates

1. **Label Check**: Issue must have `zen-brain-dogfood` label
2. **Class Check**: Task class must be in allowed list
3. **Component Check**: Component must be allowed (docs, operations, foreman)
4. **Priority Check**: Priority must be Medium or Low
5. **Epic Check**: Must not have Epic Link set
6. **Title Check**: Title must be clear and descriptive (≥10 chars)

### Pre-Execution Gates

1. **File Path Check**: Objective must not reference forbidden paths
2. **Description Check**: Must have constraints (e.g., "DO NOT modify X")
3. **Scope Check**: Parse objective for file path mentions, validate against allowed_paths
4. **Test Coverage**: If code changes, require test files mentioned

### Post-Execution Gates

1. **File Count Check**: ≤10 files changed
2. **Line Count Check**: ≤500 lines changed
3. **Deletion Check**: 0 files deleted
4. **Path Check**: All changed files must be in allowed_paths
5. **Test Check**: If code changed, test files must be present

## Escalation Paths

### Automatic Escalation

Triggers:
- Task fails after max retries
- File count exceeds limits
- Line count exceeds limits
- Forbidden path referenced

Actions:
- Mark BrainTask as Failed
- Add comment to Jira with escalation reason
- Set Jira status to "Blocked"

### Manual Review Required

Triggers:
- Task class not in safe list
- Component not in allowed list
- Multiple file scope violations

Actions:
- Don't auto-ingest
- Add comment: "Requires manual review - task class/component not in dogfood scope"

## Deduplication / Idempotency

### Ingestion Idempotency

```go
// Check if BrainTask already exists for this Jira issue
func IsAlreadyIngested(jiraKey string) bool {
    // Query for BrainTasks with metadata.labels.source_key == jiraKey
    // Return true if found
}
```

### Re-Run Handling

If BrainTask already exists:
1. Check status
   - Completed: Do nothing (already processed)
   - Running: Do nothing (already processing)
   - Failed: Consider re-create based on age (e.g., if >1h old)

2. Check if Jira issue was updated
   - If issue changed significantly, may want to re-create
   - If minor change, skip

## Change History

- 2026-03-20: Initial version for ZB-025 Jira intake contract
