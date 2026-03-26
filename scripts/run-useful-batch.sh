#!/bin/bash
set -e

TASKS_FILE="/home/neves/zen/zen-brain1/config/usefulness-tasks-v1.json"
OUTPUT_DIR="/tmp/zen-brain1-foreman-run/final"
DISPATCHER="/tmp/mlq-dispatcher"

echo "[P24C] Dispatching usefulness tasks through proven mlq-dispatcher path..."

# Execute mlq-dispatcher with tasks file
"$DISPATCHER" --tasks "$TASKS_FILE" \
    --output "$OUTPUT_DIR" \
    --parallel 10 \
    --model qwen3.5:0.8b-q4 \
    --endpoint http://localhost:56227 \
    --batch-id "useful-$(date +%s)"

EXIT_CODE=$?

# Check for artifacts
SUCCESS_COUNT=0
FAILED_COUNT=0

for task_id in useful-001 useful-002 useful-003; do
    ARTIFACT="$OUTPUT_DIR/${task_id}.md"
    if [ -f "$ARTIFACT" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        echo "✅ $task_id: artifact produced at $ARTIFACT"
    else
        FAILED_COUNT=$((FAILED_COUNT + 1))
        echo "❌ $task_id: no artifact"
    fi
done

echo "[P24C] BATCH COMPLETE: $SUCCESS_COUNT OK, $FAILED_COUNT FAIL"

# Write telemetry
cat > "/tmp/phase24c-useful-telemetry.json" << JSONEOF
{
  "batch_id": "useful-$(date +%s)",
  "session_id": "phase24c-useful",
  "tasks_total": 3,
  "tasks_ok": $SUCCESS_COUNT,
  "tasks_failed": $FAILED_COUNT,
  "task_shape": "usefulness-evidence-reporting (L1-first, markdown artifacts)",
  "config": {
    "l1_slots": 10,
    "l1_ctx_total": 65536,
    "l1_ctx_per_slot": 6556
  },
  "reuse_proof": "Reused proven mlq-dispatcher path (PHASE 22: 10/10 OK, 71s wall time)",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "tasks": [
    $(cat "$TASKS_FILE")
  ]
}
JSONEOF

exit $EXIT_CODE
