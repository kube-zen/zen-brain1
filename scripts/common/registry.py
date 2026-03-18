#!/usr/bin/env python3
"""
Local registry lifecycle for zen-brain. Uses shared registry (zen-registry:5000).
Ensures the registry container exists and is running; does not remove by default.
"""
from __future__ import annotations

import os
import subprocess
import sys

_common_dir = os.path.dirname(os.path.abspath(__file__))
if _common_dir not in sys.path:
    sys.path.insert(0, _common_dir)
import config as _config  # noqa: E402


def _err(msg: str) -> None:
    print(msg, file=sys.stderr, flush=True)


def _registry_running(container_name: str) -> bool:
    r = subprocess.run(
        ["docker", "ps", "-q", "-f", f"name=^{container_name}$"],
        capture_output=True,
        text=True,
        timeout=5,
    )
    return r.returncode == 0 and bool((r.stdout or "").strip())


def cmd_ensure(config_path: str | None = None) -> int:
    """Ensure local registry container exists and is running on configured port."""
    container = _config.get_registry_container_name(config_path)
    port = _config.get_registry_host_port(config_path)
    if _registry_running(container):
        print(f"Registry already running: {container} (localhost:{port})", flush=True)
        return 0
    r = subprocess.run(
        [
            "docker", "run", "-d", "--restart=unless-stopped",
            "--name", container,
            "-p", f"{port}:5000",
            "registry:2",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    if r.returncode != 0:
        _err(f"Failed to start registry: {r.stderr or r.stdout or 'unknown'}")
        return 1
    print(f"Registry started: {container} (localhost:{port})", flush=True)
    return 0


def cmd_stop(config_path: str | None = None, remove: bool = False) -> int:
    """Stop (and optionally remove) the registry container."""
    container = _config.get_registry_container_name(config_path)
    if not _registry_running(container):
        r = subprocess.run(["docker", "ps", "-a", "-q", "-f", f"name=^{container}$"], capture_output=True, text=True, timeout=5)
        if r.returncode == 0 and (r.stdout or "").strip():
            if remove:
                subprocess.run(["docker", "rm", "-f", container], capture_output=True, timeout=10)
        print("Registry not running.", flush=True)
        return 0
    subprocess.run(["docker", "stop", container], check=True, capture_output=True, timeout=15)
    if remove:
        subprocess.run(["docker", "rm", container], check=True, capture_output=True, timeout=5)
    print(f"Registry stopped (remove={remove}).", flush=True)
    return 0
