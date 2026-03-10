# K3D Deployment

This directory contains Kubernetes manifests for deploying Zen‑Brain on a local k3d cluster.

## Prerequisites

- [k3d](https://k3d.io/) v5.6.0+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/) (optional)

## Quick Start

1. Create a k3d cluster and apply dependencies (includes zen-context namespace):

   ```bash
   make dev-up
   ```

   Or manually:

   ```bash
   k3d cluster create zen-brain-dev -p "8080:80@loadbalancer" -p "26257:26257@loadbalancer" --registry-create zen-registry:5000
   kubectl apply -f deployments/k3d/dependencies.yaml
   ```

2. Apply the CRDs:

   ```bash
   kubectl apply -f deployments/crds/
   ```

3. (Optional) Deploy ZenContext in-cluster (Redis + MinIO):

   ```bash
   kubectl apply -f deployments/zencontext-in-cluster/
   ```

## In-cluster deployment (Foreman + API server)

Foreman and API server can run **inside** the k3d cluster using the manifests in this directory.

1. **Build the image and load into k3d** (from repo root):

   ```bash
   make dev-image
   ```

   Or manually:

   ```bash
   docker build -t zen-brain:dev .
   k3d image import zen-brain:dev -c zen-brain-dev
   ```

   **Note:** If the repo has private Go module dependencies (e.g. `zen-sdk`), ensure Docker can access them (e.g. `GOPRIVATE`, `GOPROXY`, or build with `--build-arg` and a mounted `~/.netrc` / token). Otherwise build the image on a host that already has `go mod download` working.

2. **Apply the zen-brain namespace and in-cluster manifests** (after CRDs and optional ZenContext):

   ```bash
   kubectl apply -f deployments/crds/
   kubectl apply -f deployments/k3d/zen-brain-namespace.yaml
   kubectl apply -f deployments/k3d/foreman.yaml
   kubectl apply -f deployments/k3d/apiserver.yaml
   ```

   Or in one go (after CRDs):

   ```bash
   kubectl apply -f deployments/k3d/zen-brain-namespace.yaml -f deployments/k3d/foreman.yaml -f deployments/k3d/apiserver.yaml
   ```

3. **Verify:**

   ```bash
   kubectl get pods -n zen-brain
   kubectl logs -n zen-brain -l app.kubernetes.io/name=foreman -f
   kubectl port-forward -n zen-brain svc/apiserver 8080:8080   # then curl http://localhost:8080/healthz)
   ```

- **Foreman** runs in `zen-brain` with a ServiceAccount and Role for `braintasks`/`brainqueues`. To enable ReMe (session context), deploy ZenContext Redis (`kubectl apply -f deployments/zencontext-in-cluster/`) and set env `ZEN_CONTEXT_REDIS_URL=redis://zencontext-redis.zen-context.svc.cluster.local:6379` in `foreman.yaml`.
- **API server** serves `/healthz`, `/readyz`, `/api/v1/sessions`, `/api/v1/health`, `/api/v1/version`, `/api/v1/evidence`. Optional API key auth via `ZEN_API_KEY` (e.g. from a Secret).

## Running components locally (alternative)

You can still run Foreman and API server as **local binaries** with kubeconfig pointing at the k3d cluster (e.g. `export KUBECONFIG=~/.kube/config` after `make dev-up`).

- **Foreman:** `make build-foreman && ./bin/foreman` — apply CRDs first. Optional: `-zen-context-redis=redis://...` for ReMe; **Git worktree mode:** set `ZEN_FOREMAN_USE_GIT_WORKTREE=true`, `ZEN_FOREMAN_SOURCE_REPO=/path/to/repo`, etc.
- **API server:** `make build-apiserver && ./bin/apiserver` — serves the same endpoints as above.
- **zen-brain:** Run from repo root for the vertical slice (Office → Analyzer → Planner → Factory).

## Components

- **Zen‑Brain Controller**: Manages ZenProject and ZenCluster CRDs (TBD in-cluster).
- **Foreman**: Reconciles BrainTask; run locally with kubeconfig (see above).
- **ZenContext**: Redis + MinIO via `deployments/zencontext-in-cluster/`.
- **CockroachDB**: For ZenLedger; use `make db-up` for local or deploy separately.
- **Ingress**: Exposes the API and UI (future).

## Development

Use `make dev-up` and `make dev-down` (see root Makefile) to manage the cluster.

## Configuration

For in-cluster deploy, edit the YAML files in this directory (e.g. `foreman.yaml` env, `apiserver.yaml` env or Secret for `ZEN_API_KEY`) to customize.