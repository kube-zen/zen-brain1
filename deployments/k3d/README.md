# K3D Deployment

This directory contains Kubernetes manifests for deploying Zen‑Brain on a local k3d cluster.

## Prerequisites

- [k3d](https://k3d.io/) v5.6.0+
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- Docker
- Python 3 with PyYAML (for config-driven lifecycle)

## Canonical path (config-driven)

Deployment is driven by **config/clusters.yaml** (127.0.1.x, zen-brain-registry:5000). Single entrypoint: **scripts/zen.py**.

- **Bring up sandbox (cluster + registry + manifests + image + rollout):**

  ```bash
  make dev-up
  ```
  or explicitly:
  ```bash
  python3 scripts/zen.py env redeploy --env sandbox
  ```

- **Tear down:**

  ```bash
  make dev-down
  ```
  or:
  ```bash
  CONFIRM_DESTROY=1 python3 scripts/zen.py env destroy --env sandbox --confirm-destroy
  ```

- **Build and load image only (cluster already up):**
- ```bash
  make dev-image
  ```
- **Note:** Use shared registry :5000 instead of k3d image import. The canonical deployment path uses registry container pull.
  or:
  ```bash
  python3 scripts/zen.py image build --env sandbox
  ```

- **Status:**

  ```bash
  python3 scripts/zen.py env status --env sandbox
  ```

  Environments: `sandbox` (127.0.1.6), `staging` (127.0.1.2), `uat` (127.0.1.3). Apiserver is exposed at `http://<env_ip>:8080/healthz` after redeploy.
 
## Zen-Lock Integration

**Status:** ✅ Integrated into canonical deployment path

Zen-Lock is now deployed via Helmfile before zen-brain-core, ensuring secret injection capabilities are available.

**Secret Management:**
- Master key secret: `zen-lock-master-key` in zen-lock-system namespace
- Sourced from: `~/.zen-lock/private-key.age`
- Auto-applied during redeploy
- Shared registry path: `zen-registry:5000/kubezen/zen-lock:0.0.3-alpha-zb1fix2` (pinned, fixed)

**Manifests:**
- ✅ No manual kubectl apply of zen-lock manifests required
- ✅ Uses canonical Helm chart: `kube-zen/zen-lock@0.0.3-alpha`
- ✅ Release order: crds → zen-lock → dependencies → ollama → core

**Check Zen-Lock:**
```bash
kubectl -n zen-lock-system get pods
kubectl -n zen-lock-system get secret zen-lock-master-key
```

**Note:** Jira credentials are deployed via `deploy/zen-lock/jira-credentials.zenlock.yaml` (encrypted ZenLock resource). Enable foreman Jira Zen-Lock injection in values to use them.

## Quick Start (manual)

1. Create a k3d cluster and apply dependencies (includes zen-context namespace):

   ```bash
   make dev-up
   ```

   Or manually (legacy; prefer canonical path above):

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

   **Canonical local dependency path:** k3d + `dependencies.yaml` + `deployments/zencontext-in-cluster/` only. Docker Compose is not used for ZenContext; the repo is k8s-first for local dev.

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

2. **Apply the zen-brain namespace and in-cluster stack** (after CRDs and optional ZenContext):

   One-command bootstrap (Block 6):

   ```bash
   make dev-apply
   ```

   Or manually:

   ```bash
   kubectl apply -f deployments/k3d/zen-brain-namespace.yaml
   kubectl apply -f deployments/k3d/foreman.yaml
   kubectl apply -f deployments/k3d/apiserver.yaml
   kubectl apply -f deployments/k3d/controller.yaml
   ```

3. **Verify:**

   ```bash
   kubectl get pods -n zen-brain
   kubectl logs -n zen-brain -l app.kubernetes.io/name=foreman -f
   kubectl logs -n zen-brain -l app.kubernetes.io/name=zen-brain-controller -f
   kubectl port-forward -n zen-brain svc/apiserver 8080:8080   # then curl http://localhost:8080/healthz
   ```

- **Foreman** runs in `zen-brain` with a ServiceAccount and Role for `braintasks`/`brainqueues`. To enable ReMe (session context), deploy ZenContext Redis (`kubectl apply -f deployments/zencontext-in-cluster/`) and set env `ZEN_CONTEXT_REDIS_URL=redis://zencontext-redis.zen-context.svc.cluster.local:6379` in `foreman.yaml`.
- **API server** serves `/healthz`, `/readyz`, `/api/v1/sessions`, `/api/v1/health`, `/api/v1/version`, `/api/v1/evidence`. Optional API key auth via `ZEN_API_KEY` (e.g. from a Secret).

## Running components locally (alternative)

You can still run Foreman and API server as **local binaries** with kubeconfig pointing at the k3d cluster (e.g. `export KUBECONFIG=~/.kube/config` after `make dev-up`).

- **Foreman:** `make build-foreman && ./bin/foreman` — apply CRDs first. Optional: `-zen-context-redis=redis://...` for ReMe; **Git worktree mode:** set `ZEN_FOREMAN_USE_GIT_WORKTREE=true`, `ZEN_FOREMAN_SOURCE_REPO=/path/to/repo`, etc.
- **API server:** `make build-apiserver && ./bin/apiserver` — serves the same endpoints as above.
- **zen-brain:** Run from repo root for the vertical slice (Office → Analyzer → Planner → Factory).

## Components

- **Zen‑Brain Controller**: Manages ZenProject and ZenCluster CRDs in-cluster; deployed via `controller.yaml` and `make dev-apply`. Status-only reconciliation (Phase, Ready condition).
- **Foreman**: Reconciles BrainTask; run locally with kubeconfig or in-cluster via `foreman.yaml`.
- **ZenContext**: Redis + MinIO via `deployments/zencontext-in-cluster/`.
- **CockroachDB**: For ZenLedger; use `make db-up` for local or deploy separately.
- **Ingress**: Exposes the API and UI (future).

## Development

Use `make dev-up` and `make dev-down` (see root Makefile) to manage the cluster.

## Configuration

- **Cluster lifecycle:** Edit **config/clusters.yaml** (registry port, env IPs, k3d servers/agents, deploy options). No hidden defaults; scripts fail fast if env or keys are missing.
- **In-cluster deploy:** Edit the YAML files in this directory (e.g. `foreman.yaml` env, `apiserver.yaml` env or Secret for `ZEN_API_KEY`) to customize.