# Zen-Brain1 Roadmap

**Updated:** 2026-03-27
**Version Model:** Iterative (1.0 → 1.1 → 1.2 → 2.0)

## Operating Principle

Factory + Office must be rock solid before full Board productionization. Strategy / Portfolio Office is the missing layer between Board and Office. Roadmap must be iterative, not frozen. Board rounds should regularly adjust roadmap and release slicing. Jira is the central human/AI work ledger across all layers.

---

## 1.0 — Foundation ✅ Complete

**Status:** Stabilizing. All construction blocks done. System in live production.

Completed:
- Construction blocks 0–6 (repo, Office, Nervous System, Factory, Intelligence, DX)
- Scheduled discovery batches (hourly, quad-hourly, daily)
- 10 report classes with evidence injection and validation
- Jira parent/child lifecycle with per-task labels
- Finding-to-ticket pipeline with dedup and L1 ticketization
- Roadmap-to-ticket pipeline with dedup
- L1 (0.8B Qwen3.5) first-pass execution, L2 quality gate

---

## 1.1 — Operational Hardening (Next)

**Goal:** Close the gap between "it runs" and "it runs reliably at scale."

### Scope

| Item | ID | Source | Priority |
|------|----|--------|----------|
| L2 quality gate policy | IL-1 | ZB-614 | Medium |
| Test_gaps report stability | IL-2 | ZB-615 | Medium |
| Stub-hunting dedup improvement | IL-3 | ROADMAP_ITEMS | Medium |
| Finding remediation template | DT-1 | ZB-616 | High |
| Finding remediation queue | DT-2 | ZB-617 | High |
| Stub-hunting ticketizer wiring | DT-3 | ROADMAP_ITEMS | Medium |
| Retention policy enforcement | OP-1 | ZB-618 | Medium |
| Health check integration | OP-2 | ZB-619 | Medium |
| Binary guardrail CI | OP-3 | ROADMAP_ITEMS | Low |
| Factory template upgrade | AC-1 | ROADMAP_ITEMS | Low |
| Jira workflow state machine | AC-2 | ROADMAP_ITEMS | Medium |
| Dead-code cleanup | AC-3 | ROADMAP_ITEMS | Low |

### Success Criteria
- Factory sustained >90% success rate for 30+ days
- Finding remediation loop closed (discover → ticketize → fix → validate)
- All scheduled batches have zero anonymous outcomes
- Retention cleanup automated

---

## 1.2 — Portfolio Office First Slice

**Goal:** Introduce the Strategy / Portfolio Office layer between Board and Office.

### Scope
- Structured roadmap with explicit version assignments
- Dependency tracking across Jira items
- Capacity-based prioritization (Factory throughput → priority signals)
- Release scope definitions with explicit boundaries
- Automated blocker detection and escalation
- Department-level capacity reporting

### Success Criteria
- Every Jira ticket traceable to a program/epic
- Every release has explicit scope boundary
- Blockers detected and escalated within hours
- Roadmap reflects completed + remaining, not just aspirations

---

## 2.0 — Board Runtime

**Goal:** Productionize the Board/executive layer for strategic decision support.

### Scope
- Multi-model synthesis pipeline (Board Design Manual v5)
- Adversarial refinement loops (3+ convergence)
- Calibration weighting with model performance ledger
- Strategic decision support
- Portfolio-level steering
- Monthly Board review cadence

### Trigger Conditions
All of the following must be true before 2.0 begins:
- Factory sustained >90% success rate for 30+ days
- Office workflows cover ≥3 departments
- Portfolio Office operational for ≥1 release cycle
- Calibration data exists for ≥10 model-task combinations
- Human operator has run ≥3 manual Board rounds

### Explicit Out of Scope for 2.0
- zen-brain as Board voting member (dangerous without calibration)
- Meta-board (Board reviewing itself)
- Market survey / GTM sessions
- Tournament structure (only valuable at 50+ ideas)

---

## Decision Authority

| Decision | Owner | Jira Signal |
|----------|-------|-------------|
| Task priority within release | Factory/Office | Priority field |
| What goes in next release | Portfolio Office (human) | `portfolio:` label |
| Release scope | Portfolio Office | Epic scope definition |
| Architecture change | Portfolio Office + human | ADR + Jira epic |
| Strategic direction | Board (future) | Board minutes → Jira |

## Related Documents
- [Operating Model](../03-DESIGN/OPERATING_MODEL.md) — layer architecture
- [Portfolio Office Layer](../03-DESIGN/PORTFOLIO_OFFICE_LAYER.md) — missing layer definition
- [Roadmap Governance](../03-DESIGN/ROADMAP_GOVERNANCE.md) — how version decisions are made
- [Board Review Cadence](../05-OPERATIONS/BOARD_REVIEW_CADENCE.md) — Board process template
- [Jira as System Ledger](../05-OPERATIONS/JIRA_AS_SYSTEM_LEDGER.md) — Jira governance model
- [ROADMAP_ITEMS.md](../../ROADMAP_ITEMS.md) — canonical actionable items list
