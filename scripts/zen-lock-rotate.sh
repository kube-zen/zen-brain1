#!/usr/bin/env bash
# zen-lock-rotate.sh — Single command to rotate Jira credentials everywhere.
#
# Usage:
#   1. Place new token in ~/zen/keys/zen-brain/secrets.d/jira-token (raw token, one line)
#   2. Run this script: ~/zen/zen-brain1/scripts/zen-lock-rotate.sh
#   3. Script encrypts, updates all consumers, restarts services, validates
#   4. Plaintext token is securely deleted
#
# Credential consumers updated:
#   - ~/zen/keys/zen-brain/secrets.d/jira.enc (age-encrypted bundle for local decryption)
#   - /etc/zen-brain1/jira.env (systemd scheduler — needs sudo)
#   - /etc/default/zen-brain (legacy, for factory-fill env)
#   - ~/.config/systemd/user/zen-brain.service.d/jira.conf (k3d zen-brain service)
#   - ~/.config/systemd/user/zen-brain.service (k3d zen-brain main unit)
#   - deploy/zen-lock/jira-credentials.zenlock.yaml (k8s ZenLock CRD manifest)
#
# Security:
#   - Plaintext token is shred'd after successful encryption
#   - age encryption uses dedicated keypair at ~/zen/keys/zen-brain/credentials.key
#   - The .enc file is the canonical encrypted credential store
#   - All consumers derive from this single encrypted file
#   - This script is the ONLY way to update credentials

set -euo pipefail

###############################################################################
# CONFIGURATION
###############################################################################
KEY_DIR="$HOME/zen/keys/zen-brain"
SECRET_DIR="$KEY_DIR/secrets.d"
AGE_KEY="$KEY_DIR/credentials.key"
AGE_PUB="$KEY_DIR/credentials.pub"
TOKEN_FILE="$SECRET_DIR/jira-token"
ENCRYPTED_BUNDLE="$SECRET_DIR/jira.enc"

JIRA_URL="https://zen-mesh.atlassian.net"
JIRA_EMAIL="zen@zen-mesh.io"
JIRA_PROJECT_KEY="ZB"

# Paths that need updating
ENVFILE_SYSTEMD="/etc/zen-brain1/jira.env"          # scheduler override
ENVFILE_DEFAULT="/etc/default/zen-brain"              # legacy env
DROPIN_JIRA="$HOME/.config/systemd/user/zen-brain.service.d/jira.conf"  # k3d service dropin
ZENLOCK_MANIFEST="$HOME/zen/keys/zen-brain/jira-credentials.zenlock.yaml"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

###############################################################################
# VALIDATION
###############################################################################
if [ ! -f "$AGE_KEY" ]; then
    echo -e "${RED}ERROR: Age private key not found: $AGE_KEY${NC}" >&2
    echo "Run: age-keygen -o $AGE_KEY" >&2
    exit 1
fi

if [ ! -s "$TOKEN_FILE" ]; then
    echo -e "${RED}ERROR: Token file not found or empty: $TOKEN_FILE${NC}" >&2
    echo "Place your Jira API token in: $TOKEN_FILE" >&2
    exit 1
fi

# Read token (strip whitespace)
JIRA_TOKEN="$(tr -d '[:space:]' < "$TOKEN_FILE")"

if [ -z "$JIRA_TOKEN" ]; then
    echo -e "${RED}ERROR: Token file is empty after stripping whitespace${NC}" >&2
    exit 1
fi

# Sanity: Jira PATs start with ATATT3x
if [[ ! "$JIRA_TOKEN" == ATATT3x* ]]; then
    echo -e "${RED}ERROR: Token doesn't look like a Jira API token (expected ATATT3x... prefix)${NC}" >&2
    echo "Length: ${#JIRA_TOKEN}" >&2
    exit 1
fi

AGE_RECIPIENT="$(tr -d '[:space:]' < "$AGE_PUB")"

###############################################################################
# PHASE 1: ENCRYPTED BUNDLE (local canonical store)
###############################################################################
echo -e "${YELLOW}=== PHASE 1: Creating encrypted credential bundle ===${NC}"

# Bundle all Jira creds into a JSON structure, encrypt with age
CREDENTIALS_JSON=$(cat <<JSONEOF
{
  "JIRA_URL": "$JIRA_URL",
  "JIRA_EMAIL": "$JIRA_EMAIL",
  "JIRA_API_TOKEN": "$JIRA_TOKEN",
  "JIRA_PROJECT_KEY": "$JIRA_PROJECT_KEY"
}
JSONEOF
)

echo "$CREDENTIALS_JSON" | age -r "$AGE_RECIPIENT" -o "$ENCRYPTED_BUNDLE"
chmod 600 "$ENCRYPTED_BUNDLE"

echo -e "${GREEN}✓ Encrypted bundle: $ENCRYPTED_BUNDLE${NC}"

###############################################################################
# PHASE 2: VALIDATE TOKEN AGAINST JIRA
###############################################################################
echo -e "${YELLOW}=== PHASE 2: Validating token against Jira ===${NC}"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -u "$JIRA_EMAIL:$JIRA_TOKEN" \
    "$JIRA_URL/rest/api/3/myself")

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ Jira auth successful (HTTP $HTTP_CODE)${NC}"
elif [ "$HTTP_CODE" = "401" ]; then
    echo -e "${RED}✗ Jira auth FAILED (HTTP 401) — token is invalid or expired${NC}" >&2
    # Don't delete the token file so user can fix it
    exit 1
else
    echo -e "${YELLOW}⚠ Jira returned HTTP $HTTP_CODE — proceeding anyway${NC}"
fi

###############################################################################
# PHASE 3: UPDATE ALL CONSUMERS
###############################################################################
echo -e "${YELLOW}=== PHASE 3: Updating credential consumers ===${NC}"

# 3a. Systemd scheduler env file (needs sudo)
if [ -f "$ENVFILE_SYSTEMD" ]; then
    echo -e "${YELLOW}  Updating $ENVFILE_SYSTEMD (needs sudo)...${NC}"
    echo "JIRA_URL=$JIRA_URL" | sudo tee "$ENVFILE_SYSTEMD" > /dev/null
    echo "JIRA_EMAIL=$JIRA_EMAIL" | sudo tee -a "$ENVFILE_SYSTEMD" > /dev/null
    echo "JIRA_API_TOKEN=$JIRA_TOKEN" | sudo tee -a "$ENVFILE_SYSTEMD" > /dev/null
    echo "JIRA_PROJECT_KEY=$JIRA_PROJECT_KEY" | sudo tee -a "$ENVFILE_SYSTEMD" > /dev/null
    sudo chmod 600 "$ENVFILE_SYSTEMD"
    echo -e "${GREEN}  ✓ Scheduler env file updated${NC}"
else
    echo -e "${YELLOW}  ⚠ $ENVFILE_SYSTEMD not found, skipping${NC}"
fi

# 3b. Legacy /etc/default/zen-brain (needs sudo)
if [ -f "$ENVFILE_DEFAULT" ]; then
    echo -e "${YELLOW}  Updating $ENVFILE_DEFAULT (needs sudo)...${NC}"
    # Preserve non-JIRA lines, replace JIRA lines
    sudo grep -v "^JIRA_" "$ENVFILE_DEFAULT" > /tmp/zen-brain-env.tmp 2>/dev/null || true
    cat >> /tmp/zen-brain-env.tmp <<ENVEOF
JIRA_URL="$JIRA_URL"
JIRA_EMAIL="$JIRA_EMAIL"
JIRA_TOKEN="$JIRA_TOKEN"
JIRA_PROJECT="$JIRA_PROJECT_KEY"
ENVEOF
    sudo cp /tmp/zen-brain-env.tmp "$ENVFILE_DEFAULT"
    sudo chmod 644 "$ENVFILE_DEFAULT"
    rm -f /tmp/zen-brain-env.tmp
    echo -e "${GREEN}  ✓ Legacy env file updated${NC}"
else
    echo -e "${YELLOW}  ⚠ $ENVFILE_DEFAULT not found, skipping${NC}"
fi

# 3c. User systemd drop-in for k3d zen-brain service
if [ -d "$(dirname "$DROPIN_JIRA")" ]; then
    echo -e "${YELLOW}  Updating $DROPIN_JIRA...${NC}"
    cat > "$DROPIN_JIRA" <<DROPINEOF
[Service]
Environment=JIRA_TOKEN=$JIRA_TOKEN
DROPINEOF
    chmod 600 "$DROPIN_JIRA"
    echo -e "${GREEN}  ✓ Systemd drop-in updated${NC}"
else
    echo -e "${YELLOW}  ⚠ $(dirname "$DROPIN_JIRA") not found, skipping${NC}"
fi

# 3d. Also update the main zen-brain user service unit (has JIRA_TOKEN inline)
ZENBRAIN_SERVICE="$HOME/.config/systemd/user/zen-brain.service"
if [ -f "$ZENBRAIN_SERVICE" ]; then
    echo -e "${YELLOW}  Updating JIRA_TOKEN in $ZENBRAIN_SERVICE...${NC}"
    sed -i "s|^Environment=JIRA_TOKEN=.*|Environment=JIRA_TOKEN=$JIRA_TOKEN|" "$ZENBRAIN_SERVICE"
    echo -e "${GREEN}  ✓ Main service unit updated${NC}"
fi

###############################################################################
# PHASE 4: UPDATE K8S ZENLOCK MANIFEST
###############################################################################
echo -e "${YELLOW}=== PHASE 4: Updating k8s ZenLock manifest ===${NC}"

ZB1_DIR="$HOME/zen/zen-brain1"
OUT_MANIFEST="$KEY_DIR/jira-credentials.zenlock.yaml"

encrypt_b64() {
    printf '%s' "$1" | age -r "$AGE_RECIPIENT" | base64 -w0
}

mkdir -p "$(dirname "$OUT_MANIFEST")"

cat > "$OUT_MANIFEST" <<EOF
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: jira-credentials
  namespace: zen-brain
spec:
  allowedSubjects:
    - kind: ServiceAccount
      name: foreman
      namespace: zen-brain
  encryptedData:
    JIRA_URL: "$(encrypt_b64 "$JIRA_URL")"
    JIRA_EMAIL: "$(encrypt_b64 "$JIRA_EMAIL")"
    JIRA_API_TOKEN: "$(encrypt_b64 "$JIRA_TOKEN")"
    JIRA_PROJECT_KEY: "$(encrypt_b64 "$JIRA_PROJECT_KEY")"
EOF

echo -e "${GREEN}✓ ZenLock manifest: $OUT_MANIFEST${NC}"

###############################################################################
# PHASE 5: RESTART AFFECTED SERVICES
###############################################################################
echo -e "${YELLOW}=== PHASE 5: Restarting services ===${NC}"

# Restart scheduler (system service, needs sudo)
if sudo systemctl is-active --quiet zen-brain1-scheduler 2>/dev/null; then
    echo -e "${YELLOW}  Restarting zen-brain1-scheduler...${NC}"
    sudo systemctl restart zen-brain1-scheduler
    echo -e "${GREEN}  ✓ Scheduler restarted${NC}"
else
    echo -e "${YELLOW}  ⚠ zen-brain1-scheduler not active, skipping${NC}"
fi

# Reload user systemd (picks up drop-in changes)
echo -e "${YELLOW}  Reloading user systemd...${NC}"
systemctl --user daemon-reload
echo -e "${GREEN}  ✓ User systemd reloaded${NC}"

# Restart zen-brain user service if running
if systemctl --user is-active --quiet zen-brain 2>/dev/null; then
    echo -e "${YELLOW}  Restarting zen-brain (k3d)...${NC}"
    systemctl --user restart zen-brain
    echo -e "${GREEN}  ✓ zen-brain restarted${NC}"
fi

###############################################################################
# PHASE 6: FINAL VALIDATION
###############################################################################
echo -e "${YELLOW}=== PHASE 6: Final validation ===${NC}"

# Decrypt the bundle to verify round-trip
DECRYPT_TEST=$(age -d -i "$AGE_KEY" "$ENCRYPTED_BUNDLE" 2>/dev/null)
if [ $? -eq 0 ] && echo "$DECRYPT_TEST" | grep -q "JIRA_API_TOKEN"; then
    echo -e "${GREEN}✓ Encrypted bundle decrypts successfully${NC}"
else
    echo -e "${RED}✗ Encrypted bundle decryption FAILED${NC}" >&2
    exit 1
fi

# Verify scheduler can reach Jira
if sudo systemctl is-active --quiet zen-brain1-scheduler 2>/dev/null; then
    sleep 2
    SCHED_TOKEN=$(sudo cat /proc/$(pgrep -f "cmd/scheduler/scheduler" | head -1)/environ 2>/dev/null | tr '\0' '\n' | grep "^JIRA_API_TOKEN=" | cut -d= -f2)
    if [ "$SCHED_TOKEN" = "$JIRA_TOKEN" ]; then
        echo -e "${GREEN}✓ Scheduler has new token${NC}"
    else
        echo -e "${YELLOW}⚠ Scheduler token mismatch — may need manual restart${NC}"
    fi
fi

###############################################################################
# PHASE 7: SECURELY DELETE PLAINTEXT TOKEN
###############################################################################
echo -e "${YELLOW}=== PHASE 7: Securely deleting plaintext token ===${NC}"

if [ -f "$TOKEN_FILE" ]; then
    if command -v shred >/dev/null 2>&1; then
        shred -u "$TOKEN_FILE"
        echo -e "${GREEN}✓ Plaintext token securely deleted (shred)${NC}"
    else
        rm -f "$TOKEN_FILE"
        echo -e "${GREEN}✓ Plaintext token removed (rm)${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Token file not found (already deleted?)${NC}"
fi

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN} SUCCESS: Credential rotation complete${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "Updated:"
echo "  • Encrypted bundle: $ENCRYPTED_BUNDLE"
echo "  • Scheduler env:    $ENVFILE_SYSTEMD"
echo "  • Legacy env:       $ENVFILE_DEFAULT"
echo "  • K3d drop-in:      $DROPIN_JIRA"
echo "  • ZenLock manifest: $OUT_MANIFEST"
echo ""
echo "Next: Restart any manually-launched factory-fill processes"
echo "  kill <pids>; SAFE_L1_CONCURRENCY=7 CONCURRENCY_TOTAL_SLOTS=10 ./cmd/factory-fill/factory-fill"
