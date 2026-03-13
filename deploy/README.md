# Zen-Brain deployment (Helmfile)

**IMPORTANT: Ollama deployment model for Zen-Brain 1.0 dev/sandbox**

- **Default path:** Host Docker Ollama (outside Kubernetes), accessed via `host.k3d.internal:11434`
- **In-cluster Ollama:** Optional, legacy, experimental — not the default path
- **Reasoning:** In-cluster Ollama has shown performance issues; host Docker Ollama provides better GPU passthrough and isolation

**Canonical path:** Helm/Helmfile. No manual `kubectl apply`, `kubectl exec ... ollama pull`, or post-sync patch in the standard flow. The full deployment plane (`deploy/`, `charts/`) is in the repo and included in git archive.

- **Env contract:** `config/clusters.yaml` (includes `k3d.k8s_image` for Kubernetes version, e.g. 1.35.x)
- **Entrypoint:** `python3 scripts/zen.py env redeploy --env <env>` (or `make dev-up`)
- **Flow:** ensure cluster/registry → build/load image → render values → helmfile sync → wait. Apiserver external exposure (LoadBalancer, port) is in chart values; no patch step.

## Layout

- `helmfile/zen-brain/helmfile.yaml.gotmpl` – Helmfile (releases: crds, dependencies, **ollama**, core). Ollama before core so local-worker has a real service from first boot.
- `values/<env>/` – optional per-env overrides
- Charts: `charts/zen-brain-crds`, `zen-brain-dependencies`, `zen-brain`, `zen-brain-ollama`

## Requirements

- `helm` and `helmfile` on PATH
- k3d cluster (created with image from `config/clusters.yaml` → `k3d.k8s_image`, default `rancher/k3s:v1.35.2-k3s1`)

## Ollama (Block 5)

- **One shared Ollama per cluster:** StatefulSet, one replica, PVC for model cache.
- **VPA:** Optional; default `updateMode: Initial` (rightsizing on pod create/restart; VPA does not yet support in-place resize in 1.35). Requires **Metrics Server** and **VPA** installed in the cluster (install separately; not shipped in-chart).
- **Config:** `deploy.use_ollama: true`, `deploy.ollama.models`, `deploy.ollama.keepAlive` (e.g. `"2m"`), `deploy.ollama.vpa.enabled`, `updateMode`, `minAllowed`/`maxAllowed`. Model preload is a Helm hook Job; no manual `kubectl exec ... ollama pull`.

**Validation:** See [DEPLOYMENT_VALIDATION.md](../docs/04-DEVELOPMENT/DEPLOYMENT_VALIDATION.md) for the full end-to-end checklist (offline checks + live `make dev-up` → Helmfile → workloads → Ollama preload → apiserver inference).
