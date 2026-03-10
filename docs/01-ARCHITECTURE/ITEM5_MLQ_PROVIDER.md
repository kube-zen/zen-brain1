# Item #5: MLQ/Provider - Keep Provider Set Small; Tune Prompts and Calibration

**Status:** 🎯 **IN PROGRESS - 60% Complete**  
**Date:** 2026-03-09  
**Focus:** Provider management, prompt optimization, model calibration

## Executive Summary

Item #5 focuses on optimizing the MLQ (Multi-Level Queue) provider strategy by maintaining a small, well-calibrated provider set with tuned prompts. This aligns with the CPU-first small-model strategy while ensuring reliability and cost efficiency.

## Current State Analysis (2026-03-09)

### Provider Set ✅ **SMALL & SIMPLE**

**Current Provider Configuration:**
1. **Local Worker** (`qwen3.5:0.8b`) - CPU-local small model for bounded tasks
2. **Planner** (`glm-4.7`) - Cloud model for complex planning tasks  
3. **Fallback** (`glm-4.7`) - Same as planner for redundancy

**Provider Count:** 3 providers, 2 model types ✅ **Small set achieved**

**Routing Logic:**
- Simple policy: Local worker first, escalate to planner for complex tasks
- Fallback chain for intelligent provider selection
- Cost-aware routing (local max $0.01, planner min $0.10)

### Prompt Tuning ⚠️ **BASIC/INCOMPLETE**

**Current Prompt Status:**
- **Analyzer:** Has structured prompts for classification, requirements, breakdown, evidence, cost estimation
- **LLM Providers:** Only simulation responses (`generateLocalWorkerResponse`, `generatePlannerResponse`)
- **Role Profiles:** Not implemented (planner, implementer, reviewer, ops roles)
- **Temperature Settings:** Hardcoded in analyzer (0.1), not configurable

**Issues:**
1. No actual LLM integration (simulation only)
2. Prompts not tuned for specific roles/tasks
3. No structured prompt templates
4. No prompt versioning or A/B testing

### Calibration ⚠️ **MISSING**

**Missing Calibration Components:**
1. **Model capability registry** - Track performance by task class
2. **Evaluation harness** - Benchmark against real work
3. **Warmup strategy** - Pre-load models for consistent latency
4. **Performance tracking** - Tokens/hour, completion rate, accuracy
5. **Thresholds & escalation** - When to escalate to better models

## Implementation Plan

### Phase 1: Prompt System Enhancement (Current Priority)

#### 1.1 Structured Prompt Templates
- Create `prompts/` directory with YAML/JSON prompt templates
- Define role-based profiles (planner, implementer, reviewer, ops)
- Support variables and context injection
- Version prompts for A/B testing

#### 1.2 LLM Integration
- Replace simulation responses with actual model calls
- Support Ollama for local models (qwen3.5:0.8b)
- Support OpenAI-compatible APIs for cloud models
- Maintain provider-agnostic design

#### 1.3 Temperature & Parameter Tuning
- Configurable temperature by task type:
  - Code generation: 0.1-0.3
  - Planning: 0.5-0.7  
  - Creative tasks: 0.8-1.0 (rare)
- Configurable max tokens by provider
- Timeout adjustments based on empirical data

### Phase 2: Calibration System

#### 2.1 Model Capability Registry
```go
type ModelCapability struct {
    Model          string
    MaxContext     int
    CostPerToken   float64
    SupportsTools  bool
    TaskClassStats map[string]TaskStats  // by work type
    LatencyProfile LatencyStats
    WarmupRequired bool
}
```

#### 2.2 Evaluation Harness
- Task classes: planning, implementation, testing, documentation, review
- Metrics: completion rate, tool-call accuracy, code correctness, time-to-completion
- Baselines: Human baseline for each task class
- Automated evaluation against test suite

#### 2.3 Warmup & Performance Tracking
- Pre-load models before first task (30-60s)
- Keep workers warm between tasks
- Track tokens/hour, yield (tasks/tokens), cost efficiency
- ZenLedger integration for token/cost accounting

### Phase 3: Provider Set Optimization

#### 3.1 Provider Reduction Analysis
- Evaluate if 3 providers are optimal or can be reduced to 2
- Consider merging planner/fallback (same model anyway)
- Local worker + escalation provider (2-provider model)

#### 3.2 Smart Routing Rules
- Task complexity detection (heuristic + ML)
- Automatic escalation based on calibration data
- Cost/performance optimization (best model for task)

#### 3.3 Configuration Management
- Centralized provider configuration
- Environment-based model selection
- Graceful degradation when providers unavailable

## Technical Implementation

### Files to Create/Modify

#### New Files:
1. `internal/llm/prompts/` - Prompt template system
2. `internal/llm/calibration/` - Calibration and evaluation
3. `internal/llm/registry.go` - Model capability registry
4. `internal/llm/ollama_provider.go` - Actual Ollama integration
5. `internal/llm/openai_provider.go` - OpenAI-compatible API integration

#### Modified Files:
1. `internal/llm/local_worker.go` - Replace simulation with real calls
2. `internal/llm/planner.go` - Replace simulation with real calls  
3. `internal/llm/gateway.go` - Integrate calibration data into routing
4. `internal/config/load.go` - Add prompt/calibration configuration
5. `configs/config.dev.yaml` - Add prompt/calibration settings

### Configuration Schema

```yaml
# Enhanced LLM configuration
llm:
  # Provider configurations
  providers:
    local:
      model: "qwen3.5:0.8b"
      type: "ollama"
      endpoint: "http://localhost:11434"
      timeout: 30
      temperature: 0.1
      max_tokens: 4000
      
    planner:
      model: "glm-4.7"
      type: "openai"
      endpoint: "https://api.openai.com/v1"
      timeout: 60
      temperature: 0.3
      max_tokens: 128000
  
  # Prompt configuration
  prompts:
    planner:
      system: "You are a strategic planner. Break down complex problems into executable steps."
      temperature: 0.5
      
    implementer:
      system: "You are a software engineer. Write correct, tested code following best practices."
      temperature: 0.1
      
    reviewer:
      system: "You are a code reviewer. Identify bugs, style issues, and improvements."
      temperature: 0.3
  
  # Calibration settings
  calibration:
    enable_warmup: true
    warmup_timeout: 60
    evaluation_sample_size: 100
    performance_thresholds:
      completion_rate: 0.8
      accuracy_threshold: 0.7
      max_latency_seconds: 300
```

## Success Criteria

### Phase 1 Complete (Prompt Tuning):
- [ ] Structured prompt templates implemented
- [ ] Actual LLM integration (Ollama + OpenAI)
- [ ] Configurable temperature/max tokens
- [ ] Role-based prompt profiles

### Phase 2 Complete (Calibration):
- [ ] Model capability registry tracking performance
- [ ] Evaluation harness with test suite
- [ ] Warmup strategy implemented
- [ ] Performance metrics dashboard

### Phase 3 Complete (Provider Optimization):
- [ ] Provider set optimized (2-3 providers)
- [ ] Smart routing based on calibration data
- [ ] Graceful degradation when providers fail
- [ ] Configuration-driven provider management

## Dependencies & Risks

### Dependencies:
1. **Ollama installed** for local model testing
2. **API keys** for cloud providers (if used)
3. **Test suite** for evaluation harness
4. **Monitoring** for performance tracking

### Risks:
1. **Local model instability** - Small models may be unreliable
2. **API costs** - Cloud provider usage must be monitored
3. **Calibration complexity** - Hard to define accurate metrics
4. **Prompt tuning effort** - Requires iterative testing

## Next Immediate Actions

### Week 1: Prompt System Foundation
1. Create prompt template system (`internal/llm/prompts/`)
2. Implement Ollama provider integration
3. Replace simulation responses with real calls
4. Add temperature/max token configuration

### Week 2: Basic Calibration
1. Create model capability registry
2. Implement warmup strategy
3. Add basic performance tracking
4. Create test suite for evaluation

### Week 3: Provider Optimization
1. Analyze provider set for reduction opportunities
2. Implement smart routing with calibration data
3. Add graceful degradation
4. Document provider strategy

## Related Documents

- [Small Model Strategy](../03-DESIGN/SMALL_MODEL_STRATEGY.md) - CPU-first design
- [LLM Gateway](../03-DESIGN/LLM_GATEWAY.md) - Provider-agnostic interface
- [ZenLedger](../03-DESIGN/ZEN_LEDGER.md) - Token/cost accounting

## Team Notes

- **Primary Owner:** @neves
- **Stakeholders:** Engineering, Operations
- **Timeline:** 3 weeks for full implementation
- **Priority:** Medium (supports reliability and cost optimization)

---

**Last Updated:** 2026-03-09  
**Next Review:** 2026-03-16