# Development Environment Setup

This guide walks through setting up a complete Zen‑Brain development environment on a fresh Linux/macOS machine.

## Prerequisites

### 1. Operating System
- **Linux** (Ubuntu 22.04+ recommended) or **macOS** (Sonoma+)
- 8+ GB RAM, 4+ CPU cores, 20 GB free disk space

### 2. Install Base Tools

```bash
# Git
sudo apt install git  # Ubuntu
brew install git      # macOS

# Make, curl, wget
sudo apt install make curl wget
```

### 3. Install Go 1.25+

```bash
# Download from https://go.dev/dl/
wget https://go.dev/dl/go1.25.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### 4. Install Docker

```bash
# Ubuntu
sudo apt install docker.io
sudo systemctl enable --now docker
sudo usermod -aG docker $USER  # logout/login required

# macOS: Docker Desktop from https://www.docker.com/products/docker-desktop/
```

### 5. Install k3d (Kubernetes)

```bash
# Linux/macOS
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
k3d version
```

### 6. Install kubectl and helm

```bash
# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

## Repository Setup

```bash
# Clone the repository
git clone git@github.com:kube‑zen/zen‑brain1.git
cd zen‑brain1

# Install Go dependencies
make deps

# Verify the build
make build
```

## Local Kubernetes Cluster

### Create a k3d Cluster

```bash
make dev-up
```

This script runs:

```bash
k3d cluster create zen-brain-dev \
  -p "8080:80@loadbalancer" \
  -p "26257:26257@loadbalancer" \
  --registry-create zen-registry:5000
```

### Deploy Dependencies

```bash
kubectl apply -f deployments/k3d/dependencies.yaml
```

This installs:

- **CockroachDB** (single‑node, insecure) – for ZenLedger and KB metadata.
- **Redis** – for ZenContext Tier 1.
- **MinIO** (S3‑compatible) – for object storage (Tier 3, journal backups).
- **Ollama** – for local LLM models (glm‑4.7, nomic‑embed‑text).

### Verify Cluster

```bash
kubectl get pods --all-namespaces
```

Wait until all pods are `Running`.

## Configuration

### Create Local Configuration

Copy the development configuration template:

```bash
mkdir -p ~/.zen‑brain
cp configs/config.dev.yaml ~/.zen‑brain/config.yaml
```

Edit `~/.zen‑brain/config.yaml` to match your environment (API keys, endpoints). Set `ZEN_BRAIN_DEV=true` in your shell:

```bash
echo 'export ZEN_BRAIN_DEV=true' >> ~/.bashrc
source ~/.bashrc
```

### Database Migrations

Run initial database migrations:

```bash
make db-migrate
```

This creates the CockroachDB schema (ZenLedger tables, policy rules, etc.).

## Building and Running

### Build the Binary

```bash
make build
```

Output: `bin/zen‑brain`

### Run Locally (Outside Kubernetes)

```bash
make run
```

The binary reads configuration from `~/.zen‑brain/config.yaml` and connects to the local k3d cluster services.

### Build and Load into k3d Registry

To deploy Zen‑Brain into the k3d cluster:

```bash
make dev-build
```

This builds a Docker image and loads it into the k3d registry.

### Deploy to k3d Cluster

```bash
kubectl apply -f deployments/k3d/zen-brain-dev.yaml
```

Check deployment status:

```bash
kubectl get pods -n zen-brain
```

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

Requires the k3d cluster to be running:

```bash
go test -tags=integration ./...
```

### End‑to‑End Tests

First, ensure the cluster is running and Zen‑Brain is deployed:

```bash
make dev-up
make dev-build
kubectl apply -f deployments/k3d/zen-brain-dev.yaml
```

Then run e2e tests:

```bash
cd tests/e2e
go test -v
```

## Common Tasks

### View Logs

```bash
make dev-logs
```

### Reset Database (Development Only)

```bash
make db-reset
```

### Stop and Delete Cluster

```bash
make dev-down
```

### Clean Everything

```bash
make dev-clean  # stops cluster, removes images, deletes config
```

## IDE Setup

### Visual Studio Code

Recommended extensions:

- **Go** (by Google)
- **Kubernetes** (by Microsoft)
- **Docker** (by Microsoft)
- **YAML** (by Red Hat)

Settings (`settings.json`):

```json
{
    "go.testFlags": ["-v"],
    "go.lintTool": "golangci-lint",
    "editor.formatOnSave": true
}
```

### IntelliJ / GoLand

Enable Go modules, Kubernetes plugin.

## Troubleshooting

### k3d Cluster Fails to Start

- Ensure Docker is running.
- Check for port conflicts (8080, 26257).
- Try `k3d cluster delete zen-brain-dev` then `make dev-up`.

### Database Connection Refused

- Verify CockroachDB pod is running: `kubectl get pods -n dependencies`.
- Check service endpoint: `kubectl get svc -n dependencies cockroachdb-public`.
- Ensure `config.yaml` uses the correct service name (`cockroachdb-public:26257`).

### Ollama Model Not Loading

Canonical path: set `deploy.ollama.models` in `config/clusters.yaml` and run `make dev-up` (Helm preload Job pulls them). For emergency manual pull (StatefulSet in zen-brain):

```bash
kubectl exec -it ollama-0 -n zen-brain -- ollama pull glm-4.7
kubectl exec -it ollama-0 -n zen-brain -- ollama pull nomic-embed-text
```

### Insufficient Memory

If pods are evicted, increase k3d memory limit:

```bash
k3d cluster delete zen-brain-dev
k3d cluster create zen-brain-dev \
  -p "8080:80@loadbalancer" \
  -p "26257:26257@loadbalancer" \
  --registry-create zen-registry:5000 \
  --memory 8G  # increase from default 2G
```

## Next Steps

- Read the [Contributing Guide](../../CONTRIBUTING.md) for coding standards.
- Explore the [Architecture Decision Records](../01-ARCHITECTURE/ADR/) to understand design decisions.
- Run through the [Workflow Examples](../06-EXAMPLES/WORKFLOW_EXAMPLES.md) to see the system in action.

---

*This guide is for development only. Production deployment uses different configurations and security practices.*