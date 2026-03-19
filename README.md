# zen-brain1

**Version:** 1.0.0
**Status:** Policy-Based Configuration System

## Overview

zen-brain1 is an AI orchestration service with policy-based configuration. This replaces hardcoded provider and routing configuration with flexible YAML files in `config/policy/`.

## Features

### ✅ Policy-Based Configuration
- **Roles** - AI agent roles with system prompts and capabilities
- **Tasks** - Task classes with timeouts and output schemas
- **Providers** - AI provider definitions (DeepSeek, OpenAI, Anthropic)
- **Routing** - Request routing and model selection policies
- **Prompts** - System prompts and templates for each role
- **Chains** - Task execution chains and workflows

### Key Capabilities
- ✅ Declarative YAML configuration
- ✅ Cross-reference validation
- ✅ Multi-provider arbitration
- ✅ BYOK (Bring Your Own Key) support
- ✅ Customizable system prompts
- ✅ Task chains with dependencies

## Quick Start

### Build and Run

```bash
# Clone repository
git clone https://github.com/kube-zen/zen-brain1.git
cd zen-brain1

# Install dependencies
make install

# Build
make build

# Run with policy configuration
make run-policy

# Or run manually
POLICY_CONFIG_DIR=./config/policy/ LOG_LEVEL=debug ./bin/zen-brain1
```

### Validate Policy

```bash
# Validate YAML syntax
make validate-yaml

# Validate policy configuration (cross-references)
make validate-policy

# Show policy summary
make show-policy
```

## Policy Configuration

### Policy Files

All policy files are in `config/policy/`:

| File | Purpose |
|------|----------|
| `roles.yaml` | AI agent roles and capabilities |
| `tasks.yaml` | Task classes and requirements |
| `providers.yaml` | AI provider definitions and pricing |
| `routing.yaml` | Request routing and model selection |
| `prompts.yaml` | System prompts and templates |
| `chains.yaml` | Task execution chains and workflows |

See **`config/policy/README.md`** for complete policy documentation.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POLICY_CONFIG_DIR` | Path to policy config directory | `./config/policy/` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `ANTHROPIC_API_KEY` | Anthropic API key | - |
| `DEEPSEEK_API_KEY` | DeepSeek API key | - |

## Architecture

### Policy Loader

The policy loader (`src/config/policy/loader.go`) reads all YAML files and validates cross-references:

```go
// Load all policy files
policyConfig, errors := policy.LoadConfig("./config/policy/")
if len(errors) > 0 {
    log.Fatal(errors)
}
```

### Validation

Policy validation (`src/config/policy/validation.go`) ensures:
- ✅ Roles exist in `roles.yaml`
- ✅ Tasks reference valid roles
- ✅ Chains reference valid tasks
- ✅ Roles reference valid providers
- ✅ Circular dependency detection in chains
- ✅ Model availability checks

### Provider Factory

The provider factory (`src/config/policy/provider_factory.go`) creates AI providers based on policy:

```go
// Build registry from policy
factory := policy.NewConfiguredProviderFactory(policyConfig)
registry, err := factory.BuildRegistry(apiKeys, enabledProviders)
```

## Examples

### Use Specific Provider

```bash
# Set environment variable
export AI_DEFAULT_PROVIDER=openai

# Run zen-brain1
./bin/zen-brain1
```

### Use Custom Policy Directory

```bash
# Point to custom policy directory
export POLICY_CONFIG_DIR=/custom/policy/

# Run zen-brain1
./bin/zen-brain1
```

### Define Custom Task Chain

Edit `config/policy/chains.yaml`:

```yaml
chains:
  - name: my-custom-chain
    description: "My custom analysis workflow"
    tasks:
      - name: step1
        task_class: event-analysis
        role: security-analyst
      - name: step2
        task_class: intelligence
        role: security-analyst
        depends_on: [step1]
```

## Documentation

- **`config/policy/README.md`** - Complete policy configuration reference
- **`deploy/README.md`** - Deployment guide
- **`docs/05-OPERATIONS/policy-migration-guide.md`** - Migration from hardcoded config

## Development

### Project Structure

```
zen-brain1/
├── main.go                  # Application entry point
├── go.mod                   # Go module definition
├── Makefile                  # Build and test targets
├── config/
│   └── policy/              # Policy configuration files
│       ├── roles.yaml
│       ├── tasks.yaml
│       ├── providers.yaml
│       ├── routing.yaml
│       ├── prompts.yaml
│       ├── chains.yaml
│       └── README.md
├── src/
│   └── config/
│       └── policy/          # Policy loader and validation
│           ├── loader.go
│           ├── validation.go
│           ├── provider_factory.go
│           └── policy.go
├── deploy/
│   └── README.md           # Deployment guide
└── docs/
    └── 05-OPERATIONS/
        └── policy-migration-guide.md
```

### Build Targets

```bash
# Build binary
make build

# Run tests
make test

# Format code
make fmt

# Lint code
make lint

# Validate policy
make validate-policy
make validate-yaml
make show-policy
```

## License

Copyright © 2026 kube-zen

## Support

- **Documentation:** `config/policy/README.md`
- **Issues:** https://github.com/kube-zen/zen-brain1/issues
