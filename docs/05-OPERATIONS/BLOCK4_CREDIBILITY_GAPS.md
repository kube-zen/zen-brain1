# Block 4 Factory - Credibility Gap Analysis

**Date**: 2026-03-11
**Status**: Honest assessment of remaining weaknesses
**Previous Claim**: "96% complete" (misleading)

## Executive Summary

Block 4 has real substance but contains critical stub/fallback components that undermine trust. The Factory behaves more like a "smart scaffolder" than a fully trustworthy execution fabric.

## Critical Gaps

### 1. NoOpDispatcher (`internal/foreman/dispatcher.go`)

```go
// NoOpDispatcher is a TaskDispatcher that does nothing (placeholder until workers are implemented).
type NoOpDispatcher struct{}
```

**Impact**: Core dispatcher does nothing. Real worker dispatch exists but dispatcher abstraction is incomplete.

**Credibility Hit**: HIGH - "dispatcher" implies work distribution, but it's a no-op

### 2. StubManager (`internal/worktree/manager.go`)

```go
// StubManager is a Manager that uses a temp directory per Prepare (Block 4 stub).
type StubManager struct{}

func (s *StubManager) Prepare(ctx context.Context, taskID, sessionID string) (string, func(), error) {
    dir, err := os.MkdirTemp("", s.Prefix)
    // ... returns temp dir, not real git worktree
}
```

**Impact**: Factory can run without real git worktrees.

**Mitigating Factor**: Real `GitManager` exists and is used in production paths (`factory.go`, `factory_runner.go`)

**Credibility Hit**: MEDIUM - stub exists but real implementation is wired up

### 3. StubGate (`internal/gate/stub.go`)

```go
// StubGate implements gate.ZenGate by allowing all requests and returning no validation errors.
type StubGate struct{}

func (s *StubGate) Admit(...) (*AdmissionResponse, error) {
    return &AdmissionResponse{Allowed: true}, nil  // Always allows
}

func (s *StubGate) Validate(...) ([]ValidationError, error) {
    return nil, nil  // Never validates
}
```

**Impact**: Gate can be bypassed entirely with stub.

**Mitigating Factor**: `PolicyGate` exists and is wired in `cmd/foreman/main.go` when policies are configured

**Credibility Hit**: MEDIUM - stub is fallback, not default in production

### 4. StubGuardian (`internal/guardian/stub.go`)

```go
// StubGuardian implements guardian.ZenGuardian with no-op monitoring and allow-all safety.
type StubGuardian struct{}

func (StubGuardian) CheckSafety(...) (SafetyCheckResult, error) {
    return SafetyCheckResult{Safe: true}, nil  // Always safe
}
```

**Impact**: Safety monitoring can be bypassed.

**Mitigating Factor**: `CircuitBreaker` exists for real failure tracking

**Credibility Hit**: MEDIUM - stub is used when guardian not configured

### 5. Templates with TODO Content (`internal/factory/repo_aware_templates.go`)

6 explicit TODOs in generated output:
- Documentation templates: "TODO: Add getting started content"
- Deployment templates: "TODO: Configure deployment strategy"
- Metrics middleware: "TODO: Add metrics collection here"
- Migration SQL: "TODO: Add migration SQL here"
- Rollback SQL: "TODO: Add rollback SQL here"
- Migrations doc: "TODO: List migrations here"

**Impact**: Factory generates placeholder content that requires manual completion.

**Credibility Hit**: MEDIUM - Factory is "smart scaffolder", not autonomous creator

## Real vs Stub Matrix

| Component | Stub Exists | Real Exists | Default Path |
|-----------|-------------|-------------|--------------|
| Dispatcher | ✅ NoOpDispatcher | ⚠️ Partial | Stub |
| Worktree | ✅ StubManager | ✅ GitManager | Real (when configured) |
| Gate | ✅ StubGate | ✅ PolicyGate | Real (when policies exist) |
| Guardian | ✅ StubGuardian | ⚠️ CircuitBreaker | Stub (in foreman main) |

## Honest Assessment

### What's Real

1. **GitManager** - Real git worktree management
2. **PolicyGate** - Real policy-based admission
3. **Static Analysis** - staticcheck, golangci-lint, pylint, eslint
4. **Multi-Language Execution** - Go, Python, Node.js with real execution
5. **Cryptographic Signing** - RSA/ECDSA signature verification
6. **Proof Verification** - SHA256 checksums, before/after state

### What's Still Stub/Scaffold

1. **NoOpDispatcher** - No real dispatch abstraction
2. **StubGuardian default** - foreman main.go defaults to stub
3. **TODO templates** - Generated content needs manual completion
4. **Smart scaffolder behavior** - Not autonomous creation

## Recommended Remediation

### Priority 1: Remove NoOpDispatcher

Either:
- Remove the stub entirely and force real implementation
- Or rename to `NullDispatcher` to make intent explicit

### Priority 2: Default to Real Components

Change `cmd/foreman/main.go`:
- Default to `PolicyGate` with empty policy list (not stub)
- Default to real `Guardian` with logging (not stub)

### Priority 3: Template Quality

Either:
- Remove TODO placeholders and generate minimal valid content
- Or mark templates as "scaffolding only" in documentation

## Revised Completion Estimate

| Aspect | Previous Claim | Honest Assessment |
|--------|----------------|-------------------|
| Tests passing | 100% | 100% ✅ |
| Real components | 96% | 75% ⚠️ |
| Production-ready | 96% | 70% ⚠️ |
| Credibility | High | **Medium** ⚠️ |

**True Block 4 Status**: ~75% complete for production trustworthiness

## Action Items

1. [ ] Remove or rename `NoOpDispatcher`
2. [ ] Change foreman defaults to use real Gate/Guardian
3. [ ] Add integration test that fails if stubs are used in production path
4. [ ] Update all status reports with honest percentages
5. [ ] Create "Factory Trustworthiness" test suite

---

*Created: 2026-03-11 - Response to credibility concerns*
