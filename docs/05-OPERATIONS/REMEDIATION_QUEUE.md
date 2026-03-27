# Remediation Queue

**Version:** 2.0
**Updated:** 2026-03-27
**Status:** Pilot Proven

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

## Mandatory Statements

- 0.8b drafts first-pass ticket content; code must normalize and quality-gate it
- Jira receives structured, evidence-backed, bounded work items only
- Tickets must be execution-ready, not just present
- Evidence packs must be updated as the work happens, not reconstructed later
