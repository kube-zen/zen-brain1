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


def _run(cmd: list[str], check: bool = True, capture: bool = False, timeout: int = 120, cwd: str | None = None) -> subprocess.CompletedProcess:
    return subprocess.run(
        cmd,
        check=check,
        capture_output=capture,
        text=True,
        timeout=timeout,
        cwd=cwd or _repo_root(),
    )


def _kubectl(args: list[str], context: str, capture: bool = False, timeout: int = 60) -> subprocess.CompletedProcess:
    return _run(["kubectl", "--context", context] + args, capture=capture, timeout=timeout)


def _cluster_exists(cluster_name: str) -> bool:
    return _k3d._cluster_exists(cluster_name)


def _ensure_namespaces(context_name: str) -> None:
    """Ensure zen-brain and zen-context namespaces exist (for createNamespace: false)."""
    for ns in ("zen-brain", "zen-context"):
        r = _kubectl(["get", "namespace", ns], context_name, capture=True, timeout=10)
        if r.returncode != 0:
            _kubectl(["create", "namespace", ns], context_name, timeout=10)
            _log(f"Created namespace {ns}")


def _run_helmfile(env: str, config_path: str | None, context_name: str) -> None:
    """Canonical deployment: render values from clusters.yaml then helmfile sync."""
    import helmfile_values  # noqa: E402
    root = _repo_root()
    _ensure_namespaces(context_name)
    _log("Rendering Helm values from config/clusters.yaml...")
    helmfile_values.render(env, config_path)
    helmfile_path = os.path.join(root, "deploy", "helmfile", "zen-brain", "helmfile.yaml.gotmpl")
    if not os.path.isfile(helmfile_path):
        _err(f"ERROR: Helmfile not found: {helmfile_path}")
        raise FileNotFoundError(helmfile_path)
    _log("Running Helmfile (canonical deployment path)...")
    _run(
        ["helmfile", "-e", env, "-f", helmfile_path, "--kube-context", context_name, "sync"],
        timeout=900,
        cwd=root,
    )


def _build_and_load_image(env: str, config_path: str | None, build: bool) -> None:
    root = _repo_root()
    tag = _config.get_zen_brain_tag(env, config_path)
    reg_host = _config.get_registry_host_ref(config_path)
    reg_ref = _config.get_registry_cluster_ref(config_path)
    image_local = f"zen-brain:{tag}"
    image_reg = f"{reg_host}/zen-brain:{tag}"
    if build:
        _log("Building zen-brain image...")
        _run(["docker", "build", "-t", image_local, "."], timeout=600, cwd=root)
    _log("Tagging and pushing to local registry...")
    _run(["docker", "tag", image_local, image_reg], timeout=10)
    _run(["docker", "push", image_reg], timeout=120)
    # Tag for k3d image import (host must have image under cluster ref name)
    _run(["docker", "tag", image_local, f"{reg_ref}/zen-brain:{tag}"], timeout=10)
    block = _config.get_cluster_block(env, config_path)
    context_name = (block.get("context_name") or f"k3d-zen-brain-{env}").strip()
    k3d_block = block.get("k3d") or {}
    cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{env}").strip()
    _log("Importing image into k3d cluster...")
    _run(["k3d", "image", "import", f"{reg_ref}/zen-brain:{tag}", "-c", cluster_name], timeout=120)


def _wait_rollout(context_name: str) -> None:
    _log("Waiting for foreman rollout...")
    _kubectl(["rollout", "status", "deployment/foreman", "-n", "zen-brain", "--timeout=120s"], context_name, timeout=130)
    _log("Waiting for apiserver rollout...")
    _kubectl(["rollout", "status", "deployment/apiserver", "-n", "zen-brain", "--timeout=120s"], context_name, timeout=130)


def cmd_redeploy(
    env: str,
    config_path: str | None,
    skip_hosts: bool,
    skip_registry: bool,
    skip_k3d: bool,
    skip_manifests: bool,
    skip_build: bool,
    skip_image_load: bool,
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
        _run_helmfile(env, config_path, context_name)
    _wait_rollout(context_name)

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
