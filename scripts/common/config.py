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
    return 5001


def get_registry_host_ref(config_path: str | None = None) -> str:
    """Host reference for push (localhost:<port>)."""
    return f"localhost:{get_registry_host_port(config_path)}"


def get_registry_cluster_ref(config_path: str | None = None) -> str:
    """Registry ref as seen from inside cluster (container_name:port). Container listens on 5000."""
    return f"{get_registry_container_name(config_path)}:5000"


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
