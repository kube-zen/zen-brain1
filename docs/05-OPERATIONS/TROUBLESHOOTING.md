# Zen-Brain Troubleshooting Guide

**Last Updated:** 2026-03-11

## Ollama Runtime Policy

- **Dev/sandbox default:** Host Docker Ollama (outside Kubernetes), accessed via `host.k3d.internal:11434`
- **Kubernetes Ollama:** Optional, legacy, experimental — not the default path
- **Reasoning:** In-cluster Ollama has shown performance issues; host Docker Ollama provides better GPU passthrough and isolation
- **For details:** See `deploy/README.md` (Ollama deployment model section)

---

## Deployment Issues

### Pods stuck in ErrImagePull / ImagePullBackOff

**Symptoms:**
```
kubectl get pods -n zen-brain
NAME                        READY   STATUS             RESTARTS   AGE
apiserver-xxx               0/1     ImagePullBackOff   0          5m
foreman-xxx                 0/1     ErrImagePull       0          5m
```

**Root Cause:** Cluster cannot pull images from shared registry. Check that registry is running and image was pushed.

**Solution:**
```bash
# Check registry is running
docker ps | grep zen-brain-registry

# If not running, start registry and rebuild/load image
python3 scripts/zen.py image build --env sandbox

# Delete old pods to force restart with new image
kubectl delete pods -n zen-brain -l app=apiserver --context k3d-sandbox
kubectl delete pods -n zen-brain -l app=foreman --context k3d-sandbox
```

**Prevention:** The `zen.py env redeploy` script automatically pushes to shared registry. Always use the canonical path:
```bash
python3 scripts/zen.py env redeploy --env sandbox
```

---

### Pods not becoming Ready after rollout

**Symptoms:**
- `kubectl get pods` shows Running but 0/1 Ready
- Health endpoints return 500 or connection refused

**Diagnostic Steps:**
```bash
# Check pod logs
kubectl logs -n zen-brain -l app=apiserver --context k3d-sandbox

# Check events
kubectl get events -n zen-brain --context k3d-sandbox --sort-by='.lastTimestamp'

# Check pod description
kubectl describe pod -n zen-brain -l app=apiserver --context k3d-sandbox
```

**Common Causes:**
1. **Ollama not running** - Start with `docker start ollama` or redeploy with ollama
2. **Redis not reachable** - Check TIER1_REDIS_ADDR env var
3. **Config missing** - Verify helmfile values are rendered correctly

---

### Health endpoints not responding after redeploy

**Symptoms:**
```
curl http://127.0.1.1:8080/healthz
curl: (7) Failed to connect to 127.0.1.1 port 8080: Connection refused
```

**Diagnostic Steps:**
```bash
# Check if apiserver is running
kubectl get pods -n zen-brain --context k3d-sandbox

# Check service endpoints
kubectl get endpoints -n zen-brain --context k3d-sandbox

# Port-forward to test directly
kubectl port-forward -n zen-brain svc/apiserver 18080:8080 --context k3d-sandbox
curl http://localhost:18080/healthz
```

**Solution:** If pods are running but endpoints are empty, check the service selector matches pod labels.

---

## Ollama Issues

### Ollama container not running

**Symptoms:**
- apiserver logs show "connection refused" to localhost:11434
- LLM requests timeout

**Solution:**
```bash
# Check container status
docker ps -a | grep ollama

# Start container
docker start ollama

# Verify model is loaded
curl http://localhost:11434/api/tags
```

### Slow inference (3-5+ minutes per request)

**Root Cause:** Ollama running in k8s has significant overhead for CPU-only inference.

**Solution:** Run Ollama as Docker container with host networking:
```bash
docker run -d --name ollama \
  --network host \
  -e OLLAMA_HOST=0.0.0.0:11434 \
  -e OLLAMA_KEEP_ALIVE=-1 \
  -e OLLAMA_NUM_PARALLEL=12 \
  --memory=15g \
  ollama/ollama

# Pull the model
docker exec ollama ollama pull qwen3.5:0.8b
```

See `OLLAMA_08B_OPERATIONS_GUIDE.md` for full configuration.

---

## k3d Cluster Issues

### Cluster creation fails

**Symptoms:**
```
k3d cluster create sandbox
ERROR: failed to create cluster
```

**Diagnostic Steps:**
```bash
# Check docker is running
docker info

# Check for conflicting clusters
k3d cluster list

# Check registry is running
docker ps | grep registry
```

### Cannot connect to cluster

**Symptoms:**
```
kubectl get nodes --context k3d-sandbox
The connection to the server 127.0.1.1:6443 was refused
```

**Solution:**
```bash
# Check hosts file has correct entry
grep zen-brain /etc/hosts

# Verify with zen.py
python3 scripts/zen.py hosts verify --env sandbox

# Apply hosts if needed
python3 scripts/zen.py hosts apply --env sandbox
```

---

## Helmfile Issues

### Helmfile sync fails

**Symptoms:**
```
helmfile sync
ERROR: failed to render values
```

**Diagnostic Steps:**
```bash
# Check values file exists
ls -la deploy/helmfile/zen-brain/values-sandbox.yaml

# Verify config is valid
python3 -c "import yaml; yaml.safe_load(open('config/clusters.yaml'))"

# Check helmfile template
helmfile -e sandbox -f deploy/helmfile/zen-brain/helmfile.yaml.gotmpl template
```

---

## Quick Reference

| Issue | Check Command | Fix |
|-------|---------------|-----|
| ImagePullBackOff | `docker ps \| grep zen-brain-registry` | `python3 scripts/zen.py image build --env sandbox` |
| Ollama down | `docker ps \| grep ollama` | `docker start ollama` |
| Hosts missing | `zen.py hosts verify` | `zen.py hosts apply` |
| Cluster gone | `k3d cluster list` | `zen.py env redeploy` |
| Health fail | `kubectl logs -l app=apiserver` | Check dependencies |

---

## Canonical Deployment Path

Always use the single entrypoint for deployments:

```bash
# Full redeploy (recommended)
python3 scripts/zen.py env redeploy --env sandbox

# Check status
python3 scripts/zen.py env status --env sandbox

# Quick image rebuild + redeploy
python3 scripts/zen.py image build --env sandbox
python3 scripts/zen.py env redeploy --env sandbox --skip-build
```

**No manual kubectl operations should be needed.**
