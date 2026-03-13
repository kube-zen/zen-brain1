#!/bin/bash
# Canonical Jira credential consumption for Zen-Brain (host runtime)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZEN_BRAIN_DIR="$(dirname "$SCRIPT_DIR/..")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== Zen-Brain Jira Credential Loader ==="
echo ""

# Check runtime mode
if [ -n "$KUBERNETES_SERVICE_HOST" ]; then
    echo -e "${GREEN}Detected Kubernetes runtime${NC}"
    echo "  Jira credentials will be consumed from ZenLock-injected env vars"
    echo "  No action needed - Zen-Brain will read from env vars"
    echo ""
    exit 0
fi

# Host runtime mode
echo -e "${YELLOW}Detected host runtime mode${NC}"
echo ""

# Check for ZenLock-managed env file
ENV_FILE="$HOME/.zen-brain/jira-credentials.env"

if [ -f "$ENV_FILE" ]; then
    echo -e "${GREEN}Loading Jira credentials from: $ENV_FILE${NC}"
    source "$ENV_FILE"
    echo -e "${GREEN}✓ Credentials loaded${NC}"
    echo ""

    # Export for child processes
    export JIRA_URL="$JIRA_URL"
    export JIRA_EMAIL="$JIRA_EMAIL"
    export JIRA_TOKEN="$JIRA_TOKEN"
    export JIRA_PROJECT_KEY="$JIRA_PROJECT_KEY"

    # Validate
    echo "Validating credentials..."
    echo "  JIRA_URL: $JIRA_URL"
    echo "  JIRA_EMAIL: $JIRA_EMAIL"
    echo "  JIRA_PROJECT_KEY: $JIRA_PROJECT_KEY"
    echo "  JIRA_TOKEN: ${JIRA_TOKEN:0:10}..."  # Show first 10 chars only
    echo ""

    # Check for missing values
    if [ -z "$JIRA_URL" ] || [ -z "$JIRA_EMAIL" ] || [ -z "$JIRA_TOKEN" ] || [ -z "$JIRA_PROJECT_KEY" ]; then
        echo -e "${RED}Error: One or more Jira credentials are missing${NC}"
        echo ""
        echo "Check $ENV_FILE and ensure all required fields are set:"
        echo "  JIRA_URL"
        echo "  JIRA_EMAIL"
        echo "  JIRA_TOKEN"
        echo "  JIRA_PROJECT_KEY"
        exit 1
    fi

    echo -e "${GREEN}✓ All credentials present${NC}"
    echo ""
    echo "Credentials are now available in environment for Zen-Brain commands"
    echo ""
    echo "Example usage:"
    echo "  ./bin/zen-brain office doctor"
    echo "  ./bin/zen-brain self-improvement"
    echo ""
else
    echo -e "${YELLOW}Jira credential file not found: $ENV_FILE${NC}"
    echo ""
    echo "For Kubernetes runtime:"
    echo "  1. Deploy ZenLock resource: kubectl apply -f deploy/zen-lock/jira-zenlock.yaml"
    echo "  2. Zen-Brain will consume credentials from env vars injected by ZenLock"
    echo ""
    echo "For host runtime:"
    echo "  1. Create credential file at: $ENV_FILE"
    echo "  2. Add the following content:"
    echo ""
    cat <<'EXAMPLE'
JIRA_URL="https://zen-mesh.atlassian.net"
JIRA_EMAIL="zen@zen-mesh.io"
JIRA_TOKEN="REDACTED_TOKEN_0E2E83FF"
JIRA_PROJECT_KEY="ZB"
EXAMPLE
    echo ""
    echo "IMPORTANT: Never commit $ENV_FILE to Git"
    echo "IMPORTANT: Add $ENV_FILE to .gitignore"
    echo ""
    exit 1
fi
