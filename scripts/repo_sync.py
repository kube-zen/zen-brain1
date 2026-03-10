#!/usr/bin/env python3
"""
Sync knowledge-base repositories for QMD population (Block 5.1).

Clones or pulls configured KB repos so ZenContext Tier 2 (QMD) can index them.
See docs/01-ARCHITECTURE/BLOCK5_QMD_POPULATION.md and COMPLETENESS_MATRIX.md.

Usage:
  python3 scripts/repo_sync.py

Environment:
  ZEN_KB_REPO_URL  Optional. Git URL to clone (e.g. https://github.com/org/zen-docs.git).
                   If set, repo is cloned into ZEN_KB_REPO_DIR if missing, or pulled if present.
  ZEN_KB_REPO_DIR  Target directory (default: ../zen-docs). Must match tier2_qmd.repo_path in config.
"""

import os
import subprocess
import sys


def is_git_repo(path: str) -> bool:
    return os.path.isdir(os.path.join(path, ".git"))


def clone(url: str, dest: str) -> int:
    os.makedirs(os.path.dirname(dest) or ".", exist_ok=True)
    r = subprocess.run(
        ["git", "clone", "--depth", "1", url, dest],
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print(f"git clone failed: {r.stderr or r.stdout}", file=sys.stderr)
    return r.returncode


def pull(repo_dir: str) -> int:
    r = subprocess.run(
        ["git", "pull", "--rebase"],
        cwd=repo_dir,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        print(f"git pull failed in {repo_dir}: {r.stderr or r.stdout}", file=sys.stderr)
    return r.returncode


def main() -> int:
    url = os.environ.get("ZEN_KB_REPO_URL", "").strip()
    dest = os.environ.get("ZEN_KB_REPO_DIR", "../zen-docs").strip()
    dest = os.path.abspath(dest)

    if not url:
        if os.path.isdir(dest) and is_git_repo(dest):
            print(f"ZEN_KB_REPO_URL not set; pulling existing repo at {dest}")
            return pull(dest)
        print(
            "ZEN_KB_REPO_URL not set. Set it to clone a KB repo, e.g.:",
            file=sys.stderr,
        )
        print("  export ZEN_KB_REPO_URL=https://github.com/org/zen-docs.git", file=sys.stderr)
        print(f"  export ZEN_KB_REPO_DIR={dest}  # optional; default ../zen-docs", file=sys.stderr)
        return 0

    if not os.path.exists(dest):
        print(f"Cloning {url} -> {dest}")
        return clone(url, dest)
    if is_git_repo(dest):
        print(f"Pulling {dest}")
        return pull(dest)
    print(f"ERROR: {dest} exists but is not a git repo.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
