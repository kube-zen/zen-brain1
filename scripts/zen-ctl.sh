#!/bin/bash
# zen-brain1 operator control panel
# Usage: ./zen-ctl.sh {status|schedule|run|logs|latest|restart|warmup|health}
#
# All commands work with systemd services and timers.
# No need to manually manage llama-server processes.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

ok()  { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; }
warn() { echo -e "${YELLOW}⚠️  $1${NC}"; }

case "${1:-help}" in
    status)
        echo "=== Worker Services ==="
        for svc in zen-brain1-l1 zen-brain1-l2; do
            state=$(sudo systemctl is-active "$svc" 2>/dev/null || echo "unknown")
            enabled=$(sudo systemctl is-enabled "$svc" 2>/dev/null || echo "unknown")
            if [[ "$state" == "active" ]]; then ok "$svc: $state (enabled=$enabled)"; else fail "$svc: $state"; fi
        done
        echo ""
        echo "=== Schedule Timers ==="
        for tmr in zen-brain1-hourly-scan zen-brain1-quad-hourly-summary zen-brain1-daily-sweep; do
            state=$(sudo systemctl is-active "$tmr.timer" 2>/dev/null || echo "inactive")
            enabled=$(sudo systemctl is-enabled "$tmr.timer" 2>/dev/null || echo "unknown")
            echo "  $tmr: $state (enabled=$enabled)"
        done
        echo ""
        echo "=== Recent Batch Runs ==="
        sudo journalctl -u 'zen-brain1-*' --since "6 hours ago" --no-pager -o short-precise 2>/dev/null | grep -E "COMPLETE|FAIL" | tail -5
        ;;

    schedule)
        echo "=== Active Schedule ==="
        echo "  Hourly (defects, bug-hunting, stub-hunting):"
        echo "    → OnCalendar=hourly"
        echo "    → systemctl start zen-brain1-hourly-scan.service"
        echo ""
        echo "  Quad-Hourly (dead-code, tech-debt, hotspots, test-gaps, config-drift, roadmap):"
        echo "    → OnCalendar=0 */4 * * *"
        echo "    → systemctl start zen-brain1-quad-hourly-summary.service"
        echo ""
        echo "  Daily (all 10 tasks):"
        echo "    → OnCalendar=0 6 * * *"
        echo "    → systemctl start zen-brain1-daily-sweep.service"
        ;;

    run)
        # Force an immediate batch run
        BATCH="${2:-hourly-scan}"
        case "$BATCH" in
            hourly|scan) SVC="zen-brain1-hourly-scan";;
            quad|summary) SVC="zen-brain1-quad-hourly-summary";;
            daily|full|all) SVC="zen-brain1-daily-sweep";;
            *) echo "Unknown batch: $BATCH"; echo "Usage: $0 run {hourly|quad|daily}"; exit 1;;
        esac
        echo "Triggering $SVC..."
        sudo systemctl start "$SVC.service"
        echo "Started. Monitor with: $0 logs $SVC"
        ;;

    logs)
        SVC="${2:-zen-brain1-hourly-scan}"
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
                echo "  telemetry: $LATEST/telemetry/batch-index.json"
            fi
        done
        ;;

    restart)
        echo "Restarting workers..."
        sudo systemctl restart zen-brain1-l1 zen-brain1-l2
        sleep 15
        echo ""
        for svc in zen-brain1-l1 zen-brain1-l2; do
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
        echo "  status    Show worker and timer status"
        echo "  schedule  Show active schedule configuration"
        echo "  run       Force an immediate batch (hourly|quad|daily)"
        echo "  logs      Tail logs for a service (default: hourly-scan)"
        echo "  latest    Show latest artifacts from each batch"
        echo "  restart   Restart L1 and L2 workers"
        echo "  warmup    Warmup L1 with a test request"
        echo "  health    Run health checks"
        ;;
esac
