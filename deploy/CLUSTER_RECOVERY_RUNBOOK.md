# Cluster Recovery Runbook

## When cluster is nuked/recreated:

1. **Create namespaces:**
\`\`\`bash
kubectl create namespace zen-brain
kubectl label namespace zen-brain zen-lock=enabled
kubectl create namespace zen-lock-system
\`\`\`

2. **Install CRDs:**
\`\`\`bash
# BrainTask, BrainQueue, BrainPolicy, etc.
kubectl apply -f deployments/crds/*.yaml
# ZenLock CRDs
kubectl apply -f ~/zen/zen-lock/config/crd/bases/
\`\`\`

3. **Create master key secret:**
\`\`\`bash
kubectl create secret generic zen-lock-master-key -n zen-lock-system \\
  --from-file=key.txt=/home/neves/zen/ZENBRAINPRIVATEKEYNEVERDELETETHISSHIT.age
\`\`\`

4. **Create webhook certificate (with SANs):**
\`\`\`bash
cd /tmp
cat > cert.conf << 'CERT'
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = zen-lock-webhook.zen-lock-system.svc

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = zen-lock-webhook.zen-lock-system.svc
DNS.2 = zen-lock-webhook.zen-lock-system.svc.cluster.local
IP.1 = 127.0.0.1
CERT

openssl genrsa -out zen-lock-ca.key 2048
openssl req -new -x509 -key zen-lock-ca.key -out zen-lock-ca.crt -days 365 -subj "/CN=zen-lock-ca"
openssl genrsa -out zen-lock-webhook.key 2048
openssl req -new -key zen-lock-webhook.key -out zen-lock-webhook.csr -subj "/CN=zen-lock-webhook.zen-lock-system.svc"
openssl x509 -req -in zen-lock-webhook.csr -CA zen-lock-ca.crt -CAkey zen-lock-ca.key -CAcreateserial -out zen-lock-webhook.crt -days 365 -extensions SAN -extfile <(echo -e "[SAN]\\nsubjectAltName=DNS:zen-lock-webhook.zen-lock-system.svc,DNS:zen-lock-webhook.zen-lock-system.svc.cluster.local,IP:127.0.0.1\\n")
\`\`\`

5. **Create webhook cert secret:**
\`\`\`bash
kubectl create secret generic zen-lock-webhook-cert -n zen-lock-system \\
  --from-file=tls.crt=/tmp/zen-lock-webhook.crt \\
  --from-file=tls.key=/tmp/zen-lock-webhook.key \\
  --from-file=ca.crt=/tmp/zen-lock-ca.crt
\`\`\`

6. **Deploy ZenLock:**
\`\`\`bash
# RBAC and serviceaccounts
kubectl apply -f ~/zen/zen-lock/config/webhook/manifests.yaml
kubectl apply -f ~/zen/zen-lock/config/rbac/*.yaml
# Deployments with correct images
kubectl set image deployment/zen-lock-webhook webhook=zen-registry:5000/kubezen/zen-lock:0.0.3-alpha-zb025b-1 -n zen-lock-system
kubectl set image deployment/zen-lock-controller controller=zen-registry:5000/kubezen/zen-lock:0.0.3-alpha-zb025b-1 -n zen-lock-system
\`\`\`

7. **Add CA bundle to webhook config:**
\`\`\`bash
CABUNDLE=$(cat /tmp/zen-lock-ca.crt | base64 -w0 | tr -d '\\n')
kubectl patch mutatingwebhookconfigurations zen-lock-mutating-webhook --type='json' -p='[
  {
    "op": "add",
    "path": "/webhooks/0/clientConfig/caBundle",
    "value": "$CABUNDLE"
  }
]'
\`\`\`

8. **Create BrainPolicy:**
\`\`\`bash
kubectl apply -f - <(cat << 'POLICY'
apiVersion: zen.kube-zen.com/v1alpha1
kind: BrainPolicy
metadata:
  name: dogfood-default
  namespace: zen-brain
spec:
  rules:
    - name: allow-docs-update
      action: execute_task
      requiresApproval: false
      maxCostUSD: 0.01
      allowedModels:
        - qwen3.5:0.8b
        - zen-glm/GLM-5-turbo
    - name: allow-call-llm
      action: call_llm
      requiresApproval: false
      maxCostUSD: 0.10
      allowedModels:
        - qwen3.5:0.8b
        - zen-glm/GLM-5-turbo
POLICY
)
\`\`\`

9. **Deploy foreman:**
\`\`\`bash
kubectl apply -f deployments/k3d/foreman.yaml
# Note: Do NOT include manual zen-lock-secrets volumeMount
# Webhook will inject it automatically
\`\`\`

10. **Bootstrap Jira ZenLock:**
\`\`\`bash
cd ~/zen/zen-brain1
./deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
# Restart foreman to trigger webhook
kubectl rollout restart deployment/foreman -n zen-brain
\`\`\`

## Preflight Checks (MUST ALL PASS):

\`\`\`bash
# ZenLock components
kubectl get pods -n zen-lock-system | grep -E "controller|webhook" | grep Running
kubectl get deployment -n zen-lock-system zen-lock-controller -o jsonpath='{.status.readyReplicas}'
kubectl get deployment -n zen-lock-system zen-lock-webhook -o jsonpath='{.status.readyReplicas}'

# BrainPolicy exists
kubectl get brainpolicy dogfood-default -n zen-brain -o jsonpath='{.metadata.name}'

# Foreman healthy
kubectl get pods -n zen-brain -l app.kubernetes.io/name=foreman | grep Running

# Jira path
kubectl exec -n zen-brain deployment/foreman -- ./zen-brain office doctor | grep "API reachability: ok"
\`\`\`

## Post-Recovery Validation:

1. **Verify ZenLock injection:**
\`\`\`bash
kubectl exec -n zen-brain deployment/foreman -- ls -la /zen-lock/secrets/
# Should see: JIRA_API_TOKEN, JIRA_EMAIL, JIRA_URL, JIRA_PROJECT_KEY
\`\`\`

2. **Verify office path:**
\`\`\`bash
kubectl exec -n zen-brain deployment/foreman -- ./zen-brain office smoke-real
# Should see: API reachability: PASS
\`\`\`

3. **Verify qwen model (when Factory selects LLM):**
\`\`\`bash
kubectl logs -n zen-brain deployment/foreman --tail=200 | grep "intelligence selection"
# Should see: source=llm, model=qwen3.5:0.8b (NOT source=static)
\`\`\`
