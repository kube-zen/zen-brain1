#!/usr/bin/env bash
# deploy.sh — Build, push, deploy zen-brain1 to K8s
#
# Usage: ./deploy.sh [build|deploy|stop-host|status|all|nuke]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
K8S_DIR="$SCRIPT_DIR"
REGISTRY="zen-registry:5000"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[$(date +%H:%M:%S)]${NC} $*"; }
warn() { echo -e "${YELLOW}[$(date +%H:%M:%S)] WARN${NC} $*"; }
err() { echo -e "${RED}[$(date +%H:%M:%S)] ERROR${NC} $*" >&2; }

build_binaries() {
    log "Building all Go binaries..."
    cd "$REPO_ROOT"

    CGO_ENABLED=0 go build -ldflags="-s -w" -o scheduler          ./cmd/scheduler
    CGO_ENABLED=0 go build -ldflags="-s -w" -o useful-batch       ./cmd/useful-batch
    CGO_ENABLED=0 go build -ldflags="-s -w" -o finding-ticketizer ./cmd/finding-ticketizer
    CGO_ENABLED=0 go build -ldflags="-s -w" -o factory-fill       ./cmd/factory-fill
    CGO_ENABLED=0 go build -ldflags="-s -w" -o remediation-worker ./cmd/remediation-worker

    log "All binaries built."
    ls -lh scheduler useful-batch finding-ticketizer factory-fill remediation-worker
}

build_images() {
    cd "$REPO_ROOT"

    log "Building scheduler image (scheduler + useful-batch + finding-ticketizer)..."
    docker build -f Dockerfile.zen-brain1 --target scheduler \
        -t "$REGISTRY/zen-brain-scheduler:latest" .
    docker push "$REGISTRY/zen-brain-scheduler:latest" 2>&1 | tail -3

    log "Building factory-fill image (factory-fill + remediation-worker)..."
    docker build -f Dockerfile.zen-brain1 --target factory-fill \
        -t "$REGISTRY/zen-brain-factory-fill:latest" .
    docker push "$REGISTRY/zen-brain-factory-fill:latest" 2>&1 | tail -3

    log "All images built and pushed."

    # Cleanup copied binaries from repo root
    rm -f scheduler useful-batch finding-ticketizer factory-fill remediation-worker
}

nuke_old() {
    log "Removing ALL zen-brain deployment objects (clean slate)..."

    kubectl delete deploy --all -n zen-brain 2>/dev/null || true
    kubectl delete sa zen-brain-scheduler zen-brain-factory zen-brain-serve -n zen-brain 2>/dev/null || true
    kubectl delete rolebinding zen-brain-scheduler zen-brain-factory -n zen-brain 2>/dev/null || true
    kubectl delete clusterrole zen-brain-scheduler zen-brain-factory 2>/dev/null || true

    log "Old objects removed."
}

deploy() {
    log "Deploying zen-brain1 control-plane to K8s..."

    # Apply in dependency order
    kubectl apply -f "$K8S_DIR/00-namespace.yaml"
    kubectl apply -f "$K8S_DIR/02-crd-braintask.yaml"
    kubectl apply -f "$K8S_DIR/crd-tasksession.yaml"
    kubectl apply -f "$K8S_DIR/01-rbac.yaml"

    # ConfigMaps
    log "Creating ConfigMaps..."
    kubectl create configmap schedules --from-file="$REPO_ROOT/config/schedules/" \
        -n zen-brain --dry-run=client -o yaml 2>/dev/null | kubectl apply -f -
    kubectl create configmap task-templates --from-file="$REPO_ROOT/config/task-templates/v2/" \
        -n zen-brain --dry-run=client -o yaml 2>/dev/null | kubectl apply -f -

    # ZenLock (from external path)
    local ZENLOCK_SRC="$HOME/zen/keys/zen-brain/jira-credentials.zenlock.yaml"
    if [ ! -f "$ZENLOCK_SRC" ]; then
        err "ZenLock not found at $ZENLOCK_SRC"
        err "Run: ~/zen/zen-brain1/scripts/zen-lock-rotate.sh"
        exit 1
    fi
    log "Applying ZenLock..."
    kubectl apply -f "$ZENLOCK_SRC" 2>&1 | tail -3

    # Deployments (unified — uses single zen-brain SA)
    kubectl apply -f "$K8S_DIR/03-deployments.yaml"

    # Services
    kubectl apply -f "$K8S_DIR/04-services.yaml"

    log "Waiting for scheduler rollout..."
    kubectl rollout status deployment/scheduler -n zen-brain --timeout=120s 2>&1 | tail -1

    log "Waiting for factory-fill rollout..."
    kubectl rollout status deployment/factory-fill -n zen-brain --timeout=120s 2>&1 | tail -1

    log "Deployment complete."
}

stop_host_daemons() {
    log "Stopping host-side zen-brain1 daemons..."

    # systemd services
    for svc in zen-brain1-scheduler zen-brain1-factory-fill; do
        if sudo systemctl is-active --quiet "$svc" 2>/dev/null; then
            sudo systemctl stop "$svc"
            sudo systemctl disable "$svc" 2>/dev/null || true
            log "  Stopped + disabled $svc (systemd)"
        fi
    done

    # Bare processes
    local pids
    for bin in scheduler factory-fill; do
        pids=$(pgrep -f "/$bin" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            kill $pids 2>/dev/null || true
            log "  Killed host $bin: $pids"
        fi
    done
}

status() {
    log "=== K8s Deployments ==="
    kubectl get deploy,pods -n zen-brain -o wide 2>/dev/null || echo "Namespace not found"
    echo ""
    log "=== Image details ==="
    kubectl get pods -n zen-brain -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\t"}{.status.containerStatuses[0].imageID}{"\n"}{end}' 2>/dev/null || true
    echo ""
    log "=== Host Processes ==="
    ps aux | grep -E '/scheduler|/factory-fill' | grep -v grep || echo "  None"
}

case "${1:-all}" in
    build)
        build_binaries
        build_images
        ;;
    deploy) deploy ;;
    stop-host) stop_host_daemons ;;
    status) status ;;
    nuke) nuke_old ;;
    all)
        build_binaries
        build_images
        nuke_old
        deploy
        echo ""
        echo "Next: Verify with './deploy.sh status'"
        echo "Then stop host daemons: './deploy.sh stop-host'"
        ;;
    *)
        echo "Usage: $0 {build|deploy|stop-host|status|all|nuke}"
        exit 1
        ;;
esac
