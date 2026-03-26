#!/bin/bash
# zen-brain1 worker health check
# Usage: ./health-check.sh [--json]

set -euo pipefail

L1_PORT=56227
L2_PORT=60509
L0_PORT=11434

ok()  { echo "✅ $1"; }
fail() { echo "❌ $1"; exit 1; }
warn() { echo "⚠️  $1"; }

if [[ "${1:-}" == "--json" ]]; then
    JSON_MODE=1
else
    JSON_MODE=0
fi

check_port() {
    local name=$1 port=$2
    if curl -sf --max-time 3 "http://localhost:${port}/health" > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

check_slots() {
    local name=$1 port=$2
    local resp
    resp=$(curl -sf --max-time 5 "http://localhost:${port}/slots" 2>/dev/null) || return 1
    # Extract idle/busy slots
    local idle=$(echo "$resp" | grep -c '"idle":true' 2>/dev/null || echo "0")
    local total=$(echo "$resp" | grep -c '"idle":' 2>/dev/null || echo "0")
    echo "${name}: ${idle}/${total} idle slots"
    return 0
}

# Check L1
L1_UP=0
if check_port "L1" $L1_PORT; then
    L1_UP=1
    L1_SLOTS=$(check_slots "L1" $L1_PORT)
else
    L1_SLOTS="L1: DOWN"
fi

# Check L2
L2_UP=0
if check_port "L2" $L2_PORT; then
    L2_UP=1
    L2_SLOTS=$(check_slots "L2" $L2_PORT)
else
    L2_SLOTS="L2: DOWN"
fi

# Check L0 (optional fallback)
L0_UP=0
if check_port "L0" $L0_PORT; then
    L0_UP=1
else
    L0_UP=0
fi

if [[ $JSON_MODE -eq 1 ]]; then
    cat << EOF
{"l1":{"up":$L1_UP,"port":$L1_PORT,"slots":"$L1_SLOTS"},"l2":{"up":$L2_UP,"port":$L2_PORT,"slots":"$L2_SLOTS"},"l0":{"up":$L0_UP,"port":$L0_PORT,"role":"fallback"},"timestamp":"$(date -u +%Y-%m-%dT%H:%M:%SZ)"}
EOF
else
    echo "=== zen-brain1 Worker Health ==="
    if [[ $L1_UP -eq 1 ]]; then ok "L1 (llama.cpp 0.8B): port $L1_PORT — $L1_SLOTS"; else fail "L1 (llama.cpp 0.8B): port $L1_PORT — DOWN"; fi
    if [[ $L2_UP -eq 1 ]]; then ok "L2 (llama.cpp 2B):   port $L2_PORT — $L2_SLOTS"; else warn "L2 (llama.cpp 2B):   port $L2_PORT — DOWN"; fi
    if [[ $L0_UP -eq 1 ]]; then warn "L0 (Ollama):          port $L0_PORT — UP (fallback only)"; else echo "   L0 (Ollama):          port $L0_PORT — down (fallback, non-critical)"; fi
    echo ""
    if [[ $L1_UP -eq 1 ]]; then
        echo "Status: READY for useful work"
    else
        echo "Status: NOT READY — L1 worker is down"
        exit 1
    fi
fi
