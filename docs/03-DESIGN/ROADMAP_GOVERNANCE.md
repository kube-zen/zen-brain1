# Roadmap Governance

**Version:** 1.0
**Updated:** 2026-03-27
**Status:** Active — defines how roadmap decisions are made

## Core Principle

**Roadmap must be iterative, not frozen.** The roadmap adjusts regularly based on reality — what was completed, what was learned, what was blocked. This is not scope creep; it is empirical planning.

## Version Model

| Version | Scope | Status |
|---------|-------|--------|
| **1.0** | Foundation and stabilization | Complete — construction blocks 0–6 done, Factory + Office operational |
| **1.1** | Close operational gaps, strengthen governance | Planned |
| **1.2** | Strategy / Portfolio Office first slice | Planned |
| **1.x** | Incremental improvements | Ongoing |
| **2.0** | Board/executive runtime support | Future — after 1.x maturity |

### 1.0 — Foundation (Current)
Factory runs scheduled discovery, produces validated reports, creates Jira lifecycle, ticketizes findings. Office has Jira connector, intent analysis, session management. System is live and producing real output.

### 1.1 — Operational Hardening
Close the gaps between "it runs" and "it runs reliably at scale":
- Strengthen Jira lifecycle (richer state machine, not just labels)
- Improve remediation loop (L1 fix attempts for bounded tickets)
- Improve metrics and governance (utilization tracking, capacity signals)
- Close documentation gaps
- Stabilize intermittent report failures (test_gaps, formatting)
- Wire retention cleanup into scheduler
- Extend finding ticketizer to all discovery classes

### 1.2 — Portfolio Office First Slice
Introduce the Strategy / Portfolio Office layer:
- Structured roadmap with version assignments
- Dependency tracking across Jira items
- Capacity-based prioritization
- Release scope definitions
- Automated blocker detection

### 2.0 — Board Runtime
Productionize the Board/executive layer:
- Multi-model synthesis pipeline (Board Design Manual v5)
- Adversarial refinement loops
- Calibration weighting
- Strategic decision support
- Portfolio-level steering

**2.0 is explicitly deferred.** Do not begin Board productionization until 1.x shows sustained stability and the Portfolio Office is operational.

## How Version Increments Are Chosen

Version increments follow these rules:

1. **Patch (1.0.x):** Bug fixes, config changes, minor prompt tweaks — no architecture change
2. **Minor (1.1, 1.2):** New capability, workflow addition, layer introduction — bounded scope
3. **Major (2.0):** New architectural layer, fundamental operating model change — requires Board review

### Decision Process
- **1.1 scope** is set by the Portfolio Office (human + AI) based on 1.0 stability evidence
- **1.2 scope** is set based on 1.1 completion and capacity signals
- **2.0 trigger:** Portfolio Office recommends Board productionization when:
  - Factory has sustained >90% success rate for 30+ days
  - Office workflows cover ≥3 departments
  - Portfolio Office has been operational for ≥1 release cycle
  - Calibration data exists for ≥10 model-task combinations

## Roadmap Decision Authority

| Decision Type | Owner | Escalation |
|--------------|-------|------------|
| Task priority within release | Factory/Office | Portfolio Office |
| What goes in next release | Portfolio Office | Board (when operational) |
| Release scope boundaries | Portfolio Office | Board |
| Architecture changes | Portfolio Office + human | Board |
| Strategic direction | Board | Human (Layer 0) |

## Source of Truth

- **`ROADMAP_ITEMS.md`** — canonical list of actionable items with version assignments
- **Jira** — execution tracking, progress, blockers
- **Board Review minutes** — strategic decisions, priority changes (when Board is operational)

Roadmap changes must update `ROADMAP_ITEMS.md` AND corresponding Jira items. If they diverge, Jira is authoritative for execution status, `ROADMAP_ITEMS.md` for intent.

## Anti-Patterns

❌ **Frozen roadmap** — setting a 6-month plan and never adjusting
❌ **Reactive roadmap** — only adding items when someone reports a bug
❌ **Over-scoped release** — trying to do 2.0 before 1.0 is stable
❌ **Undocumented decisions** — changing priorities without updating ROADMAP_ITEMS.md or Jira
❌ **Skipping versions** — jumping from 1.0 to 2.0 because 1.1 seems boring
