# Zen-Brain 0.1 → 1.0: Effective Patterns

**Date:** 2026-03-27 (PHASE 34)
**Purpose:** Extract what made 0.1 effective and apply it to zen-brain1.

---

## What 0.1 Did Effectively

### 1. Deterministic Control Plane (Code, Not AI)

0.1's `jira_autowork.go` was pure Go code that:
- Searched Jira for candidate tickets via JQL
- Claimed lease on a ticket
- Transitioned ticket state (`To Do → In Progress`)
- Created a structured `execute_ticket` task
- Enqueued it to the dispatcher
- Logged metrics (claimed, delegated, failed, etc.)
- Released lease on completion/failure

The AI was never asked to manage scheduling, state, or lifecycle.
**Proof:** `internal/gateway/jira_autowork.go` — `ClaimNextTicket()`, `RunOnce()`, lease acquire/release.

### 2. Strong Task Templates with Explicit Sections

0.1 task packets (`quickwin.yaml`, `execute_ticket.yaml`, `worker_execute.yaml`) had:
- Explicit `output_template` with typed sections (enum, string, textarea, boolean, object)
- Required/optional fields with defaults
- Explicit `success_criteria` and `testing` sections
- Exact output format specification
- Post-actions (Jira update, status transition, audit log)
- Fail-closed guardrails at every step

**Proof:** `task-templates/quickwin.yaml` — 12-section output template with validation types.
**Proof:** `task-templates/execute_ticket.yaml` — 13-step workflow with fail-closed checks.

### 3. Small-Model-Friendly Structure

The model got:
- Bounded scope per step (single ticket, single phase)
- Explicit required output sections (not "analyze the repo")
- Exact success criteria with checkbox format
- Typed inputs and outputs
- Step-by-step execution with checkpoint/resume

**Proof:** `task-templates/execute_ticket.yaml` — "Return JSON with: {files_changed, summary, status}".

### 4. Ticket-Centric Lifecycle

0.1 was Jira-led end-to-end:
- Ticket selection from backlog
- Lease claim to prevent duplicate work
- State transitions at every phase
- Evidence written back as Jira comments
- Post-actions: label, transition, audit

**Proof:** `task-templates/execute_ticket.yaml` — 13 steps from claim to done.
**Proof:** `internal/gateway/jira_autowork.go` — `WorkerLoopMetrics` with 7 counter types.

### 5. Code Does Discovery, AI Does Bounded Reasoning

The system prepared evidence before the model saw anything:
- `search_jira` step gathered ticket candidates
- `preflight_check` verified clean working directory
- `sync_upstream` ensured fresh repo state
- Only then did `implement_changes` invoke the AI

The model received a specific ticket with a specific implementation plan.
It did NOT receive "analyze the whole repo."

**Proof:** `task-templates/worker_execute.yaml` — steps 1-6 are pure code, step 7 is AI.

---

## What 1.0 Currently Lacks

| 0.1 Pattern | 1.0 Current State | Gap |
|-------------|-------------------|-----|
| Deterministic control plane | Scheduler works but task packets are loose | Task structure is weak |
| Strong output templates | `taskClasses` has only title+prompt+output filename | No validation, no typed sections |
| Bounded scope per step | "Scan cmd/, internal/, pkg/" as raw instruction | Too vague, no bounds |
| Ticket lifecycle | Jira parent/child creation exists | No state transitions, no lease |
| Code does prep | `gatherEvidence()` added but raw | Evidence not structured/shaped |
| Fail-closed validation | Added but minimal | Needs grounding checks |

---

## What Must Be Copied Into 1.0

1. **Structured evidence bundles** — Code collects, clusters, and trims evidence before L1 sees it. Not raw `find` output.
2. **Canonical task packets** — Each report task gets a typed, sectioned packet like 0.1's `output_template`.
3. **Explicit output contract** — Required sections, max items, exact format — not "produce a markdown report."
4. **Fail-closed validation** — Check file references exist, sections present, findings grounded in evidence.
5. **Task lifecycle** — Work item ID, state tracking, validation result, artifact path.
6. **Evidence-first prompting** — Model summarizes and prioritizes what code already found. Model does NOT discover from scratch.

---

## Reference Files (0.1 Source of Truth)

| File | What It Proves |
|------|---------------|
| `task-templates/quickwin.yaml` | 12-section typed output template with Jira post-actions |
| `task-templates/worker_execute.yaml` | 13-step deterministic workflow with guardrails |
| `task-templates/execute_ticket.yaml` | Bounded execution with checkpoint/resume and fail-closed |
| `internal/gateway/jira_autowork.go` | Pure-code control plane: claim, transition, enqueue, metrics |
| `MODULAR_TICKETS_IMPLEMENTATION_SUMMARY.md` | Ticket backend system with typed contracts |
| `TICKET_SYSTEM_IMPLEMENTATION_SUMMARY.md` | Full ticket lifecycle design |
| `JIRA_ORGANIZATION_PROPOSAL.md` | Jira-first work ledger architecture |
