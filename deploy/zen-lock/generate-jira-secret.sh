#!/bin/bash
# Generate ZenLock resource for Jira credentials

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZEN_BRAIN_DIR="$(dirname "$SCRIPT_DIR/..")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== Jira ZenLock Secret Generator ==="
echo ""

# Check for zen-lock CLI
if ! command -v zen-lock &> /dev/null; then
    echo -e "${RED}Error: zen-lock CLI not found${NC}"
    echo "Install zen-lock first: https://github.com/kube-zen/zen-lock"
    exit 1
fi

# Check for existing private key
PRIVATE_KEY="$HOME/.zen-lock/private-key.age"
if [ ! -f "$PRIVATE_KEY" ]; then
    echo -e "${YELLOW}Generating zen-lock keypair...${NC}"
    zen-lock keygen --output "$PRIVATE_KEY"
    echo -e "${GREEN}✓ Keys generated${NC}"
    echo ""

    # Export public key
    PUBLIC_KEY="$HOME/.zen-lock/public-key.age"
    zen-lock pubkey --input "$PRIVATE_KEY" > "$PUBLIC_KEY"
    echo -e "${GREEN}✓ Public key: $PUBLIC_KEY${NC}"
    echo ""
fi

# Prompt for Jira credentials
echo "Enter Jira credentials (will be encrypted and stored as ZenLock CRD):"
echo ""

read -p "Jira Base URL (e.g., https://zen-mesh.atlassian.net): " JIRA_URL
read -p "Jira Email (e.g., zen@zen-mesh.io): " JIRA_EMAIL
read -sp "Jira API Token: " JIRA_TOKEN
echo ""
read -p "Jira Project Key (e.g., ZB): " JIRA_PROJECT_KEY
echo ""

# Validate inputs
if [ -z "$JIRA_URL" ] || [ -z "$JIRA_EMAIL" ] || [ -z "$JIRA_TOKEN" ] || [ -z "$JIRA_PROJECT_KEY" ]; then
    echo -e "${RED}Error: All fields are required${NC}"
    exit 1
fi

# Create secret YAML
SECRET_YAML="$ZEN_BRAIN_DIR/deploy/zen-lock/secrets/jira-secret.yaml.tmp"
mkdir -p "$(dirname "$SECRET_YAML")"

cat > "$SECRET_YAML" <<EOF
metadata:
  name: jira-credentials
  namespace: zen-brain
stringData:
  JIRA_URL: "$JIRA_URL"
  JIRA_EMAIL: "$JIRA_EMAIL"
  JIRA_API_TOKEN: "$JIRA_TOKEN"
  JIRA_PROJECT_KEY: "$JIRA_PROJECT_KEY"
EOF

# Encrypt the secret
PUBLIC_KEY="$HOME/.zen-lock/public-key.age"
ENCRYPTED_YAML="$ZEN_BRAIN_DIR/deploy/zen-lock/secrets/jira-zenlock.yaml"

echo -e "${YELLOW}Encrypting secret with zen-lock...${NC}"
zen-lock encrypt --pubkey "$(cat "$PUBLIC_KEY")" --input "$SECRET_YAML" --output "$ENCRYPTED_YAML"

# Clean up temp file
rm -f "$SECRET_YAML"

# Create ZenLock CRD manifest
ZENLOCK_YAML="$ZEN_BRAIN_DIR/deploy/zen-lock/jira-zenlock.yaml"

cat > "$ZENLOCK_YAML" <<'EOF'
apiVersion: security.kube-zen.io/v1alpha1
kind: ZenLock
metadata:
  name: jira-credentials
  namespace: zen-brain
spec:
  algorithm: age
  allowedSubjects:
    - kind: ServiceAccount
      name: zb-nightshift-sa
      namespace: zen-brain
    - kind: ServiceAccount
      name: zb-reporter-sa
      namespace: zen-brain
    - kind: ServiceAccount
      name: zb-planner-sa
      namespace: zen-brain
EOF

# Read encrypted data and append to ZenLock manifest
echo "  encryptedData:" >> "$ZENLOCK_YAML"
grep -A 1000 "encryptedData:" "$ENCRYPTED_YAML" | tail -n +2 >> "$ZENLOCK_YAML"

echo ""
echo -e "${GREEN}✓ ZenLock resource created${NC}"
echo "  Location: $ZENLOCK_YAML"
echo "  Encrypted secret: $ENCRYPTED_YAML"
echo ""
echo "To deploy to cluster:"
echo "  kubectl apply -f $ZENLOCK_YAML"
echo ""
echo "To rotate credentials:"
echo "  1. Run this script again with new credentials"
echo "  2. kubectl apply -f $ZENLOCK_YAML"
echo ""
echo "Allowed service accounts:"
echo "  - zb-nightshift-sa"
echo "  - zb-reporter-sa"
echo "  - zb-planner-sa"
echo ""
echo "To add more service accounts, edit the allowedSubjects list in $ZENLOCK_YAML"
