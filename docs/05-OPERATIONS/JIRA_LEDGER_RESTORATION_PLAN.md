# Jira Ledger Restoration Plan

**Date:** 2026-03-26 (PHASE 31)
**Status:** Design complete, implementation pending valid Jira API token

## 0.1 Comparison

zen-brain 0.1 operated in a more Jira-centric model:
- Tickets were the primary work unit
- Backlog sync existed
- Jira backend was a first-class concept
- The system was ticket-led, not artifact-led

zen-brain1 drifted from this model:
- Current 24/7 loop is scheduler/artifact-driven
- Useful-task runs produce files, not Jira issues
- Findings are not ticketed
- No audit trail for IRAP / SR&ED

## Target Model

### Split of Responsibilities

| Layer | Owner | Responsibility |
|-------|-------|---------------|
| Cadence | zen-brain internal scheduler | When to run (hourly, quad-hourly, daily) |
| Ledger | Jira | What was found, what needs doing, what is done |
| Execution | zen-brain runtime | Produces artifacts, updates Jira |

### Evidence Chain

```
Scheduler run
  → run_id (timestamp-based)
  → artifact bundle (/var/lib/zen-brain1/runs/<batch>/<run_id>/final/)
  → Jira parent issue (one per scheduled run)
  → finding child issues (actionable items extracted from artifacts)
  → remediation follow-up issues
  → execution result → closure
```

### Per-Run Metadata (stored in Jira parent issue)

- Run ID
- Schedule name
- Start/end time
- Model lane used (L1/L2)
- Artifact root path
- Succeeded/failed task counts
- Child issue keys

### Per-Finding Metadata (stored in Jira child issues)

- Finding ID
- Source report/artifact
- Severity / priority
- Package / path
- Recommended action
- Linked parent issue

## Issue Model

### Parent Issue (per scheduled run)
- **Type:** Task
- **Summary:** `[zen-brain] <schedule-name> — <date> <run_id>`
- **Description:** Run summary, artifact counts, model lanes, execution status
- **Labels:** `zen-brain`, `discovery`, `<schedule-name>`
- **Children:** Finding issues linked as sub-tasks or linked issues

### Child Issues (actionable findings)
- **Type:** Task or Bug (depending on finding type)
- **Summary:** `[<finding-type>] <package/path>: <description>`
- **Description:** Evidence snippet, severity, remediation recommendation
- **Labels:** `zen-brain`, `finding`, `<finding-type>`
- **Priority:** Mapped from severity (critical=Highest, high=High, etc.)

### Follow-up Issues (remediation tasks)
- **Type:** Task
- **Summary:** `[remediation] <what to fix>`
- **Labels:** `zen-brain`, `remediation`
- **Linked to:** Parent finding issue

## Implementation Path

### Existing Jira Plumbing (reuse, don't rebuild)

| Component | Location | Purpose |
|-----------|----------|---------|
| Jira connector | `internal/office/jira/connector.go` | CRUD, search, transitions |
| Create issue | `internal/office/jira/create_issue.go` | Issue creation with ADF |
| Types | `internal/office/jira/types.go` | Full type system |
| Integration init | `internal/integration/office.go` | OfficePipeline setup |
| Config | `internal/config/` | Jira config from env/yaml |
| Feedback | `internal/feedback/braintask_to_jira.go` | BrainTask → Jira status sync |
| Existing binary | `cmd/create-jira-issues/main.go` | Standalone issue creation |

### First Slice: `cmd/jira-ledger/main.go`

A new binary that:
1. Reads a completed batch's `telemetry/batch-index.json`
2. Reads each artifact in `final/`
3. Extracts top findings (headings, severity, action items)
4. Creates one Jira parent issue for the batch
5. Creates up to N child issues for actionable findings
6. Stores the Jira keys back into a `jira-mapping.json` alongside telemetry

No new Jira client code — reuses `internal/office/jira/`.

### Scheduler Integration

After batch completion in `cmd/scheduler/main.go`:
1. Execute `jira-ledger` binary (or import as package)
2. Record Jira parent key in schedule state
3. Next batch can reference prior Jira keys

## Policy

1. **Scheduler discovers, Jira tracks, zen-brain executes**
2. Every scheduled discovery run creates a Jira parent issue
3. Actionable findings get child issues with priority
4. Follow-up remediation comes from Jira keys back into zen-brain
5. Internal scheduler still owns cadence — Jira is the ledger, not the clock
6. IRAP / SR&ED traceability runs through Jira-linked artifacts

## Current Blocker

Jira API token in `~/zen/DONOTASKMOREFORTHISSHIT.txt` returns 401 — the token has been rotated since the bootstrap file was created. A fresh token is needed from Atlassian to proceed with live integration.

Token recovery options:
1. Generate new API token at https://zen-mesh.atlassian.net/account/security → API tokens
2. Update bootstrap file → re-encrypt via zen-lock → push new ZenLock CRD
3. Delete bootstrap file after successful encryption

## Non-Goals

- Do NOT move schedule ownership into Jira
- Do NOT replace internal scheduling with Jira polling
- Do NOT build a second Jira client
- Do NOT overdesign taxonomy before first slice works
