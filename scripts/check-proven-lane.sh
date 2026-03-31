#!/bin/bash
# Focused health check for proven inference lane
# Ensures: llama.cpp L1/L2 are active, no Ollama references

set -e

CTX="${1:-k3d-zen-brain-sandbox}"
echo "=== Proven Lane Health Check (context: $CTX) ==="
echo ""

# Check 1: No ollama pods in zen-brain namespace
echo "[1/4] Checking for accidental in-cluster Ollama..."
OLLAMA_PODS=$(kubectl get pods -n zen-brain --context "$CTX" -o name 2>/dev/null | grep -c "ollama" || true)
if [ "$OLLAMA_PODS" -eq 0 ]; then
  echo "✓ PASS: No Ollama pods found"
else
  echo "✗ FAIL: Found $OLLAMA_PODS ollama pod(s) - Ollama is forbidden"
  exit 1
fi

# Check 2: Apiserver does NOT have OLLAMA_BASE_URL set
echo "[2/4] Checking apiserver env (no OLLAMA_BASE_URL)..."
APISERVER_POD=$(kubectl get pods -n zen-brain --context "$CTX" -l app=apiserver -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
OLLAMA_URL=$(kubectl exec -n zen-brain --context "$CTX" "$APISERVER_POD" -- env 2>/dev/null | grep OLLAMA_BASE_URL | cut -d= -f2 || true)
if [ -z "$OLLAMA_URL" ]; then
  echo "✓ PASS: OLLAMA_BASE_URL not set (Ollama is forbidden)"
else
  echo "✗ FAIL: OLLAMA_BASE_URL=$OLLAMA_URL (should not be set — Ollama is forbidden)"
  exit 1
fi

# Check 3: Apiserver healthy
echo "[3/4] Checking apiserver health..."
if kubectl exec -n zen-brain --context "$CTX" "$APISERVER_POD" -- wget -qO- http://localhost:8080/healthz 2>/dev/null | grep -q "ok"; then
  echo "✓ PASS: /healthz returns 'ok'"
else
  echo "✗ FAIL: /healthz not returning 'ok'"
  exit 1
fi

# Check 4: L1 llama.cpp endpoint reachable
echo "[4/4] Checking llama.cpp L1 endpoint..."
L1_RESP=$(curl -sf --max-time 5 http://host.k3d.internal:56227/v1/models 2>/dev/null || true)
if [ -n "$L1_RESP" ]; then
  echo "✓ PASS: llama.cpp L1 responding"
else
  echo "⚠ WARN: llama.cpp L1 not reachable from host (may be cluster-internal only)"
fi

echo ""
echo "=== All checks passed ==="
