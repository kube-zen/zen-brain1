# Finding → Ticket Flow

**Version:** 2.0
**Updated:** 2026-03-27
**Status:** Active

## Flow

1. **Scanner/analysis** identifies a finding (dead code, defect, tech debt, etc.)
2. **Finding ticketizer** creates a Jira ticket with `ai:finding` label
3. **Ticket body** must include: problem, evidence, impact, fix direction
4. **Remediation queue** picks up tickets marked `bounded_fix_l1` or `bounded_synthesis_l2`
5. **Remediation worker** builds a packet → dispatches to L1 → normalizes → quality-gates → validates → updates Jira + evidence pack

## Ticket Quality Requirements

A finding ticket must be execution-ready:
- **Title:** concise, action-oriented (no "Improve X")
- **Summary:** 2-4 sentences (what, why, expected outcome)
- **Problem:** concrete statement with component/path
- **Evidence:** source artifact paths + snippets
- **Expected outcome:** bounded result
- **Fix direction:** short bounded direction
- **Target files:** exact files if known
- **Validation:** command or check for success
- **Governance:** approval level, project, SR&ED/IRAP if applicable
- **Routing:** bounded_fix_l1, bounded_synthesis_l2, or manual_review

## Quality Gate

Before a finding ticket enters remediation:
- Score ≥15/25 on the quality rubric
- Missing required fields = blocked_missing_context
- Must have routing recommendation

## Phase 39 Status

0.8b drafts the ticket content, code normalizes and quality-gates it, Jira only gets execution-ready structured tickets.
