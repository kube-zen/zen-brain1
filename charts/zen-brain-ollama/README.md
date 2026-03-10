# zen-brain-ollama

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
