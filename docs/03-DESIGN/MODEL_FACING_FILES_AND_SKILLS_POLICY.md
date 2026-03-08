# Model-Facing Files and Skills Policy

## Status

**Draft** (2026-03-08)

## Context

Zen-Brain has model-facing files (`AGENTS.md`, `WORKFLOW.md`) that provide guidance to AI agents and subagents. These files serve as an **adaptation layer** – they help AI systems understand how to work effectively with Zen-Brain.

However, it is critical that these files **do not become a source of truth**. The canonical source of truth must remain:

- **Code** – Go implementation, interfaces, contracts
- **Structured config** – YAML/JSON config files, CRDs
- **Canonical documentation** – ADRs, design docs, API specs

This policy ensures that:
1. Model-facing files are advisory and convenient
2. No critical policy lives only in markdown
3. Skills/subagents cannot exceed parent authority
4. Clear distinction between guidance and enforcement

## Decision

**Model-facing files (`AGENTS.md`, `WORKFLOW.md`) are advisory-only, not authoritative. Canonical truth lives in code and structured config.**

### AGENTS.md – Advisory Model Instructions

#### Purpose
Provide guidance to AI agents on how to work effectively with Zen-Brain:
- What agents are and their roles
- How to interpret WorkItems and tasks
- How to use tools appropriately
- Best practices for safety and quality

#### What IS Allowed in AGENTS.md
- **Behavioral guidance** – How to approach tasks, plan steps, use context
- **Tool usage patterns** – When and how to use specific tools
- **Role-specific advice** – Different guidance for planner, implementer, reviewer, ops
- **Safety reminders** – When to escalate, what to avoid
- **Examples** – Good and bad patterns for reference

#### What IS NOT Allowed in AGENTS.md
- **Policy definitions** – No policy (what must happen) should live here
- **Code-level details** – No implementation specifics or API contracts
- **Workflow state machines** – No canonical workflow definitions (belongs to `WORKFLOW.md` or design docs)
- **Configuration values** – No hardcoded config (belongs to config files or CRDs)
- **Schema definitions** – No data model changes (belongs to `pkg/contracts`)

#### Example of Good AGENTS.md Content
```markdown
## Planner Role

Your job is to break down Jira tickets into executable steps.

**Best Practices:**
- Always read the full WorkItem before planning
- Ask clarifying questions if requirements are ambiguous
- Prefer multi-step plans over monolithic actions
- Consider rollback steps for risky operations

**Tool Usage:**
- Use `kb-search` for similar issues
- Use `jira-read` to understand current state
- Use `git-read` to understand codebase

**Safety:**
- If task involves data deletion, escalate to human
- If task is unclear, ask before assuming
```

#### Example of Bad AGENTS.md Content
```markdown
## Planner Role Policy

**RULE:** All plans must be approved by Ops before execution.

This is BAD because:
- It defines a policy (what must happen)
- It lives in markdown, not code or config
- Ops approval logic belongs in `ZenExecutionPolicy`, not markdown
```

### WORKFLOW.md – Advisory Workflow Overview

#### Purpose
Provide high-level overview of execution lifecycle and major workflows:
- How work flows from Jira to completion
- Major phases: intake, planning, execution, verification
- Integration points with external systems
- Common patterns and anti-patterns

#### What IS Allowed in WORKFLOW.md
- **Lifecycle overview** – End-to-end view of work flow
- **Phase descriptions** – What happens in intake, planning, execution, verification
- **Integration diagrams** – How Office, Factory, Journal, Ledger interact
- **Common scenarios** – Example workflows for incident response, deployment, feature work

#### What IS NOT Allowed in WORKFLOW.md
- **State machine definitions** – Detailed state transitions belong in design docs or code
- **Canonical process** – Workflow variations (e.g., different approval flows) belong in config/CRDs
- **API specifications** – No interface definitions (belongs to Go code)
- **Policy enforcement** – No rules about what must happen (belongs to `ZenExecutionPolicy`)

#### Example of Good WORKFLOW.md Content
```markdown
## Incident Response Workflow

Typical flow for incident resolution:

1. **Triage** – Ops receives alert, creates Jira Incident
2. **Planner** – AI breaks down incident into diagnostic steps
3. **Implementer** – AI executes diagnostics, logs findings
4. **Reviewer** – AI validates diagnosis against known issues
5. **Ops** – Human reviews, approves fix plan
6. **Execution** – AI implements fix, monitors results
7. **Verification** – Ops confirms resolution, closes incident
```

#### Example of Bad WORKFLOW.md Content
```markdown
## Incident Response State Machine

**States:** Open → InProgress → AwaitingApproval → Resolved → Closed
**Transitions:**
- Open → InProgress: When Ops assigns incident
- InProgress → AwaitingApproval: When AI suggests production change

This is BAD because:
- It defines a canonical state machine (belongs in design doc or code)
- It hardcodes transitions (should be configurable or policy-driven)
```

### Skills and Subagents Policy

#### Purpose
Skills and subagents are **bounded execution helpers** for specific tasks. They provide:
- Specialized capabilities (e.g., "create-PR", "run-tests", "restart-service")
- Encapsulated workflows (e.g., "diagnose-incident", "deploy-service")
- Reusable patterns across multiple tasks

#### Policy: Skills/Subagents Cannot Exceed Parent Authority

**Core Rule:** A skill or subagent can never have broader authority than the parent task or role that invoked it.

##### Authority Hierarchy
```
Parent Task/Role (ZenRoleProfile)
    └─> Defines: allowed tools, execution policies, trust level
         └─> Invokes: Skill or Subagent
                  └─> Inherits: Same or narrower authority
                           └─> Cannot: Use tools not in parent scope
                           └─> Cannot: Skip policies required by parent
                           └─> Cannot: Operate at higher trust level
```

##### Boundedness Examples
| Parent Role | Allowed Tools | Skill Can Use | Skill Cannot Use |
|-------------|---------------|---------------|------------------|
| **Ops** | jira-read, git-write, kb-search | deploy-service | delete-data |
| **Implementer** | git-write, run-tests | create-PR | deploy-production |
| **Planner** | jira-read, kb-search, git-read | analyze-logs | modify-config |

##### Inheritance Rules
1. **Tool access** – Skill can only use tools allowed by parent role
2. **Execution policies** – Skill must respect parent's `ZenExecutionPolicy`
3. **Trust level** – Skill cannot operate at higher trust than parent
4. **Side effects** – Skill's side effects must be subset of parent's allowed effects
5. **Escalation** – Skill can only escalate to roles permitted by parent's `ZenHandoffPolicy`

#### When to Use Skills/Subagents

##### Good Use Cases
- **Encapsulated patterns** – "Create GitHub PR" is used across many tasks
- **Specialized tools** – "Diagnose database deadlock" requires deep domain knowledge
- **Multi-step workflows** – "Deploy and verify" has clear sub-steps
- **Cross-role reuse** – Same skill used by planner, implementer, and ops

##### Anti-Patterns
- **Unbounded authority** – Skills with full access to production are dangerous
- **Recursive spawning** – Skills that spawn other skills without limits
- **Policy bypassing** – Skills that ignore or override parent policies
- **Hidden escalation** – Skills that escalate without tracking

#### Implementation Guidance

##### In Design Docs
- Document skill purpose, inputs, outputs, side effects
- Specify authority requirements (which tools, which policies, which trust level)
- Provide examples of correct and incorrect usage

##### In Code
- Implement skill as function or struct with clear interface
- Validate skill invocation against parent role's `ZenRoleProfile`
- Log all skill invocations for audit trail

##### In Config
- Define skills as reusable entities with metadata
- Link skills to `ZenToolBinding` for tool access
- Tag skills with risk level and required trust

### Canonical Source of Truth

#### What IS Source of Truth
| Component | Source of Truth | Notes |
|-----------|------------------|-------|
| **Interfaces** | `pkg/*/interface.go` | Go code defines contracts |
| **Data Models** | `pkg/contracts/` | Canonical types and enums |
| **CRDs** | `api/v1alpha1/` | Kubernetes objects |
| **Config** | `configs/*.yaml`, CRDs | Runtime configuration |
| **Policies** | `ZenExecutionPolicy` CRDs, Go policy engine | Enforced rules |
| **Workflows** | Design docs + orchestrator code | State machines and flows |
| **Architecture** | ADRs, design docs | Design decisions |

#### What IS NOT Source of Truth
| Component | Not Source of Truth | Reason |
|-----------|-------------------|--------|
| **AGENTS.md** | Advisory | Guidance only, changes don't affect behavior |
| **WORKFLOW.md** | Advisory | Overview only, not canonical flow |
| **Comments in code** | Not source | Implementation details, not contracts |
| **Examples** | Not source | Illustrative only |

### Policy Enforcement

#### Repo Hygiene Gates
The `repo_layout_gate.py` ensures:
- `AGENTS.md` and `WORKFLOW.md` exist and are properly structured
- No policy definitions live in these files (policy belongs in CRDs or code)
- No schema definitions live in these files (schema belongs in `pkg/contracts`)

#### Pre-Commit Hooks
The `.githooks/pre-commit` hook can check:
- No hardcoded config values in AGENTS.md or WORKFLOW.md
- No policy statements (must, shall, required) in AGENTS.md or WORKFLOW.md
- No state machine definitions in WORKFLOW.md (belongs to design docs)

#### Design Review
When reviewing changes to AGENTS.md or WORKFLOW.md:
1. **Is this guidance or definition?** – Guidance is OK, definition belongs elsewhere
2. **Does this change behavior?** – If yes, move to code/config/CRDs
3. **Is this redundant?** – If already defined in contracts, don't duplicate
4. **Is this consistent?** – Must align with ADRs and design docs

## Consequences

### Positive
- **Clear separation** – Guidance (markdown) vs enforcement (code/config)
- **Easier to update** – Change config/CRDs, don't rewrite markdown
- **Auditable** – Code and config are versioned, reviewable
- **Testable** – Policies in code can be unit tested
- **Bounded skills** – Skills cannot exceed parent authority, reducing risk

### Negative
- **Learning curve** – New contributors must understand what belongs where
- **More files** – Config and CRDs in addition to markdown
- **Coordination** – Changes require updating both markdown and code

### Neutral
- **Guidance still valuable** – AGENTS.md and WORKFLOW.md help AI systems
- **Evolutionary** – Can start simple and add more structured policies over time

## Alternatives Considered

### Alternative 1: AGENTS.md and WORKFLOW.md as source of truth
- **Pros:** Single source of guidance, simpler
- **Cons:** Not auditable, not versionable, cannot be tested
- **Rejected:** Trusted operator needs enforceable, auditable source of truth

### Alternative 2: No model-facing files at all
- **Pros:** No confusion about what is authoritative
- **Cons:** AI systems have no guidance, harder to use effectively
- **Rejected:** Guidance layer is valuable for AI understanding

### Alternative 3: All policy in markdown
- **Pros:** Easy to read and edit
- **Cons:** Not enforceable, not testable, hard to audit
- **Rejected:** Policy must be in code/config/CRDs to be trusted

## Related Decisions

- [ADR-0003](../01-ARCHITECTURE/ADR/0003_CONTRACTS_PACKAGE.md) – `pkg/contracts` is canonical source of truth for types
- [ADR-0004](../01-ARCHITECTURE/ADR/0004_MULTI_CLUSTER_CRDS.md) – CRDs as control-plane foundation
- [Control Plane Vocabulary](../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md) – Policy objects as CRDs
- [Bounded Orchestrator Loop](../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – Orchestrator enforces policies
- [Skills and Subagents](SKILLS_AND_SUBAGENTS.md) – Design doc for execution helpers

## Implementation Status

### Current State
- [x] `AGENTS.md` exists and is advisory-only
- [x] `WORKFLOW.md` exists and is advisory-only
- [x] Skills/subagents documented with bounded authority
- [x] Repo gates prevent policy in markdown

### Next Steps
- [ ] Update `repo_layout_gate.py` to check for policy statements in AGENTS.md/WORKFLOW.md
- [ ] Add control-plane CRDs (`ZenRoleProfile`, `ZenExecutionPolicy`, etc.) to `api/v1alpha1/`
- [ ] Implement policy engine in orchestrator to enforce control-plane objects
- [ ] Add skill registration system linked to `ZenToolBinding`

## References

- AGENTS.md: [../../AGENTS.md](../../AGENTS.md)
- WORKFLOW.md: [../../WORKFLOW.md](../../WORKFLOW.md)
- Control Plane Vocabulary: [../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md](../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md)
- Skills and Subagents: [SKILLS_AND_SUBAGENTS.md](SKILLS_AND_SUBAGENTS.md)
- Bounded Orchestrator: [BOUNDED_ORCHESTRATOR_LOOP.md](BOUNDED_ORCHESTRATOR_LOOP.md)