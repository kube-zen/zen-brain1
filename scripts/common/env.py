#!/usr/bin/env python3
"""
Zen-brain environment lifecycle: redeploy, destroy, status.
Orchestrates registry, k3d, manifests, image build/load, and apiserver exposure.
"""
from __future__ import annotations

import os
import subprocess
import sys
import time

_common_dir = os.path.dirname(os.path.abspath(__file__))
if _common_dir not in sys.path:
    sys.path.insert(0, _common_dir)
import config as _config  # noqa: E402
import k3d as _k3d  # noqa: E402
import registry as _registry  # noqa: E402
import hosts as _hosts  # noqa: E402


def _repo_root() -> str:
    return os.path.abspath(os.path.join(_common_dir, "..", ".."))


def _log(msg: str) -> None:
    print(msg, file=sys.stdout, flush=True)


def _err(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


def _run(cmd: list, check: bool = True, capture: bool = False, timeout: int = 120, cwd: str | None = None, shell: bool = False) -> subprocess.CompletedProcess:
    return subprocess.run(
        cmd,
        check=check,
        capture_output=capture,
        text=True,
        timeout=timeout,
        cwd=cwd or _repo_root(),
        shell=shell,
    )


def _kubectl(args: list[str], context: str, capture: bool = False, timeout: int = 60) -> subprocess.CompletedProcess:
    return _run(["kubectl", "--context", context] + args, capture=capture, timeout=timeout)


def _cluster_exists(cluster_name: str) -> bool:
    return _k3d._cluster_exists(cluster_name)


def _ensure_namespaces(context_name: str) -> None:
    """Ensure zen-brain, zen-context, and zen-lock-system namespaces exist (for createNamespace: false)."""
    for ns in ("zen-brain", "zen-context", "zen-lock-system"):
        r = subprocess.run(
            ["kubectl", "--context", context_name, "get", "namespace", ns],
            capture_output=True,
            text=True,
            timeout=10,
            check=False
        )
        if r.returncode != 0:
            # Create namespace
            create = subprocess.run(
                ["kubectl", "--context", context_name, "create", "namespace", ns],
                capture_output=True,
                text=True,
                timeout=10,
                check=False
            )
            if create.returncode != 0:
                _err(create.stderr or f"Failed to create namespace {ns}")
                raise RuntimeError(f"Failed to create namespace {ns}")
            _log(f"Created namespace {ns}")
            
            # Add Helm ownership metadata for zen-lock-system namespace
            if ns == "zen-lock-system":
                label_cmd = ["kubectl", "--context", context_name, "label", "namespace", ns, "app.kubernetes.io/managed-by=Helm"]
                label = subprocess.run(label_cmd, capture_output=True, text=True, timeout=10, check=False)
                if label.returncode != 0:
                    _err(label.stderr or f"Failed to label namespace {ns}")
                    raise RuntimeError(f"Failed to label namespace {ns}")
                
                annot_cmd = ["kubectl", "--context", context_name, "annotate", "namespace", ns, "meta.helm.sh/release-name=zen-lock", "meta.helm.sh/release-namespace=zen-lock-system"]
                annot = subprocess.run(annot_cmd, capture_output=True, text=True, timeout=10, check=False)
                if annot.returncode != 0:
                    _err(annot.stderr or f"Failed to annotate namespace {ns}")
                    raise RuntimeError(f"Failed to annotate namespace {ns}")
                
                _log(f"Added Helm ownership metadata to namespace {ns}")
def _ensure_zen_glm_secret(context_name: str, config_path: str | None, env: str) -> None:
    """If deploy.use_zen_glm and ZEN_GLM_API_KEY is set, create/update secret (key from env only, never committed)."""
    if not _config.get_deploy_use_zen_glm(env, config_path):
        return
    key = os.environ.get("ZEN_GLM_API_KEY")
    if not key or not key.strip():
        _err("ZEN_GLM_API_KEY is required when deploy.use_zen_glm is true. Set it in the environment and re-run.")
        raise RuntimeError("ZEN_GLM_API_KEY required for zen-glm")
    create = subprocess.run(
        ["kubectl", "--context", context_name, "create", "secret", "generic", "zen-glm-api-key", "-n", "zen-brain",
         "--from-literal=api-key=" + key.strip(), "--dry-run=client", "-o", "yaml"],
        capture_output=True,
        text=True,
        timeout=15,
        cwd=_repo_root(),
    )
    if create.returncode != 0:
        _err(create.stderr or "kubectl create secret failed")
        raise RuntimeError("zen-glm secret create failed")
    apply = subprocess.run(
        ["kubectl", "--context", context_name, "apply", "-f", "-"],
        input=create.stdout,
        capture_output=True,
        text=True,
        timeout=15,
        cwd=_repo_root(),
    )
    if apply.returncode != 0:
        _err(apply.stderr or "kubectl apply secret failed")
        raise RuntimeError("zen-glm secret apply failed")
    _log("zen-glm-api-key secret created/updated (from ZEN_GLM_API_KEY)")


def _ensure_zen_lock_secret(context_name: str, config_path: str | None) -> None:
    """Ensure zen-lock master key secret exists (from ~/.zen-lock/private-key.age)."""
    private_key_path = os.path.expanduser("~/.zen-lock/private-key.age")
    if not os.path.exists(private_key_path):
        _log(f"Zen-lock private key not found at {private_key_path}, skipping secret creation")
        return
    secret_name = "zen-lock-master-key"
    namespace = "zen-lock-system"
    
    # Check if namespace exists first (Helm will create it)
    ns_check = subprocess.run(
        ["kubectl", "--context", context_name, "get", "namespace", namespace],
        capture_output=True,
        text=True,
        timeout=10,
        check=False
    )
    if ns_check.returncode != 0:
        _log(f"Namespace '{namespace}' does not exist yet - skipping secret creation (Helm will create it)")
        return
    
    check = subprocess.run(
        ["kubectl", "--context", context_name, "get", "secret", secret_name, "-n", namespace],
        capture_output=True,
        text=True,
        timeout=10,
        check=False
    )
    if check.returncode == 0:
        _log(f"Zen-lock secret '{secret_name}' already exists")
        return
    
    create = subprocess.run(
        ["kubectl", "--context", context_name, "create", "secret", "generic", secret_name,
         "-n", namespace, "--from-literal=key.txt=" + private_key_path],
        capture_output=True,
        text=True,
        timeout=15,
        check=False
    )
    if create.returncode != 0:
        _err(create.stderr or f"Zen-lock secret creation failed")
        raise RuntimeError("zen-lock secret creation failed")
    _log(f"Zen-lock secret '{secret_name}' created from {private_key_path}")
    
    # Add Helm ownership metadata to the secret
    label_cmd = ["kubectl", "--context", context_name, "label", "secret", secret_name,
                 "-n", namespace, "app.kubernetes.io/managed-by=Helm"]
    label = subprocess.run(label_cmd, capture_output=True, text=True, timeout=10, check=False)
    if label.returncode != 0:
        _err(label.stderr or f"Failed to label secret {secret_name}")
        raise RuntimeError(f"Failed to label secret {secret_name}")
    
    annot_cmd = ["kubectl", "--context", context_name, "annotate", "secret", secret_name,
                 "-n", namespace, "meta.helm.sh/release-name=zen-lock", "meta.helm.sh/release-namespace=zen-lock-system"]
    annot = subprocess.run(annot_cmd, capture_output=True, text=True, timeout=10, check=False)
    if annot.returncode != 0:
        _err(annot.stderr or f"Failed to annotate secret {secret_name}")
        raise RuntimeError(f"Failed to annotate secret {secret_name}")
    
    _log(f"Added Helm ownership metadata to secret {secret_name}")
def _run_helmfile(env: str, config_path: str | None, context_name: str, skip_ollama: bool = False) -> None:
    """Canonical deployment: render values from clusters.yaml then helmfile sync."""
    import helmfile_values  # noqa: E402
    root = _repo_root()
    _ensure_namespaces(context_name)
    _ensure_zen_lock_secret(context_name, config_path)
    _ensure_zen_glm_secret(context_name, config_path, env)
    _log("Rendering Helm values from config/clusters.yaml...")
    helmfile_values.render(env, config_path)
    helmfile_path = os.path.join(root, "deploy", "helmfile", "zen-brain", "helmfile.yaml.gotmpl")
    if not os.path.isfile(helmfile_path):
        _err(f"ERROR: Helmfile not found: {helmfile_path}")
        raise FileNotFoundError(helmfile_path)
    _log("Running Helmfile (canonical deployment path)...")
    cmd = ["helmfile", "-e", env, "-f", helmfile_path, "--kube-context", context_name]
    if skip_ollama:
        _log("Skipping ollama release (using selector)...")
        cmd.extend(["--selector", "component!=ollama"])
    cmd.append("sync")
    _run(cmd, timeout=900, cwd=root)


def _build_and_load_image(env: str, config_path: str | None, build: bool) -> None:
    root = _repo_root()
    tag = _config.get_zen_brain_tag(env, config_path)
    reg_host = _config.get_registry_host_ref(config_path)
    image_local = f"zen-brain:{tag}"
    image_reg = f"{reg_host}/zen-brain:{tag}"
    if build:
        _log("Building zen-brain image...")
        _run(["docker", "build", "-t", image_local, "."], timeout=600, cwd=root)
    _log("Tagging and pushing to shared registry...")
    _run(["docker", "tag", image_local, image_reg], timeout=10)
    _run(["docker", "push", image_reg], timeout=120)
    _log(f"Image zen-brain:{tag} pushed to shared registry {reg_host}")


def _wait_rollout(context_name: str) -> None:
    _log("Waiting for foreman rollout...")
    _kubectl(["rollout", "status", "deployment/foreman", "-n", "zen-brain", "--timeout=120s"], context_name, timeout=130)
    _log("Waiting for apiserver rollout...")
    _kubectl(["rollout", "status", "deployment/apiserver", "-n", "zen-brain", "--timeout=120s"], context_name, timeout=130)


def _verify_health_endpoints(env_ip: str, apiserver_port: int, timeout: int = 60) -> bool:
    """Verify apiserver health and ready endpoints respond successfully."""
    import socket
    import urllib.request
    import urllib.error

    base_url = f"http://{env_ip}:{apiserver_port}"
    endpoints = ["/healthz", "/readyz"]
    start = time.time()
    last_err = None

    while time.time() - start < timeout:
        all_healthy = True
        for ep in endpoints:
            try:
                url = f"{base_url}{ep}"
                req = urllib.request.Request(url, method="GET")
                with urllib.request.urlopen(req, timeout=5) as resp:
                    if resp.status != 200:
                        all_healthy = False
                        break
            except (urllib.error.URLError, socket.timeout, ConnectionRefusedError) as e:
                all_healthy = False
                last_err = e
                break

        if all_healthy:
            _log("Health endpoints verified: /healthz ✓ /readyz ✓")
            return True

        time.sleep(2)

    _err(f"Health check failed after {timeout}s: {last_err}")
    return False


def cmd_redeploy(
    env: str,
    config_path: str | None,
    skip_hosts: bool,
    skip_registry: bool,
    skip_k3d: bool,
    skip_manifests: bool,
    skip_build: bool,
    skip_image_load: bool,
    skip_ollama: bool,
    force_recreate: bool,
) -> int:
    if not env:
        _err("ERROR: --env required")
        return 1
    root = _repo_root()
    if config_path is None:
        config_path = os.path.join(root, "config", "clusters.yaml")
    block = _config.get_cluster_block(env, config_path)
    context_name = (block.get("context_name") or f"k3d-zen-brain-{env}").strip()
    k3d_block = block.get("k3d") or {}
    cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{env}").strip()
    use_zencontext = _config.get_deploy_use_zencontext(env, config_path)
    env_ip = (block.get("env_ip") or "").strip()
    apiserver_port = _config.get_deploy_apiserver_external_port(env, config_path)

    _k3d._ensure_prereqs()
    if not skip_registry:
        rc = _registry.cmd_ensure(config_path)
        if rc != 0:
            return rc
    if not skip_hosts and _config.get_hosts_manage(env, config_path):
        _hosts.cmd_apply(env, config_path, _hosts.HOSTS_FILE, dry_run=False)
    if not skip_k3d:
        rc = _k3d.cmd_ensure(env, config_path, force_recreate)
        if rc != 0:
            return rc
    if not skip_image_load:
        _build_and_load_image(env, config_path, build=not skip_build)
    if not skip_manifests:
        _run_helmfile(env, config_path, context_name, skip_ollama=skip_ollama)
    _wait_rollout(context_name)

    # Verify health endpoints (fail-fast if services aren't truly ready)
    if not _verify_health_endpoints(env_ip, apiserver_port, timeout=60):
        _err("WARNING: Health endpoints not responding, but pods may still be starting")

    _log("")
    _log("Zen-brain environment ready.")
    _log(f"  Cluster: {cluster_name}")
    _log(f"  Context: {context_name}")
    _log(f"  Apiserver: http://{env_ip}:{apiserver_port}/healthz")
    _log(f"  Readyz:    http://{env_ip}:{apiserver_port}/readyz")
    return 0


def cmd_destroy(
    env: str,
    config_path: str | None,
    confirm_destroy: bool,
    remove_hosts: bool,
    remove_registry: bool,
) -> int:
    if not env:
        _err("ERROR: --env required")
        return 1
    root = _repo_root()
    if config_path is None:
        config_path = os.path.join(root, "config", "clusters.yaml")
    if remove_hosts and _config.get_hosts_manage(env, config_path):
        _hosts.cmd_remove(env, config_path, _hosts.HOSTS_FILE, dry_run=False)
    rc = _k3d.cmd_destroy(env, config_path, confirm_destroy)
    if rc != 0:
        return rc
    if remove_registry:
        _registry.cmd_stop(config_path, remove=True)
    return 0


def cmd_status(env: str, config_path: str | None) -> int:
    if not env:
        _err("ERROR: --env required")
        return 1
    root = _repo_root()
    if config_path is None:
        config_path = os.path.join(root, "config", "clusters.yaml")
    block = _config.get_cluster_block(env, config_path)
    context_name = (block.get("context_name") or f"k3d-zen-brain-{env}").strip()
    k3d_block = block.get("k3d") or {}
    cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{env}").strip()
    env_ip = (block.get("env_ip") or "").strip()
    apiserver_port = _config.get_deploy_apiserver_external_port(env, config_path)

    _log(f"Environment: {env}")
    _log(f"  Cluster name: {cluster_name}")
    _log(f"  Context:      {context_name}")
    exists = _cluster_exists(cluster_name)
    _log(f"  Cluster exists: {exists}")
    if not exists:
        return 0
    r = _kubectl(["get", "nodes", "--no-headers"], context_name, capture=True, timeout=15)
    if r.returncode == 0:
        _log(f"  Nodes:\n{r.stdout}")
    r = _kubectl(["get", "pods", "-n", "zen-brain", "--no-headers"], context_name, capture=True, timeout=15)
    if r.returncode == 0:
        _log(f"  Pods (zen-brain):\n{r.stdout}")
    _log(f"  Apiserver endpoint: http://{env_ip}:{apiserver_port}/healthz")
    return 0
