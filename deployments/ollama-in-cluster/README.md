# Ollama in-cluster (Block 5) – UNSUPPORTED for CPU-only sandbox/dev

> ⚠️ **WARNING: This path is UNSUPPORTED for CPU-only sandbox/dev environments**
>
> In-cluster Ollama has **severe performance issues** on CPU:
> - Latency: 3-5+ minutes per request (vs 8-23s with host Docker Ollama)
> - Success rate: ~50% (frequent 500 errors)
> - Root cause: k8s networking overhead + resource contention
>
> **Use host Docker Ollama instead:** See `deploy/README.md` for the supported path.

**Default for Zen-Brain 1.0 dev/sandbox:** Host Docker Ollama (outside Kubernetes), accessed via `host.k3d.internal:11434`.

**This in-cluster Ollama path is optional, legacy, and experimental** — not the default choice. In-cluster Ollama has shown performance issues; host Docker Ollama provides better GPU passthrough and isolation.

**Canonical deployment is Helm/Helmfile.** See `deploy/README.md` and `charts/zen-brain-ollama/`.

- **Optional in-cluster path:** If you have a strong reason to use in-cluster Ollama and understand the performance tradeoffs, set `deploy.use_ollama: true` and `deploy.ollama.models: ["qwen3.5:0.8b"]` in `config/clusters.yaml`, then run `make dev-up` or `python3 scripts/zen.py env redeploy --env <env>`. Model preload is **declarative** (Helm hook Job); no manual `kubectl exec ... ollama pull`.
- **Emergency/manual:** If you need to pull a model outside the declarative flow, use the StatefulSet pod: `kubectl exec -it ollama-0 -n zen-brain -- ollama pull <model>`.

The files in this directory (`ollama.yaml`) are **legacy**; they are no longer applied by the canonical redeploy path (which uses Helmfile and `charts/zen-brain-ollama`). Kept for reference or one-off apply.
