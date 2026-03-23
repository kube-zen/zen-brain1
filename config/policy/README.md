# Policy Configuration

This directory contains policy configuration for zen-brain1 timeout, roles, tasks, and chains.

## ZB-024 Timeout Policy (AUTHORITATIVE)

**CRITICAL RULE**: Any real execution path using qwen3.5:0.8b on the active local CPU lane MUST use:
- **timeout = 2700s** (45 minutes)
- **keep_alive = 45m**
- **stale threshold > 45m**

### Rationale

qwen3.5:0.8b on CPU is slow but reliable:
- First token generation can take 10-20 minutes
- Complex tasks can take 30-45 minutes
- The 45m timeout ensures real work completes while preventing indefinite hangs

### Exceptions

**Short timeouts (300s, 600s, 1200s) are WRONG for normal lane** and will cause spurious failures.

The ONLY allowed exception:
- **controlled_failure templates** with `controlled_failure: true` may use short timeouts
- These templates are ONLY for intentional timeout testing to verify error handling

## Policy Files

### providers.yaml
Defines available LLM providers and models.

Key settings:
- `timeout_seconds`: LLM request timeout (2700s for qwen3.5:0.8b)
- `keep_alive`: How long to keep model resident (45m for qwen3.5:0.8b)
- `certified_local_cpu`: Only qwen3.5:0.8b is certified for local CPU (ZB-023)

### roles.yaml
Defines agent roles (worker, planner, reviewer, summarizer, debugger) and their capabilities.

Key settings:
- `timeout_seconds`: Per-role timeout (2700s for roles using qwen3.5:0.8b)
- `default_provider`: Which provider to use (local-worker = qwen3.5:0.8b)
- `supports_thinking`: Whether role uses chain-of-thought

### tasks.yaml
Defines task types and their execution parameters.

Key settings:
- `timeout_seconds`: Per-task timeout (2700s for tasks using qwen3.5:0.8b)
- `max_retries`: Number of retry attempts

### chains.yaml
Defines task chains (sequences of tasks with dependencies).

Key settings:
- Per-task `timeout_seconds`: Each task in chain uses 2700s for qwen3.5:0.8b

## Timeout Profiles

### normal-45m
- Timeout: 2700 seconds (45 minutes)
- keep_alive: 45m
- Use for: All normal execution paths
- Applies to: All roles, tasks, and chains using qwen3.5:0.8b on CPU

### short-test
- Timeout: 300 seconds (5 minutes)
- keep_alive: 5m
- Use for: Controlled failure testing ONLY
- Applies to: Templates with `controlled_failure: true` flag

## Verification

To verify timeout compliance:

```bash
# Check providers.yaml uses 2700s/45m
grep -A2 "certified_local_cpu: true" config/policy/providers.yaml | grep -E "timeout|keep_alive"

# Check roles.yaml uses 2700s
grep "timeout_seconds:" config/policy/roles.yaml

# Check tasks.yaml uses 2700s
grep "timeout_seconds:" config/policy/tasks.yaml

# Check chains.yaml uses 2700s
grep "timeout_seconds:" config/policy/chains.yaml

# Check charts/zen-brain/values.yaml uses 2700s/45m
grep -E "timeoutSeconds|keepAlive" charts/zen-brain/values.yaml
```

All should show:
- `timeout_seconds: 2700` (not 300, 600, or 1200)
- `keep_alive: "45m"` (not "30m")

## Preflight Checks

Any CI or preflight checks must enforce:
1. No 300s/600s/1200s timeouts for qwen3.5:0.8b paths
2. No 30m keep_alive for qwen3.5:0.8b paths
3. Short timeouts only allowed when `controlled_failure: true`
4. Stale threshold > 45m (typically 50m in reconciler.go)

## Breaking Changes

Changing timeout values for qwen3.5:0.8b from 2700s to anything lower IS A BREAKING CHANGE and will cause:

- Spurious task failures
- Incomplete work execution
- User frustration and loss of trust

Do NOT reduce timeouts below 2700s for normal paths.

## Related Files

- `internal/llm/gateway.go`: Gateway default config (2700s/45m)
- `internal/llm/ollama_warmup.go`: Ollama warmup timeout (2700s/45m)
- `cmd/foreman/main.go`: Foreman CLI flags (default 2700s)
- `cmd/zen-brain/main.go`: Zen-brain CLI timeout (2700s)
- `charts/zen-brain/values.yaml`: Helm chart values (2700s/45m)
- `internal/foreman/reconciler.go`: Stale threshold (50m)
