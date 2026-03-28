# Remediation Queue

**Version:** 3.0
**Updated:** 2026-03-28
**Status:** OPERATIONAL — Backlog Drained

## Operating Statement

**Success is tickets reaching Done, not tickets created.**

As of 2026-03-28:
- Backlog: 517 → 0 (fully drained)
- Done: 0 → 529
- Jira now reflects real workflow movement
- State transitions are operational (transitionJiraStatus wired into pipeline)

## Core Principle

The remediation queue turns ai:finding tickets into bounded, executable work items that flow through L1 → validation → Jira update → evidence pack — without human intervention for each step.

## How It Works

1. **Finding ticket** is created (ai:finding label) with problem, evidence, fix direction
2. **Remediation worker** picks up bounded tickets from the queue
3. **L1 (0.8b)** drafts the first-pass remediation output
4. **Normalization layer** cleans L1 output (strips fences, fixes YAML, removes repetitive content)
5. **Quality gate** scores the normalized payload (0-25 scale, must be ≥15 to proceed)
6. **Validation** runs the defined validation command(s)
7. **Jira update** writes quality-gated comment + governance labels
8. **Evidence pack** is written as the work happens

## Bounded Remediation Pilot — Phase 39

**Status:** PROVEN (3/3 success-needs-review)

Three tickets were processed through the full loop:
- ZB-614: L2 quality gate policy (16.7s L1, config_change)
- ZB-616: Remediation template (67.9s L1, doc_update)
- ZB-618: Retention enforcement (64.2s L1, config_change)

### Key Findings
- 0.8b L1 produces directionally correct output (60-90% raw quality)
- Normalization is mandatory — raw L1 output cannot go directly to Jira
- No L2 escalation needed for bounded config/template tasks
- All 3 tickets scored ≥20/25 on the quality rubric

### Next Rollout Gate
Before expanding beyond 3 tickets:
1. Wire `ticket_normalize.go` into the worker binary's runPilot() path
2. Add programmatic quality gate enforcement (reject <15/25)
3. Then expand to next bounded set of 5-10 tickets

## Ticket Quality Gate

Every remediation ticket must score on these dimensions (0-5 each):
- **Clarity:** Is the problem clearly stated?
- **Evidence Quality:** Is evidence specific (paths, snippets)?
- **Boundedness:** Is scope bounded (target files, expected outcome)?
- **Validation Clarity:** Can success be verified?
- **Governance Completion:** Are SR&ED/IRAP/project fields filled?

Readiness levels:
- ≥20/25 → ready_for_execution or ready_with_review
- 15-19/25 → needs_review
- <15/25 → blocked (do not write to Jira as final)

## L1 Attribution (Phase 40)

**Updated:** 2026-03-28

Every remediation task must record attribution — who actually did the work.

### Required Attribution Fields

```json
{
  "produced_by": "l1 | l2 | supervisor | script | none",
  "first_pass_model": "qwen3.5:0.8b-q4 | qwen3.5:2b-q4 | none",
  "supervisor_intervention": "none | normalization_only | prompt_fix | manual_rewrite | script_override",
  "artifact_authorship": "l1 | mixed | supervisor | none",
  "final_disposition": "l1-produced | l1-produced-needs-review | supervisor-written | script-only | failed"
}
```

### L1 Artifact Proof Requirement

Before claiming a task as L1-produced, require one saved artifact:
- Raw L1 output → `evidence/l1-attribution-pilot/{KEY}_raw.json`
- Normalized output → `evidence/l1-attribution-pilot/{KEY}_normalized.json`

**No artifact = no attribution claim.** Non-negotiable.

### 10-Ticket Attribution Pilot Results

| Jira Key | Type | L1 Parsed | Has Content | Time | Produced By | Final State |
|----------|------|-----------|-------------|------|-------------|-------------|
| ZB-817 | config_change | ✅ | ✅ | 4.7s | l1 | Done |
| ZB-818 | code_edit | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |
| ZB-819 | doc_update | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |
| ZB-820 | config_change | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |
| ZB-824 | code_edit | ❌ | ❌ | 12.5s | l1-failed-parse | PAUSED |
| ZB-826 | doc_update | ✅ | ✅ | 18.0s | l1 | Done |
| ZB-827 | doc_update | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |
| ZB-829 | config_change | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |
| ZB-832 | config_change | ✅ | ✅ | 20.4s | l1 | Done |
| ZB-834 | doc_update | ❌ | ❌ | 120s | l1-failed-parse | PAUSED |

**Counts:** l1-produced: 3 (30%) | l1-produced-needs-review: 7 (70%) | supervisor-written: 0 | script-only: 0

**Honest assessment:** L1 handles small config_change/doc_update in <25s. L1 fails on code_edit and large-file tasks (timeout or truncation). Not ready for autonomous expansion. Full scoreboard: `docs/05-OPERATIONS/evidence/l1-attribution-scoreboard.md`

### Policy Decision (Scenario B)

Most bounded tickets are NOT truly l1-produced (30% vs 70%).
- Stop claiming L1 is doing the work for most tasks
- Tighten packet design — instruct L1 to produce descriptions only, not full file contents
- Re-run pilot with description-only output format before expanding

## Category Separation

| Category | What It Measures | Counts as L1 Work? |
|----------|-----------------|-------------------|
| Ops cleanup | Bulk Jira state changes, stale closure | No |
| L1 production | Tasks with attributable L1 artifacts | Yes |
| Supervisor work | GLM-5/human did the actual work | No |
| Discovery output | Scanner reports, findings generation | No |

**Do not mix these in reporting.**

## Mandatory Statements

- 0.8b drafts first-pass ticket content; code must normalize and quality-gate it
- Jira receives structured, evidence-backed, bounded work items only
- Tickets must be execution-ready, not just present
- Evidence packs must be updated as the work happens, not reconstructed later
- **Backlog cleanup is useful ops work, but it is not factory production**
- **L1 usefulness must be backed by attributable artifacts**
- **The factory must be measured honestly before expanding claims**
- **See also:** L1_ATTRIBUTION_POLICY.md, BACKLOG_DRAIN_MODE.md
