# Abstraction Boundaries for Zen-Brain 1.0

## Intent

Zen-Brain 1.0 must be modular enough that major components can be replaced later without redesigning the whole system.

**Stable orchestration, replaceable components.**

The product value should live in:
- control model
- canonical contracts
- policies
- proof-of-work
- role/profile design
- handoff logic
- safe execution

Not in hardcoding to one provider or one platform.

## Core Architectural Rule

**All major external or variable subsystems must be abstracted behind small, explicit interfaces.**

This applies at minimum to:
- Office / work-intake system
- LLM / model providers
- KB / QMD retrieval
- Message bus transport
- Journal persistence
- Proof-of-work storage
- Tool execution
- Handoff routing
- Compliance overlays
- Future department-specific systems

## Boundary List

### Office Boundary
The Office layer owns interaction with work-intake systems (Jira now, possibly others later).

**Allowed inside Office:**
- Jira-specific fields
- Jira API models
- Jira-specific transitions/ADF/search details

**Forbidden outside Office:**
- raw Jira issue models
- Jira field names/status constants
- Jira-specific business logic in Factory or planner

**Current Implementation:**
- `pkg/office/interface.go` defines the `Office` interface
- `internal/office/jira/` implements the Jira adapter
- Factory depends on `contracts.WorkItem`, not Jira types

### LLM Boundary
The provider layer owns interaction with local and remote models.

**Allowed inside provider layer:**
- provider-specific payloads
- API quirks
- local engine quirks
- token accounting details

**Forbidden outside provider layer:**
- direct provider API calls
- business logic that branches on model vendor names
- direct coupling to one model

**Current Implementation:**
- `pkg/llm/provider.go` defines the `Provider` interface
- `internal/llm/gateway.go` implements routing and provider abstraction
- `internal/llm/local_worker.go` implements local model provider
- `internal/llm/planner.go` implements cloud/expensive model provider
- All LLM calls go through the gateway interface

### KB/QMD Boundary
KB retrieval depends on a small query interface.

**Allowed inside adapter:**
- qmd invocation details
- stdout/stderr parsing
- refresh/index logic

**Forbidden outside adapter:**
- direct qmd process invocation
- qmd-specific result assumptions everywhere

**Current Implementation:**
- `pkg/qmd/client.go` defines the `Client` interface
- `pkg/kb/store.go` defines the `Store` interface
- `internal/qmd/adapter.go` implements both interfaces for qmd
- `internal/qmd/kb_store.go` implements knowledge base storage
- All knowledge queries go through the KB store interface

### Runtime Primitive Boundary
Generic runtime building blocks belong to zen-sdk.

**Owned by zen-sdk unless documented exception:**
- retry
- dedup
- dlq
- receiptlog
- observability
- health
- leader
- logging
- crypto
- generic scheduler/event helpers where appropriate

**Current Implementation:**
- `zen-sdk/pkg/retry` used in `internal/llm/gateway.go`
- `zen-sdk/pkg/crypto` for proof-of-work verification
- `zen-sdk/pkg/receiptlog` for ZenJournal implementation
- `zen-sdk/pkg/ledger` for ZenLedger

### Factory Boundary
Factory executes work from canonical task/spec objects only.

**Allowed:**
- canonical `contracts.BrainTaskSpec`
- generic workspace management
- tool execution via policy
- proof-of-work generation

**Forbidden:**
- Jira object handling
- provider-specific request building
- direct qmd logic
- broad undeclared tool access

**Current Implementation:**
- `internal/factory/factory.go` depends only on `contracts.BrainTaskSpec`
- `internal/factory/workspace.go` handles workspace isolation
- `internal/factory/proof.go` generates provider-agnostic proof artifacts
- No Jira-specific types in factory

### Tool Boundary
Tools must be mediated and policy-bound.

**Desired future model:**
- declarative tool definitions
- tool bindings
- policy checks before execution

**Current Implementation:**
- Factory provides isolated workspace for tool execution
- Tool access controlled by session/role policies
- Future: explicit tool registry with capability declarations

### Handoff Boundary
Cross-area/domain triggers must pass through explicit handoff policy or at minimum a typed handoff contract.

**Current Implementation:**
- Session state transitions trigger notifications
- Factory completion triggers proof-of-work generation
- Office updates triggered via `contracts.WorkUpdate`
- Future: explicit handoff interface for cross-component coordination

## Design Consequence

The orchestration logic should remain stable even if:
- Jira is replaced
- qmd is replaced
- models change
- tool implementations change
- storage backends evolve

## Verification Checklist

- [ ] Factory imports only `contracts.*`, not `jira.*` or `qmd.*`
- [ ] Planner imports only `pkg/llm`, not specific provider implementations
- [ ] Analyzer imports only `pkg/kb`, not `internal/qmd`
- [ ] All external system interactions go through defined interfaces
- [ ] No business logic depends on provider/vendor names
- [ ] No reimplementation of zen-sdk primitives
- [ ] All component boundaries have interface definitions

## Evolution Policy

When adding new external systems:
1. Define interface in appropriate `pkg/` package
2. Implement adapter in `internal/<system>/`
3. Update boundary documentation
4. Verify no coupling leaks

When replacing existing systems:
1. Implement new adapter satisfying existing interface
2. Update configuration to use new adapter
3. Remove old adapter when migration complete
4. Core orchestration logic remains unchanged

## Related Documents

- [SOURCE_OF_TRUTH.md](./SOURCE_OF_TRUTH.md) - Data ownership and canonical sources
- [CONSTRUCTION_PLAN.md](./CONSTRUCTION_PLAN.md) - Overall build sequence
- [ADR/](ADR/) - Architecture decision records
- Design documents in `docs/03-DESIGN/` - Component-specific design