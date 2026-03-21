#!/usr/bin/env bash
# ZB-014 Bootstrap Script: Jira ZenLock from Local Files
# This script sets up Jira integration using ONLY local credential files.
# NEVER asks for credentials. NEVER prints secrets.

set -euo pipefail
umask 077

###############################################################################
# CONFIGURATION - Edit these paths if needed
###############################################################################
HOME="${HOME:-/root}"
ZEN_DIR="$HOME/zen"
ZB1_DIR="$ZEN_DIR/zen-brain1"
ZB0_DIR="$ZEN_DIR/zen-brain"
AGE_PRIV="$ZEN_DIR/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age"
AGE_PUB="$ZEN_DIR/ZENBRAINPUBLICKEYNEVERDELETETHISSHIT.age"
JIRA_TOKEN_FILE="$ZEN_DIR/DONOTASKMOREFORTHISSHIT.txt"
OUT_MANIFEST="$ZB1_DIR/deploy/zen-lock/jira-credentials.zenlock.yaml"
GEN_SCRIPT="$ZB1_DIR/deploy/zen-lock/generate-jira-secret.sh"

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

# Extract public key
age-keygen -y "$AGE_PRIV" > "$AGE_PUB"
chmod 600 "$AGE_PRIV" "$AGE_PUB"

# Validate
grep -q '^AGE-SECRET-KEY-1' "$AGE_PRIV" || { echo "ERROR: Invalid private key" >&2; exit 1; }
grep -q '^age1' "$AGE_PUB" || { echo "ERROR: Invalid public key" >&2; exit 1; }

# Create key-only file for k8s (no comments, no trailing newline)
KEY_ONLY=$(grep '^AGE-SECRET-KEY-1' "$AGE_PRIV")
printf '%s' "$KEY_ONLY" > "$AGE_PRIV"
echo "✓ AGE keypair ready"

###############################################################################
# PHASE 2: LOAD JIRA SETTINGS FROM LOCAL FILES
###############################################################################
echo "=== PHASE 2: Loading Jira settings ==="

pick_var() {
  local var="$1"; shift
  local f line val
  for f in "$@"; do
    [ -f "$f" ] || continue
    line="$(grep -m1 "^${var}=" "$f" 2>/dev/null || true)"
    [ -n "$line" ] || continue
    val="${line#*=}"
    [ -n "$val" ] && { printf '%s' "$val"; return 0; }
  done
  return 1
}

JIRA_URL="${JIRA_URL:-$(pick_var JIRA_URL "$HOME/.env.jira.local" "$ZB0_DIR/.env.jira.local" "$ZB1_DIR/.env.jira.local" 2>/dev/null || true)}"
JIRA_EMAIL="${JIRA_EMAIL:-$(pick_var JIRA_EMAIL "$HOME/.env.jira.local" "$ZB0_DIR/.env.jira.local" "$ZB1_DIR/.env.jira.local" 2>/dev/null || true)}"
JIRA_PROJECT_KEY="${JIRA_PROJECT_KEY:-$(pick_var JIRA_PROJECT_KEY "$HOME/.env.jira.local" "$ZB0_DIR/.env.jira.local" "$ZB1_DIR/.env.jira.local" 2>/dev/null || true)}"

: "${JIRA_URL:=https://zen-mesh.atlassian.net}"
: "${JIRA_PROJECT_KEY:=ZB}"

[ -s "$JIRA_TOKEN_FILE" ] || { echo "ERROR: Token file missing: $JIRA_TOKEN_FILE" >&2; exit 1; }
JIRA_API_TOKEN="$(tr -d '\r\n' < "$JIRA_TOKEN_FILE")"
[ -n "$JIRA_EMAIL" ] || { echo "ERROR: JIRA_EMAIL not found" >&2; exit 1; }

AGE_RECIPIENT="$(tr -d '\r\n' < "$AGE_PUB")"
echo "✓ Settings loaded from local files"

###############################################################################
# PHASE 3: GENERATE ZEN-LOCKED CREDENTIALS
###############################################################################
echo "=== PHASE 3: Generating zen-locked credentials ==="

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
# PHASE 4: UPDATE K8S SECRET
###############################################################################
echo "=== PHASE 4: Updating zen-lock master key ==="

SECRET_NAME="zen-lock-master-key"
KEY_FIELD="key.txt"

kubectl -n zen-lock-system create secret generic "$SECRET_NAME" \
  --from-file="${KEY_FIELD}=${AGE_PRIV}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n zen-lock-system rollout restart deployment/zen-lock-controller
kubectl -n zen-lock-system rollout status deployment/zen-lock-controller --timeout=180s

kubectl -n zen-lock-system rollout restart deployment/zen-lock-webhook
kubectl -n zen-lock-system rollout status deployment/zen-lock-webhook --timeout=180s

echo "✓ Zen-lock components updated"

###############################################################################
# PHASE 5: APPLY ZENLOCK AND FOREMAN
###############################################################################
echo "=== PHASE 5: Applying ZenLock ==="

kubectl apply -f "$OUT_MANIFEST"

kubectl label namespace zen-brain zen-lock=enabled --overwrite

kubectl apply -f - << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: foreman-config
  namespace: zen-brain
data:
  config.yaml: |
    jira:
      enabled: true
      base_url: "$JIRA_URL"
      email: "$JIRA_EMAIL"
      project_key: "$JIRA_PROJECT_KEY"
      credentials_dir: "/zen-lock/secrets"
      allow_env_fallback: false
      status_mapping:
        "To Do": "requested"
        "In Progress": "running"
        "Done": "completed"
        "Blocked": "blocked"
      worktype_mapping:
        "Bug": "debug"
        "Task": "implementation"
        "Story": "design"
        "Epic": "research"
      priority_mapping:
        "Highest": "critical"
        "High": "high"
        "Medium": "medium"
        "Low": "low"
        "Lowest": "background"
EOF

kubectl patch deployment foreman -n zen-brain --type merge -p '{
  "spec": {
    "template": {
      "metadata": {
        "annotations": {
          "zen-lock/inject": "jira-credentials",
          "zen-lock/mount-path": "/zen-lock/secrets"
        }
      },
      "spec": {
        "volumes": [
          {
            "name": "config-volume",
            "configMap": { "name": "foreman-config" }
          }
        ],
        "containers": [
          {
            "name": "foreman",
            "volumeMounts": [
              {
                "name": "config-volume",
                "mountPath": "/home/zenuser/.zen-brain/config.yaml",
                "subPath": "config.yaml"
              }
            ]
          }
        ]
      }
    }
  }
}' 2>/dev/null || true

kubectl rollout restart deployment/foreman -n zen-brain
kubectl rollout status deployment/foreman -n zen-brain --timeout=180s

echo "✓ Foreman deployment updated"

###############################################################################
# PHASE 6: VALIDATE
###############################################################################
echo "=== PHASE 6: Validating ==="

FOREMAN_POD=$(kubectl get pod -n zen-brain -l app.kubernetes.io/name=foreman -o jsonpath='{.items[0].metadata.name}')

kubectl exec -n zen-brain "$FOREMAN_POD" -- sh -c '
  set -eu
  test -d /zen-lock/secrets
  for f in JIRA_URL JIRA_EMAIL JIRA_API_TOKEN JIRA_PROJECT_KEY; do
    test -s "/zen-lock/secrets/$f"
  done
' && echo "✓ All secret files present"

kubectl exec -n zen-brain "$FOREMAN_POD" -- /app/zen-brain office doctor

echo ""
echo "=== SUCCESS ==="
echo "Jira zen-lock integration is ready!"
echo "Run 'kubectl exec -n zen-brain deployment/foreman -- /app/zen-brain office smoke-real' to validate API access."

###############################################################################
# PHASE 7: DELETE PLAINTEXT BOOTSTRAP FILE (ZB-025B-SEC)
###############################################################################
echo "=== PHASE 7: Securely removing plaintext bootstrap file ==="

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

echo "⚠ SECURITY REMINDER: Plaintext Jira credentials should NEVER be used again."
echo "⚠ All future Jira operations must use ZenLock-mounted credentials at /zen-lock/secrets"
