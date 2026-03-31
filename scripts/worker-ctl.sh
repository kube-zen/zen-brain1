#!/bin/bash
# zen-brain1 worker operator runbook
# Usage: ./worker-ctl.sh {start|stop|status|restart|warmup}

set -euo pipefail

LLAMA_BIN="/home/neves/git/llama.cpp/build/bin/llama-server"
L1_MODEL="/home/neves/git/ai/Qwen3.5-0.8B-Q4_K_M.gguf"
L2_MODEL="/home/neves/git/ai/zen-go-q4_k_m-latest.gguf"
L1_PORT=56227
L2_PORT=60509
LOG_DIR="/tmp/zen-brain1-logs"
PID_DIR="/tmp/zen-brain1-pids"

mkdir -p "$LOG_DIR" "$PID_DIR"

is_running() {
    local port=$1
    curl -sf --max-time 2 "http://localhost:${port}/health" > /dev/null 2>&1
}

kill_existing() {
    local port=$1 name=$2
    if is_running "$port"; then
        echo "Stopping $name (port $port)..."
        # Find PID from /proc
        local pid=$(ss -tlnp "sport = :${port}" 2>/dev/null | grep -oP 'pid=\K[0-9]+' | head -1)
        if [[ -n "$pid" ]]; then
            kill "$pid" 2>/dev/null || true
            sleep 2
            kill -9 "$pid" 2>/dev/null || true
        fi
        echo "  $name stopped"
    else
        echo "  $name not running"
    fi
}

start_worker() {
    local name=$1 model=$2 port=$3 args=$4
    if is_running "$port"; then
        echo "  $name already running on port $port"
        return 0
    fi
    echo "Starting $name (port $port)..."
    nohup "$LLAMA_BIN" -m "$model" --port "$port" --host 0.0.0.0 $args \
        > "$LOG_DIR/${name}.log" 2>&1 &
    local pid=$!
    echo "$pid" > "$PID_DIR/${name}.pid"
    echo "  $name started (PID $pid)"
}

start_all() {
    echo "=== Starting zen-brain1 workers ==="
    kill_existing $L1_PORT "L1"
    kill_existing $L2_PORT "L2"
    sleep 1
    # 393216 total → ~39k tokens/slot (10 parallel); covers OpenClaw long sessions + foreman (~4k).
    start_worker "l1" "$L1_MODEL" $L1_PORT "--parallel 10 --ctx-size 393216"
    start_worker "l2" "$L2_MODEL" $L2_PORT "--ctx-size 16384"
    echo ""
    echo "Waiting for warmup..."
    sleep 12
    bash "$(dirname "$0")/health-check.sh"
}

stop_all() {
    echo "=== Stopping zen-brain1 workers ==="
    kill_existing $L1_PORT "L1"
    kill_existing $L2_PORT "L2"
    echo "All workers stopped"
}

show_status() {
    bash "$(dirname "$0")/health-check.sh"
    echo ""
    echo "=== Worker Details ==="
    for name in l1 l2; do
        if [[ -f "$PID_DIR/${name}.pid" ]]; then
            local pid=$(cat "$PID_DIR/${name}.pid")
            if kill -0 "$pid" 2>/dev/null; then
                echo "$name: PID $pid (running), log: $LOG_DIR/${name}.log"
            else
                echo "$name: PID $pid (NOT running), log: $LOG_DIR/${name}.log"
            fi
        else
            echo "$name: no PID file"
        fi
    done
}

warmup() {
    echo "=== Warming up L1 ==="
    curl -sf --max-time 30 "http://localhost:${L1_PORT}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"qwen3.5:0.8b-q4","messages":[{"role":"user","content":"say ok"}],"max_tokens":5,"chat_template_kwargs":{"enable_thinking":false}}' \
        | head -c 200
    echo ""
    echo "L1 warmup complete"
}

case "${1:-status}" in
    start)   start_all ;;
    stop)    stop_all ;;
    status)  show_status ;;
    restart) stop_all; sleep 2; start_all ;;
    warmup)  warmup ;;
    *)       echo "Usage: $0 {start|stop|status|restart|warmup}"; exit 1 ;;
esac
