# Policy Configuration

This directory contains policy configuration for zen-brain1 timeout, roles, tasks, MLQ levels, and chains.

## Certified Runtime (PHASE 26)

| Lane | Model | Inference | Port | Timeout |
|------|-------|-----------|------|---------|
| L1 | qwen3.5:0.8b Q4_K_M | llama.cpp | 56227 | 300s per task |
| L2 | qwen3.5:2b Q4_K_M | llama.cpp | 60509 | 600s per task |
| L0 | qwen3.5:0.8b | Ollama | 11434 | Fallback only |

**Primary inference runtime is llama.cpp** (not Ollama). L0/Ollama is fallback only.

### Timeout Policy

- **L1 (0.8B):** 300s per useful task (reporting/triage). 2700s for complex implementation tasks.
- **L2 (2B):** 600s per task. 2B is slower but more capable.
- **Short timeouts (300s, 600s) are correct for useful reporting tasks.**
- 45m timeouts apply only to complex multi-step implementation tasks, not regular useful work.

## Policy Files

### mlq-levels.yaml / mlq-levels-local.yaml
Defines MLQ level routing, worker endpoints, escalation rules.

Key settings:
- `levels`: [1, 2, 0] — L1 first, L2 escalation, L0 fallback
- `escalation_rules`: retry_count trigger (L1→L2 after 2 failures)
- `workers`: endpoint URLs, model names, max_parallel

### providers.yaml
Defines available LLM providers and models.

Key settings:
- `certified_local_cpu`: llama.cpp is the primary local inference runtime
- L0 (Ollama) is fallback only (FAIL-CLOSED)

## Timeout Profiles

### useful-reporting (default for L1)
- Timeout: 300 seconds (5 minutes)
- Use for: All usefulness/reporting tasks (dead-code, defects, tech-debt, etc.)
- Applies to: L1 (0.8B) via direct HTTP path

### complex-implementation
- Timeout: 2700 seconds (45 minutes)
- Use for: Complex multi-step implementation tasks requiring codegen
- Applies to: L1/L2 via FactoryTaskRunner path

## Related Files

- `internal/mlq/task_executor.go`: MLQ retry/escalation logic
- `internal/factory/factory.go`: Factory execution path
- `internal/llm/openai_compatible_provider.go`: llama.cpp provider
- `config/task-templates/usefulness-l1.yaml`: Usefulness task template
- `config/task-templates/quickwin-l1.yaml`: Quick-win L1 template
