#!/usr/bin/env python3
"""
Generate Helm values from config/clusters.yaml for Helmfile.
Writes .artifacts/state/<env>/*-values.yaml so Helmfile consumes env contract without manual edits.
"""
from __future__ import annotations

import os
import sys

if os.path.dirname(os.path.abspath(__file__)) not in sys.path:
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import config as _config  # noqa: E402

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore[assignment]


def _repo_root() -> str:
    return os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _state_dir(env: str) -> str:
    return os.path.join(_repo_root(), ".artifacts", "state", env)


def render(env: str, config_path: str | None = None) -> None:
    """Generate .artifacts/state/<env>/*-values.yaml from clusters.yaml."""
    if yaml is None:
        raise RuntimeError("PyYAML required. pip install pyyaml")
    root = _repo_root()
    if config_path is None:
        config_path = os.path.join(root, "config", "clusters.yaml")
    state_dir = _state_dir(env)
    os.makedirs(state_dir, exist_ok=True)

    use_zencontext = _config.get_deploy_use_zencontext(env, config_path)
    use_ollama = _config.get_deploy_use_ollama(env, config_path)
    use_zen_glm = _config.get_deploy_use_zen_glm(env, config_path)
    tag = _config.get_zen_brain_tag(env, config_path)
    ollama_block = _config.get_deploy_ollama(env, config_path)
    apiserver_port = _config.get_deploy_apiserver_external_port(env, config_path)

    # zen-brain-dependencies
    deps_values = {
        "zencontext": {"enabled": use_zencontext},
    }
    path = os.path.join(state_dir, "zen-brain-dependencies-values.yaml")
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(deps_values, f, default_flow_style=False, sort_keys=False)

    # zen-brain (core): use cluster registry ref for shared registry (zen-registry:5000/zen-brain)
    reg_ref = _config.get_registry_cluster_ref(config_path)
    # Priority: host_ollama_base_url (Docker on host) > k8s ollama > empty
    host_ollama_url = _config.get_deploy_host_ollama_base_url(env, config_path)
    ollama_base_url = host_ollama_url or ("" if use_zen_glm else ("http://ollama:11434" if use_ollama else ""))
    apiserver_port = _config.get_deploy_apiserver_external_port(env, config_path)
    apiserver_extra = {
        "service": {"type": "LoadBalancer", "externalPort": apiserver_port},
        "env": [{"name": "ZEN_RUNTIME_PROFILE", "value": "dev"}]
    }
    if use_zen_glm:
        apiserver_extra["zenGlmSecretName"] = "zen-glm-api-key"
    foreman_extra = {"env": [{"name": "ZEN_RUNTIME_PROFILE", "value": "dev"}]}
    zen_values = {
        "image": {"repository": f"{reg_ref}/zen-brain", "tag": tag, "pullPolicy": "IfNotPresent"},
        "ollama": {"baseUrl": ollama_base_url, "timeoutSeconds": 3600},
        "apiserver": apiserver_extra,
        "foreman": foreman_extra,
    }
    path = os.path.join(state_dir, "zen-brain-values.yaml")
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(zen_values, f, default_flow_style=False, sort_keys=False)

    # zen-lock: use shared registry path (zen-registry:5000/kubezen/zen-lock)
    zen_lock_values = {}
    zen_lock_values["image"] = {"repository": "zen-registry:5000/kubezen/zen-lock", "tag": "0.0.3-alpha", "pullPolicy": "IfNotPresent"}
    zen_lock_values["replicaCount"] = 1
    zen_lock_values["controller"] = {}
    zen_lock_values["controller"]["enabled"] = True
    zen_lock_values["controller"]["replicaCount"] = 1
    zen_lock_values["controller"]["resources"] = {}
    zen_lock_values["controller"]["resources"]["requests"] = {"cpu": "100m", "memory": "128Mi"}
    zen_lock_values["controller"]["resources"]["limits"] = {"cpu": "500m", "memory": "512Mi"}
    zen_lock_values["webhook"] = {}
    zen_lock_values["webhook"]["enabled"] = True
    zen_lock_values["webhook"]["replicaCount"] = 1
    zen_lock_values["webhook"]["resources"] = {}
    zen_lock_values["webhook"]["resources"]["requests"] = {"cpu": "100m", "memory": "128Mi"}
    zen_lock_values["webhook"]["resources"]["limits"] = {"cpu": "500m", "memory": "512Mi"}
    zen_lock_path = os.path.join(state_dir, "zen-lock-values.yaml")
    with open(zen_lock_path, "w", encoding="utf-8") as f:
        yaml.safe_dump(zen_lock_values, f, default_flow_style=False, sort_keys=False)

    # zen-brain-ollama: full contract (StatefulSet, VPA Initial, keepAlive, etc.)
    ollama_values = {
        "enabled": ollama_block.get("enabled", False),
        "kind": ollama_block.get("kind", "StatefulSet"),
        "replicas": ollama_block.get("replicas", 1),
        "models": ollama_block.get("models") or [],
        "keepAlive": ollama_block.get("keepAlive", "2m"),
        "persistence": ollama_block.get("persistence") or {"enabled": True, "size": "50Gi", "storageClassName": ""},
        "service": ollama_block.get("service") or {"port": 11434},
        "resources": ollama_block.get("resources") or {"requests": {"cpu": "500m", "memory": "2Gi"}, "limits": {"cpu": "8", "memory": "32Gi"}},
        "vpa": ollama_block.get("vpa") or {"enabled": True, "updateMode": "Initial", "minAllowed": {"cpu": "500m", "memory": "2Gi"}, "maxAllowed": {"cpu": "8", "memory": "32Gi"}},
        "extraEnv": ollama_block.get("extraEnv") or [],
    }
    path = os.path.join(state_dir, "zen-brain-ollama-values.yaml")
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(ollama_values, f, default_flow_style=False, sort_keys=False)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: helmfile_values.py <env> [config_path]", file=sys.stderr)
        sys.exit(1)
    env_name = sys.argv[1]
    cfg_path = sys.argv[2] if len(sys.argv) > 2 else None
    render(env_name, cfg_path)
