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

## Current path: run Zen‑Brain components locally

Foreman and API server are not yet deployed as in-cluster workloads. Use local binaries with kubeconfig pointing at the k3d cluster:

- **Foreman:** `make build-foreman && ./bin/foreman` (set `KUBECONFIG` or use default; apply CRDs first).
- **API server:** `make build-apiserver && ./bin/apiserver`.

**TBD:** Helm charts or manifests for Foreman and API server as in-cluster deployments (Deployments/Services).

## Components

- **Zen‑Brain Controller**: Manages ZenProject and ZenCluster CRDs (TBD in-cluster).
- **Foreman**: Reconciles BrainTask; run locally with kubeconfig (see above).
- **ZenContext**: Redis + MinIO via `deployments/zencontext-in-cluster/`.
- **CockroachDB**: For ZenLedger; use `make db-up` for local or deploy separately.
- **Ingress**: Exposes the API and UI (future).

## Development

Use `make dev-up` and `make dev-down` (see root Makefile) to manage the cluster.

## Configuration

Edit `values.yaml` (when Helm charts are ready) to customize the deployment.