# IRAP / SR&ED Evidence Flow

**Version:** 2.0
**Updated:** 2026-03-27
**Status:** Active — Pilot Proven

## Core Principle

Evidence packs must be updated as remediation work happens, not reconstructed later. Every compliance-relevant remediation produces a traceable evidence artifact linked to Jira.

## Evidence Flow

```
Jira ticket (ai:finding)
  → Remediation packet built
  → L1 processes packet
  → Output normalized + quality-gated
  → Evidence pack written immediately:
      manifest/manifest.json (structured metadata)
      reports/remediation-result.md (human-readable)
      rejection-log/ (if blocked/failed)
  → Jira updated with evidence pack link
```

## Evidence Pack Structure

```
/var/lib/zen-brain1/evidence/evidence-YYYY-MM/
  rem-ZB-XXX/
    index.md              — human-readable summary
    manifest/
      manifest.json       — structured metadata (Jira key, SR&ED, IRAP, run ID, timestamps)
    reports/
      remediation-result.md — what was done, validation, outcome
    rejection-log/
      blocker-note.md     — only if blocked/failed
```

## Compliance Fields

Every evidence pack manifest includes:
- **SR&ED Uncertainty:** What technological uncertainty does this address?
- **IRAP Work Package:** Which IRAP work package does this fall under?
- **Related Project:** zen-brain1
- **Approval Level:** Human approval required (1=auto, 2=review, 3=manual)
- **Run ID:** Traceable run identifier (rem-ZB-XXX-YYYYMMDD-HHMMSS)
- **Timestamps:** When the work was done

## Jira Labels for Compliance

| Label | Meaning |
|-------|---------|
| sred:* | SR&ED uncertainty category |
| irap:* | IRAP work package |
| ai:remediated | AI performed the remediation |
| pilot:phase39 | Part of the Phase 39 pilot |
| quality:ready-with-review | Passed quality gate, needs human ack |

## Phase 39 Pilot Evidence

All 3 pilot tickets have evidence packs:
- `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-614`
- `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-616`
- `/var/lib/zen-brain1/evidence/evidence-2026-03/rem-ZB-618`

Each has manifest.json + index.md + reports/remediation-result.md.

## Mandatory Statements

- Evidence packs are written during remediation, not after
- 0.8b L1 is the first-pass heavy lift for remediation content
- Jira labels carry compliance metadata alongside the evidence packs
- No compliance-relevant remediation is invisible — if it happened, it's in the evidence pack
