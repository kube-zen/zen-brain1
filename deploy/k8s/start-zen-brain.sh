#!/bin/bash
# Zen-Brain 1.0 Startup Script
# Bootstraps all infrastructure and services for zen-brain1
#
# Usage: ./start-zen-brain.sh [phase]
#   phase: infra | services | all (default: all)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
KUBECTL="kubectl --context k3d-zen-brain"
NAMESPACE="zen-brain"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check k3d cluster
    if ! k3d cluster list | grep -q "zen-brain"; then
        log_error "k3d cluster 'zen-brain' not found. Create it first:"
        echo "  k3d cluster create zen-brain -p 8080:80@loadbalancer -p 8443:443@loadbalancer"
        exit 1
    fi
    
    # Check kubectl context
    if ! $KUBECTL cluster-info &>/dev/null; then
        log_error "Cannot connect to k3d-zen-brain cluster"
        exit 1
    fi
    
    log_success "Prerequisites OK"
}

create_namespace() {
    log_info "Creating namespace..."
    $KUBECTL create namespace $NAMESPACE --dry-run=client -o yaml | $KUBECTL apply -f -
    $KUBECTL label namespace $NAMESPACE zen-lock=enabled --overwrite || true
    log_success "Namespace ready"
}

wait_for_pods() {
    local selector="$1"
    local timeout="${2:-300}"
    log_info "Waiting for pods: $selector (timeout: ${timeout}s)"
    
    $KUBECTL wait --for=condition=Ready pods -l "$selector" -n $NAMESPACE --timeout="${timeout}s" || {
        log_warn "Timeout waiting for pods, continuing anyway..."
    }
}

deploy_infrastructure() {
    log_info "=== Phase 1: Infrastructure ==="
    
    # Create namespace first
    create_namespace
    
    # Deploy Redis
    log_info "Deploying Redis..."
    $KUBECTL apply -f "${SCRIPT_DIR}/infrastructure/redis.yaml"
    wait_for_pods "app.kubernetes.io/component=redis" 120
    log_success "Redis ready"
    
    # Deploy MinIO
    log_info "Deploying MinIO..."
    $KUBECTL apply -f "${SCRIPT_DIR}/infrastructure/minio.yaml"
    wait_for_pods "app.kubernetes.io/component=minio" 120
    log_success "MinIO ready"
    
    # Check CockroachDB (assumes it's already deployed via Helm)
    log_info "Checking CockroachDB..."
    if $KUBECTL get pods -l app.kubernetes.io/name=cockroachdb -n $NAMESPACE | grep -q "Running"; then
        log_info "CockroachDB pods found, initializing..."
        $KUBECTL apply -f "${SCRIPT_DIR}/infrastructure/cockroachdb-init.yaml" || true
        sleep 10
        log_success "CockroachDB initialized"
    else
        log_warn "CockroachDB not found. Install it first with:"
        echo "  helm repo add cockroachdb https://charts.cockroachdb.com/"
        echo "  helm install zen-brain-crdb cockroachdb/cockroachdb -n zen-brain --set cockroachdb.conf.single-node=true,cockroachdb.conf.insecure=true"
    fi
    
    log_success "=== Infrastructure Phase Complete ==="
}

bootstrap_jira() {
    log_info "Bootstrapping Jira credentials..."
    
    local zenlock_dir="${REPO_ROOT}/deploy/zen-lock"
    local bootstrap_script="${zenlock_dir}/bootstrap-jira-zenlock-from-local.sh"
    
    if [[ -x "$bootstrap_script" ]]; then
        "$bootstrap_script"
        log_success "Jira credentials bootstrapped"
    else
        log_warn "Jira bootstrap script not found or not executable: $bootstrap_script"
        log_warn "Create Jira secret manually or run:"
        echo "  $bootstrap_script"
    fi
}

deploy_services() {
    log_info "=== Phase 2: Services ==="
    
    # Bootstrap Jira first
    bootstrap_jira
    
    # Deploy CRDs
    log_info "Applying CRDs..."
    $KUBECTL apply -f "${SCRIPT_DIR}/02-crd-braintask.yaml" || true
    $KUBECTL apply -f "${SCRIPT_DIR}/crd-tasksession.yaml" || true
    log_success "CRDs ready"
    
    # Deploy RBAC
    log_info "Applying RBAC..."
    $KUBECTL apply -f "${SCRIPT_DIR}/01-rbac.yaml"
    log_success "RBAC ready"
    
    # Deploy configs
    log_info "Applying ConfigMaps..."
    $KUBECTL apply -f "${SCRIPT_DIR}/config-schedules.yaml" || true
    
    # Create task-templates configmap if not exists
    if ! $KUBECTL get configmap task-templates -n $NAMESPACE &>/dev/null; then
        $KUBECTL create configmap task-templates -n $NAMESPACE \
            --from-file="${REPO_ROOT}/config/task-templates/" || true
    fi
    log_success "ConfigMaps ready"
    
    # Deploy services
    log_info "Deploying services..."
    $KUBECTL apply -f "${SCRIPT_DIR}/03-deployments.yaml"
    $KUBECTL apply -f "${SCRIPT_DIR}/04-services.yaml"
    
    # Wait for services
    log_info "Waiting for services to start..."
    sleep 10
    $KUBECTL get pods -n $NAMESPACE
    
    log_success "=== Services Phase Complete ==="
}

verify_deployment() {
    log_info "=== Verification ==="
    
    echo ""
    log_info "Pods:"
    $KUBECTL get pods -n $NAMESPACE
    
    echo ""
    log_info "Services:"
    $KUBECTL get svc -n $NAMESPACE
    
    echo ""
    log_info "Health checks:"
    
    # Check Redis
    if $KUBECTL exec -n $NAMESPACE deploy/redis -- redis-cli ping 2>/dev/null | grep -q PONG; then
        log_success "Redis: OK"
    else
        log_warn "Redis: Not responding"
    fi
    
    # Check MinIO
    if curl -s http://localhost:9000/minio/health/live 2>/dev/null | grep -q "OK"; then
        log_success "MinIO: OK"
    else
        log_warn "MinIO: Not responding (may need port-forward)"
    fi
    
    echo ""
    log_success "=== Deployment Complete ==="
    echo ""
    echo "Next steps:"
    echo "  1. Port-forward API server: kubectl port-forward -n zen-brain svc/apiserver 8080:8080"
    echo "  2. Check health: curl http://localhost:8080/healthz"
    echo "  3. View logs: kubectl logs -n zen-brain -l app.kubernetes.io/name=zen-brain -f"
}

# Main
PHASE="${1:-all}"

case "$PHASE" in
    infra)
        check_prerequisites
        deploy_infrastructure
        ;;
    services)
        deploy_services
        ;;
    all)
        check_prerequisites
        deploy_infrastructure
        deploy_services
        verify_deployment
        ;;
    *)
        echo "Usage: $0 [infra|services|all]"
        exit 1
        ;;
esac
