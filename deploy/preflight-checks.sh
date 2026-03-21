#!/bin/bash
set -e

export KUBECONFIG=/tmp/k3d-kubeconfig.yaml

echo "=== Zen-Brain Preflight Checks ==="

FAIL=0

# 1. ZenLock controller/webhook
echo -n "1. ZenLock controller/webhook: "
if kubectl get pods -n zen-lock-system | grep -E "controller.*Running|webhook.*Running" | grep -q Running; then
  echo "PASS"
else
  echo "FAIL"
  FAIL=$((FAIL + 1))
fi

# 2. BrainPolicy CRD
echo -n "2. BrainPolicy CRD exists: "
if kubectl get crd brainpolicies.zen.kube-zen.com &>/dev/null; then
  echo "PASS"
else
  echo "FAIL"
  FAIL=$((FAIL + 1))
fi

# 3. At least one BrainPolicy (cluster-scoped)
echo -n "3. At least one BrainPolicy (cluster-scoped): "
if kubectl get brainpolicy 2>&1 | grep -q "dogfood-default"; then
  echo "PASS"
else
  echo "FAIL"
  FAIL=$((FAIL + 1))
fi

# 4. Foreman healthy
echo -n "4. Foreman healthy: "
if kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman | grep -q Running; then
  echo "PASS"
else
  echo "FAIL"
  FAIL=$((FAIL + 1))
fi

# 5. Jira path healthy
echo -n "5. Jira path healthy: "
if kubectl exec -n zen-brain deployment/foreman -- ./zen-brain office doctor 2>&1 | grep -q "API reachability: ok"; then
  echo "PASS"
else
  echo "FAIL"
  FAIL=$((FAIL + 1))
fi

# 6. Local model path or LLM selection working
echo -n "6. Local model / LLM path: "
# PHASE 5: Fix preflight accordingly - validate actual LLM-routed implementation path
# Check for LLM gate logs showing implementation tasks routing to LLM
LLM_IMPL_GATE=$(kubectl logs -n zen-brain deployment/foreman --tail=1000 2>&1 | grep -E "llm gate.*FORCING_LLM_PATH.*work_type=implementation" | head -1 || echo "")
LLM_INTELLIGENCE=$(kubectl logs -n zen-brain deployment/foreman --tail=1000 2>&1 | grep -E "intelligence selection.*source=llm_generator" | head -1 || echo "")
LLM_EXEC_MODE=$(kubectl logs -n zen-brain deployment/foreman --tail=1000 2>&1 | grep -E "execution_mode.*llm" | head -1 || echo "")

if [ -n "$LLM_IMPL_GATE" ]; then
  echo "PASS (LLM implementation path confirmed: $LLM_IMPL_GATE)"
elif [ -n "$LLM_INTELLIGENCE" ]; then
  echo "PASS (LLM intelligence selection confirmed: $LLM_INTELLIGENCE)"
elif [ -n "$LLM_EXEC_MODE" ]; then
  echo "PASS (LLM execution mode confirmed: $LLM_EXEC_MODE)"
elif kubectl logs -n zen-brain deployment/foreman --tail=1000 2>&1 | grep -q "llm gate"; then
  echo "INFO (LLM gate active but no implementation tasks routed to LLM in recent logs)"
elif kubectl logs -n zen-brain deployment/foreman --tail=1000 2>&1 | grep -q "intelligence selection"; then
  echo "INFO (Factory running but using static templates for current workType - not an LLM-capable task)"
else
  echo "FAIL (Factory not processing tasks or LLM path not healthy)"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Preflight Complete ==="
if [ $FAIL -eq 0 ]; then
  echo "ALL CHECKS PASSED"
  exit 0
else
  echo "$FAIL CHECK(S) FAILED"
  exit 1
fi
