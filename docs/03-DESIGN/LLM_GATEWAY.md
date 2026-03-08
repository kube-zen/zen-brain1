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

## Next Steps

1. Implement `internal/llm/openai.go`, `internal/llm/ollama.go`, etc.
2. Implement `internal/llm/router.go` with cost‑aware routing.
3. Integrate with ZenLedger for token recording.
4. Write unit and integration tests (mock provider).
5. Create provider health checks and fallback logic.

---

*This document is a living design spec; update as implementation progresses.*