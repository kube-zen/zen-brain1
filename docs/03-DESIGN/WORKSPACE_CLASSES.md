# Workspace Classes and Trust Levels

## Purpose

Workspace Classes and Trust Levels provide **layered safety boundaries** for AI-driven code execution:

- **Prevent accidental damage** to production codebases
- **Control access** to sensitive repositories and paths
- **Limit destructive operations** based on trust level
- **Enable performance acceleration** (tmpfs) with safety guards

---

## Workspace Classes

### 1. Sandbox (Isolated) - `class:sandbox`
**Purpose:** Complete isolation for untrusted code execution and experimentation
**Characteristics:**
- No git integration (standalone directory)
- No access to production codebases
- Filesystem access limited to workspace only
- Cannot write to protected paths
- Temporary - auto-cleaned after timeout
**Use Cases:**
- AI-generated code verification
- Malicious pattern testing
- Prototype/experimentation
- Security-sensitive operations
**Default for:** Unknown AI agents, third-party integrations, experimental features

### 2. Protected (Restricted) - `class:protected`
**Purpose:** Restricted access to trusted codebases with controlled modifications
**Characteristics:**
- Git worktree isolation from main branch
- Read access to protected repos
- Write access limited to non-critical paths
- Requires explicit approval for writes
- Diffs generated before any apply
**Use Cases:**
- Bug fixes in production code
- Feature implementation in trusted repos
- Code refactoring
- Documentation updates
**Default for:** Known AI agents with proven track record, approved tasks

### 3. Production (Unrestricted) - `class:production`
**Purpose:** Full access for trusted operations with minimal restrictions
**Characteristics:**
- Full git access (branch, merge, push)
- Write access to any path in repo
- Destructive operations allowed with approval
- Performance optimizations enabled (tmpfs)
**Use Cases:**
- Human-supervised sessions
- Critical bug fixes
- Deployment operations
- Emergency responses
**Default for:** Human-initiated work, high-trust AI agents

---

## Trust Levels

| Trust Level | Workspace Class | Allowed Operations | Requires Approval |
|-------------|------------------|-------------------|-------------------|
| `trust:none` | Sandbox | Read-only within workspace | N/A |
| `trust:low` | Sandbox | Read/write within workspace | Yes (all writes) |
| `trust:medium` | Protected | Read/write in protected paths | Yes (writes to protected paths) |
| `trust:high` | Protected | Read/write in protected paths | No (non-critical) |
| `trust:full` | Production | Full access including destructive ops | No |

**Approval Rules:**
- `trust:none` → No approval needed (no write access)
- `trust:low` → Approval needed for any write operation
- `trust:medium` → Approval needed for writes to protected paths
- `trust:high` → Approval needed only for writes to critical paths
- `trust:full` → No approval needed (full trust)

---

## Protected Repositories

Protected repositories are defined in configuration:

```yaml
workspace:
  protected_repos:
    - path: /home/neves/zen/zen-platform
      class: protected
      allowed_paths:
        - internal/**
        - src/saas/back/api/**
      forbidden_paths:
        - src/saas/front/**          # UI is high-risk
        - infrastructure/**            # Infra is critical
      critical_paths:
        - src/saas/back/api/auth/**   # Auth is critical
    - path: /home/neves/zen/zen-brain1
      class: production
      allowed_paths:
        - internal/**
        - docs/**
      forbidden_paths: []
      critical_paths: []
```

**Rules:**
- `allowed_paths`: Explicit allowlist of paths that can be written to
- `forbidden_paths`: Explicit denylist that blocks write access
- `critical_paths`: High-risk paths requiring special approval
- If both `allowed_paths` and `forbidden_paths` are specified, `allowed_paths` takes precedence

---

## Delete Protections

### Protected Path Delete Rules
```go
// Cannot delete:
- Protected repository root directories
- Critical paths (auth, infra, config, secrets)
- Workspace root directories (already enforced)

// Can delete:
- Workspace subdirectories (with workspace class restrictions)
- Non-protected repository files (outside critical paths)
- Temporary artifacts
```

### Delete Approval Matrix
| Path Type | Trust Level | Approval Required |
|-----------|-------------|------------------|
| Protected repo root | Any | Always (block) |
| Critical path | `trust:high` | Yes |
| Critical path | `trust:full` | Yes |
| Non-critical protected path | `trust:medium` | Yes |
| Non-critical protected path | `trust:high` | No |
| Non-protected path | Any | No |

---

## Tmpfs Acceleration

Tmpfs provides **in-memory filesystem acceleration** for I/O-heavy operations:

### Configuration
```yaml
workspace:
  tmpfs:
    enabled: true
    min_memory_mb: 1024      # Minimum required RAM
    safety_margin: 0.2         # 20% headroom margin
    enabled_classes:            # Which workspace classes can use tmpfs
      - sandbox
      - protected
    # Production class excluded by default for safety
```

### Memory Check
Before enabling tmpfs for a workspace:
```go
// 1. Get available memory
availableMem := getAvailableMemoryMB()

// 2. Check minimum requirement
if availableMem < config.Tmpfs.MinMemoryMB {
    return fmt.Errorf("insufficient memory for tmpfs: %dMB available, %dMB required",
        availableMem, config.Tmpfs.MinMemoryMB)
}

// 3. Apply safety margin
usableMem := float64(availableMem) * (1.0 - config.Tmpfs.SafetyMargin)
tmpfsSize := int(usableMem * config.Tmpfs.UsageRatio)

// 4. Create tmpfs mount
mountTmpfs(workspacePath, tmpfsSize)
```

### Tmpfs Rules
- **Only for specified classes** (sandbox, protected by default)
- **Production class excluded** (data loss risk on crash)
- **Explicit opt-in** per task or session
- **Auto-unmount on workspace deletion**
- **Warning logged** when memory is marginal

---

## Implementation Plan

### Phase 1: Workspace Class Enumeration (30 min)
- [ ] Define `WorkspaceClass` enum (sandbox, protected, production)
- [ ] Define `TrustLevel` enum (none, low, medium, high, full)
- [ ] Add fields to `WorkspaceMetadata` struct
- [ ] Add configuration structures for protected repos
- [ ] Add configuration for tmpfs settings

### Phase 2: Protected Path Validation (1 hour)
- [ ] Implement `ProtectedPathChecker` module
- [ ] Load protected repo configuration
- [ ] Validate write operations against allowed/forbidden paths
- [ ] Validate delete operations against protected paths
- [ ] Implement approval matrix logic

### Phase 3: Workspace Class Selection (1 hour)
- [ ] Implement `WorkspaceClassSelector` based on trust level
- [ ] Add trust level assignment logic (default: `trust:low`)
- [ ] Integrate with `WorkspaceManager.CreateWorkspace()`
- [ ] Set workspace class in metadata
- [ ] Log class selection decisions

### Phase 4: Delete Protections (45 min)
- [ ] Enhance `WorkspaceManager.DeleteWorkspace()` with class checks
- [ ] Add protected path validation before deletes
- [ ] Implement delete approval matrix
- [ ] Add tests for delete protection rules

### Phase 5: Tmpfs Acceleration (1.5 hours)
- [ ] Implement `TmpfsManager` module
- [ ] Add memory checking logic
- [ ] Implement tmpfs mount/unmount
- [ ] Add tmpfs support to `WorkspaceManager.CreateWorkspace()`
- [ ] Add auto-unmount on cleanup
- [ ] Add tests for tmpfs scenarios

### Phase 6: Testing and Documentation (1 hour)
- [ ] Unit tests for each module
- [ ] Integration tests for end-to-end workflows
- [ ] Update ROADMAP.md with implementation status
- [ ] Update documentation with examples
- [ ] Add troubleshooting guide

**Total Effort:** ~5.5 hours

---

## Configuration Examples

### Development Environment
```yaml
workspace:
  default_class: sandbox
  default_trust_level: low
  protected_repos: []
  tmpfs:
    enabled: false
```

### Production Environment
```yaml
workspace:
  default_class: protected
  default_trust_level: medium
  protected_repos:
    - path: /home/neves/zen/zen-platform
      class: production
      allowed_paths:
        - internal/**
        - src/saas/back/**
      critical_paths:
        - src/saas/back/api/auth/**
        - infrastructure/**
  tmpfs:
    enabled: true
    min_memory_mb: 2048
    enabled_classes:
      - sandbox
      - protected
```

---

## Security Considerations

1. **Fail Closed** - Unknown paths are rejected by default
2. **Explicit Allowlist** - Protected paths require explicit configuration
3. **Trust Level Escalation** - Cannot increase trust level during session
4. **Class Immutability** - Workspace class cannot change after creation
5. **Tmpfs Data Loss** - Tmpfs is lost on crash/power failure (documented)
6. **Approval Audit Trail** - All approvals logged to ZenLedger

---

## Related Documents

- [Bounded Orchestrator Loop](BOUNDED_ORCHESTRATOR_LOOP.md) - Session lifecycle management
- [Proof of Work](PROOF_OF_WORK.md) - Workspace evidence collection
- [Small Model Strategy](SMALL_MODEL_STRATEGY.md) - Performance optimization

---

*Last updated: 2026-03-11*
