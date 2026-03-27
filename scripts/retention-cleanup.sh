#!/bin/bash
# zen-brain1 run retention cleaner.
# Removes old scheduled batch runs while preserving important artifacts.
#
# Policy:
#   - Keep all runs from the last 24 hours
#   - Keep the latest successful run per schedule
#   - Keep the latest failed run per schedule
#   - Delete runs older than 7 days (configurable via RETENTION_DAYS)
#   - Dry run by default; set DRY_RUN=false to actually delete

set -euo pipefail

RUNS_ROOT="${RUNS_ROOT:-/var/lib/zen-brain1/runs}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
DRY_RUN="${DRY_RUN:-true}"
KEEP_HOURS="${KEEP_HOURS:-24}"

deleted=0
preserved=0
errors=0

log() { echo "[$(date +%Y%m%d-%H%M%S)] $*"; }

if [ ! -d "$RUNS_ROOT" ]; then
    log "Runs root not found: $RUNS_ROOT"
    exit 0
fi

# Find all run directories older than KEEP_HOURS
cutoff=$(date -d "${KEEP_HOURS} hours ago" +%s 2>/dev/null || date -v-${KEEP_HOURS}H +%s 2>/dev/null)

for schedule_dir in "$RUNS_ROOT"/*/; do
    [ -d "$schedule_dir" ] || continue
    schedule_name=$(basename "$schedule_dir")
    
    # Find latest successful and failed runs (preserve these)
    latest_success=""
    latest_failed=""
    
    for run_dir in "$schedule_dir"*/; do
        [ -d "$run_dir" ] || continue
        run_ts=$(basename "$run_dir")
        
        # Check if run has a success marker
        if [ -f "$run_dir/telemetry/batch-index.json" ]; then
            succeeded=$(python3 -c "import json,sys; d=json.load(open('$run_dir/telemetry/batch-index.json')); print(d.get('succeeded',0))" 2>/dev/null || echo "0")
            total=$(python3 -c "import json,sys; d=json.load(open('$run_dir/telemetry/batch-index.json')); print(d.get('total',0))" 2>/dev/null || echo "0")
            if [ "$succeeded" -gt 0 ] && [ "$succeeded" -ge "$total" ]; then
                [ -z "$latest_success" ] || [ "$run_ts" \> "$latest_success" ] && latest_success="$run_ts"
            else
                [ -z "$latest_failed" ] || [ "$run_ts" \> "$latest_failed" ] && latest_failed="$run_ts"
            fi
        fi
    done
    
    # Now delete old runs that aren't preserved
    for run_dir in "$schedule_dir"*/; do
        [ -d "$run_dir" ] || continue
        run_ts=$(basename "$run_dir")
        
        # Parse timestamp
        run_epoch=$(date -d "${run_ts:0:4}-${run_ts:4:2}-${run_ts:6:2} ${run_ts:9:2}:${run_ts:11:2}:${run_ts:13:2}" +%s 2>/dev/null || echo "0")
        
        # Skip if within keep window
        if [ "$run_epoch" -ge "$cutoff" ] 2>/dev/null; then
            preserved=$((preserved + 1))
            continue
        fi
        
        # Skip if latest success or failed for this schedule
        if [ "$run_ts" = "$latest_success" ] || [ "$run_ts" = "$latest_failed" ]; then
            log "PRESERVED: $schedule_dir$run_ts (latest success/failed)"
            preserved=$((preserved + 1))
            continue
        fi
        
        # Skip Jira-linked runs (have jira-ledger.json)
        if [ -f "$run_dir/telemetry/jira-ledger.json" ]; then
            # Check if Jira parent exists
            has_parent=$(python3 -c "import json,sys; d=json.load(open('$run_dir/telemetry/jira-ledger.json')); print('yes' if d.get('parent_jira_key') else 'no')" 2>/dev/null || echo "no")
            if [ "$has_parent" = "yes" ]; then
                # Still delete if older than RETENTION_DAYS
                retention_cutoff=$(date -d "${RETENTION_DAYS} days ago" +%s 2>/dev/null || echo "0")
                if [ "$run_epoch" -ge "$retention_cutoff" ] 2>/dev/null; then
                    log "PRESERVED: $schedule_dir$run_ts (Jira-linked, within retention)"
                    preserved=$((preserved + 1))
                    continue
                fi
            fi
        fi
        
        # Delete
        if [ "$DRY_RUN" = "false" ]; then
            if rm -rf "$run_dir" 2>/dev/null; then
                log "DELETED: $schedule_dir$run_ts"
                deleted=$((deleted + 1))
            else
                log "ERROR: failed to delete $schedule_dir$run_ts"
                errors=$((errors + 1))
            fi
        else
            log "WOULD DELETE: $schedule_dir$run_ts"
            deleted=$((deleted + 1))
        fi
    done
done

log "Retention complete: deleted=$deleted preserved=$preserved errors=$errors (dry_run=$DRY_RUN)"
