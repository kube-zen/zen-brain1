> ⛔ **DEPRECATED:** This document references Ollama (L0), which is now FORBIDDEN for zen-brain1.
> The current primary inference runtime is **llama.cpp** (L1/L2). Ollama has been removed from all active paths.
> Retained for historical/architectural reference only.

> **NOTE:** This document references Ollama. The current primary inference runtime is **llama.cpp** (L1/L2 lanes). Ollama (L0) is fallback only. See `docs/03-DESIGN/LOCAL_LLM_ESCALATION_LADDER.md`.

# LLM Gateway Design

## Overview

The LLM Gateway is the provider‑agnostic interface for Large Language Model interactions in Zen‑Brain. It abstracts differences between LLM providers (OpenAI, Anthropic, local models, etc.) and provides intelligent routing based on cost, latency, capabilities, and project context.

**Key capabilities:**

- **Provider abstraction** – uniform interface for any LLM backend.
- **Intelligent routing** – automatically selects the best provider/model for each request.
- **Streaming support** – real‑time token streaming for responsive agents.
- **Tool calling** – structured function calling across providers.
- **Cost tracking** – integrates with ZenLedger for token/cost accounting.
- **Multi‑cluster aware** – routes requests to the appropriate cluster’s local models.

## Interface

The LLM Gateway defines three core interfaces in `pkg/llm/`:

### Provider

```go
type Provider interface {
    Name() string
    SupportsTools() bool
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) (*ChatResponse, error)
    Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}
```

### ProviderFactory

```go
type ProviderFactory interface {
    CreateProvider(name string) (Provider, error)
    CreateProviderWithModel(name, model string) (Provider, error)
    ListProviders() []string
}
```

### Router

```go
type Router interface {
    Route(ctx context.Context, req ChatRequest) (Provider, string, error)
    RouteForEmbedding(ctx context.Context, req EmbeddingRequest) (Provider, string, error)
}
```

## Data Structures

### ChatRequest

Contains all parameters for a chat completion:

- `Messages` – conversation history with roles (`user`, `assistant`, `system`, `tool`).
- `Tools` – available tools for function calling.
- `Model` – optional model override.
- `Temperature`, `MaxTokens`, `ContextLimit`.
- `ThinkingLevel` – `off`, `low`, `medium`, `high` for chain‑of‑thought reasoning.
- `Stream` – enable streaming.
- `ClusterID`, `ProjectID`, `SessionID`, `TaskID` – multi‑cluster and tracking context.

### ChatResponse

- `Content`, `ReasoningContent`, `FinishReason`, `ToolCalls`.
- `Model` – actual model used.
- `Usage` – token counts (input, output, cached).
- `LatencyMs` – request latency.

### EmbeddingRequest / EmbeddingResponse

For generating text embeddings (used by QMD and other vector‑based components).

## Provider Implementations

### Built‑in Providers

| Provider | Supports Tools | Supports Embedding | Notes |
|----------|----------------|-------------------|-------|
| **OpenAI** (`openai`) | Yes | Yes (text‑embedding‑3‑small) | GPT‑4, GPT‑3.5, etc. |
| **Anthropic** (`anthropic`) | Yes | No | Claude models |
| **Ollama** (`ollama`) | Yes | Yes (nomic‑embed‑text) | Local models (glm‑4.7, llama3, etc.) |
| **Google Gemini** (`gemini`) | Yes | Yes (text‑embedding‑004) | Via Google AI Studio |
| **Azure OpenAI** (`azure`) | Yes | Yes | Azure‑hosted OpenAI models |
| **Mock** (`mock`) | Yes | Yes | For testing |

### Provider Configuration

Each provider is configured via `config.yaml`:

```yaml
llm:
  providers:
    openai:
      enabled: true
      api_key: "${OPENAI_API_KEY}"
      default_model: "gpt‑4‑turbo‑preview"
      timeout_seconds: 30
    anthropic:
      enabled: true
      api_key: "${ANTHROPIC_API_KEY}"
      default_model: "claude‑sonnet‑4‑6"
      timeout_seconds: 30
    ollama:
      enabled: true
      base_url: "http://ollama‑service:11434"
      default_model: "glm‑4.7"
      timeout_seconds: 120
    mock:
      enabled: false
```

Providers are instantiated via the `ProviderFactory` on demand.

## Intelligent Routing

The `Router` selects the best provider based on:

1. **Capability matching** – does the provider support tools? embeddings? streaming?
2. **Cost optimization** – uses ZenLedger historical data to pick the most cost‑effective model.
3. **Latency requirements** – local models for low‑latency, high‑throughput tasks; cloud models for complex reasoning.
4. **Project budget** – stays within project budget limits.
5. **Cluster affinity** – prefers local models in the same cluster to reduce network latency.

**Routing decision process:**

1. Parse `ChatRequest` (or `EmbeddingRequest`).
2. Query ZenLedger for model efficiency data (`GetModelEfficiency`).
3. Apply routing policies (configurable per project).
4. Select provider/model.
5. Return provider and decision reason (for logging).

**Example routing policies:**

```yaml
llm:
  routing:
    default_strategy: "cost_optimized"  # cost_optimized | quality_optimized | balanced
    prefer_local: true
    fallback_to_api: true
    task_overrides:
      - task_type: "debug"
        preferred_models: ["glm‑4.7‑local", "claude‑sonnet‑4‑6‑api"]
        max_cost_usd: 0.30
      - task_type: "documentation"
        preferred_models: ["glm‑4.7‑local"]
        max_cost_usd: 0.10
```

## Token Usage and Cost Tracking

Every LLM call is recorded in ZenLedger for cost accounting:

1. Provider’s `Chat` or `Embed` method calls `TokenRecorder.Record` with a `TokenRecord`.
2. `TokenRecord` includes tokens, cost (estimated for local), latency, outcome.
3. ZenLedger aggregates records for model efficiency reports.

**Local inference cost estimation:** Uses the local cost model (CPU/GPU time + memory overhead) to assign a comparable dollar cost.

## Multi‑cluster Considerations

- Each cluster runs its own LLM Gateway instance.
- Local models (Ollama) are cluster‑local; API providers are global.
- Routing decisions consider cluster affinity: tasks originating in cluster A should use cluster A’s local models when possible.
- Cross‑cluster model invocation is allowed but adds network latency (recorded in `TokenRecord`).

## Streaming Support

Providers that support streaming implement `ChatStream`. The gateway handles:

- **Backpressure** – if the consumer cannot keep up, buffer tokens.
- **Error recovery** – if streaming fails, fall back to non‑streaming.
- **Interruption** – allow cancellation mid‑stream.

Streaming is used for real‑time agent responses and long‑form generation.

## Tool Calling

The LLM Gateway standardizes tool calling across providers:

- **Tool definition** – JSON Schema for parameters.
- **Tool execution** – tools are executed by the agent (not the gateway).
- **Tool results** – fed back into the conversation as `tool` role messages.

Providers that don’t support native tool calling (e.g., some local models) can use a **client‑side shim** that injects tool descriptions into the prompt and parses model output.

## Configuration

Example `config.yaml`:

```yaml
llm:
  gateway:
    # Provider configuration
    providers:
      openai: ...
      anthropic: ...
      ollama: ...

    # Routing
    routing:
      default_strategy: "cost_optimized"
      prefer_local: true
      fallback_to_api: true
      max_retries: 3
      retry_delay_ms: 1000

    # Token recording
    token_recorder:
      enabled: true
      batch_size: 10
      flush_interval_seconds: 5

    # Embedding model selection
    embedding:
      default_model: "nomic‑embed‑text"
      dimension: 768
      provider: "ollama"

    # Monitoring
    metrics:
      enabled: true
      latency_buckets: [0.1, 0.5, 1, 2, 5, 10]
```

## Monitoring

**Metrics (Prometheus):**

- `llm_gateway_requests_total` – total requests by provider and model.
- `llm_gateway_request_latency_seconds` – histogram of request latency.
- `llm_gateway_tokens_total` – cumulative tokens (input, output).
- `llm_gateway_routing_decisions_total` – routing decisions by reason.
- `llm_gateway_errors_total` – errors by provider and error type.

**Dashboards (Grafana):**

- Request rate and latency by provider/model.
- Token usage over time (input vs output).
- Routing decision breakdown (local vs API, cost‑optimized vs quality‑optimized).
- Error rate and retry rate.

## Integration Points

- **Worker Agents** – call `llm.Gateway.Chat()` for reasoning and tool calling.
- **KB Ingestion Service** – calls `llm.Gateway.Embed()` for generating document embeddings.
- **ZenLedger** – receives `TokenRecord`s for cost accounting.
- **ZenGate** – can enforce policies on LLM calls (e.g., max tokens per session).
- **ZenContext** – caches embeddings and frequently used prompts.

## Open Questions

1. **Should we support model‑specific prompt templates?** – Some models require specific formatting (e.g., ChatML). Could be handled in provider implementations.
2. **How to handle provider rate limits?** – Implement token bucket per provider; queue requests when limits exceeded.
3. **Should we cache embeddings?** – Yes, with TTL; embedding generation can be expensive.
4. **How to handle model deprecation/upgrades?** – Versioned model names; migration scripts for stored embeddings.

## MVP Implementation (Batch D)

**Current implementation location:** `internal/llm/`

**Key MVP components implemented:**

1. **`Gateway`** (`internal/llm/gateway.go`) - Unified implementation of `Provider`, `Router`, and `ProviderFactory` interfaces
2. **`LocalWorkerProvider`** (`internal/llm/local_worker.go`) - Local worker lane using small CPU-efficient models
3. **`PlannerProvider`** (`internal/llm/planner.go`) - Planner/escalation lane using more powerful models
4. **`OllamaProvider`** (`internal/llm/ollama_provider.go`) - Real Ollama integration with warmup support
5. **`fallback_chain`** (`internal/llm/routing/fallback_chain.go`) - Ordered provider attempts on retryable errors when fallback chain is enabled
6. **Configuration** - `GatewayConfig` with sensible defaults and routing policies

**Production Features:**

- **Dual-lane routing**: Local worker vs planner escalation based on task complexity
- **Real Ollama integration**: 100% success rate with Docker host networking
- **Warmup support**: OllamaWarmupCoordinator for cold-start optimization
- **Keep-alive**: OLLAMA_KEEP_ALIVE=-1 keeps models loaded
- **Health checks**: LiveHealthChecker for readiness probes
- **Retries / fallback**: When the fallback chain is enabled (typical), requests move to the next provider on retryable failures; otherwise zen-sdk-style retries may apply on a single provider path — see `gateway.go`. This is **not** Multi-Level Queue (MLQ) or subtask checkpoint replay; MLQ remains roadmap ([ROADMAP.md](../01-ARCHITECTURE/ROADMAP.md)).
- **Timeout handling**: Configurable per lane (local: 120s default)
- **Tool support**: Both lanes support tool calling
- **Statistics tracking**: Success rates, latencies, error counts

**Validated Performance (qwen3.5:0.8b on Docker):**

| Metric | Value |
|--------|-------|
| Latency (warm) | 8-57 seconds |
| Latency (cold) | 22-82 seconds |
| Throughput | ~12 tokens/sec |
| Success rate | 100% |
| Parallel workers | 20+ |
| Memory | 15GB limit |

**Integration ready:** The `Gateway` implements `llm.Provider` interface and can be injected anywhere a provider is needed (e.g., `analyzer.New(config, gateway, kbStore)`).

**Configuration example:**
```go
config := llm.DefaultGatewayConfig()
config.RoutingPolicy = "simple"
config.AutoEscalateComplexTasks = true
config.LocalWorkerModel = "qwen3.5:0.8b"
config.LocalWorkerTimeout = 120
config.LocalWorkerKeepAlive = "30m"
gateway, err := llm.NewGateway(config)
```

**Testing:** Comprehensive test suite (`internal/llm/gateway_test.go`) with tests covering routing, timeouts, tool support, and statistics. Real inference tests in `internal/integration/real_inference_test.go`.

## Next Steps (Post-MVP)

1. Connect to real provider backends (Ollama for local, OpenAI/Anthropic/GLM for cloud)
2. Integrate with ZenLedger for actual cost tracking
3. Implement more sophisticated routing algorithms
4. Add embedding support
5. Add real streaming support
6. Integrate with configuration management system

---

*This document is a living design spec; update as implementation progresses.*