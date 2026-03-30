# Makefile for zen-brain1

VERSION ?= dev
BUILD_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

.PHONY: build run test clean fmt lint vet install db-up db-down db-reset db-migrate

# ═══════════════════════════════════════════════════════════════════════════════
# BUILD
# ═══════════════════════════════════════════════════════════════════════════════

build:
	@echo "Building zen-brain1..."
	@go build -o bin/zen-brain1 ./cmd/zen-brain
	@echo "Build complete: bin/zen-brain1"

run:
	@go run ./cmd/zen-brain

test:
	@go test -v ./...

fmt:
	@go fmt ./...

lint:
	@golangci-lint run ./... 2>/dev/null || true

vet:
	@go vet ./...

install:
	@go mod download && go mod tidy

clean:
	@rm -rf bin/ zen-brain

# ═══════════════════════════════════════════════════════════════════════════════
# DATABASE (CockroachDB in k3d)
# ═══════════════════════════════════════════════════════════════════════════════

K3D_CLUSTER ?= zen-brain
CRDB_RELEASE ?= zen-brain-crdb
CRDB_URL ?= postgresql://root@localhost:26257/zenbrain?sslmode=disable

db-up: ## Start CockroachDB in k3d (single-node, insecure)
	@echo "Starting k3d cluster..."
	@k3d cluster create $(K3D_CLUSTER) \
		-p "26257:26257@loadbalancer" \
		-p "8082:8080@loadbalancer" \
		--k3s-arg "--disable=traefik@server:0" 2>/dev/null || true
	@echo "Installing CockroachDB..."
	@helm repo add cockroachdb https://charts.cockroachdb.com/ 2>/dev/null || true
	@helm repo update
	@helm upgrade --install $(CRDB_RELEASE) cockroachdb/cockroachdb \
		--set replicaCount=1 \
		--set insecure.mode=true \
		--set storage.persistentVolume.size=5Gi \
		--set service.ports.grpc.external.name=grpc \
		--set service.ports.grpc.external.port=26257 \
		--set service.ports.grpc.external.nodePort=26257 \
		--set service.type=LoadBalancer \
		--set config.maxSQLMemory="256MiB" \
		--set config.cache="128MiB" \
		--namespace zen-brain --create-namespace \
		--wait --timeout 120s || echo "Helm install failed - check logs"
	@echo "Waiting for CockroachDB..."
	@sleep 10
	@kubectl rollout status statefulset/$(CRDB_RELEASE) -n zen-brain --timeout=60s 2>/dev/null || true
	@echo "Creating database 'zenbrain'..."
	@kubectl exec -n zen-brain statefulset/$(CRDB_RELEASE) -- \
		./cockroach sql --insecure -e "CREATE DATABASE IF NOT EXISTS zenbrain;" 2>/dev/null || true
	@echo ""
	@echo "✅ CockroachDB ready:"
	@echo "   SQL: $(CRDB_URL)"
	@echo "   UI:  http://localhost:8082"

db-down: ## Stop CockroachDB (keep data)
	@helm uninstall $(CRDB_RELEASE) -n zen-brain 2>/dev/null || true
	@echo "CockroachDB stopped."

db-reset: ## Reset database (destroys data)
	@kubectl exec -n zen-brain statefulset/$(CRDB_RELEASE) -- \
		./cockroach sql --insecure -e "DROP DATABASE IF EXISTS zenbrain CASCADE; CREATE DATABASE zenbrain;" 2>/dev/null || \
		echo "Could not reset - is CockroachDB running?"
	@echo "✅ Database reset."

db-migrate: ## Apply schema migrations
	@echo "Applying migrations..."
	@if ! command -v migrate >/dev/null 2>&1; then \
		echo "Installing golang-migrate..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	@migrate -path migrations -database "$(CRDB_URL)" up
	@echo "✅ Migrations applied."

db-shell: ## SQL shell
	@kubectl exec -n zen-brain -it statefulset/$(CRDB_RELEASE) -- \
		./cockroach sql --insecure -d zenbrain

# ═══════════════════════════════════════════════════════════════════════════════
# DEV ENVIRONMENT
# ═══════════════════════════════════════════════════════════════════════════════

dev-up: db-up db-migrate ## Full dev setup

dev-down: db-down ## Full teardown
	@k3d cluster delete $(K3D_CLUSTER) 2>/dev/null || true

k3d-down: ## Delete k3d cluster
	@k3d cluster delete $(K3D_CLUSTER) 2>/dev/null || true

# ═══════════════════════════════════════════════════════════════════════════════
# HELP
# ═══════════════════════════════════════════════════════════════════════════════

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS=":.*?##"}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
