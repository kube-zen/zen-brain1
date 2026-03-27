# Ollama in-cluster (Block 5) — **DEPRECATED**

> ⚠️ **DEPRECATED (2026-03-26):** This directory is legacy and no longer maintained. The primary runtime is **llama.cpp** (L1/L2). Ollama (L0) is fallback only via **host Docker**, never in-cluster. See `deploy/README.md` and `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.
>
> **Original warning (ZB-023):** This path is LEGACY and UNSUPPORTED for active local CPU path

> ⚠️ **WARNING: This path is LEGACY and UNSUPPORTED for active local CPU path (ZB-023)**
>
> In-cluster Ollama has **severe performance issues** on CPU:
> - Latency: 3-5+ minutes per request (vs 8-23s with host Docker Ollama)
> - Success rate: ~50% (frequent 500 errors)
> - Root cause: k8s networking overhead + resource contention
>
> **Use host Docker Ollama instead:** See `deploy/README.md` for the supported path.

## Canonical Source of Truth

| Document | Purpose | Link |
|----------|---------|-------|
| **Canonical Policy** | Local CPU inference policy (qwen3.5:0.8b ONLY) | [SMALL_MODEL_STRATEGY.md](../../docs/03-DESIGN/SMALL_MODEL_STRATEGY.md) |
| **Operational Guide** | Operations for local Ollama | [OLLAMA_08B_OPERATIONS_GUIDE.md](../../docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md) |
| **Deployment Guide** | Deployment instructions (host Docker Ollama) | [deploy/README.md](../README.md) |

## Legacy Status

**This in-cluster Ollama path is legacy and optional** — NOT the default choice for Zen-Brain 1.0.

### ZB-023 Policy

- **FORBIDDEN:** In-cluster Ollama for active local CPU path
- **SUPPORTED:** Host Docker Ollama (http://host.k3d.internal:11434) ONLY
- **CERTIFIED:** qwen3.5:0.8b is ONLY certified local model

### Historical Context

The files in this directory (`ollama.yaml`) are **legacy**; they are no longer applied by the canonical redeploy path (which uses Helmfile and `charts/zen-brain-ollama`). Kept for reference or one-off apply only.

### When In-Cluster Ollama Might Be Used

**DO NOT USE** unless you have:
1. **EXPLICIT OPERATOR APPROVAL** (not just casual experimentation)
2. **Strong reason** (e.g., specific k8s environment requirement)
3. **Understanding** of severe performance tradeoffs
4. **Approved override** documented in ZB-023 runbook

### Deployment (Legacy)

If you must use in-cluster Ollama despite the warning (requires explicit operator approval):

- Set `deploy.use_ollama: true` and `deploy.ollama.models: ["qwen3.5:0.8b"]` in `config/clusters.yaml`
- Run `make dev-up` or `python3 scripts/zen.py env redeploy --env <env>`
- Model preload is **declarative** (Helm hook Job); no manual `kubectl exec ... ollama pull`
- **Emergency/manual:** If you need to pull a model outside the declarative flow, use the StatefulSet pod: `kubectl exec -it ollama-0 -n zen-brain -- ollama pull <model>`.

## References

- ZB-023: Local CPU Inference Rule - [docs/05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md](../../docs/05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md)
- Small Model Strategy - [docs/03-DESIGN/SMALL_MODEL_STRATEGY.md](../../docs/03-DESIGN/SMALL_MODEL_STRATEGY.md)
