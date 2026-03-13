.PHONY: build test clean fmt lint help

# Build variables
BINARY_NAME := zen-brain
# Prefer VERSION file for release builds (e.g. 1.2.3); else git describe or "dev"
VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n' || git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Go parameters
GOCMD := go
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

COVERAGE_DIR := .artifacts/coverage
COVERAGE_OUT := $(COVERAGE_DIR)/coverage.out

test: ## Run tests
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_OUT) $(PKG_DIRS) $(INTERNAL_DIRS)

coverage: test ## Run tests and show coverage
	$(GOCMD) tool cover -html=$(COVERAGE_OUT)

fmt: ## Format code
	$(GOFMT) -w -s pkg internal cmd

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

clean: ## Clean build artifacts (coverage lives in .artifacts/coverage/)
	rm -rf bin/
	rm -rf $(COVERAGE_DIR)

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
## Canonical path: config/clusters.yaml + scripts/zen.py (127.0.1.x, zen-brain-registry:5001).
## Override env: make dev-up ZEN_DEV_ENV=staging

ZEN_DEV_ENV ?= sandbox

dev-up: ## Start k3d cluster and deploy (thin wrapper: zen.py env redeploy --env $(ZEN_DEV_ENV))
	python3 scripts/zen.py env redeploy --env $(ZEN_DEV_ENV)

dev-down: ## Stop k3d cluster (thin wrapper: zen.py env destroy; set CONFIRM_DESTROY=1 or --confirm-destroy)
	CONFIRM_DESTROY=1 python3 scripts/zen.py env destroy --env $(ZEN_DEV_ENV)

dev-logs: ## Tail logs from all pods
	kubectl logs -f --all-containers -l app.kubernetes.io/part-of=zen-brain --tail=100

dev-clean: ## Reset databases (local Docker: db-reset; for k3d full reset run dev-down then dev-up)
	$(MAKE) db-reset

dev-build: build-all ## Build all binaries (foreman, apiserver, zen-brain).

dev-image: ## Build zen-brain image and load into k3d (thin wrapper: zen.py image build --env $(ZEN_DEV_ENV))
	python3 scripts/zen.py image build --env $(ZEN_DEV_ENV)

# Deprecated: canonical deploy is Helmfile (make dev-up). Use only if you need a one-off raw apply without Helmfile.
dev-apply: ## [Deprecated] Raw kubectl apply; prefer: make dev-up (Helmfile sync)
	@echo "Deprecated: canonical path is 'make dev-up' (Helmfile). Use 'make dev-up' or 'python3 scripts/zen.py env redeploy --env $${ZEN_DEV_ENV:-sandbox}'."
	@echo "To apply without cluster create: python3 scripts/zen.py env redeploy --env $${ZEN_DEV_ENV:-sandbox} --skip-registry --skip-k3d --skip-build --skip-image-load"
	kubectl apply -f deployments/k3d/zen-brain-namespace.yaml
	kubectl apply -f deployments/k3d/foreman.yaml
	kubectl apply -f deployments/k3d/apiserver.yaml

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
