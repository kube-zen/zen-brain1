#!/bin/bash
# Runtime Health Check for Zen-Lock / Jira Integration
# Part of ZB-013 operational hardening
#
# Usage: ./scripts/runtime_health_check.sh [--json]

set -euo pipefail

NAMESPACE="${NAMESPACE:-zen-brain}"
ZENLOCK_NAMESPACE="${ZENLOCK_NAMESPACE:-zen-lock-system}"
OUTPUT_FORMAT="${1:-text}"

# Colors for text output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Results storage
declare -A RESULTS

check_pass() {
    RESULTS["$1"]="PASS"
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo -e "${GREEN}✓${NC} $2"
    fi
}

check_fail() {
    RESULTS["$1"]="FAIL"
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo -e "${RED}✗${NC} $2"
    fi
}

check_warn() {
    RESULTS["$1"]="WARN"
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo -e "${YELLOW}!${NC} $2"
    fi
}

# ============================================
# 1. ZenLock Controller Health
# ============================================
check_zenlock_controller() {
    local section="zenlock_controller"
    
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== ZenLock Controller ==="
    fi
    
    # Check controller pod is running
    local controller_status
    controller_status=$(kubectl get pods -n "$ZENLOCK_NAMESPACE" \
        -l app.kubernetes.io/component=controller \
        -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
    
    if [ "$controller_status" = "Running" ]; then
        check_pass "controller_running" "Controller pod running"
    else
        check_fail "controller_running" "Controller pod not running (status: $controller_status)"
    fi
    
    # Check restart count
    local restarts
    restarts=$(kubectl get pods -n "$ZENLOCK_NAMESPACE" \
        -l app.kubernetes.io/component=controller \
        -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
    
    if [ "$restarts" -lt 3 ]; then
        check_pass "controller_restarts" "Controller restarts: $restarts"
    else
        check_warn "controller_restarts" "High restart count: $restarts"
    fi
    
    # Check for recent errors in logs
    local error_count
    error_count=$(kubectl logs -n "$ZENLOCK_NAMESPACE" \
        -l app.kubernetes.io/component=controller \
        --tail=100 2>/dev/null | grep -c "ERROR\|error\|fatal" || echo "0")
    
    # Handle multi-line output from grep -c
    error_count=$(echo "$error_count" | head -1)
    
    if [ "$error_count" -lt 5 ]; then
        check_pass "controller_logs" "Controller log errors: $error_count (last 100 lines)"
    else
        check_warn "controller_logs" "Elevated log errors: $error_count"
    fi
}

# ============================================
# 2. ZenLock Webhook Health
# ============================================
check_zenlock_webhook() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== ZenLock Webhook ==="
    fi
    
    local webhook_status
    webhook_status=$(kubectl get pods -n "$ZENLOCK_NAMESPACE" \
        -l app.kubernetes.io/component=webhook \
        -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
    
    if [ "$webhook_status" = "Running" ]; then
        check_pass "webhook_running" "Webhook pod running"
    else
        check_fail "webhook_running" "Webhook pod not running (status: $webhook_status)"
    fi
    
    # Check webhook is reachable
    local webhook_ready
    webhook_ready=$(kubectl get endpoints zen-lock-webhook -n "$ZENLOCK_NAMESPACE" \
        -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null || echo "")
    
    if [ -n "$webhook_ready" ]; then
        check_pass "webhook_endpoints" "Webhook endpoints ready"
    else
        check_fail "webhook_endpoints" "Webhook has no endpoints"
    fi
}

# ============================================
# 3. ZenLock Resource Health
# ============================================
check_zenlock_resources() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== ZenLock Resources ==="
    fi
    
    # Check jira-credentials ZenLock exists and is Ready
    local phase
    phase=$(kubectl get zenlock jira-credentials -n "$NAMESPACE" \
        -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    
    if [ "$phase" = "Ready" ]; then
        check_pass "zenlock_ready" "jira-credentials ZenLock Ready"
    elif [ "$phase" = "NotFound" ]; then
        check_fail "zenlock_ready" "jira-credentials ZenLock not found"
    else
        check_fail "zenlock_ready" "jira-credentials ZenLock phase: $phase"
    fi
    
    # Check decryption condition
    local decrypt_status
    decrypt_status=$(kubectl get zenlock jira-credentials -n "$NAMESPACE" \
        -o jsonpath='{.status.conditions[?(@.type=="Decryptable")].status}' 2>/dev/null || echo "Unknown")
    
    if [ "$decrypt_status" = "True" ]; then
        check_pass "zenlock_decryptable" "ZenLock decryption working"
    else
        check_fail "zenlock_decryptable" "ZenLock decryption failed"
    fi
}

# ============================================
# 4. Foreman Pod Health
# ============================================
check_foreman_health() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== Foreman Deployment ==="
    fi
    
    # Check foreman deployment exists
    local replicas ready_replicas
    replicas=$(kubectl get deployment foreman -n "$NAMESPACE" \
        -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
    ready_replicas=$(kubectl get deployment foreman -n "$NAMESPACE" \
        -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    
    if [ "$ready_replicas" = "$replicas" ] && [ "$replicas" -gt 0 ]; then
        check_pass "foreman_ready" "Foreman deployment ready ($ready_replicas/$replicas)"
    else
        check_fail "foreman_ready" "Foreman not ready ($ready_replicas/$replicas)"
    fi
    
    # Check ZenLock injection annotation
    local annotation
    annotation=$(kubectl get deployment foreman -n "$NAMESPACE" \
        -o jsonpath='{.spec.template.metadata.annotations.zen-lock\/inject}' 2>/dev/null || echo "")
    
    if [ "$annotation" = "jira-credentials" ]; then
        check_pass "foreman_annotation" "Foreman has ZenLock injection annotation"
    else
        check_warn "foreman_annotation" "Foreman missing ZenLock annotation"
    fi
    
    # Check pod has secret volume
    local has_volume
    has_volume=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=foreman \
        -o jsonpath='{.items[0].spec.volumes[?(@.name=="zen-secrets")]}' 2>/dev/null || echo "")
    
    if [ -n "$has_volume" ]; then
        check_pass "foreman_volume" "Foreman pod has zen-secrets volume"
    else
        check_fail "foreman_volume" "Foreman pod missing zen-secrets volume"
    fi
}

# ============================================
# 5. Secret Injection Validation
# ============================================
check_secret_injection() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== Secret Injection ==="
    fi
    
    # Get a foreman pod name
    local pod
    pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=foreman \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$pod" ]; then
        check_fail "secrets_check" "No foreman pod to check"
        return
    fi
    
    # Check all four files exist
    local files=("JIRA_URL" "JIRA_EMAIL" "JIRA_API_TOKEN" "JIRA_PROJECT_KEY")
    local all_present=true
    
    for file in "${files[@]}"; do
        if kubectl exec -n "$NAMESPACE" "$pod" -- test -f "/zen-lock/secrets/$file" 2>/dev/null; then
            check_pass "secret_$file" "$file present in /zen-lock/secrets"
        else
            check_fail "secret_$file" "$file missing in /zen-lock/secrets"
            all_present=false
        fi
    done
    
    if [ "$all_present" = true ]; then
        check_pass "secrets_all" "All Jira credential files present"
    fi
}

# ============================================
# 6. Jira Connectivity (if binary exists)
# ============================================
check_jira_connectivity() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== Jira Connectivity ==="
    fi
    
    if [ ! -f "./bin/zen-brain" ]; then
        check_warn "jira_doctor" "zen-brain binary not found, skipping"
        return
    fi
    
    # Run office doctor
    if ./bin/zen-brain office doctor 2>&1 | grep -q "PASS"; then
        check_pass "jira_doctor" "office doctor: PASS"
    else
        check_fail "jira_doctor" "office doctor: FAIL"
    fi
    
    # Run smoke-real if available
    if ./bin/zen-brain office smoke-real 2>&1 | grep -q "PASS"; then
        check_pass "jira_smoke" "office smoke-real: PASS"
    else
        check_warn "jira_smoke" "office smoke-real: not passing or not available"
    fi
}

# ============================================
# Output Summary
# ============================================
print_summary() {
    local pass=0 fail=0 warn=0
    
    for key in "${!RESULTS[@]}"; do
        case "${RESULTS[$key]}" in
            PASS) ((pass++)) ;;
            FAIL) ((fail++)) ;;
            WARN) ((warn++)) ;;
        esac
    done
    
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo ""
        echo "=== Summary ==="
        echo -e "PASS: ${GREEN}$pass${NC}"
        echo -e "FAIL: ${RED}$fail${NC}"
        echo -e "WARN: ${YELLOW}$warn${NC}"
        
        if [ "$fail" -gt 0 ]; then
            echo ""
            echo "Action required. See BREAK_GLASS_RUNBOOK.md"
            exit 1
        fi
    elif [ "$OUTPUT_FORMAT" = "--json" ]; then
        echo "{\"pass\": $pass, \"fail\": $fail, \"warn\": $warn}"
    fi
}

# ============================================
# Main
# ============================================
main() {
    if [ "$OUTPUT_FORMAT" = "text" ]; then
        echo "Runtime Health Check - $(date)"
        echo "================================"
    fi
    
    check_zenlock_controller
    check_zenlock_webhook
    check_zenlock_resources
    check_foreman_health
    check_secret_injection
    check_jira_connectivity
    print_summary
}

main "$@"
