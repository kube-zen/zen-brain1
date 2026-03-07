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
