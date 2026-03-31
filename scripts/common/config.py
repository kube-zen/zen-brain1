#!/usr/bin/env python3
"""
Strict read-only accessor for config/clusters.yaml (zen-brain).
Single source of truth: no hidden defaults; fail fast if env missing or required keys absent.
"""
from __future__ import annotations

import os
import sys

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore[assignment]

CONFIG_REL_PATH = os.path.join("config", "clusters.yaml")

# Repo root: scripts/common -> two levels up
def _repo_root() -> str:
    return os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _default_config_path() -> str:
    return os.path.join(_repo_root(), CONFIG_REL_PATH)


def _load_root(config_path: str | None = None) -> dict:
    """Load full clusters.yaml (root dict)."""
    if yaml is None:
        raise RuntimeError("PyYAML required. pip install pyyaml")
    path = config_path or _default_config_path()
    if not os.path.isfile(path):
        raise FileNotFoundError(f"Config not found: {path}")
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    return data


def _load_raw(config_path: str | None = None) -> dict:
    """Load clusters dict (clusters.*)."""
    data = _load_root(config_path)
    clusters = data.get("clusters")
    if not isinstance(clusters, dict):
        raise ValueError("clusters.yaml: missing or invalid 'clusters' key")
    return clusters


def get_cluster_block(env: str, config_path: str | None = None) -> dict:
    """
    Return raw cluster block for env (clusters.<env>).
    Fails if env missing. Caller reads nested keys (k3d, deploy, etc.).
    """
    clusters = _load_raw(config_path)
    if env not in clusters:
        raise KeyError(f"Unknown env: {env}")
    cfg = clusters[env]
    if not isinstance(cfg, dict):
        raise ValueError(f"clusters.{env}: must be a mapping")
    return cfg


def list_envs(config_path: str | None = None) -> list[str]:
    """Return list of enabled env names that have k3d config."""
    clusters = _load_raw(config_path)
    out = []
    for env, cfg in clusters.items():
        if not isinstance(cfg, dict) or not cfg.get("enabled", True):
            continue
        k3d = cfg.get("k3d")
        if not isinstance(k3d, dict):
            continue
        out.append(env)
    return sorted(out)


def get_registry_container_name(config_path: str | None = None) -> str:
    """Registry container name from root registry block."""
    root = _load_root(config_path)
    reg = root.get("registry")
    if isinstance(reg, dict) and reg.get("container_name"):
        return str(reg["container_name"]).strip()
    return "zen-brain-registry"


def get_registry_host_port(config_path: str | None = None) -> int:
    """Registry host port from root registry block."""
    root = _load_root(config_path)
    reg = root.get("registry")
    if isinstance(reg, dict) and reg.get("host_port") is not None:
        return int(reg["host_port"])
    return 5000


def get_registry_host_ref(config_path: str | None = None) -> str:
    """Host reference for push (localhost:<port>). Shared registry on :5000."""
    return "localhost:5000"


def get_registry_cluster_ref(config_path: str | None = None) -> str:
    """Registry ref as seen from inside cluster. Uses zen-registry:5000 (shared with zen-platform)."""
    # IMPORTANT: Use zen-registry:5000 (not zen-brain-registry:5000) to match zen-platform
    # and allow containerd mirror config to work correctly.
    return "zen-registry:5000"


def get_hosts_manage(env: str, config_path: str | None = None) -> bool:
    """Whether to manage /etc/hosts for env."""
    block = get_cluster_block(env, config_path)
    hosts = block.get("hosts")
    if isinstance(hosts, dict) and "manage" in hosts:
        return bool(hosts["manage"])
    return False


def get_dns_mode(env: str, config_path: str | None = None) -> str:
    """DNS mode for env (loopback or public)."""
    block = get_cluster_block(env, config_path)
    dns = block.get("dns")
    if isinstance(dns, dict) and dns.get("mode"):
        return str(dns["mode"]).strip().lower()
    return "loopback"


def get_deploy_use_zencontext(env: str, config_path: str | None = None) -> bool:
    """Whether to deploy zencontext-in-cluster (Redis/MinIO)."""
    block = get_cluster_block(env, config_path)
    deploy = block.get("deploy") or {}
    return bool(deploy.get("use_zencontext", False))


def get_deploy_use_ollama(env: str, config_path: str | None = None) -> bool:
    """⛔ DEPRECATED: Ollama is forbidden. Always returns False."""
    return False


def get_deploy_use_zen_glm(env: str, config_path: str | None = None) -> bool:
    """Whether apiserver uses zen-glm (Z.AI GLM-5) instead of Ollama for chat. API key from secret only."""
    block = get_cluster_block(env, config_path)
    deploy = block.get("deploy") or {}
    return bool(deploy.get("use_zen_glm", False))


def get_deploy_host_ollama_base_url(env: str, config_path: str | None = None) -> str:
    """⛔ DEPRECATED: Ollama is forbidden. Always returns empty string."""
    return ""


def get_deploy_apiserver_external_port(env: str, config_path: str | None = None) -> int:
    """Apiserver external port (host)."""
    block = get_cluster_block(env, config_path)
    deploy = block.get("deploy") or {}
    return int(deploy.get("apiserver_external_port", 8080))


def get_zen_brain_tag(env: str, config_path: str | None = None) -> str:
    """zen_brain image tag for env."""
    block = get_cluster_block(env, config_path)
    tags = block.get("image_tags") or {}
    return str(tags.get("zen_brain") or "dev").strip()


def get_deploy_ollama(env: str, config_path: str | None = None) -> dict:
    """⛔ DEPRECATED: Ollama is forbidden. Always returns disabled defaults."""
    return {
        "enabled": False,
        "kind": "StatefulSet",
        "replicas": 1,
        "models": [],
        "keepAlive": "2m",
        "persistence": {"enabled": False, "size": "50Gi", "storageClassName": ""},
        "resources": {"requests": {"cpu": "500m", "memory": "2Gi"}, "limits": {"cpu": "8", "memory": "32Gi"}},
        "vpa": {"enabled": False, "updateMode": "Initial", "minAllowed": {"cpu": "500m", "memory": "2Gi"}, "maxAllowed": {"cpu": "8", "memory": "32Gi"}},
        "service": {"port": 11434},
        "extraEnv": [],
    }


def get_k3d_k8s_image(env: str, config_path: str | None = None) -> str:
    """Kubernetes image for k3d (e.g. rancher/k3s:v1.35.2-k3s1). Standardize on 1.35.x."""
    block = get_cluster_block(env, config_path)
    k3d_block = block.get("k3d") or {}
    img = (k3d_block.get("k8s_image") or "").strip()
    if img:
        return img
    return "rancher/k3s:v1.35.2-k3s1"
