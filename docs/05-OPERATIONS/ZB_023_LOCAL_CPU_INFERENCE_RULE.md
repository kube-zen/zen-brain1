# Runbook: Local CPU Inference Policy (ZB-023)

**Task ID:** ZB-023
**Effective:** 2026-03-20
**Status:** Production-Ready

## 🚨 CRITICAL RULE (UNTIL EXPLICITLY OVERRIDDEN BY OPERATOR)

### Certified Local CPU Path

- ✅ **ONLY allowed local model:** `qwen3.5:0.8b`
- ✅ **ONLY supported local inference path:** Host Docker Ollama (http://host.k3d.internal:11434)
- ❌ **FORBIDDEN:** In-cluster Ollama for active local CPU path
- ❌ **FORBIDDEN:** Any other local model (e.g., qwen3.5:14b, llama*, mistral*)

### Provider/Model Flexibility

- Any provider/model may serve any role if configured in policy
- The outdated "planner=GLM, worker=0.8b" split is **REMOVED**
- `qwen3.5:0.8b` is NOT worker-only by architecture
- GLM is NOT planner-only by architecture
- **However:** The ONLY certified LOCAL CPU lane is `qwen3.5:0.8b` via host Docker Ollama

### Enforcement (FAIL-CLOSED)

The following layers enforce this policy:

1. **Policy Layer** (`config/policy/`)
   - `providers.yaml`: `fail_if_other_model_requested: true`
   - `routing.yaml`: `forbid_in_cluster_ollama: true`
   - `routing.yaml`: `certified_local_models: ["qwen3.5:0.8b"]`

2. **Runtime Layer** (`internal/llm/`)
   - `ollama_provider.go`: Logs warnings for non-0.8b models
   - `ollama_provider.go`: Detects and logs in-cluster Ollama usage
   - `gateway.go`: Startup logs clearly state certified local path

3. **CI Layer** (`scripts/ci/`)
   - `local_model_policy_gate.py`: Blocks PRs with disallowed models
   - `local_model_policy_gate.py`: Blocks PRs with in-cluster Ollama refs
   - `run.py`: Gate included in default, governance, and docs suites

4. **Documentation Layer** (`docs/`, `deploy/`)
   - `OLLAMA_08B_OPERATIONS_GUIDE.md`: Updated with ZB-023 policy
   - `deploy/README.md`: Critical rule section added
   - `config/policy/README.md`: Prominent local CPU inference rule

## Verification Commands (POST-DEPLOYMENT)

After deployment, verify the following:

### 1. Verify Host Docker Ollama (NOT In-Cluster)

```bash
# Check OLLAMA_BASE_URL points to host Docker
kubectl exec -n zen-brain deploy/apiserver -- env | grep OLLAMA_BASE_URL
# Expected: OLLAMA_BASE_URL=http://host.k3d.internal:11434

# FAIL if: http://ollama:11434 or http://ollama.zen-brain:11434
```

### 2. Verify Local-Worker Lane Uses qwen3.5:0.8b

```bash
# Check gateway logs for local-worker lane configuration
kubectl logs -n zen-brain deploy/apiserver | grep -E 'local-worker lane|Ollama warmup'
# Expected: [LLM Gateway] ZB-023: Local worker lane - Ollama at http://host.k3d.internal:11434 (model=qwen3.5:0.8b, CERTIFIED local CPU path)

# FAIL if: model != qwen3.5:0.8b or in-cluster URL
```

### 3. Verify Host Docker Ollama Has Model

```bash
# Check host Docker Ollama API for available models
kubectl exec -n zen-brain deploy/apiserver -- wget -qO- http://host.k3d.internal:11434/api/tags
# Expected: JSON with "qwen3.5:0.8b" in models list
```

### 4. Verify In-Cluster Ollama is NOT Running

```bash
# Check for in-cluster Ollama pods (should be NONE)
kubectl get pods -n zen-brain | grep ollama
# Expected: No ollama pods (in-cluster Ollama disabled)
```

### 5. Verify No Stale 14b References

```bash
# Check active-path files for stale 14b references
grep -r "qwen3.5:14b" config/clusters.yaml config/policy/ internal/llm/ internal/foreman/
# Expected: No matches (only qwen3.5:0.8b allowed)
```

## How to Override (NOT RECOMMENDED)

To use a different local model or in-cluster Ollama, you MUST:

1. **Get EXPLICIT OPERATOR APPROVAL**
   - No casual switching
   - Document the reason for override
   - Get sign-off from technical lead

2. **Update ALL THREE LAYERS**
   - **Policy:** Update `config/policy/providers.yaml` and `routing.yaml`
   - **Code:** Update `internal/llm/ollama_provider.go` enforcement logic
   - **Documentation:** Update all docs, guides, and runbooks

3. **Add CI GATE EXCEPTIONS**
   - Update `scripts/ci/local_model_policy_gate.py`
   - Document why the exception is needed
   - Add expiration date for temporary exceptions

4. **Document THE CHANGE**
   - Create new runbook or update existing
   - Add verification steps for new path
   - Add rollback procedure

## Common Issues and Fixes

### Issue: "In-cluster Ollama detected" warning in logs

**Cause:** `OLLAMA_BASE_URL` points to `http://ollama:11434` or similar k8s service name.

**Fix:**
```yaml
# config/clusters.yaml
deploy:
  host_ollama_base_url: "http://host.k3d.internal:11434"  # Use host Docker
  use_ollama: false  # Disable in-cluster Ollama
```

### Issue: "Non-certified local model" warning in logs

**Cause:** Request tries to use `qwen3.5:14b` or other non-certified model.

**Fix:**
- Change model to `qwen3.5:0.8b` in request
- Or update policy if override is approved (see "How to Override" above)

### Issue: CI gate fails for local model violations

**Cause:** PR introduces disallowed model or in-cluster Ollama reference.

**Fix:**
1. Run gate locally to see violations:
   ```bash
   python3 scripts/ci/local_model_policy_gate.py
   ```
2. Fix violations identified by gate
3. Re-run gate to verify fix

### Issue: Outdated "planner=GLM, worker=0.8b" reference

**Cause:** Old documentation still has strict role-model binding.

**Fix:**
- Update docs to reflect that any model can serve any role
- See ZB-023 policy update date in guide for reference

## References

- **Task:** ZB-023 (Local CPU Inference Policy)
- **Guide:** `docs/05-OPERATIONS/OLLAMA_08B_OPERATIONS_GUIDE.md`
- **Policy:** `config/policy/README.md` (Local CPU Inference Rule section)
- **CI Gate:** `scripts/ci/local_model_policy_gate.py`
- **Code:** `internal/llm/ollama_provider.go`, `internal/llm/gateway.go`

## Quick Reference

| Question | Answer |
|----------|---------|
| **Can 0.8b be a planner?** | YES - any model can serve any role |
| **Can GLM be a worker?** | YES - any model can serve any role |
| **Is 14b allowed locally?** | NO - only 0.8b is certified for local CPU |
| **Is in-cluster Ollama allowed?** | NO - only host Docker Ollama is supported |
| **How do I change this?** | Get EXPLICIT operator approval (see above) |
| **Where is this enforced?** | Policy, runtime, CI, and docs (all layers) |

---

**Last Updated:** 2026-03-20
**Maintained by:** Zen-Brain Team
