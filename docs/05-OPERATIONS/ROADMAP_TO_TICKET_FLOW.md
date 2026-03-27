# Roadmap-to-Ticket Flow

**Phase:** 38 (continued)
**Status:** Active — pilot on ROADMAP_ITEMS.md

## Overview

Roadmap items should become tracked Jira work, not remain passive docs. The roadmap-to-ticket pipeline extracts actionable items from `ROADMAP_ITEMS.md` and converts them into Jira tickets with the same infrastructure as the finding-to-ticket flow.

## Source Format

`ROADMAP_ITEMS.md` uses this format:
```markdown
## Section Name

- **ITEM-ID**: Description of the bounded work item
```

Each item must:
- Have a unique `ITEM-ID` (e.g., `IL-1`, `DT-2`, `OP-3`)
- Be a bounded unit of work suitable for L1 execution or manual implementation
- Be categorized under a priority section

The ticketizer also supports `PROGRESS.md` table format as a fallback.

## Pipeline

```
ROADMAP_ITEMS.md (canonical source)
    ↓
Deterministic item extraction (code, not AI)
    ↓
Fingerprint computation (SHA256 of item ID + title)
    ↓
Dedup ledger check (24h cooldown)
    ↓
L1 0.8B ticketization (one item per call)
    ↓
Jira create/update (based on dedup_decision)
    ↓
Ledger update (fingerprint → Jira key mapping)
```

## Running

```bash
# Ticketize top 5 roadmap items
MODE=roadmap MAX_FINDINGS=5 ./cmd/finding-ticketizer/finding-ticketizer

# Custom source
MODE=roadmap ROADMAP_SOURCE=docs/CUSTOM_ROADMAP.md MAX_FINDINGS=3 ./cmd/finding-ticketizer/finding-ticketizer
```

## Dedup Behavior

- Same roadmap item won't be re-ticketed within 24 hours
- Ledger shared with finding-ticketizer at `/var/lib/zen-brain1/ticketizer/finding-ledger.json`
- Roadmap items prefixed with `roadmap-` in fingerprint to avoid collision with findings

## Relationship to Finding Ticketizer

| Aspect | Finding Ticketizer | Roadmap Ticketizer |
|--------|-------------------|--------------------|
| Source | Discovery artifacts (markdown tables) | ROADMAP_ITEMS.md |
| Extraction | Parse markdown table rows | Parse `- **ID**: Description` lines |
| Fingerprint | `type:file:description` | `itemID:title` |
| L1 role | Draft ticket from code evidence | Draft ticket from item description |
| Ledger | Same ledger, different prefix | Same ledger, `roadmap-` prefix |

## Design Rules

1. **0.8B L1 does first-pass ticket drafting** — not GLM-5
2. **Code does extraction** — model never parses raw roadmap docs
3. **One item per L1 call** — no batch ticketization
4. **Max 5 per run** — don't flood Jira
5. **Fail-open** — ticketizer failure never blocks other work
6. **Shared dedup** — same ledger, same cooldown window
