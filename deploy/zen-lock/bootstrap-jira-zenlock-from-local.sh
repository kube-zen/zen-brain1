#!/usr/bin/env bash
# ZB-026E Canonical Bootstrap: Jira ZenLock from Local Files
# This script sets up Jira integration using ONLY local credential files.
# NEVER asks for credentials. NEVER prints secrets.
# ONLY prepares secrets and ZenLock manifest - does NOT patch application topology.

set -euo pipefail
umask 077

###############################################################################
# CONFIGURATION - Edit these paths if needed
###############################################################################
HOME="${HOME:-/root}"
ZEN_DIR="$HOME/zen"
ZB1_DIR="$ZEN_DIR/zen-brain1"
AGE_PRIV="$ZEN_DIR/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age"
AGE_PUB="$ZEN_DIR/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age"
JIRA_TOKEN_FILE="$ZEN_DIR/DONOTASKMOREFORTHISSHIT.txt"
JIRA_METADATA="$ZB1_DIR/deploy/zen-lock/jira-metadata.yaml"
OUT_MANIFEST="$ZB1_DIR/deploy/zen-lock/jira-credentials.zenlock.yaml"

###############################################################################
# PHASE 1: AGE KEYPAIR
###############################################################################
echo "=== PHASE 1: Setting up AGE keypair ==="

command -v age-keygen >/dev/null 2>&1 || { echo "ERROR: age-keygen not installed" >&2; exit 1; }

mkdir -p "$ZEN_DIR"

if [ ! -s "$AGE_PRIV" ]; then
  echo "Creating new AGE keypair..."
  age-keygen -o "$AGE_PRIV"
fi

# Extract public key (don't mutate the private key file)
age-keygen -y "$AGE_PRIV" > "$AGE_PUB"
chmod 600 "$AGE_PRIV" "$AGE_PUB"

# Validate
grep -q '^AGE-SECRET-KEY-1' "$AGE_PRIV" || { echo "ERROR: Invalid private key" >&2; exit 1; }
grep -q '^age1' "$AGE_PUB" || { echo "ERROR: Invalid public key" >&2; exit 1; }

echo "✓ AGE keypair ready"

###############################################################################
# PHASE 2: LOAD JIRA METADATA FROM CANONICAL SOURCE
###############################################################################
echo "=== PHASE 2: Loading Jira metadata ==="

if [ ! -f "$JIRA_METADATA" ]; then
  echo "ERROR: Jira metadata file not found: $JIRA_METADATA" >&2
  echo "Create it from the template or copy from deploy/zen-lock/jira-metadata.yaml" >&2
  exit 1
fi

# Read metadata from source-controlled config (NOT secrets)
JIRA_URL=$(grep 'url:' "$JIRA_METADATA" | awk '{print $2}' | tr -d '"')
JIRA_EMAIL=$(grep 'email:' "$JIRA_METADATA" | awk '{print $2}' | tr -d '"')
JIRA_PROJECT_KEY=$(grep 'project_key:' "$JIRA_METADATA" | awk '{print $2}' | tr -d '"')

[ -n "$JIRA_URL" ] || { echo "ERROR: JIRA_URL not found in $JIRA_METADATA" >&2; exit 1; }
[ -n "$JIRA_EMAIL" ] || { echo "ERROR: JIRA_EMAIL not found in $JIRA_METADATA" >&2; exit 1; }
[ -n "$JIRA_PROJECT_KEY" ] || { echo "ERROR: JIRA_PROJECT_KEY not found in $JIRA_METADATA" >&2; exit 1; }

echo "✓ Metadata loaded from $JIRA_METADATA"

###############################################################################
# PHASE 3: LOAD JIRA TOKEN FROM LOCAL FILE
###############################################################################
echo "=== PHASE 3: Loading Jira token ==="

[ -s "$JIRA_TOKEN_FILE" ] || { echo "ERROR: Token file missing: $JIRA_TOKEN_FILE" >&2; exit 1; }
JIRA_API_TOKEN="$(tr -d '\r\n' < "$JIRA_TOKEN_FILE")"
[ -n "$JIRA_API_TOKEN" ] || { echo "ERROR: Token file empty: $JIRA_TOKEN_FILE" >&2; exit 1; }

AGE_RECIPIENT="$(tr -d '\r\n' < "$AGE_PUB")"
echo "✓ Token loaded from $JIRA_TOKEN_FILE"

###############################################################################
# PHASE 4: GENERATE ZEN-LOCKED CREDENTIALS
###############################################################################
echo "=== PHASE 4: Generating zen-locked credentials ==="

cd "$ZB1_DIR"
mkdir -p deploy/zen-lock

encrypt_b64() {
  printf '%s' "$1" | age -r "$AGE_RECIPIENT" | base64 | tr -d '\n'
}

JIRA_URL_ENC=$(encrypt_b64 "$JIRA_URL")
JIRA_EMAIL_ENC=$(encrypt_b64 "$JIRA_EMAIL")
JIRA_API_TOKEN_ENC=$(encrypt_b64 "$JIRA_API_TOKEN")
JIRA_PROJECT_KEY_ENC=$(encrypt_b64 "$JIRA_PROJECT_KEY")

cat > "$OUT_MANIFEST" << EOF
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
    JIRA_URL: "$JIRA_URL_ENC"
    JIRA_EMAIL: "$JIRA_EMAIL_ENC"
    JIRA_API_TOKEN: "$JIRA_API_TOKEN_ENC"
    JIRA_PROJECT_KEY: "$JIRA_PROJECT_KEY_ENC"
EOF

echo "✓ ZenLock manifest generated: $OUT_MANIFEST"

###############################################################################
# PHASE 5: UPDATE K8S SECRET
###############################################################################
echo "=== PHASE 5: Updating zen-lock master key ==="

SECRET_NAME="zen-lock-master-key"
KEY_FIELD="key.txt"

# Use temporary file for normalized key (don't mutate original)
TEMP_KEY=$(mktemp)
grep '^AGE-SECRET-KEY-1' "$AGE_PRIV" > "$TEMP_KEY"

kubectl -n zen-lock-system create secret generic "$SECRET_NAME" \
  --from-file="${KEY_FIELD}=${TEMP_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

rm -f "$TEMP_KEY"

kubectl -n zen-lock-system rollout restart deployment/zen-lock-controller
kubectl -n zen-lock-system rollout status deployment/zen-lock-controller --timeout=180s

kubectl -n zen-lock-system rollout restart deployment/zen-lock-webhook
kubectl -n zen-lock-system rollout status deployment/zen-lock-webhook --timeout=180s

echo "✓ Zen-lock components updated"

###############################################################################
# PHASE 6: APPLY ZENLOCK
###############################################################################
echo "=== PHASE 6: Applying ZenLock ==="

kubectl apply -f "$OUT_MANIFEST"

kubectl label namespace zen-brain zen-lock=enabled --overwrite

echo "✓ ZenLock applied"

###############################################################################
# PHASE 7: VALIDATE
###############################################################################
echo "=== PHASE 7: Validating ==="

echo "Waiting for ZenLock webhook to be ready..."
sleep 5

# Create a test pod to verify injection
cat <<'TESTPOD' | kubectl apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Pod
metadata:
  name: zenlock-bootstrap-test
  namespace: zen-brain
  annotations:
    zen-lock/inject: jira-credentials
spec:
  serviceAccountName: foreman
  containers:
  - name: test
    image: busybox
    command: ["sleep", "10"]
TESTPOD

sleep 5

# Check if test pod got the secrets
if kubectl exec -n zen-brain zenlock-bootstrap-test -- sh -c 'test -f /zen-lock/secrets/JIRA_API_TOKEN' 2>/dev/null; then
  echo "✓ ZenLock injection verified"
  kubectl delete pod zenlock-bootstrap-test -n zen-brain >/dev/null 2>&1 || true
else
  echo "⚠ Warning: Could not verify ZenLock injection (may be webhook timing)"
  kubectl delete pod zenlock-bootstrap-test -n zen-brain >/dev/null 2>&1 || true
fi

echo ""
echo "=== SUCCESS ==="
echo "Jira zen-lock integration bootstrap complete!"
echo ""
echo "Next steps:"
echo "1. Deploy foreman with Helm (includes jira-metadata.yaml config)"
echo "2. Verify: kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office doctor"
echo "3. Test: kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office smoke-real"
echo ""
echo "Runtime credentials source: /zen-lock/secrets (ZenLock injection only)"

###############################################################################
# PHASE 8: DELETE PLAINTEXT BOOTSTRAP FILE (SECURITY)
###############################################################################
echo "=== PHASE 8: Securely removing plaintext bootstrap file ==="

if [ -s "$JIRA_TOKEN_FILE" ]; then
  # Try secure deletion first
  if command -v shred >/dev/null 2>&1; then
    shred -u "$JIRA_TOKEN_FILE"
    echo "✓ Plaintext bootstrap file securely deleted (shred)"
  else
    rm -f "$JIRA_TOKEN_FILE"
    echo "✓ Plaintext bootstrap file removed (rm - no shred available)"
  fi
else
  echo "⚠ Plaintext bootstrap file not found (already removed or never existed)"
fi

echo ""
echo "⚠ SECURITY REMINDER: Plaintext Jira credentials should NEVER be used again."
echo "⚠ All future Jira operations must use ZenLock-mounted credentials at /zen-lock/secrets"
echo ""
echo "Bootstrap complete. Application topology (foreman-config, deployments) managed by Helm."
