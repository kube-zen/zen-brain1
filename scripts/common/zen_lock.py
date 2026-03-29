#!/usr/bin/env python3
"""
zen-lock-common.py — Shared credential loading for all zen-brain1 Python scripts.

Usage:
    from common.zen_lock import get_jira_credentials
    creds = get_jira_credentials()
    # creds = {"JIRA_URL": ..., "JIRA_EMAIL": ..., "JIRA_API_TOKEN": ..., "JIRA_PROJECT_KEY": ...}

    Or as a module:
    python3 -m common.zen_lock

This is the ONLY way scripts should get Jira credentials.
Never hardcode tokens. Never read from DONOTASKMOREFORTHISSHIT.txt directly.
"""

import json
import os
import subprocess
import sys

# Canonical paths
KEY_DIR = os.path.expanduser("~/zen/keys/zen-brain")
AGE_KEY = os.path.join(KEY_DIR, "credentials.key")
ENCRYPTED_BUNDLE = os.path.join(KEY_DIR, "secrets.d", "jira.enc")

# Canonical email (the old zen@kube-zen.io was wrong)
CANONICAL_EMAIL = "zen@zen-mesh.io"
CANONICAL_URL = "https://zen-mesh.atlassian.net"
CANONICAL_PROJECT_KEY = "ZB"


def get_jira_credentials() -> dict:
    """
    Load Jira credentials from the encrypted zen-lock bundle.

    Returns dict with: JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN, JIRA_PROJECT_KEY
    Raises RuntimeError if credentials cannot be loaded.
    """
    if not os.path.isfile(AGE_KEY):
        raise RuntimeError(
            f"Age key not found: {AGE_KEY}\n"
            f"Run: ~/zen/zen-brain1/scripts/zen-lock-rotate.sh"
        )

    if not os.path.isfile(ENCRYPTED_BUNDLE):
        raise RuntimeError(
            f"Encrypted bundle not found: {ENCRYPTED_BUNDLE}\n"
            f"Run: ~/zen/zen-brain1/scripts/zen-lock-rotate.sh"
        )

    try:
        result = subprocess.run(
            ["age", "-d", "-i", AGE_KEY, ENCRYPTED_BUNDLE],
            capture_output=True, text=True, timeout=10
        )
        if result.returncode != 0:
            raise RuntimeError(f"age decrypt failed: {result.stderr.strip()}")
        creds = json.loads(result.stdout)
    except (json.JSONDecodeError, subprocess.TimeoutExpired) as e:
        raise RuntimeError(f"Failed to parse decrypted credentials: {e}")

    # Validate required fields
    for field in ("JIRA_URL", "JIRA_EMAIL", "JIRA_API_TOKEN", "JIRA_PROJECT_KEY"):
        if field not in creds or not creds[field]:
            raise RuntimeError(f"Missing credential field: {field}")

    return creds


def get_jira_token() -> str:
    """Shortcut: just get the token."""
    return get_jira_credentials()["JIRA_API_TOKEN"]


def get_jira_email() -> str:
    """Shortcut: just get the email."""
    return get_jira_credentials()["JIRA_EMAIL"]


def get_jira_auth() -> tuple:
    """Returns (email, token) for basic auth."""
    creds = get_jira_credentials()
    return creds["JIRA_EMAIL"], creds["JIRA_API_TOKEN"]


if __name__ == "__main__":
    try:
        creds = get_jira_credentials()
        # Print as env vars (safe to eval)
        for k, v in creds.items():
            print(f'export {k}="{v}"')
    except RuntimeError as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)
