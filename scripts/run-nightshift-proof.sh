#!/bin/bash
# Quick reference: Run tonight's self-improvement proof test

set -e

echo "=== Zen-Brain Self-Improvement Proof Test ==="
echo "Started: $(date)"
echo ""

# Step 1: Verify Jira integration
echo "[1/4] Verifying Jira integration..."
export JIRA_URL=https://zen-mesh.atlassian.net
export JIRA_EMAIL=zen@zen-mesh.io
export JIRA_PROJECT_KEY=ZB
export JIRA_TOKEN=$(grep "^token:" ~/.zen-brain1-config/jira.yaml | awk '{print $2}' | tr -d '"')
./bin/zen-brain office doctor | head -20
echo "✅ Jira integration verified"
echo ""

# Step 2: Check for nightshift tickets
echo "[2/4] Checking for nightshift tickets..."
./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"'

TICKET_COUNT=$(./bin/zen-brain office search 'project = ZB AND labels = "zen-brain-nightshift"' | grep "Found" | awk '{print $2}')

if [ "$TICKET_COUNT" -eq 0 ]; then
    echo ""
    echo "❌ No nightshift tickets found!"
    echo ""
    echo "Please create 3-5 tickets with label: zen-brain-nightshift"
    echo "See: docs/05-OPERATIONS/NIGHTSHIFT_TONIGHT.md"
    echo ""
    exit 1
fi

echo "✅ Found $TICKET_COUNT nightshift ticket(s)"
echo ""

# Step 3: Run self-improvement loop
echo "[3/4] Running self-improvement loop..."
export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1
export ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1
export ZEN_BRAIN_WORKER_ID=zb-self-improvement-1

LOG_FILE="/tmp/zen-brain-nightshift-$(date +%Y%m%d).log"

./bin/zen-brain self-improvement | tee "$LOG_FILE"

echo ""
echo "✅ Self-improvement loop complete"
echo ""

# Step 4: Show morning report
echo "[4/4] Morning Report:"
echo ""
grep -A 20 "=== Morning Report ===" "$LOG_FILE" || echo "No morning report found"

echo ""
echo "=== Proof Test Complete ==="
echo "Finished: $(date)"
echo ""
echo "Log file: $LOG_FILE"
echo ""
echo "Next steps:"
echo "1. Check Jira for comments/artifacts on each ticket"
echo "2. Review morning report for useful summaries"
echo "3. Verify no Class C tasks were processed"
echo "4. Verify worker identity (zb-self-improvement-1) is visible"
echo ""
