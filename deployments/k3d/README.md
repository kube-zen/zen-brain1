# K3D Deployment

This directory contains Kubernetes manifests for deploying Zen‑Brain on a local k3d cluster.

## Prerequisites

- [k3d](https://k3d.io/) v5.6.0+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [helm](https://helm.sh/) (optional)

## Quick Start

1. Create a k3d cluster:

   ```bash
   k3d cluster create zen-brain --port 8080:80@loadbalancer
   ```

2. Apply the CRDs:

   ```bash
   kubectl apply -f ../crds/
   ```

3. Deploy Zen‑Brain components (TBD).

## Components

- **Zen‑Brain Controller**: Manages ZenProject and ZenCluster CRDs.
- **Zen‑Brain Agent**: Runs on each cluster, executes tasks.
- **CockroachDB**: For ZenLedger (optional in development).
- **Ingress**: Exposes the API and UI (future).

## Development

Use `make dev-up` and `make dev-down` (see root Makefile) to manage the cluster.

## Configuration

Edit `values.yaml` (when Helm charts are ready) to customize the deployment.