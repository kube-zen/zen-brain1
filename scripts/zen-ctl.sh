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
SCHEDULER_STATUS="/run/zen-brain1/scheduler/scheduler-status.json"
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
        echo "  (using canonical env path from /etc/zen-brain1/jira.env)"
        # Source the same env file the systemd service uses via EnvironmentFile
        # This ensures manual runs and scheduled runs share the same auth context
        JIRA_ENV="/etc/zen-brain1/jira.env"
        if [[ -f "$JIRA_ENV" ]]; then
            if [[ -r "$JIRA_ENV" ]]; then
                set -a; source "$JIRA_ENV"; set +a
                info "Jira env loaded from $JIRA_ENV (project=$JIRA_PROJECT_KEY, email=$JIRA_EMAIL)"
            else
                # File exists but not readable — load via sudo (does not expose secrets to shell history)
                eval "$(sudo cat "$JIRA_ENV" | sed 's/^/export /')"
                info "Jira env loaded via sudo from $JIRA_ENV (project=$JIRA_PROJECT_KEY, email=$JIRA_EMAIL)"
            fi
        else
            warn "No Jira env file at $JIRA_ENV — Jira integration will be disabled"
        fi
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
        echo "=== Latest Run Metrics ==="
        LATEST_METRICS="/var/lib/zen-brain1/metrics/latest-summary.json"
        if [[ -f "$LATEST_METRICS" ]]; then
            python3 -c "
import json, sys
d = json.load(open('$LATEST_METRICS'))
print(f'  Run ID:        {d.get(\"last_run_id\",\"?\")}')
print(f'  Schedule:      {d.get(\"last_schedule_name\",\"?\")}')
print(f'  Status:        {d.get(\"last_status\",\"?\")}')
print(f'  Wall Time:     {d.get(\"last_wall_time_seconds\",0)}s')
print(f'  Tasks:         {d.get(\"last_task_count_total\",0)} total, {d.get(\"last_l1_success_count\",0)} OK, {d.get(\"last_l1_fail_count\",0)} fail')
print(f'  Jira:          {d.get(\"last_jira_parent_key\",\"none\")} (+{d.get(\"last_jira_child_count\",0)} children)')
print(f'  Artifact Root: {d.get(\"last_artifact_root\",\"?\")}')
print(f'  Updated:       {d.get(\"updated_at\",\"?\")}')
"
        else
            warn "No latest metrics found. Run a batch first."
        fi
        echo ""
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

    metrics)
        echo "=== Rolling Metrics ==="
        HISTORY="/var/lib/zen-brain1/metrics/history.jsonl"
        LATEST="/var/lib/zen-brain1/metrics/latest-summary.json"

        if [[ -f "$LATEST" ]]; then
            echo ""
            echo "Latest Run:"
            python3 -c "
import json, sys
d = json.load(open('$LATEST'))
s = '✅' if d.get('last_status') == 'success' else '⚠️' if d.get('last_status') == 'partial' else '❌'
print(f'  {s} {d.get(\"last_schedule_name\",\"?\"):25s} run={d.get(\"last_run_id\",\"?\")} wall={d.get(\"last_wall_time_seconds\",0)}s tasks={d.get(\"last_l1_success_count\",0)}/{d.get(\"last_task_count_total\",0)} jira={d.get(\"last_jira_parent_key\",\"none\")}+{d.get(\"last_jira_child_count\",0)}')
"
        else
            warn "No latest metrics found."
        fi

        if [[ -f "$HISTORY" ]]; then
            echo ""
            echo "Run History (last 10):"
            python3 << 'PYEOF'
import json
with open("/var/lib/zen-brain1/metrics/history.jsonl") as f:
    lines = f.readlines()
for line in lines[-10:]:
    d = json.loads(line)
    s = "ok" if d.get("status") == "success" else ("partial" if d.get("status") == "partial" else "fail")
    icon = {"ok":"✅","partial":"⚠️","fail":"❌"}.get(s,"?")
    t = d.get("wall_time_seconds", 0)
    ok = d.get("task_count_l1_success", 0)
    tot = d.get("task_count_total", 0)
    jk = d.get("jira_parent_issue_key", "none")
    jc = d.get("jira_child_issue_count", 0)
    nm = d.get("schedule_name", "?")
    rid = d.get("run_id", "?")
    print(f"  {icon} {nm:25s} {rid:20s} {t:>5d}s {ok}/{tot} jira={jk}+{jc}")
print(f"\n  Total runs: {len(lines)}")
PYEOF
        fi
        echo ""
        echo "Files:"
        echo "  Latest:  $LATEST"
        echo "  History: $HISTORY"
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
        echo "  metrics   Show rolling metrics and run history"
        echo ""
        echo "Metrics:"
        echo "  /var/lib/zen-brain1/metrics/latest-summary.json"
        echo "  /var/lib/zen-brain1/metrics/history.jsonl"
        echo "  <run-dir>/telemetry/run-metrics.json  (per-run canonical metrics)"
        echo "  <run-dir>/final/run-summary.md         (per-run human-readable summary)"
        echo ""
        echo "Architecture:"
        echo "  systemd → supervises processes (l1, l2, scheduler)"
        echo "  zen-brain scheduler → owns useful-task cadence"
        echo "  config/schedules/   → schedule definitions"
        ;;
esac
