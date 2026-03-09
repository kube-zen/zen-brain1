.PHONY: build test clean fmt lint help

# Build variables
BINARY_NAME := zen-brain
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
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

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) $(CMD_DIR)

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

db-migrate: ## Run database migrations (requires db-up)
	@echo "TODO: implement database migrations"

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

## Code generation

generate: ## Generate code (CRDs, deepcopy)
	@echo "TODO: implement controller-tools generation"

## Repository management

repo-sync: ## Sync knowledge base repositories
	@echo "TODO: implement repo sync"

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
