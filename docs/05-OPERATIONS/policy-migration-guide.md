# Policy Configuration Migration Guide

**Status:** ✅ Complete
**Version:** 1.0
**Date:** 2026-03-19

## Overview

This guide explains the migration from hardcoded AI provider configuration to file-based policy configuration using YAML files in `config/policy/`.

## What Changed

### Before (Hardcoded)

AI providers and routing were hardcoded in Go code:

```go
// src/main.go
registry := buildRegistry()  // Hardcoded provider factory
defaultProvider := "deepseek"  // Environment variable only
```

**Limitations:**
- Code changes required to add providers
- No declarative routing configuration
- System prompts embedded in code
- No task chains or workflows
- Difficult to customize per environment

### After (Policy-Based)

AI providers, routing, prompts, and chains are now configured via YAML files:

```
config/policy/
├── roles.yaml      # AI agent roles and capabilities
├── tasks.yaml      # Task classes and requirements
├── providers.yaml  # AI provider definitions
├── routing.yaml    # Request routing and model selection
├── prompts.yaml    # System prompts and templates
├── chains.yaml     # Task execution chains and workflows
└── README.md       # Policy documentation
```

**Benefits:**
- ✅ Add providers without code changes
- ✅ Declarative routing configuration
- ✅ System prompts in YAML (easier to customize)
- ✅ Task chains and workflows
- ✅ Git-friendly configuration
- ✅ Per-environment customization
- ✅ Validation and cross-reference checking

## Architecture

### Policy Loader

**`src/config/policy/loader.go`** loads all policy files:

```go
// Load all policy files
config, err := policy.LoadConfig("./config/policy/")
if err != nil {
    log.Fatal(err)
}

// Access policy data
role := config.GetRole("security-analyst")
provider := config.GetProvider("openai")
task := config.GetTask("analyze-security-event")
chain := config.GetChain("security-event-full-analysis")
```

### Validation

**`src/config/policy/validation.go`** validates cross-references:

```go
// Validate policy configuration
errors := policy.ValidateConfig(config)
if len(errors) > 0 {
    for _, err := range errors {
        log.Error(err)
    }
    os.Exit(1)
}
```

**Validations:**
- ✅ Roles exist in `roles.yaml`
- ✅ Tasks reference valid roles
- ✅ Chains reference valid tasks
- ✅ Roles reference valid providers
- ✅ Routing references valid providers
- ✅ Circular dependency detection in chains
- ✅ Model availability checks
- ✅ Cost constraint validation

### Provider Factory

**`src/config/policy/provider_factory.go`** creates providers from policy:

```go
// Build registry from policy
factory := policy.NewConfiguredProviderFactory(policyConfig)
registry, err := factory.BuildRegistry(apiKeys, enabledProviders)
if err != nil {
    log.Fatal(err)
}

// Get provider for task
provider := factory.GetProviderForTask("analyze-security-event")
```

## Migration Steps

### 1. Review Default Policy

```bash
# Review default configuration
ls -la config/policy/
cat config/policy/roles.yaml
cat config/policy/routing.yaml
```

### 2. Customize for Your Environment

#### Change Default Provider

Edit `config/policy/routing.yaml`:
```yaml
routing:
  default_strategy: "highest_quality"  # or fastest, lowest_cost, smart
```

#### Add Custom Role

Edit `config/policy/roles.yaml`:
```yaml
roles:
  - name: my-custom-role
    description: "My custom security analyst"
    capabilities:
      - analyze-events
    allowed_providers: [openai]
    default_provider: openai
    system_prompt_override: |
      You are my custom security analyst...
```

#### Define Custom Task

Edit `config/policy/tasks.yaml`:
```yaml
tasks:
  - name: my-custom-task
    class: custom-analysis
    required_role: my-custom-role
    timeout_seconds: 60
    output_schema:
      type: object
      properties:
        result: string
```

#### Create Task Chain

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

### 3. Validate Policy

```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config/policy/roles.yaml'))"

# Check for validation errors
grep "Policy validation error" logs/zen-brain.log
```

### 4. Deploy with Policy

```bash
# Build zen-brain
go build -o zen-brain ./cmd

# Run with default policy
./zen-brain

# Run with custom policy directory
POLICY_CONFIG_DIR=/custom/policy ./zen-brain
```

## Reference: Policy Files

### roles.yaml

**Purpose:** Define AI agent roles with capabilities and constraints

**Key Concepts:**
- `capabilities`: What tasks this role can execute
- `allowed_providers`: Which providers this role can use
- `default_provider`: Fallback provider for this role
- `system_prompt_override`: Custom system prompt

### tasks.yaml

**Purpose:** Define task classes and their requirements

**Key Concepts:**
- `class`: Task category (event-analysis, code-review, documentation, etc.)
- `required_role`: Which role executes this task
- `timeout_seconds`: Maximum allowed execution time
- `output_schema`: Expected output structure

### providers.yaml

**Purpose:** Define available AI providers and their capabilities

**Key Concepts:**
- `enabled`: Whether provider is available
- `provider_type`: `managed` (service keys) or `byok` (customer keys)
- `models`: List of available models with pricing and capabilities

### routing.yaml

**Purpose:** Define how requests are routed to providers

**Key Concepts:**
- `default_strategy`: Global routing strategy (fastest, lowest_cost, highest_quality, smart)
- `task_routing`: Routing rules per task class
- `fallback_chain`: Order of providers to try on failure
- `arbitration`: Multi-provider consensus strategy

### prompts.yaml

**Purpose:** Define system prompts and templates

**Key Concepts:**
- `role`: Which role this prompt applies to
- `template`: The prompt template with variables
- `task_overrides`: Task-specific prompt modifications

### chains.yaml

**Purpose:** Define task execution chains and workflows

**Key Concepts:**
- `tasks`: List of tasks in execution order
- `depends_on`: Task dependencies
- `parallel_with`: Execute tasks in parallel
- `output_aggregation`: How to merge task outputs

## Benefits of Policy-Based Configuration

### 1. Maintainability

**Before:**
- Edit Go code to add provider
- Recompile and redeploy
- Risk of breaking changes

**After:**
- Add provider YAML entry
- Hot-reload or restart only
- Safer configuration changes

### 2. Flexibility

**Before:**
- Same routing for all environments
- Hardcoded system prompts
- No task chains

**After:**
- Per-environment policy files
- Custom prompts per role
- Declarative task workflows

### 3. Observability

**Before:**
- No visibility into routing decisions
- Hard to debug provider selection

**After:**
- All policy decisions logged
- Traceable routing paths
- Metrics on provider usage

### 4. BYOK Support

**Before:**
- Customer keys require code changes
- Limited BYOK functionality

**After:**
- Customer keys work seamlessly
- Fallback to managed keys
- Per-tenant usage tracking

### 5. Multi-Provider Arbitration

**Before:**
- No multi-provider support
- Single provider per request

**After:**
- Configure consensus strategies
- Customizable fallback chains
- Cost-aware routing

## Troubleshooting

### "Failed to load policy files"

```bash
# Check file permissions
ls -la config/policy/

# Check YAML syntax
for file in config/policy/*.yaml; do
    python3 -c "import yaml; yaml.safe_load(open('$file'))"
done
```

### "Policy validation failed"

```bash
# Check logs
grep "Policy validation error" logs/zen-brain.log

# Check specific errors
grep "circular dependencies" logs/zen-brain.log
grep "unknown provider" logs/zen-brain.log
grep "unknown role" logs/zen-brain.log
```

### "Provider not available"

```bash
# Check if provider is enabled
grep "name: openai" config/policy/providers.yaml
grep "enabled: true" config/policy/providers.yaml | grep -A1 openai

# Check if API key is set
echo $OPENAI_API_KEY | cut -c1-10
```

### "Circular dependency detected"

```yaml
# BAD: Circular reference
chains:
  - name: bad-chain
    tasks:
      - name: task-a
        depends_on: [task-b]
      - name: task-b
        depends_on: [task-a]

# GOOD: Linear or parallel execution
chains:
  - name: good-chain
    tasks:
      - name: task-a
        depends_on: []  # No dependencies
      - name: task-b
        depends_on: [task-a]  # Single direction
```

## Examples

### Example 1: Use OpenAI for Code Review

```yaml
# config/policy/roles.yaml
roles:
  - name: code-reviewer
    default_provider: openai  # Override default
    allowed_providers: [openai, deepseek]
```

### Example 2: Cheapest Routing

```yaml
# config/policy/routing.yaml
routing:
  default_strategy: "lowest_cost"
  fallback_chain:
    - deepseek    # Cheapest
    - openai      # Fallback
    - anthropic    # Last resort
```

### Example 3: Custom Task Chain

```yaml
# config/policy/chains.yaml
chains:
  - name: security-event-full-analysis
    description: "Complete analysis workflow"
    tasks:
      - name: analyze-event
        task_class: event-analysis
        role: security-analyst
      - name: threat-intel
        task_class: intelligence
        role: security-analyst
        depends_on: [analyze-event]
      - name: correlate
        task_class: event-correlation
        role: security-analyst
        depends_on: [analyze-event, threat-intel]
```

## Migration Checklist

- [x] Create `config/policy/` directory
- [x] Create `roles.yaml`
- [x] Create `tasks.yaml`
- [x] Create `providers.yaml`
- [x] Create `routing.yaml`
- [x] Create `prompts.yaml`
- [x] Create `chains.yaml`
- [x] Create `config/policy/README.md`
- [x] Create `src/config/policy/loader.go`
- [x] Create `src/config/policy/validation.go`
- [x] Create `src/config/policy/provider_factory.go`
- [x] Update `src/config/validation.go`
- [x] Create `deploy/README.md`
- [x] Create this migration guide
- [ ] Update zen-brain README.md to reference policy config
- [ ] Update BYOK documentation to reference policy config
- [ ] Add policy hot-reload support
- [ ] Add policy metrics and observability

## Next Steps

1. **Review Default Policy** - Check if default configuration fits your needs
2. **Customize Policy** - Add providers, roles, tasks, chains as needed
3. **Validate Configuration** - Check YAML syntax and cross-references
4. **Deploy with Policy** - Use environment variables or ConfigMap to mount policy files
5. **Monitor Policy Decisions** - Check logs for routing decisions and provider selection
6. **Iterate** - Adjust policy based on usage and cost metrics

## Further Reading

- **`config/policy/README.md`** - Complete policy documentation
- **`deploy/README.md`** - Deployment and configuration guide
- **API Documentation:** `/docs/api/` directory
- **BYOK Guide:** `/docs/byok/` directory
