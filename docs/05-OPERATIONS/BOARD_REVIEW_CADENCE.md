# Board Review Cadence

**Version:** 1.0
**Updated:** 2026-03-27
**Status:** Process definition — Board not yet operational

## Purpose

Regular Board rounds provide structured checkpoints where strategic direction is reviewed, adjusted, and communicated downward. Board rounds are not status meetings — they are decision forums.

## Cadence

| Review Type | Frequency | Trigger |
|-------------|-----------|---------|
| **Strategic Board Review** | Monthly | Calendar-based |
| **Milestone Completion Review** | Per milestone | Milestone reached |
| **Release Planning Review** | Per release | Release scope defined |
| **Emergency Strategic Review** | As needed | Major reality change |

### Monthly Strategic Board Review
Default cadence. Covers the full Board Round Template (below).

### Milestone Completion Review
Triggered when a defined milestone is completed. Focus:
- Did the milestone deliver what was promised?
- What was learned?
- Does this change subsequent milestone sequencing?

### Release Planning Review
Triggered before each release (1.1, 1.2, etc.). Focus:
- What is in scope?
- What is explicitly out of scope?
- Are dependencies satisfied?
- Is capacity sufficient?

### Emergency Strategic Review
Triggered when:
- A critical blocker is identified that affects multiple releases
- An external reality change invalidates current priorities
- Factory success rate drops below threshold for sustained period
- A significant security or compliance issue is discovered

## Board Round Template

Each Board round produces the following artifacts. The template is reusable for all review types.

### 1. What Changed Since Last Round
- New Jira issues created and resolved
- Factory throughput trends (tasks/day, success rate, blocked rate)
- Any incidents or escalations
- System health changes

### 2. What Was Completed
- Completed roadmap items with Jira keys
- Completed milestones
- Key outcomes and deliverables
- Metrics improvements or regressions

### 3. What Failed / Blocked
- Failed tasks with root cause
- Blocked items with blocker identification
- Repeated failures (pattern detection)
- Capacity constraints encountered

### 4. Capacity and Throughput Review
- Factory: tasks processed, success rate, avg duration, worker utilization
- Office: department capacity, cross-department load
- Queue depth: tasks waiting vs executing
- Resource bottlenecks

### 5. Roadmap Adjustments
- Items added (with justification)
- Items removed or deferred (with justification)
- Priority changes
- Version assignment changes (item moved from 1.2 to 1.1, etc.)

### 6. Release Slicing Proposals
- Proposed scope for next release
- Explicit out-of-scope items
- Risk assessment
- Dependency status

### 7. Dependency and Critical Path Review
- Active blockers
- Cross-epic dependencies
- Critical path items for next release
- External dependencies (tools, models, services)

### 8. Approval / Policy Changes
- New policies or policy modifications
- Approval threshold changes
- Human approval level adjustments
- Governance control changes

### 9. Jira Portfolio Updates
- Epic status changes
- New epics created
- Cross-project link updates
- Label/taxonomy updates

### 10. Next Board Date
- Proposed date for next review
- Expected agenda focus
- Any interim check-ins needed

## Output Format

Each Board round produces:
1. **Board Minutes** — structured document following the template above
2. **Jira Updates** — portfolio/epic changes, priority adjustments
3. **ROADMAP_ITEMS.md Update** — version assignments, new items, completed items
4. **Release Plan Update** (if applicable) — scope, timeline, dependencies

## Current State (Pre-Board)

The Board is not yet operational. Until it is, the following substitutes apply:

| Board Function | Current Substitute |
|---------------|-------------------|
| Strategic review | Human operator + periodic status checks |
| Release planning | Human operator + ROADMAP_ITEMS.md |
| Priority setting | Human operator + finding ticketizer auto-prioritization |
| Capacity review | Factory telemetry + scheduler metrics |

Board productionization is a **2.0 objective**. The process template above exists so that when the Board layer is ready, the operating cadence is already defined.
