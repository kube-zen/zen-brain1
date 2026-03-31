# Domain Isolation, Tool RBAC, and Hard Enforcement

## Status

**Proposal** (2026-03-31)

## Problem Statement

zen-brain1 must support **multiple autonomous domains** (zen-platform, zen-lock, finance, sales, etc.) where each domain's AI agents operate **only within their own scope**: their Jira project, their repos, their files, their tools. Today, tool access is uniform — every agent sees every tool definition — which wastes context tokens on small models, creates unnecessary attack surface, and makes cross-domain leakage possible.

Additionally, certain rules (image registry allowlists, repo structure constraints, deployment gates) must be **machine-enforced, not advisory**. An AI must never be the entity deciding whether a policy applies.

This document proposes a design that leverages **Kubernetes namespaces, RBAC, NetworkPolicy, admission control, and CRD-backed tool catalogs** to achieve:

1. **Domain isolation** — Jira spaces map to K8s namespaces; agents are confined.
2. **Role-based tool projection** — Only relevant tools are serialized into the LLM prompt.
3. **Hard enforcement** — Compliance rules are enforced by infrastructure, not by models.

## Design Principles

1. **Fail closed** — If policy cannot be evaluated, the action is denied.
2. **Infrastructure enforces, AI obeys** — Rules are never "suggestions" to the model.
3. **Least privilege by default** — Agents receive the minimum tool set for their role and domain.
4. **Defense in depth** — Enforcement at prompt projection, executor, admission control, and network layers.
5. **Auditable** — Every enforcement decision is logged with subject, action, resource, and outcome.
6. **GitOps-native** — All policy is declarative YAML, version-controlled, reconciled by controllers.

---

## 1. Domain Isolation: Jira Spaces as K8s Namespaces

### Concept

Each **business domain** (zen-platform, zen-lock, finance, sales) maps to a dedicated **Kubernetes namespace**. BrainTasks, BrainAgents, tool bindings, policies, and secrets all live in the domain's namespace. Cross-namespace access is denied by default.

### Namespace Layout

```
zen-brain-platform/     # zen-platform engineering domain
zen-brain-lock/         # zen-lock engineering domain
zen-brain-finance/      # finance/accounting domain
zen-brain-sales/        # sales/CRM domain
zen-brain-system/       # control-plane services (foreman, apiserver, ingestion)
```

### ZenDomain CRD

A new **cluster-scoped** CRD that declares a domain and its mapping:

```yaml
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenDomain
metadata:
  name: zen-platform
spec:
  displayName: "Zen Platform Engineering"
  namespace: zen-brain-platform
  jira:
    projectKeys: ["ZP", "PLAT"]
    baseUrl: "https://kube-zen.atlassian.net"
  repos:
    allowed:
      - "github.com/kube-zen/zen-platform"
      - "github.com/kube-zen/zen-platform-infra"
    forbidden:
      - "github.com/kube-zen/zen-lock"       # explicit cross-domain deny
  defaultRoleProfile: worker
  costBudgetUSD: 500.0
  complianceProfiles: ["internal-engineering"]
status:
  phase: Active
  taskCount: 42
  costSpentUSD: 123.45
```

### Ingestion Flow

```
Jira webhook → Ingestion Controller (zen-brain-system)
  1. Extract project key from ticket (e.g. "ZP-1234")
  2. Resolve ZenDomain by projectKey → "zen-platform"
  3. Create BrainTask in namespace "zen-brain-platform"
  4. BrainTask.metadata.namespace = "zen-brain-platform"
  5. Foreman in zen-brain-system watches ALL domain namespaces
     but dispatches work using ServiceAccount scoped to target namespace
```

### Why Namespaces (not just ZenProject labels)

| Mechanism | What it gives |
|-----------|---------------|
| Namespace | K8s RBAC boundary, NetworkPolicy boundary, ResourceQuota, LimitRange |
| Labels alone | No enforcement — any controller can read/write across labels |
| ZenProject (ADR-0004) | Custom metadata (SRED, budgets) — **kept**, but **lives inside** the domain namespace |

ZenProject continues to exist for metadata (SRED tags, funding, cluster routing). ZenDomain is the **isolation boundary**; ZenProject is the **project descriptor** within it.

---

## 2. Role-Based Tool Projection

### Problem

A finance-domain summarizer agent does not need `git-write`, `shell-exec`, or `k8s-deploy` tools. Sending all tool definitions:
- Wastes **thousands of tokens** (critical for 0.8B models with ~26k effective context).
- Increases **attack surface** (model can attempt tool calls it should not make).
- Reduces **quality** (small models perform worse with large tool arrays).

### Solution: ZenTool + ZenToolBinding CRDs

#### ZenTool (cluster-scoped) — The Tool Catalog

Each tool the system can offer is registered as a cluster-scoped resource:

```yaml
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenTool
metadata:
  name: git-write
spec:
  displayName: "Git Write"
  description: "Create, modify, or delete files in a Git repository"
  category: git
  riskLevel: medium          # none | low | medium | high | critical
  sideEffects: [modifies]    # none | creates | modifies | deletes
  inputSchema:               # OpenAI-compatible function schema
    type: object
    properties:
      repo: { type: string }
      branch: { type: string }
      files: { type: array, items: { type: object } }
    required: [repo, branch, files]
  outputSchema:
    type: object
  tokenCost: 280             # approximate token count when serialized
```

#### ZenToolBinding (namespaced) — Who Gets What

Tool bindings live **in the domain namespace** and declare which roles may use which tools, with constraints:

```yaml
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenToolBinding
metadata:
  name: worker-git-tools
  namespace: zen-brain-platform
spec:
  roles: [worker, implementer]
  tools:
    - toolRef: git-read
    - toolRef: git-write
      constraints:
        allowedRepos: ["github.com/kube-zen/zen-platform"]
        maxFilesPerCall: 20
    - toolRef: shell-exec
      constraints:
        allowedCommands: ["go build", "go test", "make *"]
        forbiddenCommands: ["rm -rf", "kubectl delete"]
  approvalRequired: false
  auditLog: true
---
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenToolBinding
metadata:
  name: summarizer-tools
  namespace: zen-brain-finance
spec:
  roles: [summarizer, planner]
  tools:
    - toolRef: jira-read
    - toolRef: kb-search
    - toolRef: document-write
  # No git, no shell, no k8s tools
  approvalRequired: false
  auditLog: true
```

### Prompt Assembly (Tool Projection)

When Foreman assembles a prompt for a BrainTask:

```
1. task.namespace → "zen-brain-platform"
2. task.roleProfile → "worker"
3. List ZenToolBindings in namespace where roles contains "worker"
4. Resolve each toolRef → ZenTool spec
5. Serialize ONLY those tools into the OpenAI tools array
6. Attach constraints as system prompt context
```

**Token savings example:**

| Scenario | Tools serialized | Approx. tokens |
|----------|-----------------|----------------|
| All tools (current) | 15 tools | ~4,200 |
| Worker (zen-platform) | 5 tools | ~1,400 |
| Summarizer (finance) | 3 tools | ~840 |
| Reviewer (zen-lock) | 4 tools | ~1,120 |

For a 0.8B model with ~39k per-slot budget, saving 3,000+ tokens is significant headroom.

### Four-Layer Enforcement

Tool projection is **layer 1** (prompt). But the model might hallucinate tool calls not in its binding. Defense in depth:

| Layer | Mechanism | Enforces |
|-------|-----------|----------|
| **L1 — Prompt** | Only bound tools serialized | Model cannot see unbound tools |
| **L2 — Executor** | Tool dispatcher checks ZenToolBinding before execution | Hallucinated calls rejected |
| **L3 — RBAC** | ServiceAccount per domain; K8s RBAC on secrets, configmaps | No cross-namespace resource access |
| **L4 — Network** | NetworkPolicy restricts pod-to-pod and egress | Cannot reach endpoints outside domain |

---

## 3. Kubernetes RBAC Enforcement

### ServiceAccount Per Domain

Each domain namespace has a dedicated ServiceAccount used by workers:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: brain-worker
  namespace: zen-brain-platform
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: brain-worker
  namespace: zen-brain-platform
rules:
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["braintasks"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["braintasks/status"]
    verbs: ["update", "patch"]
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["zentoolbindings"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get"]
    resourceNames: ["domain-credentials"]   # only this secret
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: brain-worker
  namespace: zen-brain-platform
subjects:
  - kind: ServiceAccount
    name: brain-worker
    namespace: zen-brain-platform
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: brain-worker
```

### What RBAC Prevents

- Worker in `zen-brain-platform` **cannot** read BrainTasks in `zen-brain-lock`.
- Worker **cannot** read secrets from other namespaces.
- Worker **cannot** create or delete CRDs (only update task status).
- Foreman (in `zen-brain-system`) uses a **ClusterRole** to watch tasks across namespaces but creates child resources only in the target namespace.

### Foreman Cross-Namespace Access

Foreman needs to watch all domain namespaces but act within them:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: foreman
rules:
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["braintasks", "brainqueues", "zendomains", "zentools"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["braintasks/status", "brainqueues/status"]
    verbs: ["update", "patch"]
  - apiGroups: ["zen.kube-zen.com"]
    resources: ["zentoolbindings"]
    verbs: ["get", "list"]
```

Per-namespace RoleBindings grant Foreman's SA access to each domain it manages.

---

## 4. NetworkPolicy Enforcement

### Default Deny

Every domain namespace starts with a **default-deny** policy:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: zen-brain-platform
spec:
  podSelector: {}
  policyTypes: [Ingress, Egress]
```

### Allow Only What Is Needed

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-llm-and-system
  namespace: zen-brain-platform
spec:
  podSelector:
    matchLabels:
      app: brain-worker
  policyTypes: [Egress]
  egress:
    # DNS resolution
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
    # L1/L2 llama.cpp on host
    - to:
        - ipBlock:
            cidr: 10.0.0.0/8    # host.k3d.internal range
      ports:
        - protocol: TCP
          port: 56227
        - protocol: TCP
          port: 60509
    # Foreman API in zen-brain-system
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: zen-brain-system
          podSelector:
            matchLabels:
              app: foreman
      ports:
        - protocol: TCP
          port: 8080
    # Jira API (external)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - protocol: TCP
          port: 443
```

### What NetworkPolicy Prevents

| Threat | Blocked by |
|--------|-----------|
| Worker in zen-brain-platform reaches zen-brain-lock pods | Default deny + no cross-namespace egress rule |
| Worker reaches unauthorized external APIs | Egress allowlist (only Jira, LLM endpoints) |
| Worker reaches CockroachDB directly | No egress rule to DB ports |
| External traffic reaches worker pods | Default deny ingress |

### Domain-Specific Egress (Example)

Finance domain might need access to an accounting API but not to Git:

```yaml
# zen-brain-finance namespace
egress:
  - to:
      - ipBlock:
          cidr: 10.0.0.0/8
    ports:
      - protocol: TCP
        port: 56227       # LLM only
  - to:
      - ipBlock:
          cidr: 0.0.0.0/0
    ports:
      - protocol: TCP
        port: 443          # Jira + accounting SaaS
  # No port 22 (git SSH), no port 3000 (Gitea), etc.
```

---

## 5. Hard Enforcement: Gates, Guardrails, and CI

### Design Principle

**Rules are never advisory to the AI.** They are enforced at the infrastructure layer. The AI does not know the rule exists — it simply cannot perform the forbidden action.

### Enforcement Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    ENFORCEMENT LAYERS                     │
├─────────────┬───────────────┬───────────────┬───────────┤
│  L1: Prompt │  L2: Executor │  L3: Admission│  L4: Infra│
│  Projection │  Validation   │  Control      │  NetPol   │
├─────────────┼───────────────┼───────────────┼───────────┤
│ Only bound  │ Tool call     │ K8s admission │ Network   │
│ tools in    │ checked vs    │ webhook       │ isolation │
│ context     │ binding +     │ validates     │ prevents  │
│             │ constraints   │ resource      │ traffic   │
│             │               │ mutations     │           │
├─────────────┼───────────────┼───────────────┼───────────┤
│ Prevents:   │ Prevents:     │ Prevents:     │ Prevents: │
│ Seeing      │ Executing     │ Creating      │ Reaching  │
│ unbound     │ hallucinated  │ non-compliant │ endpoints │
│ tools       │ or OOB calls  │ resources     │ at all    │
└─────────────┴───────────────┴───────────────┴───────────┘
```

### ZenComplianceRule CRD (namespaced)

Hard rules that are **not** configurable per-agent — they apply to everything in the namespace:

```yaml
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenComplianceRule
metadata:
  name: no-dockerhub-images
  namespace: zen-brain-platform
spec:
  description: "Container images must not reference Docker Hub"
  scope: namespace           # namespace | cluster
  enforcement: block         # block | audit | warn
  rule:
    type: image-registry
    match:
      field: "spec.containers[*].image"
      operator: not_matches
      pattern: "^(docker\\.io/|library/|[^./]+$)"
    message: "Docker Hub images are forbidden. Use zen-registry:5000/ or approved registries."
---
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenComplianceRule
metadata:
  name: no-github-folder-private-repos
  namespace: zen-brain-platform
spec:
  description: "Private repositories must not contain .github/ directory"
  scope: namespace
  enforcement: block
  rule:
    type: repo-structure
    match:
      field: "files"
      operator: path_not_exists
      pattern: ".github/**"
      condition: "repo.visibility == private"
    message: ".github/ folder is forbidden in private repos. Use internal CI."
---
apiVersion: zen.kube-zen.com/v1alpha1
kind: ZenComplianceRule
metadata:
  name: require-go-test-before-merge
  namespace: zen-brain-platform
spec:
  description: "All Go changes must pass go test before merge"
  scope: namespace
  enforcement: block
  rule:
    type: ci-gate
    match:
      field: "change.files"
      operator: any_matches
      pattern: "*.go"
    requiredChecks:
      - name: "go-test"
        command: "go test ./..."
        mustPass: true
      - name: "go-vet"
        command: "go vet ./..."
        mustPass: true
    message: "Go test and vet must pass before merge."
```

### Where Each Rule Type Is Enforced

| Rule Type | Enforcement Point | Mechanism |
|-----------|-------------------|-----------|
| `image-registry` | K8s ValidatingAdmissionWebhook | Webhook inspects pod/deployment specs before creation |
| `repo-structure` | Factory preflight (before LLM call) + git push hook | Factory checks worktree; git server rejects push |
| `ci-gate` | Factory post-execution (after code generation) | Factory runs checks before committing; blocks on failure |
| `cost-limit` | ZenGate admission (before task dispatch) | Gate checks ZenDomain budget before allowing task |
| `model-allowlist` | ZenGate admission + PolicyGate | Only certified models accepted for domain |
| `tool-constraint` | Executor layer (L2) | Executor validates tool call params vs. binding constraints |

### ValidatingAdmissionWebhook for Image Registry

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: zen-compliance-webhook
webhooks:
  - name: compliance.zen.kube-zen.com
    admissionReviewVersions: ["v1"]
    sideEffects: None
    failurePolicy: Fail     # FAIL CLOSED
    namespaceSelector:
      matchLabels:
        zen.kube-zen.com/managed: "true"
    rules:
      - apiGroups: ["", "apps"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["pods", "deployments", "statefulsets", "daemonsets", "jobs"]
    clientConfig:
      service:
        name: zen-compliance-webhook
        namespace: zen-brain-system
        path: /validate
```

The webhook reads `ZenComplianceRule` CRs from the target namespace and evaluates them. If any `enforcement: block` rule fails, the admission is **rejected** with a clear message. The AI never gets a chance to override.

### Factory-Level Enforcement

For rules that apply to AI-generated artifacts (code, configs):

```go
// In Foreman / Factory, after LLM generates code but before commit:

func (f *Factory) EnforceComplianceRules(ctx context.Context, task *BrainTask, artifacts []Artifact) error {
    rules, err := f.listComplianceRules(ctx, task.Namespace)
    if err != nil {
        return fmt.Errorf("fail-closed: cannot load compliance rules: %w", err)
    }

    for _, rule := range rules {
        if rule.Spec.Enforcement != "block" {
            continue // audit/warn rules are logged but don't block
        }
        if err := f.evaluateRule(ctx, rule, artifacts); err != nil {
            // Hard block. Task fails. AI does not decide.
            return &ComplianceViolation{
                Rule:    rule.Name,
                Message: rule.Spec.Rule.Message,
                Err:     err,
            }
        }
    }
    return nil
}
```

### CI Pipeline Enforcement

For rules that must run in CI (not just in-cluster):

```yaml
# ZenComplianceRule with type: ci-gate generates
# a CI job definition that runs as part of the merge pipeline.
# The CI system (Drone, GitHub Actions, Tekton) is configured
# to block merge if the job fails.
#
# zen-brain1 does NOT rely on the AI to run CI.
# CI runs independently, triggered by git push.
```

---

## 6. Putting It All Together: Request Lifecycle

```
1. Jira ticket ZP-1234 created in project "ZP"

2. Ingestion Controller:
   - Resolves ZenDomain: projectKey "ZP" → domain "zen-platform"
   - Creates BrainTask in namespace "zen-brain-platform"

3. Foreman picks up task:
   - Reads ZenDomain → gets repos, compliance profiles
   - Reads ZenRoleProfile → "worker"
   - Lists ZenToolBindings in "zen-brain-platform" where roles contains "worker"
   - Resolves ZenTool specs → [git-read, git-write, shell-exec, jira-read, jira-write]
   - Serializes ONLY these 5 tools (not 15) into prompt

4. Factory executes:
   - Worker pod runs with SA "brain-worker" in "zen-brain-platform"
   - NetworkPolicy allows: LLM endpoints, Jira API, foreman
   - NetworkPolicy denies: zen-brain-lock pods, CockroachDB, internet (except allowed)

5. LLM generates code:
   - Model calls git-write with repo="github.com/kube-zen/zen-platform"
   - L2 Executor checks: ZenToolBinding allows git-write for worker in this namespace? YES
   - L2 Executor checks: repo in allowedRepos? YES
   - L2 Executor checks: file count < maxFilesPerCall? YES
   - Execution proceeds

6. LLM tries to call shell-exec with "kubectl delete ns zen-brain-lock":
   - L2 Executor checks: command in forbiddenCommands? YES → BLOCKED
   - Even if L2 missed it: RBAC denies SA from deleting namespaces
   - Even if RBAC missed it: NetworkPolicy blocks traffic to API server on that path

7. Factory post-execution:
   - EnforceComplianceRules(): no Docker Hub images? ✓ no .github/ folder? ✓
   - CI gate: go test passes? ✓ go vet passes? ✓
   - Commit and push

8. Admission webhook on push:
   - ValidatingWebhook checks: image registries compliant? ✓
   - Deployment admitted

9. Audit:
   - All tool calls logged to ZenJournal with namespace, role, tool, outcome
   - Compliance rule evaluations logged with pass/fail
   - Available for SOC2/audit review
```

---

## 7. Migration Path

### Phase 1: Namespace Separation (Week 1-2)

- [ ] Create domain namespaces (`zen-brain-platform`, `zen-brain-lock`, etc.)
- [ ] Deploy default-deny NetworkPolicies in each
- [ ] Create per-namespace ServiceAccounts and RBAC Roles
- [ ] Update ingestion controller to route BrainTasks by Jira project key
- [ ] Foreman watches all domain namespaces

### Phase 2: ZenDomain + ZenTool CRDs (Week 3-4)

- [ ] Define and register ZenDomain CRD (cluster-scoped)
- [ ] Define and register ZenTool CRD (cluster-scoped)
- [ ] Define and register ZenToolBinding CRD (namespaced)
- [ ] Populate initial tool catalog from existing tool definitions
- [ ] Create initial bindings for each domain/role combination

### Phase 3: Tool Projection in Foreman (Week 5-6)

- [ ] Foreman resolves ZenToolBindings before prompt assembly
- [ ] Only bound tools serialized into LLM tools array
- [ ] Executor validates every tool call against binding + constraints
- [ ] Metrics: `zen_tool_calls_total{namespace, role, tool, outcome}`

### Phase 4: Hard Compliance Rules (Week 7-8)

- [ ] Define and register ZenComplianceRule CRD (namespaced)
- [ ] Implement ValidatingAdmissionWebhook for `image-registry` rules
- [ ] Implement Factory-level enforcement for `repo-structure` and `ci-gate` rules
- [ ] Deploy webhook with `failurePolicy: Fail`
- [ ] Create initial compliance rules per domain

### Phase 5: NetworkPolicy Hardening (Week 8-9)

- [ ] Audit current pod-to-pod traffic patterns
- [ ] Deploy domain-specific egress rules (LLM, Jira, approved APIs)
- [ ] Test: worker in domain A cannot reach domain B
- [ ] Test: worker cannot reach CockroachDB directly
- [ ] Monitor with Cilium/Calico flow logs

### Phase 6: Audit and Observability (Week 10)

- [ ] All enforcement decisions logged to ZenJournal
- [ ] Grafana dashboard: denials by namespace, rule, tool
- [ ] Alert on: compliance violation rate > 0 for `enforcement: block` rules
- [ ] Quarterly compliance report generation

---

## 8. Security Model Summary

| Threat | Mitigation | Layer |
|--------|-----------|-------|
| AI reads files from another domain's repo | Tool binding restricts `allowedRepos`; RBAC denies cross-namespace secrets | L2 + L3 |
| AI calls tool not in its role | Tool not in prompt (L1); executor rejects (L2) | L1 + L2 |
| AI deploys Docker Hub image | Admission webhook blocks (L3); compliance rule `enforcement: block` | L3 |
| AI creates .github/ in private repo | Factory preflight blocks; git push hook rejects | L2 + CI |
| AI accesses another domain's pods | NetworkPolicy default-deny; no egress rule | L4 |
| AI bypasses CI checks | CI runs independently of AI; merge blocked on failure | CI |
| AI escalates its own trust level | Trust level is immutable after workspace creation; set by controller | L3 |
| AI modifies compliance rules | RBAC: worker SA has no write access to ZenComplianceRule | L3 |
| Policy engine crashes | `failurePolicy: Fail` on webhook; `fail-closed` in gate | L3 |

---

## 9. Relationship to Existing Design

| Existing Concept | Relationship |
|-----------------|-------------|
| **ZenProject** (ADR-0004) | Lives **inside** a ZenDomain namespace; provides metadata (SRED, budgets, cluster routing) |
| **ZenRoleProfile** (Control Plane Vocabulary) | Referenced by ZenToolBinding; determines which tools a role gets |
| **ZenGate / ZenPolicy** (ZEN_GATE_POLICY.md) | Evaluates admission requests; compliance rules feed into gate |
| **WorkspaceClass** (WORKSPACE_CLASSES.md) | Per-task isolation within a domain; complements namespace-level isolation |
| **PolicyGate** (internal/gate/) | Existing runtime gate; extended to check ZenToolBinding + ZenComplianceRule |
| **BrainPolicy** CRD | Existing cost/model policy; ZenComplianceRule is the broader enforcement equivalent |
| **Factory preflight** | Extended to run compliance rule evaluation before and after LLM execution |

---

## Open Questions

1. **Should ZenDomain be cluster-scoped or namespaced?** — Proposed cluster-scoped since it creates/owns namespaces.
2. **How to handle cross-domain tasks?** — Explicit `ZenHandoffPolicy` with approval required; task created in target namespace.
3. **Should compliance rules support OPA/Rego?** — Start with built-in operators; add OPA integration in Phase 5+.
4. **How to handle emergency break-glass?** — Privileged SA with time-bound token; audit-logged; requires human approval.

---

## References

- [Control Plane Vocabulary](../01-ARCHITECTURE/CONTROL_PLANE_VOCABULARY.md) — ZenTool, ZenToolBinding, ZenRoleProfile
- [ADR-0004: Multi-Cluster CRDs](../01-ARCHITECTURE/ADR/0004_MULTI_CLUSTER_CRDS.md) — ZenProject, ZenCluster
- [ZenGate & ZenPolicy Design](ZEN_GATE_POLICY.md) — Admission control and policy engine
- [Workspace Classes](WORKSPACE_CLASSES.md) — Per-task isolation
- [Small Model Strategy](SMALL_MODEL_STRATEGY.md) — Context budget motivation for tool projection

---

*This document is a proposal. Review, challenge, and refine before implementation.*
