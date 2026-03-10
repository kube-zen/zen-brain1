# Ollama in-cluster (Block 5) – legacy raw manifests

**Canonical deployment is now Helm/Helmfile.** See `deploy/README.md` and `charts/zen-brain-ollama/`.

- **Standard path:** Set `deploy.use_ollama: true` and `deploy.ollama.models: ["qwen3.5:0.8b"]` in `config/clusters.yaml`, then run `make dev-up` or `python3 scripts/zen.py env redeploy --env <env>`. Model preload is **declarative** (Helm hook Job); no manual `kubectl exec ... ollama pull` in the standard path.
- **Emergency/manual:** If you need to pull a model outside the declarative flow, use the StatefulSet pod: `kubectl exec -it ollama-0 -n zen-brain -- ollama pull <model>`.

The files in this directory (`ollama.yaml`) are **legacy**; they are no longer applied by the canonical redeploy path (which uses Helmfile and `charts/zen-brain-ollama`). Kept for reference or one-off apply.
