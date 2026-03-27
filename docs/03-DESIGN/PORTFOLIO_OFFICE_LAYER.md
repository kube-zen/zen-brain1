# Strategy / Portfolio Office Layer

**Version:** 1.0
**Updated:** 2026-03-27
**Status:** Missing layer — design specification

## Purpose

The Strategy / Portfolio Office is the **missing architectural layer** between the Board (Layer 1) and Office (Layer 3). It translates strategic direction into actionable portfolios, programs, and release slices. Without it, the system can execute tactical work but lacks structured strategic decomposition.

## The Gap

Currently, zen-brain1 has:
- ✅ Factory (Layer 4) — running, producing artifacts
- ✅ Office (Layer 3) — partially implemented, Jira integration
- ❌ Strategy / Portfolio Office (Layer 2) — **not implemented**
- ❌ Board (Layer 1) — design exists, not runtime

The gap means: when Factory discovers 15 actionable items (as it does), there is no structured process to say "these 3 go in 1.1, these 5 go in 1.2, these are 2.0 scope." Currently, roadmap slicing is ad-hoc and done by the human operator.

## Responsibilities

### Inputs
- Board-level strategy documents (when Board is operational)
- Human-defined priorities and constraints
- Factory throughput data and capacity signals
- Office department capacity reports
- Jira portfolio data (open issues, blockers, dependencies)
- Discovery findings and ticketization output

### Outputs
- Portfolio definitions (groups of related work)
- Program decompositions (epics → milestones → tasks)
- Release slicing (what goes in 1.1, 1.2, 2.0)
- Dependency maps and critical path analysis
- Capacity vs demand analysis
- Blocker identification and escalation
- Jira portfolio/epic structure updates
- Roadmap updates with version assignments

### Core Functions

1. **Strategy Decomposition**
   - Break Board-level objectives into portfolios and programs
   - Define acceptance criteria for each program
   - Assign to appropriate Office departments
   - Set timeline expectations based on capacity

2. **Release Slicing**
   - Maintain version model (1.0 → 1.1 → 1.2 → 2.0)
   - Decide what belongs in each release based on:
     - Dependencies and critical path
     - Capacity constraints
     - Risk profile
     - Strategic priority
   - Produce release plans with explicit scope boundaries

3. **Dependency Management**
   - Track cross-epic dependencies
   - Identify critical path items
   - Flag blockers for escalation
   - Coordinate cross-department work sequencing

4. **Capacity Planning**
   - Monitor Factory throughput (tasks/day, success rate, blocked rate)
   - Track Office department capacity
   - Forecast demand vs supply
   - Recommend scaling or reprioritization

5. **Roadmap Ownership**
   - Own `ROADMAP_ITEMS.md` and version assignments
   - Update priorities based on Board decisions and reality
   - Ensure roadmap reflects what actually happened, not what was planned
   - Maintain "done" list alongside "todo" list

6. **Jira Portfolio Structure**
   - Maintain epic → story → task hierarchy
   - Ensure finding tickets are linked to programs
   - Track cross-project dependencies via Jira links
   - Report portfolio health to Board

## Relation to Other Layers

### Board → Portfolio Office
Board sets strategy. Portfolio Office decomposes it. Portfolio Office does NOT set strategy — it translates and sequences. If Portfolio Office identifies a strategic conflict or impossibility, it escalates to Board.

### Portfolio Office → Office
Portfolio Office assigns programs to Office departments with scope, timeline, and acceptance criteria. Office manages execution within those constraints.

### Portfolio Office → Factory
Portfolio Office sets execution priorities. Factory executes. Portfolio Office monitors throughput and adjusts priorities based on capacity. Factory does not decide what to work on — Portfolio Office does.

### Factory → Portfolio Office
Factory reports outcomes, throughput, and blockers. This is the feedback loop that drives re-prioritization. If Factory shows 70% of tasks are blocked by a specific dependency, Portfolio Office escalates.

## Current Implementation Path

The Portfolio Office does not need to be a separate binary or service initially. It can start as:

1. **Phase 1 (immediate):** Structured `ROADMAP_ITEMS.md` with version assignments, maintained by human + AI collaboration
2. **Phase 2 (1.1):** Automated capacity monitoring — Factory throughput → priority adjustment suggestions
3. **Phase 3 (1.2):** Dependency tracking — automated blocker detection and escalation
4. **Phase 4 (2.0):** Full Portfolio Office runtime — automated decomposition, release slicing, Jira portfolio management

## Success Criteria

The Portfolio Office is working when:
- Every Jira ticket can be traced to a program/epic
- Every release has an explicit scope boundary
- Blockers are detected and escalated within hours, not days
- Roadmap reflects reality (completed + remaining), not just aspirations
- Capacity signals drive prioritization, not just urgency
