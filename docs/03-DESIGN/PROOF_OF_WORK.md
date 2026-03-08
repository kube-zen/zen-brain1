# Proof‑of‑Work Bundle

## Purpose

A proof‑of‑work bundle is a structured record of what was accomplished during a Zen‑Brain session. It serves as:

- **Audit trail** for SR&ED tax‑credit evidence.
- **Rollback/continuation** context for future sessions.
- **Quality signal** for human reviewers.
- **Input to downstream processes** (e.g., deployment, documentation).

Every completed session (successful or failed) must produce a proof‑of‑work bundle.

## Bundle Contents

### 1. Session Identifiers
```yaml
session_id: "session‑abc‑123"
work_item_id: "ZB‑456"
source_key: "ZB‑456"            # External system key (Jira, GitHub, etc.)
source_system: "jira"           # Office connector that provided the work
```

### 2. Summary of Intent
- **Original work‑item title and description**.
- **Analysis result** (if available) – confidence, estimated cost, BrainTaskSpecs.
- **Human‑approved intent** (if approval was required).

### 3. Files Changed
A list of all files modified, created, or deleted, with references to the actual diffs.

```yaml
files_changed:
  - path: "internal/office/jira/connector.go"
    diff_ref: "evidence/abc123/diff‑connector.patch"
    change_type: "modified"
  - path: "docs/03‑DESIGN/PROOF_OF_WORK.md"
    diff_ref: "evidence/abc123/diff‑proof‑work.patch"
    change_type: "created"
```

Diffs are stored as **evidence items** (type: `diff`) and referenced by their evidence ID.

### 4. Tests Run and Results
For code‑change sessions, include:

- **Test suite executed** (unit, integration, etc.).
- **Pass/fail counts**.
- **Test logs** (as evidence items).
- **Coverage delta** (if measured).

```yaml
tests:
  suite: "unit"
  total: 42
  passed: 42
  failed: 0
  logs_ref: "evidence/abc123/test‑logs.txt"
  coverage_delta: "+0.5%"
```

### 5. Lint/Build Results
Output from static‑analysis tools, linters, and build systems.

```yaml
lint:
  tool: "golangci‑lint"
  result: "passed"
  warnings: 0
  errors: 0
  output_ref: "evidence/abc123/lint‑output.txt"
build:
  result: "success"
  binary_size: "15.2 MiB"
  duration_seconds: 12
  output_ref: "evidence/abc123/build‑output.txt"
```

### 6. Artifacts / Evidence References
Pointers to all stored evidence items:

```yaml
evidence_refs:
  - id: "ev‑001"
    type: "hypothesis"
    description: "Expected Jira connector to accept V6 attribution headers."
  - id: "ev‑002"
    type: "experiment"
    description: "Ran connector test with mock Jira server."
  - id: "ev‑003"
    type: "observation"
    description: "Mock server returned 200, headers were correctly formatted."
  - id: "ev‑004"
    type: "measurement"
    description: "Latency < 100ms for Jira API call."
  - id: "ev‑005"
    type: "analysis"
    description: "Confirmed AI‑attribution header passes Jira’s webhook validation."
  - id: "ev‑006"
    type: "conclusion"
    description: "Jira connector ready for production use."
```

### 7. Risks / Open Questions
Any uncertainties, trade‑offs, or known limitations discovered during the session.

```yaml
risks:
  - description: "Jira webhook rate‑limit may be exceeded under high load."
    severity: "low"
    mitigation: "Add exponential backoff in connector."
open_questions:
  - "Should we cache Jira project metadata to reduce API calls?"
  - "Is the AI‑attribution header compatible with Jira Data Center?"
```

### 8. Recommended Next Action
If the session is part of a larger effort, suggest what should happen next.

```yaml
next_action:
  type: "create_ticket"
  summary: "Implement exponential backoff for Jira connector"
  priority: "medium"
  estimated_cost_usd: 0.75
```

## Storage

Proof‑of‑work bundles are stored as **evidence items** (type: `proof_of_work`) within the session’s evidence collection. The bundle is serialized as JSON and can be retrieved via the session manager.

## Generation

The bundle is assembled by the **Session Manager** when a session reaches a terminal state (`completed`, `failed`, `blocked`, `canceled`). It pulls together:

- Session metadata
- Evidence items collected during execution
- Git diff output (if the session involved code changes)
- Test/lint/build logs (if available)
- Planner’s analysis and approval records

## Usage Examples

### SR&ED Evidence
The bundle provides a complete narrative of the R&D activity:
- **Hypothesis** (what we thought would work)
- **Experiment** (what we tried)
- **Observation** (what we saw)
- **Measurement** (quantitative results)
- **Analysis** (what we concluded)
- **Conclusion** (decision made)

### Rollback / Continuation
If a session fails, the next agent (or human) can inspect the bundle to understand what was attempted and where it stopped. The bundle may contain enough context to resume from the last known good state.

### Quality Gates
Human reviewers can scan the bundle for:
- Missing tests
- Unaddressed risks
- Incomplete evidence
- Unreasonable next‑action recommendations

## Related Documents

- [Session Manager](ZEN_JOURNAL.md) – evidence collection and storage.
- [Bounded Orchestrator Loop](../../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – session lifecycle.
- [SR&ED Taxonomy](../01-ARCHITECTURE/ADR/0002_SRED_TAXONOMY.md) – evidence categories.