# Active Roadmap Items

**Source of truth for actionable work items.**
**Updated:** 2026-03-27 (Phase 38)

Each item is one bounded unit of work suitable for L1 execution or manual implementation.

## Intelligence Layer (P1 — Active)

- **IL-1**: L2 quality gate policy — L2 grades L1 output when flagged, bounded synthesis only
- **IL-2**: Test_gaps report stability — intermittent failure, needs evidence-shaping if it repeats
- **IL-3**: Stub-hunting dedup — repeated stubs across runs should update existing tickets, not create new ones

## Discovery-to-Ticket Pipeline (P2 — Active)

- **DT-1**: Finding remediation template — bounded L1 fix attempts for ai:finding tickets marked bounded_fix_l1
- **DT-2**: Finding remediation queue — top-N from open ai:finding tickets, validate after fix
- **DT-3**: Stub-hunting to ticketizer wiring — currently pilot on defects only, extend to stub_hunting

## Operations & Reliability (P3 — Planned)

- **OP-1**: Retention policy enforcement — script exists, wire into scheduler for automatic 24h cleanup
- **OP-2**: Health check integration — health-check.sh results → metrics → Jira if degraded
- **OP-3**: Binary guardrail CI integration — pre-commit hook for scripts/guardrails/check-no-tracked-binaries.sh

## Architecture Cleanup (P4 — Planned)

- **AC-1**: Factory template upgrade — remaining scaffold/echo templates → review:real execution
- **AC-2**: Jira workflow state machine — richer transitions beyond label-only (in-progress → review → done)
- **AC-3**: Dead-code cleanup — items identified by dead_code report, bounded removal PRs

## Platform Integration (P5 — Future)

- **PI-1**: Prompt system enhancement — YAML/JSON templates, role-based profiles
- **PI-2**: Model capability registry — evaluation harness, automatic model-task assignment
- **PI-3**: Provider set optimization — evaluate reducing to 2 providers
