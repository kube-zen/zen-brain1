#!/usr/bin/env bash
#
# run-08b-positive-control.sh — Run a bounded positive-control test for qwen3.5:0.8b-q4
#
# Usage: ./scripts/run-08b-positive-control.sh [--warmup-only | --skip-warmup | --run <task-file.yaml>]
#
# This script validates that qwen3.5:0.8b-q4 via llama.cpp can produce correct,
# grounded code for a bounded task with proper context injection.
#
# See: docs/05-OPERATIONS/08B_POSITIVE_CONTROL_RUNBOOK.md
#
set -euo pipefail

CTX="k3d-zen-brain-sandbox"
NS="zen-brain"
LLAMA_PORT=56227
LLAMA_HOST="host.k3d.internal"
MODEL="qwen3.5:0.8b-q4"
GGUF="/home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ok()   { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; exit 1; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
step() { echo -e "\n${YELLOW}━━━ $1 ━━━${NC}"; }

###############################################################################
# PRE-FLIGHT: Verify cluster, registry, and basic connectivity
###############################################################################
step "PRE-FLIGHT CHECKS"

# 1. Cluster context
kubectl --context "$CTX" get ns "$NS" > /dev/null 2>&1 \
  || fail "Cannot reach namespace $NS on context $CTX"
ok "Cluster reachable: $CTX/$NS"

# 2. Foreman running
FOREMAN_IMAGE=$(kubectl --context "$CTX" -n "$NS" get deploy foreman -o jsonpath='{.spec.template.spec.containers[0].image}')
[[ "$FOREMAN_IMAGE" == zen-registry:5000/* ]] \
  || fail "Foreman image not from zen-registry:5000: $FOREMAN_IMAGE"
ok "Foreman image: $FOREMAN_IMAGE"

# 3. No port-500 usage
if [[ "$FOREMAN_IMAGE" == *":500/"* ]]; then
  fail "BROKEN: foreman using registry port 500 (not 5000)"
fi

# 4. MLQ config mounted
POD=$(kubectl --context "$CTX" -n "$NS" get pods --no-headers -o custom-columns=':metadata.name' | grep foreman | head -1)
MLQ=$(kubectl --context "$CTX" -n "$NS" exec "$POD" -- cat /tmp/zen-brain1/config/policy/mlq-levels.yaml 2>/dev/null | head -1)
[[ -n "$MLQ" ]] || warn "MLQ config not mounted at expected path"

# 5. ZEN_SOURCE_REPO
SRC_REPO=$(kubectl --context "$CTX" -n "$NS" exec "$POD" -- env 2>/dev/null | grep ZEN_SOURCE_REPO | cut -d= -f2)
[[ -n "$SRC_REPO" ]] && ok "ZEN_SOURCE_REPO=$SRC_REPO" || warn "ZEN_SOURCE_REPO not set"

# 6. Source files in mount
if [[ -n "$SRC_REPO" ]]; then
  SRC_COUNT=$(kubectl --context "$CTX" -n "$NS" exec "$POD" -- find "$SRC_REPO" -name "*.go" -type f 2>/dev/null | wc -l)
  ok "Source repo has $SRC_COUNT .go files mounted"
fi

###############################################################################
# WARMUP: Ensure llama.cpp is running and model is loaded
###############################################################################
step "LLAMA.CPP WARMUP"

# Check if llama-server is running on host
if ! ss -tlnp 2>/dev/null | grep -q "$LLAMA_PORT"; then
  warn "llama.cpp not running on port $LLAMA_PORT — starting..."
  if [[ -f ~/git/llama.cpp/build/bin/llama-server ]]; then
    nohup ~/git/llama.cpp/build/bin/llama-server \
      -m "$GGUF" --port "$LLAMA_PORT" --host 0.0.0.0 \
      > /tmp/llama-server-positive-control.log 2>&1 &
    echo "Started llama-server PID: $!"
    sleep 5
  else
    fail "llama-server binary not found at ~/git/llama.cpp/build/bin/llama-server"
  fi
fi

# Host-side health check
HEALTH=$(curl -sf http://localhost:$LLAMA_PORT/health 2>/dev/null)
[[ "$HEALTH" == *"ok"* ]] || fail "llama.cpp health check failed on localhost:$LLAMA_PORT"
ok "llama.cpp healthy on localhost:$LLAMA_PORT"

# In-cluster reachability
CLUSTER_HEALTH=$(kubectl --context "$CTX" -n "$NS" exec "$POD" -- \
  wget -qO- --timeout=5 "http://$LLAMA_HOST:$LLAMA_PORT/health" 2>/dev/null)
[[ "$CLUSTER_HEALTH" == *"ok"* ]] || fail "llama.cpp not reachable from cluster (host=$LLAMA_HOST:$LLAMA_PORT)"
ok "llama.cpp reachable from cluster"

# Warmup inference
WARMUP_RESP=$(curl -sf http://localhost:$LLAMA_PORT/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"say ok\"}],\"max_tokens\":5}" 2>/dev/null)
WARMUP_TOKENS=$(echo "$WARMUP_RESP" | jq -r '.usage.completion_tokens // "0"')
[[ "$WARMUP_TOKENS" -gt 0 ]] || fail "Warmup inference failed (0 tokens)"
ok "Warmup complete: $WARMUP_TOKENS completion tokens"

###############################################################################
# SOURCE REPO: Ensure context files are available in the k3d node
###############################################################################
step "SOURCE REPO CONTEXT FILES"

# Copy needed source files to k3d node if ZEN_SOURCE_REPO is set
if [[ -n "$SRC_REPO" ]]; then
  # The hostPath mount on k3d node points to a path inside the k3d Docker container.
  # We docker cp the source files there.
  K3D_NODE="k3d-${CTX#k3d-}-server-0"
  
  # Detect the hostPath from the deployment
  HOST_PATH=$(kubectl --context "$CTX" -n "$NS" get deploy foreman -o jsonpath='{range .spec.template.spec.volumes[?(@.name=="source-repo-volume")]}{.hostPath.path}{end}' 2>/dev/null)
  
  if [[ -n "$HOST_PATH" ]]; then
    docker exec "$K3D_NODE" mkdir -p "$HOST_PATH/internal/scheduler" 2>/dev/null || true
    docker exec "$K3D_NODE" mkdir -p "$HOST_PATH/pkg/llm" 2>/dev/null || true
    
    # Copy relevant source files
    if [[ -f ~/zen/zen-brain1/internal/scheduler/types.go ]]; then
      docker cp ~/zen/zen-brain1/internal/scheduler/types.go "$K3D_NODE:$HOST_PATH/internal/scheduler/types.go"
      ok "Copied internal/scheduler/types.go to k3d node"
    fi
    if [[ -f ~/zen/zen-brain1/pkg/llm/types.go ]]; then
      docker cp ~/zen/zen-brain1/pkg/llm/types.go "$K3D_NODE:$HOST_PATH/pkg/llm/types.go"
    fi
    if [[ -f ~/zen/zen-brain1/pkg/llm/provider.go ]]; then
      docker cp ~/zen/zen-brain1/pkg/llm/provider.go "$K3D_NODE:$HOST_PATH/pkg/llm/provider.go"
    fi
    
    ok "Source files available in cluster mount"
  else
    warn "No source-repo-volume found in deployment — context files not mountable"
  fi
fi

###############################################################################
# RUN TASK
###############################################################################
step "SUBMITTING TASK"

TASK_FILE="${1:-}"
SKIP_WARMUP=false
WARMUP_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --warmup-only) WARMUP_ONLY=true ;;
    --skip-warmup) SKIP_WARMUP=true ;;
    --run)         shift; TASK_FILE="${1:-}" ;;
  esac
done

if [[ "$WARMUP_ONLY" == true ]]; then
  ok "Warmup only mode — exiting"
  exit 0
fi

if [[ -z "$TASK_FILE" ]]; then
  # Generate a default bounded task
  TASK_FILE="/tmp/08b-positive-control-$(date +%s).yaml"
  cat > "$TASK_FILE" <<'YAML'
apiVersion: zen.kube-zen.com/v1alpha1
kind: BrainTask
metadata:
  name: 08b-positive-control-$(date +%s | shasum | cut -c1-8)
  labels:
    zen.kube-zen.com/source: manual
    zen.kube-zen.com/work-domain: office
    zen.kube-zen.com/work-type: implementation
spec:
  title: "Add Validate() method to Schedule struct in internal/scheduler/types.go"
  workItemID: "ZB-PC-001"
  workDomain: office
  workType: implementation
  sessionID: "08b-pc-$(date +%s | shasum | cut -c1-8)"
  objective: |
    Add a Validate() method to the Schedule struct in internal/scheduler/types.go.

    The method must return an error if:
    - Name is empty
    - Type is empty
    - Type is "recurring" and Interval is empty
    - Type is "cron" and CronExpr is empty

    Use only existing imports. Do not add new packages.
    Do not modify existing methods. Only add the new method.
  acceptanceCriteria:
    - "Schedule.Validate() method added"
    - "Code compiles: go build ./..."
    - "No new imports"
  maxRetries: 0
  timeoutSeconds: 2700
  priority: high
  queueName: dogfood
YAML
  ok "Generated default task: $TASK_FILE"
fi

TASK_NAME=$(kubectl --context "$CTX" apply -f "$TASK_FILE" -n "$NS" -o jsonpath='{.metadata.name}' 2>/dev/null)
[[ -n "$TASK_NAME" ]] || fail "Failed to create BrainTask from $TASK_FILE"
ok "Task submitted: $TASK_NAME"

###############################################################################
# POLL FOR COMPLETION
###############################################################################
step "WAITING FOR COMPLETION (timeout 10m)"

for i in $(seq 1 60); do
  PHASE=$(kubectl --context "$CTX" -n "$NS" get braintask "$TASK_NAME" -o jsonpath='{.status.phase}' 2>/dev/null)
  printf "\r  [%2d/60] Phase: %-12s" "$i" "${PHASE:-Pending}"
  if [[ "$PHASE" == "Completed" || "$PHASE" == "Failed" ]]; then
    echo ""
    break
  fi
  sleep 10
done

PHASE=$(kubectl --context "$CTX" -n "$NS" get braintask "$TASK_NAME" -o jsonpath='{.status.phase}' 2>/dev/null)
[[ "$PHASE" == "Completed" ]] || fail "Task did not complete (phase=$PHASE)"
ok "Task completed: $TASK_NAME"

###############################################################################
# EVIDENCE COLLECTION
###############################################################################
step "COLLECTING EVIDENCE"

# Provider/model selection
kubectl --context "$CTX" -n "$NS" logs deploy/foreman --since=10m 2>/dev/null | \
  grep -E "$TASK_NAME.*(Selected|llama-cpp|qwen3.5)" | tail -3

# Files generated
echo ""
echo "Files generated:"
kubectl --context "$CTX" -n "$NS" exec "$POD" -- \
  find "/tmp/zen-brain-factory/workspaces" -path "*$TASK_NAME*" -name "*.go" -exec wc -l {} \; 2>/dev/null

# Proof of work
PROOF_PATH=$(kubectl --context "$CTX" -n "$NS" get braintask "$TASK_NAME" -o jsonpath='{.metadata.annotations["zen.kube-zen.com/factory-proof"]}' 2>/dev/null)
echo ""
echo "Proof of work: $PROOF_PATH"

###############################################################################
# SUMMARY
###############################################################################
step "SUMMARY"
echo ""
echo "Task:       $TASK_NAME"
echo "Model:      $MODEL via llama.cpp"
echo "Image:      $FOREMAN_IMAGE"
echo "Phase:      $PHASE"
echo "Duration:   $(kubectl --context "$CTX" -n "$NS" get braintask "$TASK_NAME" -o jsonpath='{.metadata.annotations["zen.kube-zen.com/factory-duration-seconds"]}' 2>/dev/null)s"
echo "Proof:      $PROOF_PATH"
echo ""
ok "Positive-control test complete. Inspect output above for evidence."
