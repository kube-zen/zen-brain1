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
    use_zen_glm = _config.get_deploy_use_zen_glm(env, config_path)
    tag = _config.get_zen_brain_tag(env, config_path)
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
    # Ollama is FORBIDDEN — do NOT inject ollama.baseUrl into generated values.
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

    # zen-brain-ollama: FORBIDDEN — do NOT generate values file.
    # The helmfile release is commented out; generating values would be misleading.
    # If the ollama release is somehow re-enabled, CI will block it.


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: helmfile_values.py <env> [config_path]", file=sys.stderr)
        sys.exit(1)
    env_name = sys.argv[1]
    cfg_path = sys.argv[2] if len(sys.argv) > 2 else None
    render(env_name, cfg_path)
