#!/bin/bash
# Focused health check for proven lane
# Ensures: host Docker Ollama is default, no accidental in-cluster Ollama

set -e

echo "=== Proven Lane Health Check ==="
echo ""

# Check 1: No ollama pods in zen-brain namespace
echo "[1/4] Checking for accidental in-cluster Ollama..."
OLLAMA_PODS=$(kubectl get pods -n zen-brain --context k3d-zen-brain-sandbox -o name | grep -c "^ollama-0" || true)
if [ "$OLLAMA_PODS" -eq 0 ]; then
  echo "✓ PASS: No in-cluster Ollama pods found"
else
  echo "✗ FAIL: Found $OLLAMA_PODS ollama pod(s) - in-cluster Ollama should be disabled"
  exit 1
fi

# Check 2: Apiserver uses host.k3d.internal:11434
echo "[2/4] Checking apiserver OLLAMA_BASE_URL..."
OLLAMA_URL=$(kubectl exec -n zen-brain --context k3d-zen-brain-sandbox apiserver-5d88df6b4b-dw9bl -- env | grep OLLAMA_BASE_URL | cut -d= -f2)
if [ "$OLLAMA_URL" = "http://host.k3d.internal:11434" ]; then
  echo "✓ PASS: Apiserver using host.k3d.internal:11434"
else
  echo "✗ FAIL: Apiserver OLLAMA_BASE_URL=$OLLAMA_URL (should be http://host.k3d.internal:11434)"
  exit 1
fi

# Check 3: Apiserver healthy
echo "[3/4] Checking apiserver health..."
HEALTH=$(curl -s http://127.0.1.6:8080/healthz)
if [ "$HEALTH" = "ok" ]; then
  echo "✓ PASS: /healthz returns 'ok'"
else
  echo "✗ FAIL: /healthz returned '$HEALTH'"
  exit 1
fi

# Check 4: Foreman healthy
echo "[4/4] Checking foreman health..."
FOREMAN_LINE=$(kubectl get pods -n zen-brain --context k3d-zen-brain-sandbox | grep "^foreman-" | head -1)
echo "$FOREMAN_LINE" | grep -q "Running" && echo "✓ PASS: Foreman is Running" || (echo "✗ FAIL: Foreman not Running"; exit 1)

echo ""
echo "=== All Checks Passed ==="
echo "✓ Proven lane is healthy"
echo "✓ Host Docker Ollama is canonical"
echo "✓ No accidental in-cluster Ollama dependency"
