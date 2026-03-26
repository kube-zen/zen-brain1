#!/bin/bash
# zen-brain1 operator control panel
# Usage: ./scripts/zen-ctl.sh {status|schedule|run|logs|latest|restart|warmup|health}
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
REPO="/home/neves/zen/zen-brain1"
SCHEDULER_STATUS="/var/lib/zen-brain1/scheduler/scheduler-status.json"
SCHEDULER_BIN="$REPO/cmd/scheduler/scheduler"

ok()  { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }
info() { echo -e "${CYAN}ℹ️  $1${NC}"; }

case "${1:-help}" in
    status)
        echo "=== Worker Services ==="
        for svc in zen-brain1-l1 zen-brain1-l2; do
            state=$(sudo systemctl is-active "$svc" 2>/dev/null || echo "unknown")
            enabled=$(sudo systemctl is-enabled "$svc" 2>/dev/null || echo "unknown")
            if [[ "$state" == "active" ]]; then ok "$svc: $state (enabled=$enabled)"; else fail "$svc: $state"; fi
        done

        echo ""
        echo "=== Internal Scheduler ==="
        sched_state=$(sudo systemctl is-active zen-brain1-scheduler 2>/dev/null || echo "unknown")
        sched_enabled=$(sudo systemctl is-enabled zen-brain1-scheduler 2>/dev/null || echo "unknown")
        if [[ "$sched_state" == "active" ]]; then
            ok "zen-brain1-scheduler: $sched_state (enabled=$sched_enabled)"
        else
            warn "zen-brain1-scheduler: $sched_state (enabled=$sched_enabled)"
        fi

        echo ""
        echo "=== Schedule Status ==="
        if [[ -f "$SCHEDULER_STATUS" ]]; then
            python3 -c "
import json, sys
d = json.load(open('$SCHEDULER_STATUS'))
for s in d.get('schedules', []):
    status_icon = '✅' if s.get('last_status') == 'success' else '⚠️' if s.get('last_status') == 'partial' else '❌' if s.get('last_status') == 'failed' else '⏳'
    last = s.get('last_run', 'never')
    next_d = s.get('next_due', 'now')
    count = s.get('run_count', 0)
    print(f'  {status_icon} {s[\"name\"]:25s} cadence={s[\"cadence\"]:14s} tasks={len(s[\"tasks\"]):2d} last={last}  next={next_d}  runs={count}')
"
        else
            warn "No scheduler status file. Run: $0 run hourly"
        fi

        echo ""
        echo "=== Bootstrap Timers (DEPRECATED) ==="
        for tmr in zen-brain1-hourly-scan zen-brain1-quad-hourly-summary zen-brain1-daily-sweep; do
            state=$(sudo systemctl is-active "$tmr.timer" 2>/dev/null || echo "unknown")
            if [[ "$state" == "active" ]]; then
                warn "$tmr.timer: $state (bootstrap-only, should be disabled)"
            else
                info "$tmr.timer: $state (disabled — correct)"
            fi
        done
        ;;

    schedule)
        echo "=== Active Schedule (source of truth: config/schedules/) ==="
        for f in "$REPO"/config/schedules/*.yaml; do
            name=$(grep "^name:" "$f" | sed 's/name: //')
            cadence=$(grep "^cadence:" "$f" | sed 's/cadence: //')
            desc=$(grep "^description:" "$f" | sed 's/description: //')
            tasks=$(grep "^tasks:" "$f" | sed 's/tasks: //; s/^\[//; s/\]$//')
            echo "  $name ($cadence): $desc"
            echo "    Tasks: $tasks"
            echo ""
        done
        echo "Owner: zen-brain1 internal scheduler (not systemd timers)"
        echo "Config: $REPO/config/schedules/"
        ;;

    run)
        BATCH="${2:-hourly-scan}"
        case "$BATCH" in
            hourly|scan) SCHED="hourly-scan";;
            quad|summary) SCHED="quad-hourly-summary";;
            daily|full|all) SCHED="daily-sweep";;
            *) echo "Unknown batch: $BATCH"; echo "Usage: $0 run {hourly|quad|daily}"; exit 1;;
        esac
        echo "Triggering $SCHED via internal scheduler..."
        SCHEDULE_DIR="$REPO/config/schedules" \
        STATE_DIR="/var/lib/zen-brain1/scheduler" \
        ARTIFACT_ROOT="/var/lib/zen-brain1/runs" \
        BATCH_BIN="$REPO/cmd/useful-batch/useful-batch" \
        FORCE_RUN=1 "$SCHEDULER_BIN"
        echo ""
        echo "Status updated: $SCHEDULER_STATUS"
        ;;

    logs)
        SVC="${2:-zen-brain1-scheduler}"
        sudo journalctl -u "$SVC" --since "1 hour ago" --no-pager -f
        ;;

    latest)
        echo "=== Latest Artifacts ==="
        for batch in hourly-scan quad-hourly-summary daily-sweep; do
            LATEST=$(ls -td /var/lib/zen-brain1/runs/$batch/*/ 2>/dev/null | head -1)
            if [[ -n "$LATEST" ]]; then
                echo ""
                echo "$batch: $LATEST"
                ls "$LATEST/final/"*.md 2>/dev/null | while read f; do
                    lines=$(wc -l < "$f")
                    echo "  $(basename $f): $lines lines"
                done
            fi
        done
        ;;

    restart)
        echo "Restarting all services..."
        sudo systemctl restart zen-brain1-scheduler zen-brain1-l1 zen-brain1-l2
        sleep 15
        echo ""
        for svc in zen-brain1-scheduler zen-brain1-l1 zen-brain1-l2; do
            state=$(sudo systemctl is-active "$svc" 2>/dev/null)
            if [[ "$state" == "active" ]]; then ok "$svc: $state"; else fail "$svc: $state"; fi
        done
        ;;

    warmup)
        echo "Warming up L1..."
        resp=$(curl -sf --max-time 30 "http://localhost:56227/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -d '{"model":"Qwen3.5-0.8B-Q4_K_M.gguf","messages":[{"role":"user","content":"say ok"}],"max_tokens":5,"chat_template_kwargs":{"enable_thinking":false}}')
        echo "$resp" | head -c 200
        echo ""
        ok "L1 warmup complete"
        ;;

    health)
        bash "$(dirname "$0")/health-check.sh"
        ;;

    help|*)
        echo "zen-brain1 operator control panel"
        echo ""
        echo "Usage: $0 <command>"
        echo ""
        echo "Commands:"
        echo "  status    Show worker, scheduler, and timer status"
        echo "  schedule  Show active schedule from config/schedules/"
        echo "  run       Force immediate batch via internal scheduler (hourly|quad|daily)"
        echo "  logs      Tail logs (default: scheduler)"
        echo "  latest    Show latest artifacts from each batch"
        echo "  restart   Restart scheduler + L1 + L2 workers"
        echo "  warmup    Warmup L1 with a test request"
        echo "  health    Run health checks"
        echo ""
        echo "Architecture:"
        echo "  systemd → supervises processes (l1, l2, scheduler)"
        echo "  zen-brain scheduler → owns useful-task cadence"
        echo "  config/schedules/   → schedule definitions"
        ;;
esac
