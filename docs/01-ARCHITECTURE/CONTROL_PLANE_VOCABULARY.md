# Control Plane Vocabulary

## Status

**Draft** (2026-03-08)

## Context

As Zen-Brain evolves from a simple task executor to a multi-role, multi-domain system, we need **first-class control-plane concepts** to govern:

- **Who can do what** (roles and policies)
- **What tools are available** (tool registration and binding)
- **How execution is constrained** (execution policies and handoffs)
- **How compliance is enforced** (compliance profiles)
- **What workspace is safe** (workspace classes and trust levels)

These concepts must be elevated from "brainstorm terms" to **architectural objects** that are:
- Defined in contracts or CRDs
- Enforced by the orchestrator and gatekeeper
- Auditable and versioned
- Namespace and RBAC aware

## Decision

**Control-plane vocabulary is a first-class design concept for 1.1 and later, with CRD-backed implementations.**

### Core Concepts

#### ZenRoleProfile
**Purpose:** Define a role's identity, policy bindings, and tool access.

**Fields:**
- **Name** – Unique identifier (e.g., "planner", "implementer", "reviewer", "ops")
- **Description** – What this role does, its responsibilities
- **Policies** – List of `ZenExecutionPolicy` IDs that apply
- **AllowedTools** – List of `ZenToolBinding` IDs this role can use
- **MaxExecutionTime** – Maximum time before forced stop
- **TrustLevel** – Minimum trust level required for this role
- **ModelPreference** – Preferred model class (small, medium, large)
- **EscalationPolicy** – When and how to escalate to human

**Example:**
```yaml
name: planner
description: Breaks down Jira tickets into executable steps
policies:
  - bounded-planning
  - tool-selection-gated
allowedTools:
  - jira-read
  - kb-search
  - git-read
maxExecutionTime: 10m
trustLevel: standard
modelPreference: small
escalationPolicy:
  threshold: 3 # escalate after 3 failed attempts
  targetRole: human
```

#### ZenExecutionPolicy
**Purpose:** Define admission control rules for actions and operations.

**Fields:**
- **Name** – Unique identifier (e.g., "no-delete", "production-gated", "read-only")
- **Description** – What this policy enforces
- **Rules** – List of rules (allow/deny based on action, target, context)
- **Actions** – What happens when policy is violated (block, warn, require-approval)
- **Scope** – Where policy applies (global, namespace, role-specific)

**Example:**
```yaml
name: production-gated
description: Requires approval for production deployments
rules:
  - action: deploy
    target: production
    effect: require-approval
  - action: delete
    target: data
    effect: block
  - action: restart
    target: service
    effect: require-approval
scope: role-specific
appliesTo:
  - ops
  - implementer
```

#### ZenHandoffPolicy
**Purpose:** Define rules for cross-domain agent transitions and handoffs.

**Fields:**
- **Name** – Unique identifier (e.g., "ops-to-escalate", "implementer-to-reviewer")
- **Description** – When and how agents can hand off to each other
- **FromRole** – Source role for handoff
- **ToRole** – Destination role for handoff
- **AllowedActions** – What actions can be handed off (create, update, approve)
- **ApprovalRequired** – Whether human approval is needed for handoff
- **EscalationPath** – What to do if handoff fails (escalate to human, retry, abort)

**Example:**
```yaml
name: ops-to-escalate
description: Ops can escalate to human for high-severity incidents
fromRole: ops
toRole: human
allowedActions:
  - create-incident
  - update-status
approvalRequired: false
escalationPath:
  onFail: escalate-directly
```

#### ZenTool
**Purpose:** Define a tool's interface and capabilities (independent of runtime adapter).

**Fields:**
- **Name** – Unique identifier (e.g., "jira-read", "git-write", "kb-search")
- **Description** – What this tool does
- **InputSchema** – JSON schema for tool parameters
- **OutputSchema** – JSON schema for tool results
- **RiskLevel** – Inherent risk (none, low, medium, high, critical)
- **SideEffects** – Potential side effects (none, creates, modifies, deletes)
- **Category** – Tool category (jira, git, kb, database, api, internal)

**Example:**
```yaml
name: git-write
description: Write changes to Git repository
inputSchema:
  type: object
  properties:
    repo:
      type: string
    branch:
      type: string
    files:
      type: array
      items:
        type: object
outputSchema:
  type: object
riskLevel: medium
sideEffects:
  - modifies
category: git
```

#### ZenToolBinding
**Purpose:** Define namespace/RBAC-aware tool access configuration.

**Fields:**
- **ToolName** – Reference to `ZenTool` being bound
- **Namespace** – Namespace or project where binding applies
- **Roles** – Roles that can access this tool binding
- **Constraints** – Additional constraints (max files per day, max size, allowed repos)
- **ApprovalRequired** – Whether each tool use requires approval
- **AuditLog** – Whether to log every tool invocation

**Example:**
```yaml
toolName: git-write
namespace: zen-production
roles:
  - implementer
  - ops
constraints:
  maxFilesPerDay: 100
  allowedRepos:
    - github.com/kube-zen/*
approvalRequired: false
auditLog: true
```

#### ZenComplianceProfile
**Purpose:** Define compliance overlay for production deployments or audits.

**Fields:**
- **Name** – Unique identifier (e.g., "soc2", "iso27001", "fedramp")
- **Description** – Compliance framework this profile implements
- **Policies** – List of `ZenExecutionPolicy` IDs that apply
- **Requirements** – Specific compliance requirements (encryption, audit, access control)
- **AuditConfig** – What must be logged and retained
- **ApprovalRules** – Special approval requirements for this compliance profile

**Example:**
```yaml
name: fedramp
description: FedRAMP Moderate compliance profile
policies:
  - production-gated
  - audit-required
  - encryption-at-rest
requirements:
  - encryption: AES-256
  - accessControl: RBAC
  - auditRetention: 3years
auditConfig:
  logAll: true
  logRetention: 7years
  exportFormats:
    - json
    - csv
approvalRules:
  deploy:
    minApprovers: 2
    roles:
      - ops-director
      - security
  delete:
    minApprovers: 1
    roles:
      - data-owner
```

### Workspace Protection Model

#### WorkspaceClass
**Purpose:** Define workspace isolation and protection level.

**Classes:**
- **Isolated** – Per-task workspace, no shared state, auto-cleanup
- **Protected** – Per-session workspace, some shared state, cleanup on session end
- **Standard** – Shared workspace for trusted team, manual cleanup
- **Production** – Production workspace with full protection, read-only for most roles

#### ChangeClass
**Purpose:** Define change risk and approval requirements.

**Classes:**
- **Routine** – No special approval required (e.g., documentation update)
- **Standard** – Peer approval required (e.g., code review, deployment to staging)
- **Elevated** – Manager approval required (e.g., production deployment)
- **Critical** – Director approval required (e.g., data deletion, security changes)

#### TrustLevel
**Purpose:** Define minimum trust required for actions.

**Levels:**
- **Untrusted** – Sandbox/evaluation only, no external writes
- **Standard** – Trusted for bounded tasks within defined scope
- **Elevated** – Trusted for broader tasks but with policy gates
- **Privileged** – Full trust for emergency response or specialized tasks

### Example Flow: Ops Deployment with Control Plane

1. **Task arrives:** Jira ticket for production deployment
2. **Role resolved:** Ops role assigned via `ZenRoleProfile`
3. **Policies applied:** `ZenExecutionPolicy` for "production-gated" enforced
4. **Workspace created:** `WorkspaceClass=Isolated` for task
5. **Tool access checked:** `ZenToolBinding` for "git-write" validated against Ops role
6. **Execution allowed:** If all constraints satisfied, agent proceeds
7. **Handoff considered:** If incident escalates, `ZenHandoffPolicy` for "ops-to-escalate" applies
8. **Compliance checked:** If `ZenComplianceProfile` for "fedramp" active, audit rules enforced
9. **Proof generated:** `Proof-of-Work` bundle includes all policy checks, tool calls, and approvals

## Consequences

### Positive
- **First-class control objects** – No more "brainstorm terms", everything is defined and enforced
- **Auditable governance** – All actions, roles, policies are logged and reviewable
- **RBAC ready** – Namespace and role-aware design supports multi-team deployments
- **Compliance by design** – `ZenComplianceProfile` allows overlaying security requirements
- **Clear authority boundaries** – Roles, tools, policies define exactly what agents can do

### Negative
- **Increased complexity** – More concepts to define, configure, and maintain
- **Learning curve** – New vocabulary for engineers and operators
- **Implementation overhead** – CRDs, controllers, and policy engines required

### Neutral
- **Evolutionary** – Can start simple and add more complex policies over time
- **Extensible** – New concepts (e.g., `ZenAuditProfile`) can be added later
- **Declarative** – YAML/CRD definitions are easier to audit and review than code

## Alternatives Considered

### Alternative 1: Everything is code, no control-plane objects
- **Pros:** Simpler initially, just implement logic in Go
- **Cons:** Not auditable, not configurable, requires code changes for governance
- **Rejected:** Trusted operator needs declarative, auditable control plane

### Alternative 2: Use existing Kubernetes RBAC only
- **Pros:** Standard, well-understood
- **Cons:** Too coarse-grained, no Zen-Brain specific concepts
- **Rejected:** Need Zen-Brain-specific vocabulary for roles, tools, policies

### Alternative 3: One monolithic policy file
- **Pros:** Simple configuration
- **Cons:** Hard to maintain, hard to test, no versioning
- **Rejected:** Modular policies (`ZenExecutionPolicy`, `ZenComplianceProfile`) are more maintainable

## Related Decisions

- [ADR-0001](../01-ARCHITECTURE/ADR/0001_STRUCTURED_TAGS.md) – Structured tags define task types for policies
- [ADR-0004](../01-ARCHITECTURE/ADR/0004_MULTI_CLUSTER_CRDS.md) – Multi-cluster CRDs provide control-plane foundation
- [Bounded Orchestrator Loop](../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) – Orchestrator enforces control-plane policies
- [Small-Model Strategy](../03-DESIGN/SMALL_MODEL_STRATEGY.md) – Model preference in `ZenRoleProfile`
- [Ops Department](../03-DESIGN/OPS_DEPARTMENT.md) – Ops role and policies use control-plane vocabulary

## Implementation Roadmap

### Phase 1: Contract Definitions (1.1)
- [ ] Define Go structs for `ZenRoleProfile`, `ZenExecutionPolicy`, `ZenHandoffPolicy`, `ZenTool`, `ZenToolBinding`, `ZenComplianceProfile`
- [ ] Add to `pkg/contracts`
- [ ] Write JSON/YAML schemas for validation

### Phase 2: CRD Definitions (1.1-2.0)
- [ ] Define Kubernetes CRDs for control-plane objects
- [ ] Add to `api/v1alpha1/`
- [ ] Generate client code (controller-runtime, client-go)

### Phase 3: Policy Engine (2.0)
- [ ] Implement policy evaluation logic in orchestrator
- [ ] Implement role-based tool access control
- [ ] Implement workspace class enforcement
- [ ] Implement compliance profile checking

### Phase 4: Controllers (2.0+)
- [ ] RoleProfile controller for reconciliation
- [ ] ToolBinding controller for RBAC integration
- [ ] ComplianceProfile controller for audit enforcement
- [ ] Watch and react to changes in control plane

## Future Work

### Advanced Concepts
- **ZenAuditProfile** – Detailed audit configuration for specific compliance frameworks
- **ZenRateLimitProfile** – Per-role or per-namespace rate limiting
- **ZenBudgetProfile** – Token/cost budget per team or project
- **ZenSLAProfile** – Service level agreement monitoring and enforcement

### Policy Examples
- **Data residency** – Enforce data never leaves specific region
- **PII protection** – Special handling for personally identifiable information
- **Change windows** – Only allow changes during approved maintenance windows
- **Rollback policies** – Automatic rollback criteria and procedures

## References

- Kubernetes RBAC: [https://kubernetes.io/docs/reference/access-authn-rbac/](https://kubernetes.io/docs/reference/access-authn-rbac/)
- Open Policy Agent (OPA): [https://www.openpolicyagent.org/](https://www.openpolicyagent.org/) (policy engine reference)
- Zen-Brain ROADMAP: [ROADMAP.md](ROADMAP.md)
- Bounded Orchestrator: [BOUNDED_ORCHESTRATOR_LOOP.md](../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md)