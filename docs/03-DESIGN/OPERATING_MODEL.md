# Operating Model — Zen-Brain Business OS

**Version:** 1.0
**Updated:** 2026-03-27 (aligned from Board Design Manual v5 + Business OS Master Spec v5)
**Status:** Architecture alignment document

## Current Reality Statement

**Factory and Office are the current focus** and are approaching usable production shape. All construction blocks (0–6) are complete. The system runs scheduled discovery, generates reports, creates Jira issues with lifecycle tracking, and ticketizes findings into actionable work items.

**The Board layer is conceptually defined** (see Board Design Manual v5) but should not be put into full production until lower layers are more rock solid. The Board's adversarial synthesis pipeline, multi-model voting, and calibration weighting are designed for strategic decisions — not day-to-day execution.

**The missing immediate architectural layer is Strategy / Portfolio Office.** This layer sits between Board and Office, translating strategic direction into actionable portfolios, programs, and release slices. Without it, the system can discover and ticketize tactical work but lacks structured strategic decomposition.

## Architecture Layers

```
┌─────────────────────────────────────────────────┐
│ LAYER 0 — Leonardo / Brain of the Brain         │
│ Final strategic authority                        │
├─────────────────────────────────────────────────┤
│ LAYER 1 — Board / Executive                     │
│ Strategic steering, objective function,         │
│ portfolio direction, version/release approval   │
├─────────────────────────────────────────────────┤
│ LAYER 2 — Strategy / Portfolio Office           │  ← MISSING
│ Decomposes strategy into portfolios, programs,  │
│ epics, milestones. Dependencies, blockers,      │
│ critical path, release slicing.                 │
├─────────────────────────────────────────────────┤
│ LAYER 3 — Office                                │
│ Finance, HR, grants, compliance, security,      │
│ product/program coordination                    │
├─────────────────────────────────────────────────┤
│ LAYER 4 — Factory                               │
│ Executes bounded tasks, produces artifacts,     │
│ remediates tickets, reports outcomes            │
└─────────────────────────────────────────────────┘
```

### Layer 0 — Leonardo / Brain of the Brain

**Status:** Conceptual — not in production

- Final strategic authority for the entire system
- Sets top-level intent, constraints, and priorities
- Owns the objective function that all lower layers optimize against
- Can override any lower-layer decision

This layer is currently the human operator. Future versions may include a meta-governance model, but the Board Design Manual explicitly warns against putting zen-brain itself on the Board until the system has extensive calibration data.

### Layer 1 — Board / Executive

**Status:** Conceptual — design defined (Board Design Manual v5), not runtime

- Strategic steering and major policy decisions
- Objective function definition
- Portfolio direction and prioritization
- Version and release approval
- Risk assessment and mitigation decisions
- Composition: multiple independent AI models + human

The Board does NOT:
- Execute tasks
- Manage day-to-day operations
- Make micro-decisions
- Replace the Office or Factory layers

See [BOARD_REVIEW_CADENCE.md](../05-OPERATIONS/BOARD_REVIEW_CADENCE.md) for the operational process.

### Layer 2 — Strategy / Portfolio Office

**Status:** Missing — this is the critical gap

See [PORTFOLIO_OFFICE_LAYER.md](PORTFOLIO_OFFICE_LAYER.md) for full definition.

Responsibilities:
- Decomposes Board-level strategy into portfolios, programs, epics, milestones
- Defines dependencies, blockers, critical path
- Decides what belongs in 1.1 / 1.2 / 2.0
- Maintains roadmap and Jira portfolio structure
- Coordinates Office and Factory execution
- Tracks capacity vs demand across departments

### Layer 3 — Office

**Status:** Partially implemented — Jira connector, intent analysis, session management

Converts strategic direction into department-specific operating work:

| Department | Status | Notes |
|-----------|--------|-------|
| Engineering | Active | Primary execution department |
| Operations | Active | Cross-project infra support |
| Security | Active | Scanning, compliance monitoring |
| Finance | Planned | Budget tracking, cost optimization |
| Compliance | Planned | SR&ED, IRAP, SOC2 evidence |
| Grants/Funding | Planned | Government funding applications |
| HR | Deferred | Minimal for solo/small-team ops |
| Marketing | Deferred | Brand, content, lead gen |
| Sales | Deferred | Lead qualification, proposals |
| QA | Planned | Independent testing services |

See [BLOCK2_OFFICE.md](BLOCK2_OFFICE.md) for implementation details.

### Layer 4 — Factory

**Status:** Production — running scheduled batches, creating Jira issues, ticketizing findings

- Executes bounded tasks assigned by Office or directly from Jira
- Produces artifacts (reports, code, evidence packs)
- Remediates tickets marked for automated fix
- Reports outcomes and telemetry back upward
- Uses L1 (0.8B Qwen3.5) for first-pass work, L2 for quality gates

Current Factory capabilities:
- Scheduled discovery batches (hourly, quad-hourly, daily)
- 10 report classes (defects, tech_debt, dead_code, etc.)
- Finding-to-ticket pipeline with dedup
- Roadmap-to-ticket pipeline with dedup
- Jira parent/child lifecycle with per-task labels
- Fail-closed validation with grounding checks

## The Modern Shift

The Business OS Master Spec v5 establishes a governance model that combines:

**Top-down governance** — strategic direction flows from Board → Portfolio Office → Office → Factory. Lower layers do not set strategy; they execute it.

**Bottom-up continuous improvement** — workers/cells can suggest local improvements, automation, and process changes. Factory workers can flag recurring patterns, suggest evidence improvements, or propose workflow changes. These suggestions flow upward through the telemetry and reporting chain.

The constraint: **strategic direction still comes from above.** A worker can suggest "this evidence command produces better results" but cannot decide "we should add a new report class." That decision belongs to the Portfolio Office or Board.

## Provider-Agnostic Model Policy

Every layer may use local or API models. The architecture does not assume one specific model vendor:

- **Prefer local models** where sufficient (current: Qwen3.5 0.8B on llama.cpp for Factory L1)
- **Escalate to API models** where justified (current: GLM-5 for supervision, complex analysis)
- **Model capability registry** (future) will track which models handle which task types effectively
- **No permanent vendor lock-in** — switching models should require configuration changes, not architecture changes

The LLM Gateway design (see [LLM_GATEWAY.md](LLM_GATEWAY.md)) provides provider-agnostic routing with fallback chains.

## Human-AI Interaction Model

| Interaction Type | Who Decides | Jira Signal | Example |
|-----------------|-------------|-------------|---------|
| AI-executable | AI executes autonomously | `ai:executable` label | Routine report generation |
| Approval-gated | AI proposes, human approves | `ai:needs-approval` + workflow | New Jira issue creation |
| Review-required | AI executes, human reviews | `ai:needs-review` label | Finding ticketization |
| Human-only | Human executes | No `ai:` prefix | Strategic decisions |
| Escalated | AI flags for attention | `ai:blocked` + comment | Validation failure |
| Blocked | Cannot proceed | `ai:blocked` + blocker label | Missing dependency |

These flags are Jira-driven governance controls. They apply across all layers.

## Cross-Layer Communication

```
Board → Portfolio Office: Strategy documents, objective function, constraints
Portfolio Office → Office: Programs, epics, milestones, release plans
Portfolio Office → Factory: Execution priorities, capacity allocation
Office → Factory: Task assignments, approval gates, escalation requests
Factory → Office: Outcomes, telemetry, evidence, escalation triggers
Factory → Portfolio Office: Throughput data, capacity signals, blockers
Office → Portfolio Office: Department capacity, cross-department dependencies
Portfolio Office → Board: Status reports, roadmap deltas, risk flags
Board → Human: Executive summaries, strategic recommendations
```

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-03-27 | Initial alignment from Board Design Manual v5 + Business OS Master Spec v5 |
