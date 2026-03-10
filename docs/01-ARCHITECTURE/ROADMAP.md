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

*Last updated: 2026-03-08*## 1.1 Status Updates

### Block 1.1 - ZenJournal Schema Definition ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Schema design document created (COMPONENT_JOURNAL.md, 18.5 KB)
- QueryIndex implementation created (internal/journal/receiptlog/query_index.go, 10.7 KB)
- Journal integration updated to use QueryIndex
- Tagged and pushed (commit acd7e53)

### Block 1.2 - ZenContext Tier 1 (Hot) ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Redis backend implemented with full SessionContext CRUD
- 11 passing tests (tier1/redis_test.go)
- Scratchpad, tasks, heartbeat support
- Stats tracking
- Session reconstruction support
- LastAccessedAt updates
- Composite ZenContext integration

### Block 1.2 - ZenContext Tier 2 (Warm) ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- QMD adapter integrated (internal/qmd/adapter.go)
- QMD store implementation for knowledge retrieval
- StoreKnowledge for knowledge storage
- 6 passing tests (tier2/qmd_store_test.go)
- Composite ZenContext integration

### Block 1.2 - ZenContext Tier 3 (Cold) ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- S3 backend implemented with AWS SDK v2
- Supports custom endpoints (MinIO)
- Supports path-style addressing
- Gzip compression, retention management
- Global session index
- Stats tracking
- Composite ZenContext integration

### Block 1.3 - SessionManager with ZenContext ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Updated SessionManager to integrate ZenContext
- Automatic SessionContext creation on session creation
- LastAccessedAt updates on all session operations
- Backward compatibility maintained (ZenContext optional)
- Integration tests pass

### Block 1.4 - Agent State & Planner Integration ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Agent state management system created (internal/agent/state.go, 406 lines)
- Role-based state, session tracking, task association
- Planner ZenContext config, state manager integration
- Agent state persistence and loading
- ReMe protocol for session reconstruction
- All 28+ agent/planner/session/context tests pass
- Tagged and pushed (commit 292bfd1, tag block-1.4-agent-integration)

### Block 1.5 - Wire Real Redis/S3 Clients ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Production Redis client with connection pooling (7,420 bytes)
- Production S3 client with AWS SDK v2 (10,428 bytes)
- ZenContext factory for all three tiers (9,447 bytes)
- Provider fallback chain implementation (10,396 bytes)
- All context tests pass (13 tests)
- All routing tests pass (7 tests)
- go mod tidy completes without errors
- Configuration template created
- Tagged and pushed (commit 770a590, tag block-1.5-redis-s3-clients)

### Block 1.6 - Configuration System ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Configuration loading system implemented (internal/config/load.go, 7,493 bytes)
- YAML config support with automatic path discovery
- Environment variable support for sensitive data (Jira, Confluence, AWS)
- Configuration-driven LLM Gateway initialization
- Default configuration templates (configs/config.dev.yaml)
- Config structure supports all sections: logging, kb, qmd, jira, confluence, clusters, sred, ledger, zen_context, planner
- All CI gates pass (10/10)

### Block 1.7 - Integration Tests ✅ COMPLETE
**Date:** 2026-03-09
**Status:** Complete
**Details:**
- Comprehensive integration tests added (cmd/zen-brain/main_integration_test.go, 11,982 bytes)
- 6 integration tests covering vertical slice pipeline:
  - TestVerticalSlice_EndToEnd - validates complete pipeline flow
  - TestVerticalSlice_ConfigurationLoading - tests config file parsing
  - TestVerticalSlice_SessionManagerIntegration - validates ZenContext contract
  - TestVerticalSlice_FactoryCommandExecution - validates real command execution
  - TestVerticalSlice_ProofOfWorkNoDuplicates - validates no duplicate PoW generation
  - TestVerticalSlice_CompletePipeline - validates all components work together
- All 6 integration tests pass
- CI gates all pass (10/10)
- Lint issues fixed

---


---

## Notes

- **System remains open to any model/provider** – Qwen 0.8B is important baseline, not hard dependency
- **Aim is internal force multiplier**, not market product
- **Design remains Jira-centric** – Office layer as human front door
- **Worktree isolation is critical safety feature** – prevents uncontrolled cross-contamination
- **Trust levels and change classes before dogfooding becomes dangerous**
- **1.1 reserved for safety/sandbox features** – agent sandbox is evaluation-only, not production tool
- **Local deployment uses k3d, not Docker Compose** – Aligns with CONSTRUCTION_PLAN.md V6 architecture

---

*Last updated: 2026-03-09*
