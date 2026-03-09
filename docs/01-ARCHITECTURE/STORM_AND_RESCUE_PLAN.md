# Storm and Rescue Plan for Zen-Brain 0.1

## Purpose

Zen-Brain 0.1 proved many useful ideas and contains valuable code.
It also contains structural pollution:
- gateway centralization
- queue sprawl
- script sprawl
- root-level document sprawl
- overlapping abstractions

This plan defines how to rescue the good parts without importing the pollution.

## Rule

**Rescue behavior, patterns, and selected code. Do not rescue architecture-by-accident.**

## Rescue Process

### Phase R1 — Inventory
Create a table for 0.1 modules:
- path
- category
- purpose
- maturity
- coupling risk
- target in 1.0
- rescue mode

Rescue modes:
- **LIFT**: small code can be moved almost directly
- **ADAPT**: useful but must be reshaped
- **REFERENCE**: keep as design reference only
- **DISCARD**: explicitly leave behind

### Phase R2 — Prioritize
Prioritize in this order:

1. Jira/ticketing knowledge
2. provider fallback/calibration
3. watchdog/long-run discipline
4. proof/evidence/report artifacts
5. task/work templates
6. queue semantics
7. retrieval/query-shaping ideas

### Phase R3 — Destination-first rescue
Do not copy code until the 1.0 destination is known.

**Example destinations:**
- office adapter
- llm/provider
- factory/proof
- docs/examples/blueprints
- future bounded orchestrator/watchdog
- kb query shaping

### Phase R4 — Gate each rescue batch
Every rescue PR should answer:
1. Why is this valuable?
2. Why does it fit 1.0 boundaries?
3. Why is direct copy acceptable, or why is adaptation required?
4. Does it reduce time to trustworthy usefulness?
5. Does it import pollution?

If the answer to (5) is yes, stop and rework.

## Rescue Inventory

*Note: This inventory will be populated as we analyze zen-brain 0.1*

| Path | Category | Purpose | Maturity | Coupling Risk | Target in 1.0 | Rescue Mode | Priority |
|------|----------|---------|----------|---------------|---------------|--------------|----------|
| `internal/gateway/jira_integration.go` | Jira/Ticketing | Jira API client with authentication, ticket fetching, field updates | High (production-tested) | Medium (Jira-specific) | Office adapter | ADAPT | High |
| `internal/gateway/jira_autowork.go` | Jira/Ticketing | Ticket claiming, transition handling, ADF parsing, work verification | High (production-tested) | High (gateway-coupled) | Office adapter + Factory patterns | ADAPT | High |
| `internal/gateway/fallback_chain.go` | Provider Fallback | Interface for provider fallback chains | Medium (clean interface) | Low (abstract) | LLM/provider routing | LIFT | High |
| `internal/gateway/default_fallback_chain.go` | Provider Fallback | Default implementation with context-aware routing | High (production-tested) | Medium (config-coupled) | LLM/provider routing | ADAPT | High |
| `internal/watchdog/watchdog.go` | Watchdog | Error recording, stuck detection, blocker reporting | Medium (working) | Low (standalone) | Bounded orchestrator | LIFT/ADAPT | Medium |
| `evidence-pack/templates/` | Evidence/Proof | Proof/evidence templates for different work types | High (production-used) | Low (templates only) | Proof-of-work generation | LIFT | High |
| `internal/provider/fallback.go` | Provider Fallback | Fallback provider implementation | Medium (working) | Medium (provider-coupled) | LLM/provider | ADAPT | Medium |
| `internal/providers/calibration.go` | Provider Calibration | Model calibration and performance tracking | Medium (experimental) | Medium (provider-coupled) | LLM/provider metrics | ADAPT | Medium |
| `internal/gateway/night_calibration.go` | Provider Calibration | Nightly calibration runs | Low (experimental) | High (gateway-coupled) | Reference only | REFERENCE | Low |
| `task-templates/` | Task Templates | Predefined task templates for common work | High (production-used) | Low (content only) | Work blueprints/docs | ADAPT | Medium |
| `scripts/` (selected) | Operations | Operational scripts for maintenance | Varies | High (shell sprawl) | Reference only | REFERENCE | Low |
| `internal/gateway/` (as whole) | Gateway System | Central orchestrator | High (production) | Very High (monolithic) | Do not import | DISCARD | N/A |
| `internal/gateway/queue_*.go` | Queue System | Queue management and policies | High (production) | Very High (gateway-coupled) | Reference only | REFERENCE | Low |

## Specific Rescue Notes

### Jira / Ticketing
**Likely ADAPT.**
Mine for edge cases and practical data-handling knowledge.

**Potential targets:**
- Field mapping logic
- Status transition handling
- Comment formatting/ADF handling
- Rate limiting/backoff strategies

### Provider Fallback/Calibration
**Likely ADAPT.**
Good candidate for 1.0 llm/provider routing.

**Potential targets:**
- Model selection logic
- Fallback chains
- Cost/performance calibration
- Error recovery patterns

### Watchdog
**Likely ADAPT or LIFT (small parts).**
Useful for bounded orchestration and long-run discipline.

**Potential targets:**
- Timeout enforcement
- Health checking
- Stuck task detection
- Cleanup/recovery logic

### Evidence/Proof Templates
**Likely ADAPT.**
High value for proof-of-work.

**Potential targets:**
- Report templates
- Evidence formatting
- SR&ED documentation patterns
- Compliance artifact generation

### Task Templates
**Likely ADAPT.**
Normalize into work blueprints / future role profiles.

**Potential targets:**
- Common task patterns
- Role-specific workflows
- Department templates
- Approval flow templates

### Queue Semantics
**Mostly REFERENCE / selective ADAPT.**
Do not revive the old queue system wholesale.

**Potential targets:**
- Priority handling
- Batching strategies
- Fair scheduling
- Dead letter handling

### Knowledge/RAG Stack
**Mostly REFERENCE / selective ADAPT.**
Do not restore custom 0.1 retrieval platform as the 1.0 default.

**Potential targets:**
- Query shaping patterns
- Chunking strategies
- Relevance scoring
- Cache invalidation

## Explicit Do-Not-Import List

- `internal/gateway` as a control center
- old queue subsystem as system-of-systems
- shell-script workflows
- root markdown sprawl
- unfinished board/consensus abstractions in runtime
- anything that forces Factory to know Jira/provider specifics

## Implementation Order

### Immediate (Phase 1)
1. Populate rescue inventory with high-priority items
2. Rescue Jira edge-case handling into office adapter
3. Rescue provider fallback logic into llm/provider

### Short-term (Phase 2)
1. Rescue evidence templates into proof-of-work
2. Rescue watchdog patterns for bounded orchestration
3. Adapt task templates into work blueprints

### Long-term (Phase 3)
1. Reference queue semantics for future improvements
2. Adapt retrieval patterns for KB query shaping
3. Document rescued patterns for future extension

## Output Expected from This Plan

- Complete rescue inventory (this document)
- One prioritized rescue backlog
- 1–3 initial clean rescue PRs/batches
- No architectural regression in 1.0

## Related Documents

- [ABSTRACTION_BOUNDARIES.md](./ABSTRACTION_BOUNDARIES.md) - Boundary definitions
- [SOURCE_OF_TRUTH.md](./SOURCE_OF_TRUTH.md) - Data ownership
- [CONSTRUCTION_PLAN.md](./CONSTRUCTION_PLAN.md) - Overall build sequence