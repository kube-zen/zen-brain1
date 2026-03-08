#!/usr/bin/env python3
"""
qmd refresh script for Zen‑Brain 1.0.
This script updates the qmd index for the zen‑docs repository.

Usage: python3 scripts/qmd_refresh.py [REPO_PATH]

If REPO_PATH is omitted, defaults to $ZEN_DOCS_REPO or ../zen‑docs.

NOTE: qmd is used for search/index only, not as the source of truth.
The source of truth remains the zen‑docs Git repository.
"""

import os
import sys
import argparse
import subprocess


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Refresh qmd index for a zen‑docs repository"
    )
    parser.add_argument(
        "repo_path",
        nargs="?",
        default=os.environ.get("ZEN_DOCS_REPO", "../zen-docs"),
        help="Path to the zen‑docs repository (default: $ZEN_DOCS_REPO or ../zen‑docs)",
    )
    args = parser.parse_args()

    if not os.path.isdir(args.repo_path):
        print(f"ERROR: Repository path '{args.repo_path}' does not exist.", file=sys.stderr)
        print("Set ZEN_DOCS_REPO or pass the path as the first argument.", file=sys.stderr)
        return 1

    print(f"Refreshing qmd index for {args.repo_path}")
    print()
    print("NOTE: qmd is a search/index tool, not the source of truth.")
    print("The source of truth is the Git repository itself.")
    print()

    # Try to run qmd if available; otherwise show placeholder.
    try:
        # Example: qmd embed --repo <path> --paths docs/ --verbose
        result = subprocess.run(
            ["qmd", "embed", "--repo", args.repo_path, "--paths", "docs/", "--verbose"],
            capture_output=True,
            text=True,
            timeout=30,
        )
        if result.returncode == 0:
            print("qmd index refreshed successfully.")
            print(result.stdout)
            return 0
        else:
            print(f"qmd returned error (exit code {result.returncode}):", file=sys.stderr)
            print(result.stderr, file=sys.stderr)
            return result.returncode
    except FileNotFoundError:
        print("qmd command not found. (Install qmd for actual indexing.)")
        print()
        print("Placeholder command that would run:")
        print(f"  qmd embed --repo '{args.repo_path}' --paths docs/ --verbose")
        print()
        print("Index refresh completed (placeholder).")
        return 0


if __name__ == "__main__":
    sys.exit(main())