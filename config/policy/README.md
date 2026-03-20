# Policy Configuration System

**Version:** 1.0
**Status:** ✅ Production-ready

## Overview

The zen-brain policy system provides declarative, file-based configuration for:

- **Roles** - Agent roles with system prompts and execution constraints
- **Tasks** - Task classes and their requirements
- **Providers** - AI provider definitions and capabilities
- **Routing** - Request routing and model selection policies
- **Prompts** - System prompts and templates for different contexts
- **Chains** - Task execution chains and workflows

This replaces hardcoded configuration with a flexible, maintainable YAML-based system.

## 🚨 CRITICAL: Local CPU Inference Policy (ZB-023)

**UNTIL EXPLICITLY OVERRIDDEN BY THE OPERATOR:**

### Certified Local CPU Path

- ✅ **ONLY allowed local model:** `qwen3.5:0.8b`
- ✅ **ONLY supported local inference path:** Host Docker Ollama (http://host.k3d.internal:11434)
- ❌ **FORBIDDEN:** In-cluster Ollama for active local CPU path
- ❌ **FORBIDDEN:** Any other local model (e.g., qwen3.5:14b, llama*, mistral*)

### Provider/Model Flexibility

- Any provider/model may serve any role if configured
- The outdated "planner=GLM, worker=0.8b" split is **REMOVED**
- `qwen3.5:0.8b` is NOT worker-only by architecture
- GLM is NOT planner-only by architecture
- **However:** The ONLY certified LOCAL CPU lane is `qwen3.5:0.8b` via host Docker Ollama

### Enforcement (FAIL-CLOSED)

Policy enforces the following:

1. **Local model restriction:** `providers.yaml` sets `fail_if_other_model_requested: true`
2. **In-cluster Ollama prohibition:** `routing.yaml` sets `forbid_in_cluster_ollama: true`
3. **Timeout defaults:** Local CPU path defaults to 300s timeout (generous for CPU inference)
4. **Thinking default:** Local CPU path defaults to `thinking=false` (no chain-of-thought)

### How to Override (NOT RECOMMENDED)

To use a different local model or in-cluster Ollama, you must:

1. Get **EXPLICIT OPERATOR APPROVAL** (no casual switching)
2. Update ALL three layers: policy, code, and documentation
3. Add CI gate exceptions (see `scripts/ci/` gates)
4. Document the change with a clear justification

### Verification Commands

```bash
# 1. Check OLLAMA_BASE_URL points to host Docker (NOT in-cluster)
kubectl exec -n zen-brain deploy/apiserver -- env | grep OLLAMA_BASE_URL
# Expected: OLLAMA_BASE_URL=http://host.k3d.internal:11434

# 2. Check local-worker lane is using host Docker Ollama with qwen3.5:0.8b
kubectl logs -n zen-brain deploy/apiserver | grep -E 'local-worker lane|Ollama warmup'
# Expected: [LLM Gateway] local-worker lane: Ollama at http://host.k3d.internal:11434 (model=qwen3.5:0.8b)

# 3. Verify host Docker Ollama has the 0.8b model
kubectl exec -n zen-brain deploy/apiserver -- wget -qO- http://host.k3d.internal:11434/api/tags
# Expected: JSON with "qwen3.5:0.8b" in models list
```

### See Also

- `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` - Detailed operations guide
- `deploy/README.md` - Deployment instructions with local model verification
- `scripts/ci/` - CI gates that enforce this policy automatically

---

## Policy Files

| File | Purpose | Key Features |
|-------|-----------|----------------|
| `roles.yaml` | Define AI agent roles and capabilities | System prompts, max tokens, allowed providers |
| `tasks.yaml` | Define task classes and requirements | Timeouts, output schemas, task priorities |
| `providers.yaml` | Define AI providers and models | Cost, rate limits, model capabilities |
| `routing.yaml` | Define request routing and model selection | Provider fallback, arbitration strategies |
| `prompts.yaml` | Define system prompts and templates | Role-specific prompts, task overrides |
| `chains.yaml` | Define task chains and workflows | Dependencies, parallel execution, aggregation |

## Quick Start

### 1. Default Configuration

The policy files work out of the box with sensible defaults:

```bash
# All policy files are in config/policy/
cd zen-brain
ls config/policy/
# roles.yaml
# tasks.yaml
# providers.yaml
# routing.yaml
# prompts.yaml
# chains.yaml

# Policy files are auto-loaded on startup
# No environment variables needed for default behavior
```

### 2. Customize for Your Environment

**Example: Change default provider to OpenAI**

Edit `config/policy/routing.yaml`:
```yaml
routing:
  default_strategy: "highest_quality"  # Prefer quality over cost
```

Or edit `config/policy/roles.yaml`:
```yaml
roles:
  - name: security-analyst
    default_provider: openai  # Override for this role
```

### 3. Provider Selection

**Example: Use BYOK (Bring Your Own Key) with customer keys**

No policy changes needed! BYOK is automatically used when customer keys are registered:

```bash
# Register customer key via API
curl -X POST http://localhost:8080/ai/v1/byok/keys \
  -H "Content-Type: application/json" \
  -d '{
    "provider_name": "openai",
    "api_key": "sk-...",
    "tenant_id": "...",
    "expires_at": "2024-12-31T23:59:59Z"
  }'

# zen-brain will automatically use customer key when available
# Falls back to managed keys if customer key is missing/revoked
```

**Example: Disable a provider**

Edit `config/policy/providers.yaml`:
```yaml
providers:
  - name: anthropic
    enabled: false  # Disable Anthropic
```

### 4. Routing Strategy

**Example: Use fastest provider for all requests**

Edit `config/policy/routing.yaml`:
```yaml
routing:
  default_strategy: "fastest"  # Always prioritize speed
```

**Example: Use lowest cost with custom fallback**

```yaml
routing:
  default_strategy: "lowest_cost"
  fallback_chain:
    - deepseek    # Cheapest
    - openai      # Fallback
    - anthropic    # Last resort
```

### 5. Task Chain Configuration

**Example: Add custom task chain**

Edit `config/policy/chains.yaml`:
```yaml
chains:
  - name: custom-security-review
    description: "My custom security review workflow"
    tasks:
      - name: analyze-event
        task_class: event-analysis
        role: security-analyst
        timeout_seconds: 30
      - name: document-findings
        task_class: documentation
        role: documenter
        depends_on:
          - analyze-event
```

Then use the chain via API:
```bash
curl -X POST http://localhost:8080/ai/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "chain": "custom-security-review",
    "events": [...]
  }'
```

## Reference: Policy Files

### roles.yaml

**Purpose:** Define AI agent roles with capabilities and constraints

**Key Concepts:**
- `capabilities`: What tasks this role can execute
- `allowed_providers`: Which providers this role can use
- `default_provider`: Fallback provider for this role
- `max_tokens_per_request`: Safety limit for this role
- `system_prompt_override`: Custom system prompt override

**Example:**
```yaml
roles:
  - name: security-analyst
    description: "Analyzes security events and provides remediation"
    capabilities:
      - analyze-events
      - threat-intelligence
    allowed_providers: [deepseek, openai]
    default_provider: deepseek
    system_prompt_override: |
      You are a Senior Security Analyst...
```

### tasks.yaml

**Purpose:** Define task classes and their requirements

**Key Concepts:**
- `class`: Task category (event-analysis, code-review, documentation, etc.)
- `required_role`: Which role executes this task
- `timeout_seconds`: Maximum allowed execution time
- `output_schema`: Expected output structure
- `priority`: Task priority class (P0, P1, P2)

**Example:**
```yaml
tasks:
  - name: analyze-security-event
    class: event-analysis
    required_role: security-analyst
    timeout_seconds: 60
    output_schema:
      type: object
      properties:
        severity_adjusted: string
        confidence_score: number
```

### providers.yaml

**Purpose:** Define available AI providers and their capabilities

**Key Concepts:**
- `enabled`: Whether provider is available
- `provider_type`: `managed` (service keys) or `byok` (customer keys supported)
- `models`: List of available models with pricing and capabilities
- `rate_limit_*`: Rate limits (requests per minute/day)
- `cost_per_1m_*`: Cost per 1M tokens (input/output)

**Example:**
```yaml
providers:
  - name: deepseek
    enabled: true
    provider_type: managed|byok
    models:
      - name: deepseek-chat
        cost_per_1m_input_tokens: 0.14
        rate_limit_rpm: 60
```

### routing.yaml

**Purpose:** Define how requests are routed to providers

**Key Concepts:**
- `default_strategy`: Global routing strategy (fastest, lowest_cost, highest_quality, smart)
- `task_routing`: Routing rules per task class
- `fallback_chain`: Order of providers to try on failure
- `arbitration`: Multi-provider consensus strategy

**Routing Strategies:**
- `fastest`: Select provider with lowest response time
- `lowest_cost`: Select cheapest provider
- `highest_quality`: Select best quality (usually OpenAI or Anthropic)
- `smart`: Balance cost, speed, and quality

**Example:**
```yaml
routing:
  default_strategy: "smart"
  task_routing:
    - task_class: event-analysis
      strategy: "lowest_cost"
      preferred_providers: [deepseek]
  fallback_chain:
    - deepseek
    - openai
    - anthropic
```

### prompts.yaml

**Purpose:** Define system prompts and templates

**Key Concepts:**
- `role`: Which role this prompt applies to
- `template`: The prompt template with variables
- `task_overrides`: Task-specific prompt modifications

**Example:**
```yaml
prompts:
  - role: security-analyst
    name: default-analysis-prompt
    template: |
      You are a Senior Security Analyst...
      Provide clear, actionable recommendations...
```

### chains.yaml

**Purpose:** Define task execution chains with dependencies

**Key Concepts:**
- `tasks`: List of tasks in execution order
- `depends_on`: Task dependencies
- `parallel_with`: Execute tasks in parallel
- `output_aggregation`: How to merge task outputs

**Example:**
```yaml
chains:
  - name: security-event-full-analysis
    tasks:
      - name: analyze-event
        task_class: event-analysis
      - name: threat-intelligence
        task_class: intelligence
        depends_on:
          - analyze-event
      - name: correlate-events
        parallel_with: threat-intelligence
```

## Benefits of Policy-Based Configuration

### 1. Maintainability
- No need to recompile Go code to change configuration
- Easy to review changes in YAML files
- Git-friendly configuration management

### 2. Flexibility
- Add new providers without code changes
- Define custom task chains without deployment
- Override prompts per tenant or role

### 3. Observability
- All policy decisions are logged
- Easy to trace which policy rule was applied
- Metrics on routing decisions and provider usage

### 4. BYOK Support
- Customer-provided API keys work seamlessly
- Fallback to managed keys when needed
- Per-tenant usage tracking

### 5. Multi-Provider Arbitration
- Configure consensus strategies
- Customizable fallback chains
- Cost-aware routing

## Migration from Old Configuration

### Before (Hardcoded)
```go
// Hardcoded in Go code
defaultProvider := "deepseek"
providers := map[string]Provider{
    "openai": NewOpenAIProvider(apiKey),
    "anthropic": NewAnthropicProvider(apiKey),
}
```

### After (Policy-Based)
```yaml
# In config/policy/providers.yaml
providers:
  - name: deepseek
    enabled: true
    models:
      - name: deepseek-chat
        cost_per_1m_input_tokens: 0.14

  - name: openai
    enabled: true
    models:
      - name: gpt-4o-mini
        cost_per_1m_input_tokens: 0.15
```

**Benefits:**
- Add new provider = add YAML entry (no code)
- Change routing = edit routing.yaml (no redeploy)
- Customize prompts = edit prompts.yaml (no code)

## Qwen3.5 Restrictions

**Note:** zen-brain does NOT use Qwen3.5 models. Qwen3.5:0.8b restrictions are NOT applicable to this service.

**Supported Models:**
- DeepSeek V2.5 (chat and reasoner)
- OpenAI GPT-4o and GPT-4o-mini
- Anthropic Claude 3 (Haiku, Sonnet, Opus)

## Cross-Reference Validation

The policy loader validates cross-references between files:

**Validations:**
- ✅ All roles referenced in `tasks.yaml` exist in `roles.yaml`
- ✅ All providers referenced in `roles.yaml` exist in `providers.yaml`
- ✅ All tasks referenced in `chains.yaml` exist in `tasks.yaml`
- ✅ All chains reference valid tasks and roles
- ✅ Circular dependency detection in chains

**Errors:**
- If validation fails, zen-brain will log error and NOT start
- Check logs: `logs/policy-validation.log`
- Fix validation errors before deploying

## Examples and Templates

### Example: Use Specific Provider

```bash
# Force using OpenAI for this request
curl -X POST http://localhost:8080/ai/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "events": [...],
    "provider": "openai"
  }'
```

### Example: Use Custom Chain

```bash
# Execute full analysis chain
curl -X POST http://localhost:8080/ai/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "chain": "security-event-full-analysis",
    "events": [...]
  }'
```

### Example: BYOK Usage

```bash
# Register customer OpenAI key
curl -X POST http://localhost:8080/ai/v1/byok/keys \
  -H "Content-Type: application/json" \
  -d '{
    "provider_name": "openai",
    "api_key": "sk-...",
    "tenant_id": "...",
    "expires_at": "2024-12-31"
  }'

# zen-brain automatically uses customer key
curl -X POST http://localhost:8080/ai/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "events": [...]
  }'
```

### Example: Override Provider for Role

```yaml
# In config/policy/roles.yaml
roles:
  - name: security-analyst
    default_provider: openai  # Override default
    allowed_providers: [openai, deepseek]
```

## Troubleshooting

### "Failed to load policy files"

```bash
# Check file permissions
ls -la config/policy/
# Should be readable by zen-brain process

# Check YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config/policy/roles.yaml'))"
```

### "Validation error: Role not found"

```bash
# Check if role exists
grep "name: my-role" config/policy/roles.yaml

# Check if task references it
grep "required_role: my-role" config/policy/tasks.yaml
```

### "Provider not available"

```bash
# Check if provider is enabled
grep "name: openai" config/policy/providers.yaml
grep "enabled: true" config/policy/providers.yaml | grep -A1 openai
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

## Further Reading

- **README.md** - Overall zen-brain documentation
- **deploy/README.md** - Deployment and configuration
- **API documentation** - `/docs/` directory (if available)
