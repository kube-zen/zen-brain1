# ZenGate & ZenPolicy Design

## Overview

**ZenGate** is the admission controller that validates and authorizes requests before they enter the Factory. It implements input validation, authorization checks, and policy enforcement.

**ZenPolicy** is the declarative rule engine that defines what actions are allowed, required, or forbidden. Policies are defined once and enforced everywhere (Office and Factory).

Together, they form the **policy enforcement layer** of Zen‑Brain, ensuring security, compliance, and operational guardrails.

## ZenGate Interface

The `ZenGate` interface (`pkg/gate/interface.go`) defines admission control:

```go
type ZenGate interface {
    // Admit evaluates an admission request and returns a decision.
    Admit(ctx context.Context, req AdmissionRequest) (*AdmissionResponse, error)

    // Validate validates an admission request without making a decision.
    Validate(ctx context.Context, req AdmissionRequest) ([]ValidationError, error)

    // RegisterValidator registers a custom validator for a request type.
    RegisterValidator(ctx context.Context, validator Validator) error

    // RegisterPolicy registers a policy evaluator for a request type.
    RegisterPolicy(ctx context.Context, policy ZenPolicy) error

    // Stats returns gate statistics.
    Stats(ctx context.Context) (map[string]interface{}, error)

    // Close closes the gate.
    Close() error
}
```

### AdmissionRequest

Contains all information needed to evaluate admission:

- `RequestID`, `WorkItemID`, `SessionID`, `TaskID`
- `ClusterID`, `ProjectID`
- `Action` – the requested action (from `policy.Action`).
- `Resource` – the resource being acted upon.
- `Subject` – the entity making the request (agent, human, system).
- `Payload` – request‑specific data.
- `Timestamp`

### AdmissionResponse

The gate’s decision:

- `Allowed` – whether the request is allowed.
- `Reason` – human‑readable explanation.
- `RequiresApproval` – if approval is required.
- `ApprovalLevel` – required approval level (e.g., `"team_lead"`, `"manager"`).
- `Conditions` – conditions that must be satisfied.
- `Obligations` – obligations that must be performed (logging, notification, evidence collection).

## ZenPolicy Interface

The `ZenPolicy` interface (`pkg/policy/interface.go`) defines rule evaluation:

```go
type ZenPolicy interface {
    // Evaluate evaluates a policy for the given request.
    Evaluate(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error)

    // Rule management
    LoadRule(ctx context.Context, rule PolicyRule) error
    LoadRules(ctx context.Context, rules []PolicyRule) error
    RemoveRule(ctx context.Context, ruleName string) error
    ListRules(ctx context.Context) ([]PolicyRule, error)

    // Validation
    ValidateRule(ctx context.Context, rule PolicyRule) error

    // Monitoring
    Stats(ctx context.Context) (map[string]interface{}, error)

    // Cleanup
    Close() error
}
```

### EvaluationRequest

- `Action`, `Resource`, `Subject`, `Context`

### PolicyRule

A declarative rule with:

- `Name`, `Description`, `Version`, `Priority`
- `Conditions` – field‑based conditions (e.g., `subject.roles contains "admin"`).
- `Effect` – `allow`, `deny`, `require_approval`, `require_evidence`.
- `Obligations` – actions that must be performed if the rule matches.

## Policy Language

Policies are defined in YAML for readability and version control.

**Example policy file (`policies/zen‑brain‑prod.yaml`):**

```yaml
version: v1
rules:
  - name: require_approval_for_prod
    description: Require team‑lead approval for any production‑affecting change
    version: 1.0
    priority: 100
    conditions:
      - field: resource.attributes.environment
        operator: equals
        value: prod
      - field: action
        operator: in
        value: [execute_task, update_status]
    effect: require_approval
    obligations:
      - type: require_approval
        parameters:
          level: team_lead
          timeout_minutes: 60
      - type: log
        parameters:
          level: info
          message: "Production change requires approval"

  - name: limit_llm_calls_per_session
    description: Limit LLM calls per session to prevent runaway costs
    version: 1.0
    priority: 90
    conditions:
      - field: action
        operator: equals
        value: call_llm
      - field: subject.type
        operator: equals
        value: agent
    effect: allow
    obligations:
      - type: enforce_quota
        parameters:
          quota_key: "session:{session_id}:llm_calls"
          limit: 100
          window_seconds: 3600
          violation_action: deny

  - name: sred_evidence_collection
    description: Automatically collect SR&ED evidence for experimental tasks
    version: 1.0
    priority: 80
    conditions:
      - field: resource.attributes.sred_tags
        operator: not_equals
        value: []
      - field: action
        operator: equals
        value: execute_task
    effect: require_evidence
    obligations:
      - type: collect_evidence
        parameters:
          evidence_class: experiment_card
          sred_tags: "{{resource.attributes.sred_tags}}"
```

## Architecture

### Policy Engine

The policy engine evaluates rules in **priority order** (higher priority first). The first rule that matches determines the effect. If no rules match, a default deny (`Allowed: false`) is returned.

**Evaluation flow:**

1. Parse `AdmissionRequest` into `EvaluationRequest`.
2. Evaluate each rule’s conditions against the request.
3. If all conditions match, apply the rule’s effect and obligations.
4. Stop evaluation (unless rule specifies `continue: true`).

**Condition operators:** `equals`, `not_equals`, `in`, `not_in`, `contains`, `matches` (regex), `lt`, `gt`, `lte`, `gte`, `exists`, `not_exists`.

### Validators

Custom validators can be registered for specific actions:

```go
type Validator interface {
    Name() string
    SupportedActions() []policy.Action
    Validate(ctx context.Context, req AdmissionRequest) ([]ValidationError, error)
}
```

Example: a `CostLimitValidator` that checks if a task’s estimated cost exceeds the project budget.

### Integration with ZenGate

ZenGate orchestrates validation and policy evaluation:

1. **Validation phase** – run all registered validators for the request’s action.
2. **Policy evaluation phase** – call `ZenPolicy.Evaluate`.
3. **Decision phase** – combine validation errors and policy result into `AdmissionResponse`.

If any validator fails, the request is denied with validation errors.

## Storage

Policies are stored in **CockroachDB** for durability and distributed access.

**Table schema:**

```sql
CREATE TABLE policy_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name STRING NOT NULL,
    description STRING,
    version STRING NOT NULL,
    priority INT DEFAULT 0,
    conditions JSONB NOT NULL,
    effect STRING NOT NULL,
    obligations JSONB,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (name, version)
);

CREATE INDEX policy_rules_priority ON policy_rules (priority DESC);
```

**Loading policies:** On startup, ZenGate loads all active policies from the database. Changes can be made dynamically via API or CRDs.

## Multi‑cluster Considerations

- Each cluster runs its own ZenGate instance (data plane).
- Policies can be **cluster‑scoped** or **global**.
  - Cluster‑scoped: stored in the cluster’s local database.
  - Global: stored in the control plane’s database and synced to data plane agents.
- `ClusterID` and `ProjectID` in requests enable fine‑grained policy targeting.

## Configuration

Example `config.yaml` snippet:

```yaml
gate:
  # Validation
  validators:
    - name: cost_limit
      enabled: true
    - name: resource_quota
      enabled: true

  # Policy engine
  policy_engine:
    provider: "certo"  # or "opa", "builtin"
    default_effect: "deny"
    audit_log_enabled: true

  # Database
  database:
    uri: "postgresql://root@cockroachdb‑public:26257/zen_brain?sslmode=disable"

policy:
  # Rule storage
  rule_refresh_interval_seconds: 30

  # Default policies (built‑in)
  default_policies:
    - name: deny_all
      effect: deny
      priority: 0
```

## Monitoring

**Metrics (Prometheus):**

- `zen_gate_requests_total` – total admission requests.
- `zen_gate_requests_allowed_total` – allowed requests.
- `zen_gate_requests_denied_total` – denied requests.
- `zen_gate_evaluation_latency_seconds` – histogram of evaluation latency.
- `zen_policy_rules_loaded` – gauge of loaded rules.
- `zen_policy_evaluations_total` – counter of policy evaluations.

**Dashboards (Grafana):**

- Admission decision rates (allowed/denied/requires‑approval).
- Top denied actions (by action, resource, subject).
- Policy evaluation latency (p50, p95, p99).
- Rule hit counts (which rules are most frequently matched).

## Integration Points

- **Office** – ZenGate evaluates work item ingestion and planning requests.
- **Factory** – ZenGate evaluates task execution, resource creation, LLM calls.
- **ZenLedger** – validators can check budget limits.
- **ZenJournal** – all admission decisions are logged as `gate_enforced` events.
- **Human Approval System** – when `RequiresApproval` is true, the request is routed to the approval workflow.

## Open Questions

1. **Should we support policy as code (GitOps)?** – Yes, policies can be defined in a Git repository and synced via a controller.
2. **How to handle policy conflicts?** – Priority ordering resolves conflicts; we could also add a conflict detection tool.
3. **Should policies be versioned with semantic versioning?** – Yes, and support rolling updates with canary evaluation.
4. **How to test policies?** – Create a test suite that runs policies against historical requests to verify behavior.

## Next Steps

1. Implement `internal/policy/engine.go` – basic rule evaluation engine.
2. Implement `internal/gate/core.go` – validation + policy integration.
3. Create CRDs for `PolicyRule` and `PolicyBundle` (for GitOps).
4. Write unit and integration tests with example policies.
5. Integrate with Office and Factory components.

---

*This document is a living design spec; update as implementation progresses.*