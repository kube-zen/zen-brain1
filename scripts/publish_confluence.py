#!/usr/bin/env python3
"""
Confluence publishing script for Zen-Brain 1.0.
This script performs a one‑way sync from zen‑docs to Confluence.

Usage: python3 scripts/publish_confluence.py [REPO_PATH] [CONFLUENCE_SPACE]

Environment variables:
    CONFLUENCE_BASE_URL
    CONFLUENCE_USERNAME
    CONFLUENCE_API_TOKEN

NOTE: Confluence sync is optional and may be deferred to a later phase.
This script is a placeholder that can be extended when the feature is enabled.
"""

import os
import sys
import argparse


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Publish docs from zen‑docs repository to Confluence"
    )
    parser.add_argument(
        "repo_path",
        nargs="?",
        default=os.environ.get("ZEN_DOCS_REPO", "../zen-docs"),
        help="Path to the zen‑docs repository (default: $ZEN_DOCS_REPO or ../zen‑docs)",
    )
    parser.add_argument(
        "confluence_space",
        nargs="?",
        default="ZB",
        help="Confluence space key (default: ZB)",
    )
    args = parser.parse_args()

    # Validate repo path
    if not os.path.isdir(args.repo_path):
        print(f"ERROR: Repository path '{args.repo_path}' does not exist.", file=sys.stderr)
        print("Set ZEN_DOCS_REPO or pass the path as the first argument.", file=sys.stderr)
        return 1

    # Check required environment variables
    required_env = [
        "CONFLUENCE_BASE_URL",
        "CONFLUENCE_USERNAME",
        "CONFLUENCE_API_TOKEN",
    ]
    missing = [var for var in required_env if var not in os.environ]
    if missing:
        for var in missing:
            print(f"ERROR: {var} is not set.", file=sys.stderr)
        return 1

    print(f"Publishing docs from {args.repo_path} to Confluence space {args.confluence_space}")
    print()
    print("NOTE: Confluence sync is currently an optional / deferred feature.")
    print("To enable it, implement the actual sync logic in this script.")
    print()
    print("Example integration (when enabled):")
    print("  import confluence_sync")
    print("  sync = confluence_sync.SyncClient(")
    print(f"      repo='{args.repo_path}',")
    print(f"      space='{args.confluence_space}',")
    print(f"      url='{os.environ['CONFLUENCE_BASE_URL']}',")
    print(f"      username='{os.environ['CONFLUENCE_USERNAME']}',")
    print(f"      api_token='{os.environ['CONFLUENCE_API_TOKEN']}',")
    print("  )")
    print("  sync.run()")
    print()
    print("Publish completed (placeholder).")
    return 0


if __name__ == "__main__":
    sys.exit(main())