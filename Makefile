.PHONY: build test clean fmt lint help

# Build variables
BINARY_NAME := zen-brain
# Prefer VERSION file for release builds (e.g. 1.2.3); else git describe or "dev"
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD := GOTOOLCHAIN=local go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt

# Directories
CMD_DIR := ./cmd/zen-brain
PKG_DIRS := ./pkg/...
INTERNAL_DIRS := ./internal/...

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the main binary
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) $(CMD_DIR)

build-foreman: ## Build Foreman controller (Block 4.2)
	$(GOBUILD) $(LDFLAGS) -o bin/foreman ./cmd/foreman

build-apiserver: ## Build API server (Block 3.4)
	$(GOBUILD) $(LDFLAGS) -o bin/apiserver ./cmd/apiserver

build-controller: ## Build Zen-Brain controller (Block 6; ZenProject/ZenCluster)
	$(GOBUILD) $(LDFLAGS) -o bin/controller ./cmd/controller

build-all: build build-foreman build-apiserver build-controller ## Build all binaries

test: ## Run tests
	$(GOTEST) -v -race -coverprofile=coverage.out $(PKG_DIRS) $(INTERNAL_DIRS)

coverage: test ## Run tests and show coverage
	$(GOCMD) tool cover -html=coverage.out

fmt: ## Format code
	$(GOFMT) -w -s pkg internal cmd

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

## Development helpers

run: build ## Build and run
	./bin/$(BINARY_NAME)

db-up: ## Start local database (CockroachDB via Docker)
	docker run -d --name zen-brain-db \
		-p 26257:26257 -p 8080:8080 \
		cockroachdb/cockroach:latest-v24.1 start-single-node --insecure

db-down: ## Stop local database
	docker stop zen-brain-db && docker rm zen-brain-db

# Default CockroachDB URL for local dev (use db-up first)
DATABASE_URL ?= cockroachdb://root@localhost:26257/defaultdb?sslmode=disable

db-migrate: ## Run database migrations (requires db-up; needs golang-migrate: go install -tags 'cockroachdb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)
	@command -v migrate >/dev/null 2>&1 || { echo "migrate CLI not found. Install: go install -tags 'cockroachdb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" -verbose up

db-reset: db-down db-up ## Reset database (stop, remove, start)
	@echo "Database reset complete"

## k3d cluster development (Block 6)

dev-up: ## Start k3d cluster and deploy dependencies
	k3d cluster create zen-brain-dev \
		-p "8080:80@loadbalancer" \
		-p "26257:26257@loadbalancer" \
		--registry-create zen-registry:5000
	kubectl apply -f deployments/k3d/dependencies.yaml

dev-down: ## Stop k3d cluster
	k3d cluster delete zen-brain-dev

dev-logs: ## Tail logs from all pods
	kubectl logs -f --all-containers -l app.kubernetes.io/part-of=zen-brain --tail=100

dev-clean: ## Reset databases (local Docker: db-reset; for k3d full reset run dev-down then dev-up)
	$(MAKE) db-reset

dev-build: build-all ## Build all binaries (foreman, apiserver, zen-brain).

# Build Docker image and load into k3d (Block 6 in-cluster deploy). Run after dev-up.
dev-image: ## Build zen-brain:dev image and import into k3d cluster zen-brain-dev
	docker build -t zen-brain:dev .
	k3d image import zen-brain:dev -c zen-brain-dev

# Apply in-cluster stack (Block 6 bootstrap). Run after: make dev-up, kubectl apply -f deployments/crds/, make dev-image.
dev-apply: ## Apply zen-brain namespace, foreman, apiserver, and controller to k3d
	kubectl apply -f deployments/k3d/zen-brain-namespace.yaml
	kubectl apply -f deployments/k3d/foreman.yaml
	kubectl apply -f deployments/k3d/apiserver.yaml
	kubectl apply -f deployments/k3d/controller.yaml

## Code generation

generate: ## Generate code (CRDs, deepcopy) - requires controller-gen
	@command -v controller-gen >/dev/null 2>&1 || go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0
	controller-gen object paths=./api/...
	controller-gen crd:allowDangerousTypes=true paths=./api/... output:crd:dir=./deployments/crds

## Repository management

repo-sync: ## Sync KB repos for QMD population. Set ZEN_KB_REPO_URL to clone; ZEN_KB_REPO_DIR (default ../zen-docs) must match tier2_qmd.repo_path
	python3 scripts/repo_sync.py

## Repository hygiene

install-hooks: ## Install Git hooks (.githooks/pre-commit)
	@echo "Installing Git hooks..."
	@mkdir -p .git/hooks
	@cp .githooks/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✓ Pre‑commit hook installed."

repo-check: ## Run all repository hygiene gates
	@echo "Running repository hygiene gates..."
	@python3 scripts/ci/run.py --suite default
