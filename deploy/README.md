> **NOTE:** References to Ollama in this file describe the L0 fallback lane. The primary inference runtime is **llama.cpp** (L1/L2). Ollama in-cluster deployment is disabled by default.

# zen-brain Deployment Guide

**Version:** 1.0
**Last Updated:** 2026-03-20

## Overview

This guide covers deploying zen-brain in production with policy-based configuration.

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

### Verification Commands (Post-Deployment)

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

# 4. Verify in-cluster Ollama is NOT running
kubectl get pods -n zen-brain | grep ollama
# Expected: No ollama pods (in-cluster Ollama disabled)
```

### See Also

- `../docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md` - Detailed operations guide
- `../config/policy/README.md` - Policy system documentation with local model rules

---

## 📚 Documentation Quick Reference

| Topic | Document | Purpose |
|--------|----------|---------|
| **Canonical Policy** | [SMALL_MODEL_STRATEGY.md](../docs/03-DESIGN/SMALL_MODEL_STRATEGY.md) | Local CPU inference policy (qwen3.5:0.8b ONLY) |
| **Escalation ladder (design)** | [LOCAL_LLM_ESCALATION_LADDER.md](../docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md) | 0.8B workhorse → optional local 2B → external models; subtask checkpoints & retries |
| **2B local evaluation** | [QWEN_2B_LOCAL_EVALUATION.md](../docs/05-OPERATIONS/QWEN_2B_LOCAL_EVALUATION.md) | llama.cpp 0.8B vs 2B Q4_K_M throughput/RAM (ops sizing) |
| **Operational Guide** | [OLLAMA_08B_OPERATIONS_GUIDE.md](../docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md) | Operations for local Ollama (host Docker) |
| **Warmup Runbook** | [OLLAMA_WARMUP_RUNBOOK.md](../docs/05-OPERATIONS/OLLAMA_WARMUP_RUNBOOK.md) | Warmup/keepalive procedures |
| **Operator Runbook** | [ZB_023_LOCAL_CPU_INFERENCE_RULE.md](../docs/05-OPERATIONS/ZB_023_LOCAL_CPU_INFERENCE_RULE.md) | Troubleshooting and verification commands |
| **Policy System** | [config/policy/README.md](../config/policy/README.md) | YAML-based policy configuration |

**Documentation Hierarchy:**
1. **SMALL_MODEL_STRATEGY.md** = Canonical policy/source of truth
2. **OLLAMA_08B_OPERATIONS_GUIDE.md** = Operational implementation guide
3. **OLLAMA_WARMUP_RUNBOOK.md** = Warmup/keepalive runbook only
4. **ZB_023_LOCAL_CPU_INFERENCE_RULE.md** = Operator runbook (troubleshooting, verification)
5. In-cluster Ollama docs/charts = **Legacy/unsupported** for active local CPU path

---

## Configuration

### Policy-Based Configuration (NEW)

zen-brain now uses YAML-based policy configuration instead of hardcoded values:

- **`config/policy/roles.yaml`** - AI agent roles and capabilities
- **`config/policy/tasks.yaml`** - Task classes and requirements
- **`config/policy/providers.yaml`** - AI provider definitions
- **`config/policy/routing.yaml`** - Request routing and model selection
- **`config/policy/prompts.yaml`** - System prompts and templates
- **`config/policy/chains.yaml`** - Task execution chains and workflows

See **`config/policy/README.md`** for full policy documentation.

### Environment Variables

#### Required

| Variable | Description | Default |
|----------|-------------|---------|
| `AI_DEFAULT_PROVIDER` | Default AI provider | `deepseek` |
| `POLICY_CONFIG_DIR` | Path to policy config directory | `./config/policy/` |

#### Provider API Keys (at least one required)

| Variable | Provider |
|----------|----------|
| `OPENAI_API_KEY` | OpenAI (GPT-4) |
| `ANTHROPIC_API_KEY` | Anthropic (Claude) |
| `DEEPSEEK_API_KEY` | DeepSeek |

#### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8081` | HTTP port |
| `DATABASE_URL` | - | CockroachDB connection string |
| `REDIS_URL` | - | Redis for caching and budget enforcement |
| `AI_BUDGETS_ENABLED` | `true` | Enable cost tracking |
| `MAX_DAILY_COST_CENTS` | `10000` | Daily budget cap ($100) |
| `AI_CACHE_ENABLED` | `true` | Enable multi-tier caching |
| `AI_CACHE_ROUTING_STRATEGY` | `smart` | Cache routing (fastest/smart/semantic_first) |
| `AI_EMBEDDING_PROVIDER` | `local` | Embedding provider (openai/local) |
| `AI_EMBEDDING_MODEL` | `text-embedding-ada-002` | OpenAI embedding model |
| `AI_ARBITRATION_STRATEGY` | `first_success` | Arbitration strategy |
| `TLS_ENABLED` | `false` | Enable HTTPS |

## Deployment

### Docker

```bash
# Build image
docker build -t kubezen/zen-brain:latest .

# Run with default policy
docker run -p 8081:8081 \
  -e OPENAI_API_KEY=sk-... \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  -e DEEPSEEK_API_KEY=sk-... \
  -v ./config/policy:/home/zenuser/config/policy \
  kubezen/zen-brain:latest

# Run with custom policy directory
docker run -p 8081:8081 \
  -e OPENAI_API_KEY=sk-... \
  -e POLICY_CONFIG_DIR=/custom/policy \
  -v ./custom-policy:/custom/policy \
  kubezen/zen-brain:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zen-brain
  namespace: zen-brain
spec:
  replicas: 2
  selector:
    matchLabels:
      app: zen-brain
  template:
    metadata:
      labels:
        app: zen-brain
    spec:
      containers:
      - name: zen-brain
        image: kubezen/zen-brain:latest
        ports:
        - containerPort: 8081
        env:
        # Default provider
        - name: AI_DEFAULT_PROVIDER
          value: "deepseek"
        # Provider API keys
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-secrets
              key: openai-key
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-secrets
              key: anthropic-key
        - name: DEEPSEEK_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-secrets
              key: deepseek-key
        # Policy config
        - name: POLICY_CONFIG_DIR
          value: "/home/zenuser/config/policy"
        # Optional: Redis
        - name: REDIS_URL
          value: "redis://zen-redis-master:6379"
        # Optional: Database
        - name: DATABASE_URL
          value: "postgresql://root:zen-crdb-public.zen-data.svc.cluster.local:26257/defaultdb?sslmode=disable"
        # Budget enforcement
        - name: AI_BUDGETS_ENABLED
          value: "true"
        - name: MAX_DAILY_COST_CENTS
          value: "10000"
        # Caching
        - name: AI_CACHE_ENABLED
          value: "true"
        - name: AI_CACHE_ROUTING_STRATEGY
          value: "smart"
        volumeMounts:
        - name: policy-config
          mountPath: /home/zenuser/config/policy
          readOnly: true
      volumes:
      - name: policy-config
        configMap:
          name: zen-brain-policy
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: zen-brain-policy
  namespace: zen-brain
data:
  # Policy files will be loaded from this ConfigMap
  # Alternatively, use git-sync to pull policy from repo
  roles.yaml: |
    # See config/policy/roles.yaml for full content
  tasks.yaml: |
    # See config/policy/tasks.yaml for full content
  providers.yaml: |
    # See config/policy/providers.yaml for full content
  routing.yaml: |
    # See config/policy/routing.yaml for full content
  prompts.yaml: |
    # See config/policy/prompts.yaml for full content
  chains.yaml: |
    # See config/policy/chains.yaml for full content
```

### Helm

```bash
# Add zen-brain Helm repo
helm repo add kubezen https://charts.kubezen.io

# Deploy with default values
helm install zen-brain kubezen/zen-brain \
  --namespace zen-brain \
  --create-namespace \
  --set ai.defaultProvider=deepseek \
  --set ai.openai.apiKeySecret=ai-secrets \
  --set ai.anthropic.apiKeySecret=ai-secrets \
  --set ai.deepseek.apiKeySecret=ai-secrets

# Deploy with custom policy values
helm install zen-brain kubezen/zen-brain \
  --namespace zen-brain \
  --create-namespace \
  --set-file policy.config=values/policy.yaml \
  --set ai.defaultProvider=openai
```

## Policy Configuration

### Quick Start

```bash
# 1. Copy default policy configuration
cp -r config/policy/ custom-policy/

# 2. Edit policy files
vim custom-policy/roles.yaml
vim custom-policy/routing.yaml

# 3. Deploy with custom policy
docker run -p 8081:8081 \
  -e OPENAI_API_KEY=sk-... \
  -v ./custom-policy:/home/zenuser/config/policy \
  kubezen/zen-brain:latest
```

### Changing Default Provider

Edit `config/policy/routing.yaml`:
```yaml
routing:
  default_strategy: "highest_quality"  # or fastest, lowest_cost, smart
```

### Adding a New Provider

Edit `config/policy/providers.yaml`:
```yaml
providers:
  - name: new-provider
    display_name: "New Provider"
    class: language-model
    enabled: true
    provider_type: managed|byok
    api_endpoint: https://api.new-provider.com
    models:
      - name: new-model
        display_name: "New Model"
        class: chat
        context_window: 128000
        max_output_tokens: 4000
        cost_per_1m_input_tokens: 1.0
        cost_per_1m_output_tokens: 2.0
        supports_streaming: true
        supports_functions: true
        supports_json_mode: true
        max_batch_size: 1
        rate_limit_rpm: 60
        rate_limit_rpd: 100000
```

### Creating Custom Task Chain

Edit `config/policy/chains.yaml`:
```yaml
chains:
  - name: custom-analysis
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

See **`config/policy/README.md`** for complete policy documentation.

## Monitoring

### Metrics

zen-brain exposes Prometheus metrics at `/metrics`:

**AI Provider Metrics:**
- `ai_requests_total` - Request count by provider/status
- `ai_tokens_used` - Token consumption
- `ai_cost_cents` - Cost tracking
- `ai_request_duration_seconds` - Latency
- `ai_arbitration_strategy_total` - Arbitration strategy usage

**Cache Metrics:**
- `ai_cache_hits_total` - Cache hits by tier
- `ai_cache_misses_total` - Cache misses by tier
- `ai_cache_router_strategy_total` - Router strategy usage

**Policy Metrics:**
- `policy_provider_selection_total` - Provider selection by strategy
- `policy_chain_execution_total` - Chain execution by name
- `policy_validation_errors_total` - Policy validation errors

**BYOK Metrics:**
- `ai_byok_usage_total` - BYOK key usage
- `ai_byok_cost_cents` - BYOK cost tracking

### Health Checks

```bash
# Liveness
curl http://localhost:8081/health

# Readiness (includes provider status)
curl http://localhost:8081/healthz

# Provider status
curl http://localhost:8081/metrics | grep provider_status
```

### Logging

Logs include policy decision context:

```json
{
  "level": "info",
  "msg": "Request routed to provider",
  "op": "policy_routing",
  "provider": "deepseek",
  "strategy": "lowest_cost",
  "task": "analyze-security-event",
  "chain": "security-event-full-analysis"
}
```

## Troubleshooting

### "Failed to load policy files"

```bash
# Check policy directory exists
ls -la config/policy/

# Check file permissions
chmod 644 config/policy/*.yaml

# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config/policy/roles.yaml'))"
```

### "Policy validation failed"

```bash
# Check logs for specific validation errors
grep "Policy validation error" logs/zen-brain.log

# Check policy README
cat config/policy/README.md
```

### "Provider not available"

```bash
# Check if provider is enabled in policy
grep "enabled: true" config/policy/providers.yaml | grep -A1 openai

# Check if API key is set
echo $OPENAI_API_KEY | cut -c1-10

# Check provider registration
curl http://localhost:8081/metrics | grep provider_status
```

### "Routing always uses same provider"

```bash
# Check routing policy
grep "default_strategy" config/policy/routing.yaml

# Check fallback chain
grep "fallback_chain" config/policy/routing.yaml

# Check task routing
grep "task_routing" config/policy/routing.yaml
```

## Migration from Old Configuration

### Before (Hardcoded)

```go
// src/main.go
registry := buildRegistry()  // Hardcoded provider factory
defaultProvider := "deepseek"  // Environment variable only
```

### After (Policy-Based)

```yaml
# config/policy/providers.yaml
providers:
  - name: deepseek
    enabled: true
    models:
      - name: deepseek-chat
        cost_per_1m_input_tokens: 0.14

# config/policy/routing.yaml
routing:
  default_strategy: "lowest_cost"  # Configurable
  fallback_chain:
    - deepseek
    - openai
    - anthropic
```

**Benefits:**
- Add providers without code changes
- Change routing without redeployment
- Custom prompts per role
- Define task chains declaratively
- Git-friendly configuration

## Security

### BYOK (Bring Your Own Key)

Customer AI keys are stored securely:

1. **Envelope Encryption**: Age-based encryption with tenant KEK
2. **Secure Storage**: Production uses KMS/Vault for private keys
3. **Per-Tenant Isolation**: Keys scoped to tenant ID
4. **Rotation Support**: Automatic and manual key rotation
5. **Usage Tracking**: Per-tenant cost and token tracking

### Secret Management

**Required Secrets:**

```bash
# Create secrets namespace
kubectl create namespace zen-brain

# Create AI secrets
kubectl create secret generic ai-secrets \
  --from-literal=openai-key=sk-... \
  --from-literal=anthropic-key=sk-ant-... \
  --from-literal=deepseek-key=sk-... \
  -n zen-brain
```

**Best Practices:**
- Rotate API keys regularly
- Use separate keys for production/staging/dev
- Limit key permissions (read-only for analytics, full for operations)
- Monitor BYOK usage for cost management

## Performance

### Scaling

**Horizontal Scaling:**

```yaml
# Increase replicas for high load
spec:
  replicas: 5
```

**Cache Tuning:**

```yaml
env:
  - name: AI_CACHE_ENABLED
    value: "true"
  - name: AI_CACHE_ROUTING_STRATEGY
    value: "semantic_first"  # Better cache hit rate
  - name: AI_EMBEDDING_PROVIDER
    value: "openai"  # Better semantic similarity
```

**Budget Enforcement:**

```yaml
env:
  - name: AI_BUDGETS_ENABLED
    value: "true"
  - name: MAX_DAILY_COST_CENTS
    value: "50000"  # $500/day for production
  - name: REDIS_URL
    value: "redis://zen-redis-master:6379"
```

### Rate Limits

zen-brain respects provider rate limits defined in `config/policy/providers.yaml`:

```yaml
providers:
  - name: deepseek
    models:
      - name: deepseek-chat
        rate_limit_rpm: 60      # 60 requests/minute
        rate_limit_rpd: 100000  # 100K requests/day
```

## Further Reading

- **Policy Configuration:** `config/policy/README.md`
- **API Documentation:** `/docs/api/` directory
- **BYOK Guide:** `/docs/byok/` directory
- **Architecture:** `/docs/architecture/` directory
