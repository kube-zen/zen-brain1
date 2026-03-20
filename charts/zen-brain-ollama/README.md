# zen-brain-ollama

## 🚨 LEGACY / UNSUPPORTED for Active Local CPU Path (ZB-023)

> ⚠️ **WARNING: This chart is LEGACY and UNSUPPORTED for active local CPU path**
>
> **Use Host Docker Ollama instead:** See `deploy/README.md` for the supported path.

### Canonical Source of Truth

| Document | Purpose | Link |
|----------|---------|-------|
| **Canonical Policy** | Local CPU inference policy (qwen3.5:0.8b ONLY) | [SMALL_MODEL_STRATEGY.md](../../docs/03-DESIGN/SMALL_MODEL_STRATEGY.md) |
| **Operational Guide** | Operations for local Ollama | [OLLAMA_08B_OPERATIONS_GUIDE.md](../../docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md) |
| **Deployment Guide** | Deployment instructions (host Docker Ollama) | [deploy/README.md](../../deploy/README.md) |

### Legacy Status

This chart is **legacy and optional** — NOT the default choice for Zen-Brain 1.0.

**DO NOT USE** unless you have:
1. **EXPLICIT OPERATOR APPROVAL** (not just casual experimentation)
2. **Strong reason** (e.g., specific k8s environment requirement)
3. **Understanding** of severe performance tradeoffs (3-5+ min latency vs 8-23s with host Docker)
4. **Approved override** documented in ZB-023 runbook

---

## Overview (Legacy)

One shared Ollama per cluster (Block 5): StatefulSet, PVC-backed model cache, optional VPA with `updateMode: Initial`, declarative model preload.

## Prerequisites

- **Metrics Server** and **VPA** (Vertical Pod Autoscaler) must be installed in the cluster if `vpa.enabled` is true. VPA is not built into Kubernetes; install separately. VPA uses `updateMode: Initial` by default (recommendations applied at pod creation only; no in-place resize).

## Values (from config/clusters.yaml)

- `enabled`, `kind: StatefulSet`, `replicas: 1`
- `models` – list of model names to preload via Helm hook Job
- `keepAlive` – `OLLAMA_KEEP_ALIVE` (e.g. `"2m"`) for memory reuse
- `persistence.enabled`, `persistence.size`, `persistence.storageClassName`
- `vpa.enabled`, `vpa.updateMode` (default `Initial`), `vpa.minAllowed`, `vpa.maxAllowed`
- `resources` – requests/limits (guardrails)
- `service.port` – default 11434

## Design

- **StatefulSet** so model cache is stable and one operational surface.
- **VPA Initial** for rightsizing on pod create/restart; do not rely on in-place resize yet.
- **Preload Job** (Helm hook) pulls configured models via Ollama API; no manual `kubectl exec ... ollama pull`.
