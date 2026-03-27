# Jira as System Ledger

**Version:** 1.0
**Updated:** 2026-03-27
**Status:** Active

## Core Principle

**Jira is the central human/AI work ledger across all layers.** It is not just a dev ticket tracker — it is the operating ledger for Board / Office / Factory interaction. Every actionable piece of work flows through Jira.

## What Jira Tracks

| Layer | Jira Usage | Example |
|-------|-----------|---------|
| **Board** | Strategic decisions, portfolio direction, release approval | Epics for major initiatives |
| **Portfolio Office** | Programs, milestones, release slices, dependencies | Epics with version labels |
| **Office** | Department-specific work, compliance evidence, escalations | Stories/tasks by department |
| **Factory** | Task execution, artifact output, validation results, findings | Tasks with telemetry links |

## Jira is NOT

- Just a bug tracker
- Just a dev team tool
- Optional — if it's not in Jira, it's not tracked
- A substitute for conversation — complex decisions need human/AI dialogue before Jira creation

## Current Jira Integration

### Project
- **Project Key:** ZB
- **Email:** `zen@kube-zen.io`
- **URL:** `https://zen-mesh.atlassian.net`

### Issue Structure

```
Parent Issue (batch run / program / epic)
├── Child Issue (individual task / finding / work item)
│   ├── Labels: ai:completed, ai:needs-review, ai:blocked
│   ├── Comments: validation results, telemetry, evidence
│   └── Links: source artifacts, related issues
```

### Current Labels

| Label | Meaning | Applied By |
|-------|---------|-----------|
| `ai:completed` | Task succeeded, artifact produced | Scheduler |
| `ai:needs-review` | Task produced output requiring human review | Scheduler (validation) |
| `ai:blocked` | Task failed validation or has blocker | Scheduler (validation) |
| `ai:finding` | Auto-created from discovery finding | Finding ticketizer |

### Future Labels (Portfolio Office / Board)

| Label | Meaning | Layer |
|-------|---------|-------|
| `portfolio:1.1` | Belongs to 1.1 release scope | Portfolio Office |
| `portfolio:1.2` | Belongs to 1.2 release scope | Portfolio Office |
| `portfolio:2.0` | Belongs to 2.0 release scope | Portfolio Office |
| `dept:engineering` | Engineering department | Office |
| `dept:operations` | Operations department | Office |
| `dept:security` | Security department | Office |
| `governance:executable` | AI can execute autonomously | Portfolio Office |
| `governance:approval-required` | Human approval needed | Portfolio Office |
| `governance:review-required` | Human review after execution | Portfolio Office |

## Governance Controls via Jira

Jira fields and labels implement governance without custom tooling:

| Control | Jira Mechanism | Example |
|---------|---------------|---------|
| Task state | Workflow status + labels | `ai:completed`, `ai:blocked` |
| Priority | Priority field | High/Medium/Low (normalized by ticketizer) |
| Approval gate | Custom field + workflow | `approval_level: 2` |
| Department | Label or component | `dept:engineering` |
| Release scope | Label | `portfolio:1.1` |
| Human override | Manual Jira transition | Human moves ticket to blocked |
| Evidence link | Custom field or comment link | Artifact path in comment |

## Telemetry-to-Jira Flow

```
Factory executes task
    ↓
Produces artifact (markdown, JSON)
    ↓
Validates output (grounding, structure, size)
    ↓
Creates Jira child issue with:
    - Title from task class
    - Description with evidence summary
    - Labels based on validation outcome
    - Comment with telemetry details
    ↓
Scheduler updates parent issue with:
    - Child issue count
    - Success/fail counts
    - Jira keys for all children
```

## Strategic Planning in Jira

When the Portfolio Office and Board layers are operational, Jira will also serve:

- **Portfolio views** — epics grouped by release (1.1, 1.2, 2.0)
- **Dependency links** — cross-epic issue links showing execution order
- **Capacity boards** — sprint/board views per department
- **Governance dashboards** — filter by approval level, department, release
- **Compliance evidence** — SR&ED/IRAP work items with evidence pack links

## Data Integrity

Jira is the **authoritative source** for:
- What work exists
- What state work is in
- What work was completed
- What is blocked and why

Other sources (ROADMAP_ITEMS.md, telemetry, artifacts) provide context but Jira is the ledger of record for execution status.

## Human-AI Boundary

- **AI creates issues** via ticketizer and scheduler — always labeled with `ai:` prefix
- **Human creates issues** — no `ai:` prefix, treated as human-originated
- **AI updates issues** — status transitions, comments, labels within defined policies
- **Human overrides** — can manually change any field, add comments, reassign
- **AI reads issues** — for discovery, ticketization, dedup, capacity planning
