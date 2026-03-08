# LLM Gateway - Batch D MVP

## Overview

The LLM Gateway provides intelligent routing between local worker and planner/escalation lanes for Large Language Model interactions in Zen‑Brain. This is the **Batch D MVP implementation** of the LLM Gateway design.

## Architecture

```
┌─────────────────┐
│   LLM Gateway   │ ← Implements Provider, Router, ProviderFactory
└─────────┬───────┘
          │ Routes based on:
          │ • Task complexity
          │ • Routing policy
          │ • Cost estimation
          │ • Tool requirements
          │
    ┌─────┴─────┐
    │           │
┌───▼───┐   ┌───▼───┐
│ Local │   │Planner│
│Worker │   │ Lane  │
│ Lane  │   │       │
└───────┘   └───────┘
qwen3.5:0.8b glm-4.7
(CPU-efficient) (Cloud/powerful)
```

## Key Features

### ✅ **MVP Complete**
- **Dual-lane routing**: Local worker vs planner escalation
- **Routing policies**: `simple` (default), `cost_aware`
- **Timeout handling**: Configurable per lane (30s local, 60s planner, 120s overall)
- **Tool support**: Both lanes support function calling
- **Statistics tracking**: Success rates, latencies, error counts
- **Complex task detection**: Heuristics based on message analysis
- **Context-aware**: Respects cancellation and timeouts
- **Comprehensive tests**: 16 passing unit tests

### 🔄 **Mock Providers (MVP)**
- **LocalWorkerProvider**: Simulates small CPU-efficient models (~50ms)
- **PlannerProvider**: Simulates powerful cloud models (~200ms) with reasoning
- **Ready for real backends**: Can be replaced with Ollama, OpenAI, etc.

## Usage

### Basic Example

```go
import (
    "context"
    "github.com/kube-zen/zen-brain1/internal/llm"
    "github.com/kube-zen/zen-brain1/pkg/llm"
)

// Create gateway with default config
config := llm.DefaultGatewayConfig()
config.RoutingPolicy = "simple"
config.AutoEscalateComplexTasks = true

gateway, err := llm.NewGateway(config)
if err != nil {
    log.Fatal(err)
}

// Use as llm.Provider (e.g., with analyzer)
req := llm.ChatRequest{
    Messages: []llm.Message{
        {Role: "user", Content: "Design a microservices architecture"},
    },
    TaskID: "complex-task-1",
}

resp, err := gateway.Chat(context.Background(), req)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Response from %s: %s\n", resp.Model, resp.Content)
```

### Configuration

```go
config := &llm.GatewayConfig{
    // Models
    LocalWorkerModel:   "qwen3.5:0.8b",  // Small local model
    PlannerModel:       "glm-4.7",       // Cloud model for complex tasks
    
    // Cost thresholds (USD)
    LocalWorkerMaxCost: 0.01,            // $0.01 max for local worker
    PlannerMinCost:     0.10,            // $0.10 min for planner
    
    // Timeouts (seconds)
    LocalWorkerTimeout: 30,
    PlannerTimeout:     60,
    RequestTimeout:     120,
    
    // Capabilities
    LocalWorkerSupportsTools: true,
    PlannerSupportsTools:     true,
    
    // Routing
    AutoEscalateComplexTasks: true,
    RoutingPolicy:            "simple", // "simple" or "cost_aware"
}
```

## Routing Logic

### Simple Policy (Default)
1. **Local worker** for simple, non-complex tasks without TaskID
2. **Planner** for complex tasks, planning requests, or tasks with TaskID
3. **Fallback** to planner if local worker unavailable

### Complex Task Detection
- **Message length**: >1000 characters
- **Keywords**: "plan", "design", "architecture", "analyze", "review", "strategy"
- **Task metadata**: Presence of TaskID or SessionID
- **Message count**: >5 messages in conversation

### Cost-Aware Policy (Basic MVP)
- Estimates token count and cost
- Routes to local worker if estimated cost < `LocalWorkerMaxCost`
- Routes to planner if estimated cost >= `PlannerMinCost`

## Integration Points

### With Analyzer
```go
// Gateway implements llm.Provider interface
gateway, _ := llm.NewGateway(config)
analyzer, _ := analyzer.New(analyzerConfig, gateway, kbStore)
```

### Statistics
```go
stats := gateway.GetStats()
fmt.Printf("Total requests: %d\n", stats.TotalRequests)
fmt.Printf("Local worker success: %d/%d\n", 
    stats.LocalWorkerSuccess, stats.LocalWorkerRequests)
fmt.Printf("Average latency: %dms\n", 
    stats.TotalLatencyMs / stats.TotalRequests)
```

## Testing

```bash
# Run all LLM Gateway tests
go test ./internal/llm/... -v

# Test coverage
go test ./internal/llm/... -cover
```

**Test coverage includes:**
- Gateway initialization and provider registration
- Routing decisions (simple vs complex tasks)
- Timeout handling and context cancellation
- Tool support matching
- Statistics tracking
- Stream fallback behavior
- Embedding not-supported behavior

## Design Decisions (MVP)

1. **Mock providers**: Real backend integration deferred for simplicity
2. **Simple cost estimation**: Basic token counting instead of real pricing
3. **No embeddings**: Focus on chat completion first
4. **Streaming fallback**: Falls back to regular chat (real streaming deferred)
5. **Basic routing**: Heuristics instead of ML-based prediction

## Next Steps (Post-MVP)

1. **Real providers**: Connect to Ollama, OpenAI, Anthropic, etc.
2. **Actual cost tracking**: Integrate with ZenLedger
3. **Advanced routing**: ML-based prediction, quality-of-service tiers
4. **Embedding support**: For QMD and vector search
5. **Real streaming**: Proper token-by-token streaming
6. **Configuration management**: YAML/JSON config files, env vars
7. **Health checks**: Provider availability monitoring
8. **Rate limiting**: Token bucket per provider

## Files

- `gateway.go` - Main Gateway implementation
- `local_worker.go` - Local worker lane provider
- `planner.go` - Planner/escalation lane provider  
- `gateway_test.go` - Comprehensive test suite
- **Design**: `../../docs/03-DESIGN/LLM_GATEWAY.md`

## Batch D Completion Status

✅ **MVP Requirements Met:**
- [x] Local worker lane implementation
- [x] Planner/escalation lane implementation  
- [x] Timeout / bounded request behavior
- [x] Deterministic structured outputs where possible
- [x] Routing between lanes based on task complexity
- [x] Comprehensive test coverage (16 passing tests)
- [x] Documentation updated
- [x] All existing tests continue to pass
- [x] Ready for integration with analyzer/planner components