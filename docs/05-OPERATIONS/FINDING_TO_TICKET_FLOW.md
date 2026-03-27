# Finding-to-Ticket Flow

**Phase:** 38
**Status:** Active — pilot on defects + stub_hunting

## Overview

Discovery without ticketization is low leverage. The finding-to-ticket loop converts validated discovery findings into actionable Jira tickets, with deduplication to prevent duplicate issues.

## Pipeline

```
Discovery Run (useful-batch)
    ↓
Validated Artifacts (defects.md, bug-hunting.md, stub-hunting.md)
    ↓
Finding Parser — extracts table-row findings from markdown
    ↓
Fingerprint Computation — SHA256(type:file:description)
    ↓
Dedup Ledger Check — skip if recently triaged (<24h)
    ↓
Actionability Filter — skip low-confidence findings
    ↓
L1 0.8B Ticketization — one finding per call
    ↓
Jira Create/Update — based on dedup_decision
    ↓
Ledger Update — fingerprint → Jira key mapping
```

## L1 Ticketization Output Contract

Each finding is sent to L1 individually. L1 produces structured JSON:

| Field | Description |
|-------|-------------|
| title | Concise Jira summary |
| summary | 1-2 sentence description |
| problem | What is wrong |
| evidence | Relevant code/data |
| impact | Why it matters |
| fix_direction | Suggested approach |
| labels | Comma-separated (e.g., "ai:finding,bug") |
| priority | High, Medium, Low |
| dedup_decision | create_new, update_existing, ignore_noise |
| existing_key | Jira key if matching existing |
| follow_up_type | no_followup, bounded_fix_l1, manual_review |

## Dedup Strategy

- **Fingerprint**: SHA256 of (normalized category + file path + first 80 chars of description)
- **Cooldown**: Same fingerprint won't be re-ticketed within 24 hours
- **Ledger**: `/var/lib/zen-brain1/ticketizer/finding-ledger.json`
- **Jira search**: Queries open `ai:finding` labeled issues for potential matches

## Actionability Filter

Findings are skipped if:
- No concrete file path (just package names)
- Empty or generic descriptions (N/A, Unknown)
- No dot or slash in file path (not a real file)

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `SOURCE_CLASSES` | `defects,stub_hunting` | Which discovery classes to ticketize |
| `MAX_FINDINGS` | `5` | Max actionable findings per run |
| `TICKETIZER_TIMEOUT` | `120` | L1 timeout per finding (seconds) |
| `LEDGER_PATH` | `/var/lib/zen-brain1/ticketizer/finding-ledger.json` | Dedup ledger path |

## Scheduler Integration

The ticketizer runs automatically after Jira sync for:
- `hourly-scan` (contains defects, bug_hunting, stub_hunting)
- `daily-sweep` (contains all 10 classes)

It does NOT run for `quad-hourly-summary` (no discovery classes).

## Design Rules

1. **0.8B L1 does first-pass ticketization** — GLM-5 supervises, doesn't draft
2. **Code does dedup** — model only gets one finding at a time
3. **One finding per L1 call** — no batch ticketization
4. **Max 5 per run** — don't flood Jira
5. **Fail-open** — ticketizer failure never blocks the discovery batch
6. **Priority normalization** — L1 output normalized to Jira-compatible values

## Future: Remediation Loop

The remediation template (`finding-remediation-l1`) is defined but not yet active:
- Takes a Jira key + finding summary + target files
- L1 attempts a bounded fix
- Escalates to L2 if L1 fails
- Only activates after ticketization/dedup is stable
