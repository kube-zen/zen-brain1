# Zen-Brain 1.0 Self-Improvement Loop

## Overview

The self-improvement loop enables Zen-Brain to safely work on its own backlog, closing remaining 1.0 gaps without relying on external coding AI.

## Architecture

### Loop: Pull → Claim → Classify → Gate → Act → Report

1. **Discover** - Find eligible low-risk tasks
2. **Claim** - Acquire task with lease ownership
3. **Classify** - Determine action class (A/B/C)
4. **Gate** - Check action policy before execution
5. **Execute** - Run only allowed actions
6. **Report** - Generate proof/comment/artifact
7. **Complete** - Release ownership and update status

## Action Policy (Class A/B/C)

### Class A: Always Allowed (Read & Recommend)
- Risk Level: None
- Safe to run autonomously
- Examples: fetch, analyze, summarize, classify, recommend, generate artifacts
- No approval required

### Class B: Safe Write-Back (Restricted Writes)
- Risk Level: Low
- Safe writes with proven paths only
- Examples: Jira comments, artifact attachments, safe status updates
- Requires human review of output before Jira write-back
- Log warning for review

### Class C: Approval Required (High-Impact Actions)
- Risk Level: Medium-High
- Requires explicit approval before execution
- Examples: repo writes, merges, deploys, secret/config changes, meaningful status transitions
- Default: deny unless explicitly approved
- Requires: ApprovedBy, ApprovedAt, SignOff

## Worker Identity & Claim/Lease Model

### Required Fields
- `worker_id` - Stable instance identity
- `role` - Worker role (e.g., "self-improvement")
- `claimed_at` - Timestamp when task was claimed
- `lease_expires_at` - Timestamp when claim expires
- `source_project` - Project/task source
- `action_class` - Class A/B/C for the action

### Ownership Visibility
- Worker identity in all logs
- Jira comments include worker ID
- Proof artifacts include worker metadata
- Lease prevents duplicate processing

## Safe 1.0 Task Taxonomy

### Allowed Initial Task Types

**Runtime/Ops Quality**
- Runtime doctor clarity improvements
- Runtime report noise reduction
- Config validation gap detection
- Environment-variable drift detection
- Action-policy drift detection

**Factory/Execution Quality**
- Stub/mock/default hunting
- TODO/placeholder hunting
- Hardcoded path/default hunting
- Proof formatting improvements
- Postflight warning classification

**Testing/Validation**
- Missing regression test suggestions
- Flaky-test candidate detection

### Forbidden Initial Task Types

- Deploy changes
- Merges without review
- Risky workflow transitions
- Secret/config changes
- Cloud/provider operations
- qmd/Cockroach realism
- Same-project multi-instance coordination
- Broad refactors

## Usage

### Run Self-Improvement Loop

```bash
# Set up environment
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_PROJECT_KEY=ZB
export JIRA_TOKEN=$(grep "^token:" ~/.zen-brain1-config/jira.yaml | awk '{print $2}')
export OLLAMA_BASE_URL=http://127.0.0.1:11434

# Run one iteration
./bin/zen-brain self-improvement
```

### Expected Output

```
=== Zen-Brain Self-Improvement Loop ===
Started: 2026-03-13T17:34:47-04:00

[1/7] Discovering eligible self-improvement tasks...
  Found 3 eligible task(s)
[2/7] Claiming task...
  Claimed: SI-001 (worker: zb-self-improvement-1)
[3/7] Analyzing and classifying task...
  Task ID: action-SI-001-1773437687
  Action Type: self_improvement
  Action Class: Class A (Always Allowed)
  Risk Level: none
[4/7] Checking action policy...
[5/7] Executing allowed action...
  Execution complete: completed
[6/7] Generating proof and report...
2026/03/13 17:34:47 [Report] Generated proof for task SI-001 by worker zb-self-improvement-1
[7/7] Completing task...
2026/03/13 17:34:47 [Complete] Task SI-001 completed by worker zb-self-improvement-1
  Task completed: SI-001

=== Self-Improvement Complete ===
Finished: 2026-03-13T17:34:47-04:00
```

## Implementation Status

### ✅ Priority 1: Self-Improvement Loop
- [x] Implemented 7-step loop (discover → claim → classify → gate → execute → report → complete)
- [x] Worker identity tracking (worker_id, role)
- [x] Claim/lease model with expiration
- [x] Single-task execution per iteration

### ✅ Priority 2: Committed Action Policy
- [x] `internal/office/action_policy.go` with Class A/B/C enforcement
- [x] `CanExecute()` method checks action class before execution
- [x] Class B logs warning for review before Jira write-back
- [x] Class C requires explicit approval (ApprovedBy, ApprovedAt, SignOff)
- [x] Default to safest (Class C) for unknown actions

### ✅ Priority 3: Claim/Lease/Ownership
- [x] Worker identity in logs and tasks
- [x] Claim timestamp and lease expiration
- [x] Ownership visible in console output
- [ ] TODO: Wire to Jira labels for cross-worker coordination

### ✅ Priority 4: Safe 1.0 Task Taxonomy
- [x] Defined safe task categories (runtime, factory, testing)
- [x] Forbidden task types identified (deploy, merge, secrets)
- [x] Initial task set (3 safe tasks in code)

### ⏳ Priority 5: Run the Loop on Zen-Brain Itself
- [ ] Wire Jira integration for real task discovery
- [ ] Queue safe tasks in Jira with "self-improvement" label
- [ ] Run overnight loop on zen-brain1 backlog
- [ ] Generate nightly report with processed tasks
- [ ] Track escalated tasks (Class C requiring approval)

## Next Steps

### Immediate (This Week)
1. Wire Jira integration for real task discovery
2. Create "self-improvement" label in Jira
3. Queue 5-10 safe 1.0 tasks
4. Test overnight loop on zen-brain1 backlog

### Short-Term (Next Week)
1. Add Jira label updates for claim/lease
2. Implement proof artifact upload to Jira
3. Add nightly summary report generation
4. Track and measure task completion rate

### Medium-Term (1.0 Gap Closure)
1. Use loop to close runtime doctor clarity gaps
2. Use loop to fix proof formatting issues
3. Use loop to add missing regression tests
4. Use loop to hunt and document bugs
5. Use loop to improve factory quality

## Success Criteria

- [ ] Zen-Brain can safely pull and process its own low-risk improvement tasks
- [ ] Action policy is explicit and tested
- [ ] Claim/lease prevents duplicate handling
- [ ] Self-improvement loop produces useful overnight output
- [ ] Remaining 1.0 gaps can start being closed by Zen-Brain itself
- [ ] Worker identity is visible in all Jira interactions

## Remaining Blockers

1. **Jira Integration** - Loop uses hardcoded tasks, needs real Jira discovery
2. **Proof Upload** - Reports generated but not uploaded to Jira
3. **Nightly Summary** - No aggregated report generation yet
4. **Multi-Worker Coordination** - Claim/lease not wired to Jira labels

## Files Changed

### Added
- `cmd/zen-brain/self_improvement.go` - Self-improvement loop implementation (9.9 KB)
- `docs/05-OPERATIONS/ZEN_BRAIN_1.0_SELF_IMPROVEMENT.md` - This documentation

### Modified
- `cmd/zen-brain/main.go` - Added `self-improvement` command and usage

### Existing (No Changes)
- `internal/office/action_policy.go` - Committed in 6c06f9a with Class A/B/C enforcement

## How to Invoke

```bash
# One iteration (interactive)
./bin/zen-brain self-improvement

# Nightly loop (cron)
0 2 * * * cd /home/neves/zen/zen-brain1 && ./bin/zen-brain self-improvement >> /var/log/zen-brain-nightly.log 2>&1
```

## Honest Assessment

**Working:**
- ✅ Loop structure (7 steps)
- ✅ Action policy enforcement (Class A/B/C)
- ✅ Worker identity and lease tracking
- ✅ Safe task taxonomy defined
- ✅ Executes safely on Class A/B tasks

**Not Yet Working:**
- ❌ Real Jira task discovery (uses hardcoded tasks)
- ❌ Jira write-back for Class B actions
- ❌ Proof artifact upload to Jira
- ❌ Multi-worker coordination (Jira label-based claiming)
- ❌ Nightly report generation

**Priority:**
The loop is ready for safe self-improvement work. The blocker is Jira integration for real task discovery, not the loop or action policy itself.
