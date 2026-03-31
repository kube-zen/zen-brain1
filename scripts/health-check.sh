#!/bin/bash
# zen-brain1 worker health check
# Usage: ./health-check.sh [--json]

set -euo pipefail

L1_PORT=56227
L2_PORT=60509
# L0/Ollama removed — Ollama is forbidden for zen-brain1

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
    local info=$(echo "$resp" | python3 -c "
import sys,json
try:
    slots=json.load(sys.stdin)
    idle=sum(1 for s in slots if s.get('idle',False))
    print(f'{idle}/{len(slots)} idle')
except: print('parse-error')
" 2>/dev/null)
    echo "${name}: ${info} slots"
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

if [[ $JSON_MODE -eq 1 ]]; then
    cat << EOF
{"l1":{"up":$L1_UP,"port":$L1_PORT,"slots":"$L1_SLOTS"},"l2":{"up":$L2_UP,"port":$L2_PORT,"slots":"$L2_SLOTS"},"timestamp":"$(date -u +%Y-%m-%dT%H:%M:%SZ)"}
EOF
else
    echo "=== zen-brain1 Worker Health ==="
    if [[ $L1_UP -eq 1 ]]; then ok "L1 (llama.cpp 0.8B): port $L1_PORT — $L1_SLOTS"; else fail "L1 (llama.cpp 0.8B): port $L1_PORT — DOWN"; fi
    if [[ $L2_UP -eq 1 ]]; then ok "L2 (llama.cpp 2B):   port $L2_PORT — $L2_SLOTS"; else warn "L2 (llama.cpp 2B):   port $L2_PORT — DOWN"; fi
    echo ""
    if [[ $L1_UP -eq 1 ]]; then
        echo "Status: READY for useful work"
    else
        echo "Status: NOT READY — L1 worker is down"
        exit 1
    fi
fi
