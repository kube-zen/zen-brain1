# Skills and Subagents

## Purpose

Skills and subagents are **execution techniques** for automating repetitive repo tasks, parallelizing analysis, and providing bounded exploration lanes. They are **not architectural primitives**—Zen‑Brain’s core runtime must remain independent of any single vendor’s agent‑orchestration model.

## Guiding Principles

- **Narrow and composable** – Each skill does one small thing well.
- **Backed by canonical config or code** – Skills wrap existing functionality; they do not define new business logic.
- **Never the only source of truth** – Critical behavior must be encoded in contracts, config, or core code.
- **Vendor‑agnostic** – Subagent orchestration is an implementation detail, not a core dependency.

## Skills

A **skill** is a reusable procedure for a specific repo‑management or development task. Skills are invoked by humans or other agents, typically via a CLI or API.

### Suggested Future Skills

| Skill | Purpose | Canonical Backing |
|-------|---------|-------------------|
| **repo‑governance‑sweeper** | Enforce repo hygiene: check docs taxonomy, script placement, naming conventions. | `scripts/ci/` gates |
| **docs‑mover‑renamer** | Move docs to numbered directories, rename to UPPER_SNAKE_CASE, fix links. | `docs/` taxonomy rules |
| **qmd‑refresh** | Update qmd index for zen‑docs repository (search/index only). | `scripts/qmd_refresh.py` |
| **proof‑of‑work‑assembler** | Collect evidence, diffs, test results into a proof‑of‑work bundle. | Session manager evidence hooks |
| **jira‑kb‑scope‑mapper** | Map Jira issues to KB documentation scopes (for SR&ED evidence). | Office connector + KB strategy |
| **review‑verifier** | Run code review checks: tests pass, lint clean, evidence collected. | CI gates + test suite |

### Skill Design

- **Input**: Parameters (e.g., repo path, ticket key, target directory).
- **Output**: Structured result (success/failure, logs, evidence items).
- **Side effects**: Limited to the repo (file changes, commits) and external systems (Jira comments, CI status).
- **Idempotency**: Skills should be safe to re‑run.

### Example Skill: `repo‑governance‑sweeper`

```yaml
name: repo‑governance‑sweeper
description: "Run all repo‑hygiene gates and report violations."
implementation: "scripts/ci/run.py --suite governance"
parameters:
  - name: fix
    type: boolean
    default: false
    description: "Auto‑fix trivial violations (e.g., rename files)."
outputs:
  - name: violations
    type: list
  - name: fix_count
    type: integer
  - name: evidence_ref
    type: string
```

## Subagents

A **subagent** is a short‑lived, bounded agent that performs a specific analysis or verification task, often in parallel with other subagents.

### Use Cases

- **Bounded exploration** – Investigate a codebase region for tech‑debt, dependencies, patterns.
- **Parallel codebase analysis** – Split a large code review across multiple subagents.
- **Review / verification lanes** – Independent verification of a change (e.g., security, performance, compatibility).
- **Proof‑of‑work aggregation** – Collect evidence from multiple subagents into a unified bundle.

### Subagent Design

- **Scope** – Explicitly bounded (time limit, step limit, directory boundary).
- **Isolation** – Runs in its own workspace/worktree; cannot interfere with other subagents.
- **Evidence collection** – All findings are recorded as evidence items.
- **Termination guarantee** – Subagent stops after reaching its bounds (time, steps, etc.).

### Example Subagent: `code‑review‑verifier`

```yaml
name: code‑review‑verifier
purpose: "Verify a pull request meets quality gates."
bounds:
  max_duration_minutes: 15
  max_steps: 50
  workspace: "worktree‑{pr‑number}"
tasks:
  - skill: "repo‑governance‑sweeper"
    params: { fix: false }
  - skill: "proof‑of‑work‑assembler"
    params: { session_id: "{pr‑session}" }
output:
  - verdict: "approved|needs_changes|blocked"
  - evidence_refs: [ "ev‑001", "ev‑002" ]
  - next_action: "merge|request_changes|escalate"
```

## Integration with Core Runtime

### No Hard Dependencies

Zen‑Brain’s core runtime (`internal/planner`, `internal/session`, `internal/office`) must **not** depend on a specific skill/subagent framework. Instead:

- Skills are invoked via **generic task execution** (Factory layer, Block 3).
- Subagent orchestration is a **pluggable strategy** outside the core.
- Evidence collected by skills/subagents flows into the same **session manager** as LLM‑generated evidence.

### Configuration‑Driven

Skills and subagents are defined in **configuration files**, not hard‑coded. Example:

```yaml
skills:
  repo_governance_sweeper:
    command: ["python3", "scripts/ci/run.py", "--suite", "governance"]
    timeout_seconds: 300
    evidence_types: ["observation", "measurement"]

subagent_strategies:
  parallel_review:
    max_concurrent: 4
    skills:
      - repo_governance_sweeper
      - proof_of_work_assembler
    bounds:
      max_duration_minutes: 30
```

## Implementation Notes

- **Phase 1** – Document skills/subagents as a design concept (this document).
- **Phase 2** – Add lightweight scaffolding (`.codex/` or `agents/` directories) for future skill definitions.
- **Phase 3** – Implement a generic skill‑runner that can invoke Python scripts, Go binaries, or external tools.
- **Phase 4** – Integrate skill execution into the Factory layer (Block 3) as a special kind of BrainTask.

## Related Documents

- [Bounded Orchestrator Loop](BOUNDED_ORCHESTRATOR_LOOP.md) – execution bounds and termination.
- [Proof‑of‑Work Bundle](PROOF_OF_WORK.md) – evidence aggregation.
- [Repo Governance](../01-ARCHITECTURE/PROJECT_STRUCTURE.md) – hygiene rules that skills enforce.