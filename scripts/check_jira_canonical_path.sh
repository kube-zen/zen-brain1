#!/usr/bin/env bash
# ZB-025A: Jira Canonical Path Sanity Check
# Validates that Jira credentials are using ONLY the canonical source of truth.
# This script should be run by every AI/operator before working on Jira.

set -euo pipefail

###############################################################################
# CONFIGURATION
###############################################################################
HOME="${HOME:-/root}"
ZEN_DIR="$HOME/zen"
ZB1_DIR="$ZEN_DIR/zen-brain1"
AGE_PRIV="$ZEN_DIR/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age"
AGE_PUB="$ZEN_DIR/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age"
JIRA_TOKEN_FILE="$ZEN_DIR/DONOTASKMOREFORTHISSHIT.txt"
WRONG_PATH="$HOME/.zen-brain/secrets/jira.yaml"

echo "=== ZB-025A: Jira Canonical Path Sanity Check ==="
echo ""

###############################################################################
# PHASE 1: LOCAL SOURCE FILES
###############################################################################
echo "PHASE 1: Local Source Files"
echo "--------------------------------"

PASS=1

# Check AGE private key
if [ -s "$AGE_PRIV" ]; then
  echo "✓ AGE private key: $AGE_PRIV"
else
  echo "✗ MISSING: AGE private key: $AGE_PRIV"
  PASS=0
fi

# Check AGE public key
if [ -s "$AGE_PUB" ]; then
  echo "✓ AGE public key: $AGE_PUB"
else
  echo "✗ MISSING: AGE public key: $AGE_PUB"
  PASS=0
fi

# Check Jira token file
if [ -s "$JIRA_TOKEN_FILE" ]; then
  echo "✓ Jira token file: $JIRA_TOKEN_FILE"
else
  echo "✗ MISSING: Jira token file: $JIRA_TOKEN_FILE"
  PASS=0
fi

echo ""

###############################################################################
# PHASE 2: ZENLOCK RUNTIME SOURCE
###############################################################################
echo "PHASE 2: ZenLock Runtime Source"
echo "--------------------------------"

# Check if we're in a cluster
IN_CLUSTER=false
if [ -n "${KUBERNETES_SERVICE_HOST:-}" ] || [ -n "${CONTAINER_NAME:-}" ] || [ -n "${CLUSTER_ID:-}" ]; then
  IN_CLUSTER=true
  echo "Running in cluster mode: YES"
else
  echo "Running in cluster mode: NO (local dev)"
fi
echo ""

# Check for ZenLock secrets in cluster
if [ "$IN_CLUSTER" = true ]; then
  if [ -d /zen-lock/secrets ]; then
    echo "✓ ZenLock secrets directory: /zen-lock/secrets"
  else
    echo "✗ MISSING: ZenLock secrets directory: /zen-lock/secrets"
    echo "  Cluster mode REQUIRES this directory to exist"
    PASS=0
  fi

  # Check for required secret files
  if [ -s /zen-lock/secrets/JIRA_URL ]; then
    echo "✓ /zen-lock/secrets/JIRA_URL"
  else
    echo "✗ MISSING: /zen-lock/secrets/JIRA_URL"
    PASS=0
  fi

  if [ -s /zen-lock/secrets/JIRA_EMAIL ]; then
    echo "✓ /zen-lock/secrets/JIRA_EMAIL"
  else
    echo "✗ MISSING: /zen-lock/secrets/JIRA_EMAIL"
    PASS=0
  fi

  if [ -s /zen-lock/secrets/JIRA_API_TOKEN ]; then
    echo "✓ /zen-lock/secrets/JIRA_API_TOKEN"
  else
    echo "✗ MISSING: /zen-lock/secrets/JIRA_API_TOKEN"
    PASS=0
  fi

  if [ -s /zen-lock/secrets/JIRA_PROJECT_KEY ]; then
    echo "✓ /zen-lock/secrets/JIRA_PROJECT_KEY"
  else
    echo "✗ MISSING: /zen-lock/secrets/JIRA_PROJECT_KEY"
    PASS=0
  fi
else
  echo "  (Skipping ZenLock checks - not in cluster)"
fi

echo ""

###############################################################################
# PHASE 3: FORBIDDEN PATH CHECK
###############################################################################
echo "PHASE 3: Forbidden Path Check"
echo "--------------------------------"

if [ -e "$WRONG_PATH" ]; then
  echo "✗ FORBIDDEN: $WRONG_PATH"
  echo "  This file should NOT exist"
  echo "  The ONLY canonical runtime source is: zenlock-dir:/zen-lock/secrets"
  echo ""
  echo "  If you have this file, DELETE it:"
  echo "    rm $WRONG_PATH"
  echo ""
  echo "  Or move it to the canonical local source:"
  echo "    cp $WRONG_PATH $JIRA_TOKEN_FILE"
  echo "    rm $WRONG_PATH"
  PASS=0
else
  echo "✓ No forbidden path found"
fi

echo ""

###############################################################################
# PHASE 4: OFFICE DOCTOR VALIDATION
###############################################################################
echo "PHASE 4: Office Doctor Validation"
echo "--------------------------------"

if [ -x "$ZB1_DIR/zen-brain" ] || [ -x "$ZB1_DIR/bin/zen-brain" ]; then
  ZENBRAIN="$ZB1_DIR/zen-brain"
  if [ ! -x "$ZENBRAIN" ]; then
    ZENBRAIN="$ZB1_DIR/bin/zen-brain"
  fi

  echo "Running: $ZENBRAIN office doctor"
  echo ""

  if $ZENBRAIN office doctor 2>&1; then
    echo ""
    echo "✓ Office doctor: PASS"
  else
    echo ""
    echo "✗ Office doctor: FAIL"
    echo "  Check output above for details"
    PASS=0
  fi
else
  echo "⚠ zen-brain binary not found, skipping office doctor"
  echo "  Build with: cd $ZB1_DIR && make build"
fi

echo ""

###############################################################################
# SUMMARY
###############################################################################
echo "=== SUMMARY ==="
echo ""

if [ "$PASS" -eq 1 ]; then
  echo "✓✓✓ ALL CHECKS PASS ✓✓✓"
  echo ""
  echo "Jira credentials are using canonical source of truth:"
  echo "  - Local: $JIRA_TOKEN_FILE"
  echo "  - Runtime: zenlock-dir:/zen-lock/secrets"
  echo ""
  echo "You are ready to work with Jira!"
  exit 0
else
  echo "✗✗✗ CHECKS FAILED ✗✗✗"
  echo ""
  echo "Jira credential path is NOT canonical. Fix issues above:"
  echo ""
  echo "To set up Jira correctly, run:"
  echo "  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh"
  echo ""
  echo "Required local files:"
  echo "  - $JIRA_TOKEN_FILE (contains JIRA_API_TOKEN)"
  echo "  - $AGE_PRIV (AGE private key)"
  echo "  - $AGE_PUB (AGE public key)"
  echo ""
  echo "Canonical runtime source (in cluster):"
  echo "  - /zen-lock/secrets (mounted via ZenLock)"
  echo ""
  echo "Forbidden path (should NOT exist):"
  echo "  - $WRONG_PATH"
  echo ""
  exit 1
fi
