#!/usr/bin/env bash
# zen-lock-source.sh — Source Jira credentials from the encrypted bundle.
#
# Usage:
#   eval "$(~/zen/zen-brain1/scripts/zen-lock-source.sh)"
#
# Or in a systemd service:
#   ExecStartPre=/bin/bash -c 'eval $(~/zen/zen-brain1/scripts/zen-lock-source.sh) && env > /run/zen-brain1/jira-runtime.env'
#   EnvironmentFile=/run/zen-brain1/jira-runtime.env
#
# This is the ONLY way local processes should get credentials.
# Never hardcode tokens in systemd units or env files.

set -euo pipefail

KEY_DIR="$HOME/zen/keys/zen-brain"
AGE_KEY="$KEY_DIR/credentials.key"
ENCRYPTED_BUNDLE="$KEY_DIR/secrets.d/jira.enc"

if [ ! -f "$AGE_KEY" ]; then
    echo "ERROR: Age key not found: $AGE_KEY" >&2
    exit 1
fi

if [ ! -f "$ENCRYPTED_BUNDLE" ]; then
    echo "ERROR: Encrypted bundle not found: $ENCRYPTED_BUNDLE" >&2
    echo "Run: ~/zen/zen-brain1/scripts/zen-lock-rotate.sh" >&2
    exit 1
fi

# Decrypt and export as shell variables
age -d -i "$AGE_KEY" "$ENCRYPTED_BUNDLE" 2>/dev/null | python3 -c "
import sys, json
creds = json.load(sys.stdin)
for k, v in creds.items():
    print(f'export {k}=\"{v}\"')
" 2>/dev/null || {
    echo "ERROR: Failed to decrypt credentials" >&2
    exit 1
}
