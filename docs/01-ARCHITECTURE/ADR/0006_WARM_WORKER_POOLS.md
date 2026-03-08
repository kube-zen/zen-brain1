# ADR 0006: Use warm worker pools with session affinity and git worktrees

## Status

**Accepted** (2026‑03‑07)

## Context

In the initial Zen‑Brain 0.1 design, each task spawned a fresh Kubernetes Pod that pulled the container image, started the container, loaded the model, executed the task, then terminated. This **destroy/recreate pattern** had significant overhead:

```
Task arrives → Schedule pod → Pull image → Start container → Load model → Execute → Die
    |            |            |              |              |
 ~2‑5s         ~1‑3s        ~1‑2s         ~5‑30s (CPU)
Total cold start: 10‑40 seconds per task
```

This overhead is unacceptable for interactive or multi‑step tasks where latency matters. Additionally, each task started with a blank slate—no ability to carry context between steps of the same session.

## Decision

Implement **warm worker pools** with **session affinity** and **git worktrees** for isolation:

1. **Warm worker pool** – Long‑running worker pods that keep models loaded, eliminating cold‑start overhead.
2. **Session affinity** – Multi‑step tasks stay on the same worker, preserving context in `/dev/shm`.
3. **Git worktrees** – Each task/session gets an isolated writable workspace via `git worktree add`, avoiding merge conflicts.
4. **Shared factory floor** – A shared volume (`/factory`) contains bare repositories and worktrees.

**Architecture:**

```
+---------------------------------------------------------------------+
|                      FACTORY FLOOR                                   |
|                                                                      |
|  Shared Volume: /factory/                                            |
|  |-- repos/                                                          |
|  |   |-- zen-brain-1.0/          (bare repo)                        |
|  |   +-- worktrees/                                                  |
|  |       |-- wt-task-001/        (worktree for task 001)            |
|  |       |-- wt-task-002/        (worktree for task 002)            |
|  |       +-- wt-session-abc/     (worktree for session abc)         |
|  |-- artifacts/                  (shared output)                     |
|  +-- cache/                      (model cache, shared)               |
|                                                                      |
+---------------------------------------------------------------------+
|                       WORKER POOL                                    |
|                                                                      |
|  +-------------------------------------------------------------+    |
|  | Worker Pool (Deployment, replicas=N)                         |    |
|  |                                                              |    |
|  |  Pod-1              Pod-2              Pod-3              Pod-4|    |
|  |  +----------+      +----------+      +----------+      +-----+|    |
|  |  | Session  |      | Session  |      |  Idle    |      |Task ||    |
|  |  |   ABC    |      |   DEF    |      |          |      | GHI ||    |
|  |  |          |      |          |      |          |      |     ||    |
|  |  | wt-abc/  |      | wt-def/  |      |          |      |wt-gh||    |
|  |  | /dev/shm |      | /dev/shm |      |          |      |/dev/||    |
|  |  | (ctx)    |      | (ctx)    |      |          |      |shm  ||    |
|  |  |          |      |          |      |          |      |     ||    |
|  |  | Model OK |      | Model OK |      | Model OK |      |Model||    |
|  |  +----------+      +----------+      +----------+      +-----+|    |
|  |                                                              |    |
|  |  All pods:                                                   |    |
|  |  - Mount /factory (read-write)                              |    |
|  |  - tmpfs /dev/shm 512Mi (per-pod private scratch)           |    |
|  |  - Model pre-loaded on startup                              |    |
|  |  - Long-running (do not die after task)                     |    |
|  +-------------------------------------------------------------+    |
|                                                                      |
+---------------------------------------------------------------------+
|                      DISPATCHER                                      |
|                                                                      |
|  Task arrives:                                                       |
|  1. Check if session exists → route to same worker                 |
|  2. If new task → pick idle worker or queue                        |
|  3. Worker creates worktree: git worktree add /factory/wt-task-N    |
|  4. Worker executes in worktree                                     |
|  5. Worker cleans up worktree: git worktree remove                  |
|  6. Worker marks itself available                                   |
+---------------------------------------------------------------------+
```

**Key elements:**

- **Worker pods** stay alive, pre‑load models, mount `/factory` (shared) and `/dev/shm` (private tmpfs).
- **Session affinity** – session‑to‑worker mapping stored in CockroachDB; dispatcher routes accordingly.
- **Git worktrees** – cheap isolation (`~100ms` to create/remove), full git history, no merge conflicts.
- **Shared model cache** – `/factory/cache` stores downloaded models, shared across workers.

## Consequences

### Positive

- **Eliminates cold‑start overhead** – tasks start in `~100ms` (worktree creation) vs `10‑40s`.
- **Context preservation** – session context lives in `/dev/shm` across multiple steps.
- **Resource efficiency** – model memory shared across tasks on same worker.
- **Isolation** – each task gets its own worktree, preventing file conflicts.
- **Git‑native** – all changes are commits; easy to review, revert, merge.

### Negative

- **Memory idle cost** – workers stay alive consuming memory even when idle.
- **Complexity** – need to manage worker lifecycle, session mapping, worktree cleanup.
- **Stateful** – worker failures require session recovery via ReMe protocol.
- **Storage overhead** – `/factory` volume must be large enough for all worktrees.

### Neutral

- The pattern is **Kubernetes‑native** – uses Deployments, PersistentVolumeClaims, emptyDir volumes.
- Works with **multi‑cluster** – each cluster has its own worker pool.

## Alternatives Considered

### 1. Destroy/recreate (original)

- **Pros**: Simple, stateless, excellent isolation.
- **Cons**: High latency, no context carry, wasteful of CPU/network.

### 2. Virtual environments (venv, conda, nix) per task

- **Pros**: Strong isolation, reproducible environments.
- **Cons**: Heavyweight (GBs per task), slow to create.

### 3. Container snapshots (CRIU)

- **Pros**: Can snapshot a running container and restore later.
- **Cons**: Complex, Linux‑specific, not well‑supported in Kubernetes.

### 4. Per‑task pods with shared emptyDir

- **Pros**: Simpler than worktrees, still isolated.
- **Cons**: No git integration, harder to track changes.

The warm pool + worktree approach balances latency, isolation, and git‑friendliness.

## Related Decisions

- [ADR‑0004](0004_MULTI_CLUSTER_CRDS.md) – Multi‑cluster topology (worker pools are per‑cluster).
- Construction Plan V6.0, Section 3.5 – Warm Worker Pool with Session Affinity (A+C Hybrid).

## References

- Construction Plan V6.0, Section 3.5 – detailed architecture diagram and implementation notes.
- `git‑worktree` documentation – https://git‑scm.com/docs/git‑worktree
- Kubernetes `emptyDir` with `medium: Memory` – for `/dev/shm` scratch.