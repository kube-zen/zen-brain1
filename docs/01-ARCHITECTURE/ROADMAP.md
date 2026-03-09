# Zen-Brain 1.0 Roadmap

This roadmap reflects the current architectural direction and prioritized work items.

## 0. Now / Critical Corrections

- [x] Repo hardening – numbered docs taxonomy, Python-only scripts, repo gates
- [x] Docs link coherence – fixed all broken internal markdown links
- [x] Move ROADMAP.md to docs/01-ARCHITECTURE/
- [x] AGENTS.md and WORKFLOW.md explicitly marked as advisory-only
- [x] Skills/subagents documented under bounded execution model
- [x] Small-model strategy design doc (SMALL_MODEL_STRATEGY.md)
- [x] Ops Department design doc (OPS_DEPARTMENT.md)
- [x] Agent sandbox evaluation design doc (SANDBOX_AND_EVALUATION.md)
- [x] Repo policy for model-facing files (MODEL_FACING_FILES_AND_SKILLS_POLICY.md)

### Current Focus: First Trustworthy Vertical Slice

**Goal:** Implement the end-to-end path from Jira intake to proof-of-work generation

The highest-priority work is to complete one trustworthy vertical slice:
1. Jira intake → analyze → plan → workspace execution → proof-of-work → status update
2. Thin but real Factory implementation with bounded execution
3. First real LLM/provider lane (local workers + planner)
4. First real QMD adapter for knowledge retrieval
5. Integration of all components into a working system

**Principle:** Build one trustworthy internal vertical slice first. Breadth comes later.

---

## 1.0 Must-Have

### Reliability & Control

#### Worktree + Protected Workspace Model
- **Goal:** Isolated per-task/per-session workspaces with strong safety boundaries
- **Components:**
  - Workspace classes / trust levels
  - Protected repos/paths
  - Delete protections
  - Tmpfs as optional acceleration (guarded by memory checks)
- **Status:** Design documented, implementation pending

#### Bounded Orchestrator Loop
- **Goal:** Prevent uncontrolled recursive spawning, enforce retry/backoff discipline
- **Components:**
  - Poll/reconcile loop
  - Explicit stop conditions
  - Watchdog pattern
  - No uncontrolled recursive spawning
- **Design:** docs/03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md
- **Status:** Design documented, implementation pending

#### Proof-of-Work Bundle
- **Goal:** First-class output for AI work evidence
- **Components:**
  - Summary
  - Files changed
  - Tests run
  - Command log
  - Evidence links
  - Unresolved risks
  - Recommendation (merge/review/retry/escalate)
- **Design:** docs/03-DESIGN/PROOF_OF_WORK.md
- **Status:** Design documented, implementation pending

### CPU-First Local Model Optimization Lane
- **Goal:** Maximize useful throughput and reliability for small local models (Qwen 0.8B class) on CPU-only hosts
- **Components:**
  - Warmup strategy
  - Calibration/evaluation harness
  - Provider/model capability registry
  - Prompt/profile tuning per role
  - Small-model routing rules
  - Fallback/escalation path to larger models or paid APIs
  - Token/yield tracking in ZenLedger
  - Benchmark suite for planner/implementer/ops roles
- **Design:** docs/03-DESIGN/SMALL_MODEL_STRATEGY.md (complete)
- **Status:** Design complete, implementation pending
- **Notes:**
  - Provider-agnostic approach
  - Qwen 0.8B as important baseline, not hard dependency
  - Tiny local models are extremely cheap and parallelizable
  - Yield per token matters more than raw token price
  - Local small models best used in bounded roles with strong context shaping
  - Benchmark by task class, not just generic chat quality

### Office / Jira Foundation

#### Ops Department Foundation
- **Goal:** Reduce toil around incidents, changes, deploys, and launch operations
- **Model:** Jira-centric operation (single Jira space/project for initial narrow lane)
- **Workflows:**
  - Incident/problem flows
  - Change management
  - Deployment approvals
- **Components:**
  - Runbooks / KB linkage
  - Approval/gate hooks for deploy/change work
  - Safe automation for 1.0 (risky actions require approval)
- **Design:** docs/03-DESIGN/OPS_DEPARTMENT.md (complete)
- **Status:** Design complete, implementation pending

#### Jira Connector Hardening
- **Goal:** Stable human front door with AI attribution
- **Components:**
  - Canonical WorkItem mapping
  - AI attribution on all AI-generated comments
  - Webhook listener for issue events
  - Bidirectional sync with Zen-Brain's view
- **Design:** docs/03-DESIGN/BLOCK2_OFFICE.md
- **Status:** Design documented, implementation in progress

### Knowledge & Retrieval

- **Goal:** Git + qmd as simple, reliable KB layer
- **Model:**
  - `zen-docs` git repo as source of truth
  - qmd for search/index only (no Cockroach-backed KB)
  - Confluence optional/deferred as one-way published mirror
- **Status:** Architecture decision documented (ADR-0007)

### Dogfooding Safety

- **Goal:** 1.0 as trusted internal force multiplier, not unrestricted self-modifier
- **Policies:**
  - Sensitive actions require stronger approvals/policies
  - No unrestricted authority over its own trusted core
  - May help Zen-Mesh and Zen-Brain 1.1 build, but under bounded trust levels
- **Status:** Governance model under development

---

## 1.0 Nice-To-Have (Low Risk Only)

- Enhanced runbooks auto-generation
- Metric dashboards for orchestrator/worker health
- Local k3d development experience polish
- Documentation auto-generation from CRDs
- Basic alerting integration (read-only)

---

## 1.1 Radar

### Agent Sandbox (Non-Destructive Evaluation Lane)
- **Goal:** Testbed for evaluating behavior/effectiveness without allowing real external writes
- **Capabilities:**
  - Plan, explore, and simulate changes
  - Cannot make real external changes or durable code changes without promotion
  - Capture behavior/effectiveness metrics
  - Compare: plan quality, policy adherence, proof quality, escalation behavior
  - Use as testbed for new roles, prompts, tools, and model choices
- **Status:** Radar item – design doc pending

### ReMe-Enhanced Memory Optimization
- **Goal:** Improve memory injection and recovery using actual execution history
- **Notes:** Do not overbuild before 1.0 reliability exists
- **Status:** Radar item

### Fine-Tuning / Continual Training / Distillation Investigation
- **Goal:** Research lane for lightweight model adaptation
- **Approach:**
  - Not mandatory for 1.0
  - Lightweight adapters
  - Synthetic examples
  - Role-specific datasets
  - Clear ROI gates before production use
- **Status:** Radar item

### Compliance Overlays (Beyond SR&ED/IRAP)
- **Goal:** Enterprise-level compliance posture for future production use
- **Profiles:**
  - SOC 2
  - ISO 27001 / 27k family
  - FedRAMP-oriented future profile
- **Implications:**
  - Deployment/ops/supply-chain implications captured early
- **Status:** Radar item

### Cross-Area Handoff Engine
- **Goal:** Controlled agent-to-agent triggers for multi-domain workflows
- **Components:**
  - Recommendation-only vs allowed-create routes
  - Policy-controlled cross-domain transitions
  - Namespace-scoped department/project boundaries
- **Status:** Radar item

### Multi-Level Queue (MLQ) and Escalation
- **Goal:** Intelligent task routing and escalation based on complexity, urgency, and model capability
- **Components:**
  - Multi-level task queues (L1: simple, L2: moderate, L3: complex, L4: critical)
  - Automatic escalation rules when tasks timeout or fail
  - Worker pool assignment per queue level
  - Model selection based on task class and queue level
  - Escalation tracking and metrics
- **Approach:**
  - Start with 4-level queue system from zen-brain (zen-brain1 can adopt or adapt)
  - Integrate with small-model strategy for L1/L2 worker allocation
  - Reserve stronger models (cloud APIs) for L3/L4 critical tasks
  - Build escalation metrics and dashboards
- **Status:** Radar item – design pending, zen-brain has working implementation

### Future Control-Plane Vocabulary
These concepts should be elevated to first-class architecture before implementation:

- **ZenRoleProfile** – Role definition with policy bindings
- **ZenExecutionPolicy** – Admission control for actions/operations
- **ZenHandoffPolicy** – Rules for cross-domain agent transitions
- **ZenTool** – Tool definition (independent of runtime adapter)
- **ZenToolBinding** – Namespace/RBAC-aware tool access configuration
- **ZenComplianceProfile** – Compliance overlay configuration
- **WorkspaceClass** / **ChangeClass** / **TrustLevel** – Workspace protection model

**Status:** Conceptual; needs design docs before implementation

---

## Explicitly Deferred

- Graph/knowledge-relationship systems (beyond simple qmd index)
- Broad company-wide autonomous role expansion
- Heavy Confluence sync investment (one-way mirror optional)
- Broad CRD/controller implementation for all future concepts
- Full-time production operations support (Ops Department foundation first)

---

## Acceptance Criteria

1.0 is considered feature-complete when:

- [ ] Worktree isolation with workspace classes implemented and gated
- [ ] Bounded orchestrator loop with explicit stop conditions
- [ ] Proof-of-work bundles generated for all work sessions
- [ ] CPU-first small-model lane calibrated and benchmarked
- [ ] Ops Department foundation deployed (at least one Jira space)
- [ ] Jira connector stable with AI attribution
- [ ] Git + qmd KB layer functional
- [ ] Sensitive actions require stronger approval/policy gates
- [ ] No uncontrolled recursive agent spawning possible

---

## Design Status Reference

| Design Doc | Status | Notes |
|-----------|--------|-------|
| [CONSTRUCTION_PLAN.md](CONSTRUCTION_PLAN.md) | Complete | Master build roadmap (V6.1) |
| [DATA_MODEL.md](../02-CONTRACTS/DATA_MODEL.md) | Complete | Canonical types and structured tags |
| [CONTROL_PLANE_VOCABULARY.md](CONTROL_PLANE_VOCABULARY.md) | Complete | First-class control-plane objects |
| [BLOCK2_OFFICE.md](../03-DESIGN/BLOCK2_OFFICE.md) | Complete | Jira connector design |
| [ZEN_CONTEXT.md](../03-DESIGN/ZEN_CONTEXT.md) | Draft | Tiered memory system |
| [ZEN_JOURNAL.md](../03-DESIGN/ZEN_JOURNAL.md) | Draft | Immutable event ledger |
| [ZEN_LEDGER.md](../03-DESIGN/ZEN_LEDGER.md) | Draft | Token/cost accounting |
| [ZEN_GATE_POLICY.md](../03-DESIGN/ZEN_GATE_POLICY.md) | Draft | Admission control engine |
| [LLM_GATEWAY.md](../03-DESIGN/LLM_GATEWAY.md) | Draft | Provider-agnostic LLM interface |
| [BOUNDED_ORCHESTRATOR_LOOP.md](../03-DESIGN/BOUNDED_ORCHESTRATOR_LOOP.md) | Draft | Orchestrator state machine |
| [PROOF_OF_WORK.md](../03-DESIGN/PROOF_OF_WORK.md) | Draft | Proof-of-work bundle format |
| [SKILLS_AND_SUBAGENTS.md](../03-DESIGN/SKILLS_AND_SUBAGENTS.md) | Draft | Bounded execution helpers |
| [SMALL_MODEL_STRATEGY.md](../03-DESIGN/SMALL_MODEL_STRATEGY.md) | Complete | CPU-first local lane |
| [OPS_DEPARTMENT.md](../03-DESIGN/OPS_DEPARTMENT.md) | Complete | Jira-centric ops model |
| [SANDBOX_AND_EVALUATION.md](../03-DESIGN/SANDBOX_AND_EVALUATION.md) | Complete | Non-destructive evaluation lane |
| [MODEL_FACING_FILES_AND_SKILLS_POLICY.md](../03-DESIGN/MODEL_FACING_FILES_AND_SKILLS_POLICY.md) | Complete | Advisory-only policy |

---

## Notes

- **System remains open to any model/provider** – Qwen 0.8B is important baseline, not hard dependency
- **Aim is internal force multiplier**, not market product
- **Design remains Jira-centric** – Office layer as human front door
- **Worktree isolation is critical safety feature** – prevents uncontrolled cross-contamination
- **Trust levels and change classes before dogfooding becomes dangerous**
- **1.1 reserved for safety/sandbox features** – agent sandbox is evaluation-only, not production tool

---

*Last updated: 2026-03-08*