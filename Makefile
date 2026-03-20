# Makefile for zen-brain1

.PHONY: build run test clean fmt lint vet install

# Build the application
build:
	@echo "Building zen-brain1..."
	@go build -ldflags "-X github.com/kube-zen/zen-brain1/cmd/zen-brain.Version=$(VERSION) -X github.com/kube-zen/zen-brain1/cmd/zen-brain.BuildSHA=$(BUILD_SHA) -X github.com/kube-zen/zen-brain1/cmd/zen-brain.BuildTime=$(BUILD_TIME)" -o bin/zen-brain1 ./cmd/zen-brain
	@echo "Build complete: bin/zen-brain1"

# Run the application
run:
	@echo "Running zen-brain1..."
	@go run -ldflags "-X main.Version=$(VERSION) -X main.BuildSHA=$(BUILD_SHA) -X main.BuildTime=$(BUILD_TIME)" .

# Run with default configuration
run-policy:
	@echo "Running zen-brain1 with policy configuration..."
	@POLICY_CONFIG_DIR=./config/policy/ LOG_LEVEL=debug \
		go run -ldflags "-X main.Version=$(VERSION) -X main.BuildSHA=$(BUILD_SHA) -X main.BuildTime=$(BUILD_TIME)" .

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./src/...

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./...

# Vet code
vet:
	@echo "Vetting code..."
	@go vet ./...

# Install dependencies
install:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/

# Validate policy configuration
validate-policy:
	@echo "Validating policy configuration..."
	@POLICY_CONFIG_DIR=./config/policy/ go run -ldflags "-X main.Version=$(VERSION) -X main.BuildSHA=$(BUILD_SHA) -X main.BuildTime=$(BUILD_TIME)" 2>&1 | grep -i "policy"

# Validate policy YAML syntax
validate-yaml:
	@echo "Validating YAML syntax..."
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/roles.yaml'))"
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/tasks.yaml'))"
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/providers.yaml'))"
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/routing.yaml'))"
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/prompts.yaml'))"
	@python3 -c "import yaml; yaml.safe_load(open('config/policy/chains.yaml'))"
	@echo "All YAML files are valid"

# Show policy summary
show-policy:
	@echo "=== Policy Configuration Summary ==="
	@echo ""
	@echo "Roles:"
	@grep "^  - name:" config/policy/roles.yaml | wc -l
	@echo ""
	@echo "Tasks:"
	@grep "^  - name:" config/policy/tasks.yaml | wc -l
	@echo ""
	@echo "Providers:"
	@grep "^  - name:" config/policy/providers.yaml | wc -l
	@echo ""
	@echo "Chains:"
	@grep "^  - name:" config/policy/chains.yaml | wc -l
	@echo ""
	@echo "Total policy files:"
	@ls -1 config/policy/*.yaml | wc -l

# Default targets
all: build

# Version variables (can be overridden)
VERSION ?= 1.0.0
BUILD_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
