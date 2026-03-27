# Roadmap → Ticket Flow

**Version:** 2.0
**Updated:** 2026-03-27
**Status:** Active

## Flow

1. **Roadmap item** is defined in ROADMAP_ITEMS.md or Board decision
2. **Roadmap ticketizer** decomposes into bounded Jira tasks
3. **Each ticket** gets: summary, acceptance criteria, target files, validation
4. **Tickets** enter the remediation queue if they have bounded_fix_l1 or bounded_synthesis_l2 routing
5. **Remediation worker** processes through L1 → normalize → quality gate → validate → Jira + evidence

## Quality Gate

Same as finding tickets:
- Score ≥15/25 on quality rubric
- Must have: problem, evidence, expected outcome, validation, routing
- Missing fields = blocked, not queued

## Phase 39 Status

0.8b drafts the roadmap ticket content. Code normalizes and validates before Jira write. Only execution-ready tickets proceed to remediation.
