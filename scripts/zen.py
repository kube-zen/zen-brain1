#!/usr/bin/env python3
"""
Single entrypoint for zen-brain deployment lifecycle (hosts, k3d, image, env).
Config-driven via config/clusters.yaml; uses 127.0.1.x and zen-brain-registry:5001.
"""
from __future__ import annotations

import argparse
import os
import subprocess
import sys

_scripts_dir = os.path.dirname(os.path.abspath(__file__))
_common_dir = os.path.join(_scripts_dir, "common")
if _common_dir not in sys.path:
    sys.path.insert(0, _common_dir)


def _repo_root() -> str:
    return os.path.abspath(os.path.join(_scripts_dir, ".."))


def _default_config() -> str:
    return os.path.join(_repo_root(), "config", "clusters.yaml")


def _add_env(p: argparse.ArgumentParser) -> None:
    p.add_argument("--env", required=True, metavar="ENV", help="Environment (e.g. sandbox, staging, uat)")
    p.add_argument("--config", default=_default_config(), help="Path to config/clusters.yaml")


def cmd_hosts(args: argparse.Namespace) -> int:
    import hosts
    if args.hosts_cmd == "apply":
        hosts.cmd_apply(args.env, args.config, getattr(args, "hosts_path", hosts.HOSTS_FILE), getattr(args, "dry_run", False))
    elif args.hosts_cmd == "remove":
        hosts.cmd_remove(args.env, args.config, getattr(args, "hosts_path", hosts.HOSTS_FILE), getattr(args, "dry_run", False))
    elif args.hosts_cmd == "verify":
        return hosts.cmd_verify(args.env, args.config, getattr(args, "hosts_path", hosts.HOSTS_FILE))
    return 0


def cmd_k3d(args: argparse.Namespace) -> int:
    import k3d
    if args.k3d_cmd == "ensure":
        return k3d.cmd_ensure(args.env, args.config, getattr(args, "force_recreate", False))
    if args.k3d_cmd == "destroy":
        return k3d.cmd_destroy(args.env, args.config, getattr(args, "confirm_destroy", False))
    return 1


def cmd_image(args: argparse.Namespace) -> int:
    import config as _config
    root = _repo_root()
    tag = _config.get_zen_brain_tag(args.env, args.config)
    reg_host = _config.get_registry_host_ref(args.config)
    if args.image_cmd == "build":
        r = subprocess.run(["docker", "build", "-t", f"zen-brain:{tag}", "."], cwd=root, timeout=600)
        if r.returncode != 0:
            return 1
    if args.image_cmd in ("load", "build"):
        import registry
        registry.cmd_ensure(args.config)
        subprocess.run(["docker", "tag", f"zen-brain:{tag}", f"{reg_host}/zen-brain:{tag}"], check=True, timeout=10)
        subprocess.run(["docker", "push", f"{reg_host}/zen-brain:{tag}"], check=True, timeout=120)
        block = _config.get_cluster_block(args.env, args.config)
        k3d_block = block.get("k3d") or {}
        cluster_name = str(k3d_block.get("cluster_name") or f"zen-brain-{args.env}").strip()
        reg_ref = _config.get_registry_cluster_ref(args.config)
        r = subprocess.run(
            ["k3d", "image", "import", f"{reg_ref}/zen-brain:{tag}", "-c", cluster_name],
            capture_output=True,
            text=True,
            timeout=120,
        )
        if r.returncode != 0:
            print(r.stderr or r.stdout, file=sys.stderr)
            return 1
        print(f"Image zen-brain:{tag} loaded into cluster {cluster_name}")
    return 0


def cmd_env(args: argparse.Namespace) -> int:
    import env
    if args.env_cmd == "redeploy":
        return env.cmd_redeploy(
            args.env,
            args.config,
            skip_hosts=getattr(args, "skip_hosts", False),
            skip_registry=getattr(args, "skip_registry", False),
            skip_k3d=getattr(args, "skip_k3d", False),
            skip_manifests=getattr(args, "skip_manifests", False),
            skip_build=getattr(args, "skip_build", False),
            skip_image_load=getattr(args, "skip_image_load", False),
            skip_ollama=getattr(args, "skip_ollama", False),
            force_recreate=getattr(args, "force_recreate", False),
        )
    if args.env_cmd == "destroy":
        return env.cmd_destroy(
            args.env,
            args.config,
            confirm_destroy=getattr(args, "confirm_destroy", False) or os.environ.get("CONFIRM_DESTROY") == "1",
            remove_hosts=getattr(args, "remove_hosts", False),
            remove_registry=getattr(args, "remove_registry", False),
        )
    if args.env_cmd == "status":
        return env.cmd_status(args.env, args.config)
    return 1


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Zen-brain deployment CLI (config-driven, 127.0.1.x, zen-brain-registry:5001)",
    )
    parser.add_argument("--config", default=_default_config(), help="Path to config/clusters.yaml")
    sub = parser.add_subparsers(dest="command", required=True)

    # hosts
    hosts_p = sub.add_parser("hosts", help="Manage /etc/hosts (zen-brain block)")
    hosts_sub = hosts_p.add_subparsers(dest="hosts_cmd", required=True)
    ha = hosts_sub.add_parser("apply", help="Apply hosts entries for env")
    _add_env(ha)
    ha.add_argument("--hosts-path", default="/etc/hosts", help="Hosts file path")
    ha.add_argument("--dry-run", action="store_true")
    hr = hosts_sub.add_parser("remove", help="Remove hosts block for env")
    _add_env(hr)
    hr.add_argument("--hosts-path", default="/etc/hosts")
    hr.add_argument("--dry-run", action="store_true")
    hv = hosts_sub.add_parser("verify", help="Verify hosts resolution for env")
    _add_env(hv)
    hv.add_argument("--hosts-path", default="/etc/hosts")

    # k3d
    k3d_p = sub.add_parser("k3d", help="k3d cluster ensure/destroy")
    k3d_sub = k3d_p.add_subparsers(dest="k3d_cmd", required=True)
    ke = k3d_sub.add_parser("ensure", help="Create cluster if missing")
    _add_env(ke)
    ke.add_argument("--force-recreate", action="store_true", help="Replace existing cluster if needed")
    kd = k3d_sub.add_parser("destroy", help="Destroy k3d cluster")
    _add_env(kd)
    kd.add_argument("--confirm-destroy", action="store_true", help="Confirm destroy (or set CONFIRM_DESTROY=1)")

    # image
    img_p = sub.add_parser("image", help="Build and load zen-brain image")
    img_sub = img_p.add_subparsers(dest="image_cmd", required=True)
    ib = img_sub.add_parser("build", help="Build and load image (build + load)")
    _add_env(ib)
    il = img_sub.add_parser("load", help="Load existing image into cluster (no build)")
    _add_env(il)

    # env
    env_p = sub.add_parser("env", help="Environment lifecycle (redeploy, destroy, status)")
    env_sub = env_p.add_subparsers(dest="env_cmd", required=True)
    er = env_sub.add_parser("redeploy", help="Full redeploy: registry, k3d, manifests, image, rollout")
    _add_env(er)
    er.add_argument("--skip-hosts", action="store_true")
    er.add_argument("--skip-registry", action="store_true")
    er.add_argument("--skip-k3d", action="store_true")
    er.add_argument("--skip-manifests", action="store_true")
    er.add_argument("--skip-build", action="store_true", help="Skip docker build (use existing image)")
    er.add_argument("--skip-image-load", action="store_true")
    er.add_argument("--skip-ollama", action="store_true", help="Skip ollama release (keep existing ollama pod)")
    er.add_argument("--force-recreate", action="store_true")
    ed = env_sub.add_parser("destroy", help="Destroy environment (k3d cluster)")
    _add_env(ed)
    ed.add_argument("--confirm-destroy", action="store_true")
    ed.add_argument("--remove-hosts", action="store_true")
    ed.add_argument("--remove-registry", action="store_true")
    es = env_sub.add_parser("status", help="Show cluster and endpoint status")
    _add_env(es)

    args = parser.parse_args()

    if args.command == "hosts":
        return cmd_hosts(args)
    if args.command == "k3d":
        return cmd_k3d(args)
    if args.command == "image":
        return cmd_image(args)
    if args.command == "env":
        return cmd_env(args)
    return 1


if __name__ == "__main__":
    sys.exit(main())
