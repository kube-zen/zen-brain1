#!/usr/bin/env python3
"""
k3d lifecycle for zen-brain: ensure (create) and destroy.
Reads config/clusters.yaml; uses 127.0.1.x and zen-brain-registry:5001.
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


def _repo_root() -> str:
    return os.path.abspath(os.path.join(_common_dir, "..", ".."))


def _log(msg: str) -> None:
    print(msg, file=sys.stdout, flush=True)


def _err(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


def _cluster_exists(cluster_name: str) -> bool:
    r = subprocess.run(
        ["k3d", "cluster", "list", "--no-headers"],
        capture_output=True,
        text=True,
        timeout=30,
    )
    if r.returncode != 0:
        return False
    for line in (r.stdout or "").splitlines():
        if line.strip().startswith(f"{cluster_name} "):
            return True
    return False


def _cluster_reachable(context_name: str) -> bool:
    try:
        r = subprocess.run(
            ["kubectl", "--context", context_name, "get", "nodes", "--no-headers"],
            capture_output=True,
            text=True,
            timeout=15,
        )
    except subprocess.TimeoutExpired:
        return False
    if r.returncode != 0:
        return False
    for line in (r.stdout or "").splitlines():
        if " Ready " in line:
            return True
    return False


def _get_k3d_config(env: str, config_path: str | None) -> dict:
    block = _config.get_cluster_block(env, config_path)
    k3d_block = block.get("k3d") or {}
    context_name = (block.get("context_name") or f"k3d-zen-brain-{env}").strip()
    cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{env}").strip()
    servers = int(k3d_block.get("servers") or 1)
    agents = int(k3d_block.get("agents") or 0)
    api_port = (k3d_block.get("api_port") or "").strip()
    lb = k3d_block.get("lb_ports") or {}
    http_port = (lb.get("http") or "").strip()
    https_port = (lb.get("https") or "").strip()
    apiserver_port = (lb.get("apiserver") or "").strip()
    disable = k3d_block.get("disable")
    if isinstance(disable, list):
        disable = [str(x).strip() for x in disable if x]
    else:
        disable = ["traefik"]
    if "traefik" not in disable:
        disable = ["traefik"] + [d for d in disable if d != "traefik"]
    if not api_port or not http_port or not https_port:
        raise ValueError(f"clusters.{env}.k3d: api_port and lb_ports.http/https required")
    return {
        "cluster_name": cluster_name,
        "context_name": context_name,
        "servers": servers,
        "agents": agents,
        "api_port": api_port,
        "http_port": http_port,
        "https_port": https_port,
        "apiserver_port": apiserver_port or None,
        "disable": disable,
        "env_ip": (block.get("env_ip") or "").strip(),
    }


def _ensure_prereqs() -> None:
    for name, cmd in [("docker", ["docker", "info"]), ("kubectl", ["kubectl", "version", "--client"]), ("k3d", ["k3d", "version"])]:
        r = subprocess.run(cmd, capture_output=True, timeout=10)
        if r.returncode != 0:
            _err(f"ERROR: {name} not found or not running")
            if name == "k3d":
                _err("Install: https://k3d.io/")
            sys.exit(1)


def _create_cluster(cfg: dict, config_path: str | None, repo_root: str) -> None:
    cluster_name = cfg["cluster_name"]
    context_name = cfg["context_name"]
    reg_container = _config.get_registry_container_name(config_path)
    reg_cluster_ref = _config.get_registry_cluster_ref(config_path)

    def parse_port(p: str) -> tuple[str, str]:
        parts = (p or "").split(":")
        if len(parts) >= 3:
            return f"{parts[0]}:{parts[1]}", parts[2]
        if len(parts) >= 2:
            return parts[0], parts[1]
        return "0.0.0.0", "80"

    http_spec, http_svc = parse_port(cfg["http_port"])
    https_spec, https_svc = parse_port(cfg["https_port"])

    args = [
        "k3d", "cluster", "create", cluster_name,
        "--api-port", cfg["api_port"],
        "--port", f"{http_spec}:80@loadbalancer",
        "--port", f"{https_spec}:443@loadbalancer",
        "--servers", str(cfg["servers"]),
        "--agents", str(cfg["agents"]),
    ]
    if cfg.get("apiserver_port"):
        apiserver_spec, apiserver_svc = parse_port(cfg["apiserver_port"])
        args += ["--port", f"{apiserver_spec}:{apiserver_svc}@loadbalancer"]
    for d in cfg["disable"]:
        args += ["--k3s-arg", f"--disable={d}@server:*"]
    args += ["--registry-use", reg_cluster_ref]
    if cfg["servers"] == 1:
        args += ["--k3s-arg", "--cluster-init@server:*"]
    args += ["--wait", "--timeout", "120s"]

    _log("Creating k3d cluster...")
    r = subprocess.run(args, capture_output=True, text=True, timeout=140, cwd=repo_root)
    if r.returncode != 0:
        _err(r.stderr or r.stdout or "k3d create failed")
        sys.exit(1)

    network = f"k3d-{cluster_name}"
    subprocess.run(
        ["docker", "network", "connect", "--alias", reg_container, network, reg_container],
        capture_output=True,
        timeout=10,
    )

    k3d_context = f"k3d-{cluster_name}"
    if context_name != k3d_context:
        subprocess.run(
            ["kubectl", "config", "rename-context", k3d_context, context_name],
            check=True,
            capture_output=True,
            timeout=10,
        )

    _log("Waiting for nodes ready...")
    for _ in range(24):
        r = subprocess.run(
            ["kubectl", "wait", "--for=condition=Ready", "nodes", "--all", "--context", context_name, "--timeout=15s"],
            capture_output=True,
            text=True,
            timeout=20,
        )
        if r.returncode == 0:
            break
        time.sleep(5)
    else:
        _err("ERROR: Nodes not ready within timeout")
        sys.exit(1)
    _log("Cluster created and nodes ready.")


def cmd_ensure(env: str, config_path: str | None, force_recreate: bool) -> int:
    if not env:
        _err("ERROR: --env required")
        return 1
    repo_root = _repo_root()
    if config_path is None:
        config_path = os.path.join(repo_root, "config", "clusters.yaml")
    try:
        cfg = _get_k3d_config(env, config_path)
    except (KeyError, ValueError) as e:
        _err(f"ERROR: {e}")
        return 1
    _ensure_prereqs()
    cluster_name = cfg["cluster_name"]
    context_name = cfg["context_name"]
    if _cluster_exists(cluster_name):
        if force_recreate:
            _log("--force-recreate: destroying existing cluster...")
            subprocess.run(["k3d", "cluster", "delete", cluster_name], capture_output=True, timeout=60)
            time.sleep(2)
        elif _cluster_reachable(context_name):
            _log("Cluster already available and reachable.")
            subprocess.run(["kubectl", "config", "use-context", context_name], check=False, capture_output=True)
            return 0
        else:
            _err("ERROR: Cluster exists but unreachable (use --force-recreate to replace)")
            return 1
    _create_cluster(cfg, config_path, repo_root)
    subprocess.run(["kubectl", "config", "use-context", context_name], check=True, capture_output=True)
    _log(f"Context set: {context_name}")
    return 0


def cmd_destroy(env: str, config_path: str | None, confirm_destroy: bool) -> int:
    if not env:
        _err("ERROR: --env required")
        return 1
    repo_root = _repo_root()
    if config_path is None:
        config_path = os.path.join(repo_root, "config", "clusters.yaml")
    try:
        block = _config.get_cluster_block(env, config_path)
    except KeyError as e:
        _err(f"ERROR: {e}")
        return 1
    deploy = block.get("deploy") or {}
    require_confirm = deploy.get("require_destroy_confirm", True)
    if require_confirm and not confirm_destroy:
        _err("ERROR: Destroying this environment requires confirmation.")
        _err("Remediation: Pass --confirm-destroy or set CONFIRM_DESTROY=1")
        return 1
    k3d_block = block.get("k3d") or {}
    cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{env}").strip()
    context_name = (block.get("context_name") or f"k3d-zen-brain-{env}").strip()
    if _cluster_exists(cluster_name):
        _log("Deleting k3d cluster...")
        subprocess.run(["k3d", "cluster", "delete", cluster_name], check=True, timeout=120)
        for _ in range(30):
            if not _cluster_exists(cluster_name):
                break
            time.sleep(1)
        _log("Cluster destroyed.")
    else:
        _log("Cluster does not exist.")
    for args in (
        ["kubectl", "config", "delete-context", context_name],
        ["kubectl", "config", "delete-cluster", context_name],
        ["kubectl", "config", "delete-user", f"admin@{context_name}"],
    ):
        subprocess.run(args, capture_output=True, timeout=5)
    return 0
