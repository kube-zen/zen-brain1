#!/usr/bin/env python3
"""
HARD-FAIL DEPRECATED: This script is QUARANTINED and must not be used.

███╗   ██╗███████╗██╗    ██╗    ██╗   ██╗███████╗
████╗  ██║██╔════╝██║    ██║    ██║   ██║██╔════╝
██╔██╗ ██║█████╗  ██║ █╗ ██║    ██║   ██║███████╗
██║╚██╗██║██╔══╝  ██║███╗██║    ██║   ██║╚════██║
██║ ╚████║███████╗╚███╔███╔╝    ╚██████╔╝███████║
╚═╝  ╚═══╝╚══════╝ ╚══╝╚══╝      ╚═════╝ ╚══════╝
                                                 
Credential Rails Enforcement - Layer 3

This script is QUARANTINED. It uses deprecated credential paths that violate
the canonical credential model enforced by CI gates.

CANONICAL PATH (USE THIS INSTEAD):
  Cluster: /zen-lock/secrets/* (mounted by ZenLock, read-only)
  Local:   internal/secrets/jira.go:ResolveJira() with DirPath

WHAT CHANGED:
  - Old: ~/.zen-brain/jira-credentials.env (FORBIDDEN)
  - Old: Environment variables as primary source (FORBIDDEN in cluster)
  - New: /zen-lock/secrets/* (cluster runtime ONLY)
  - New: secrets.ResolveJira() (code canonical resolver)

For local development:
  1. Use: internal/secrets/jira.go with AllowEnvFallback: true
  2. Or: source credentials from ~/zen/keys/zen-brain/secrets.d/jira.enc

CI ENFORCEMENT:
  - scripts/ci/canonical_credential_access_gate.py blocks direct env access
  - scripts/ci/zenlock_mount_only_gate.py enforces mount-only in K8s
  - scripts/ci/docs_drift_credential_rails_gate.py ensures doc consistency

DO NOT MODIFY THIS SCRIPT. It is quarantined for historical reference only.
"""

import sys

# HARD-FAIL: Prevent any execution of this deprecated script
def main() -> int:
    print("\033[0;31m╔═══════════════════════════════════════════════════════════╗\033[0m", file=sys.stderr)
    print("\033[0;31m║  QUARANTINED SCRIPT - EXECUTION BLOCKED                     ║\033[0m", file=sys.stderr)
    print("\033[0;31m╠═══════════════════════════════════════════════════════════╣\033[0m", file=sys.stderr)
    print("\033[0;31m║  scripts/load_jira_credentials.py is DEPRECATED             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║                                                             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║  This script uses forbidden credential paths:               ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    - ~/.zen-brain/jira-credentials.env (FORBIDDEN)          ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    - Environment variables in cluster mode (FORBIDDEN)      ║\033[0m", file=sys.stderr)
    print("\033[0;31m║                                                             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║  CANONICAL PATH:                                            ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    Cluster: /zen-lock/secrets/* (ZenLock mount)             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    Local:   secrets.ResolveJira() with DirPath              ║\033[0m", file=sys.stderr)
    print("\033[0;31m║                                                             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║  See docs/05-OPERATIONS/CREDENTIAL_RAILS.md for details     ║\033[0m", file=sys.stderr)
    print("\033[0;31m╚═══════════════════════════════════════════════════════════╝\033[0m", file=sys.stderr)
    print("", file=sys.stderr)
    print("Credential Rails Enforcement - Layer 3", file=sys.stderr)
    print("Exit code 1 - Quarantined script execution blocked", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())

import os
import sys
from pathlib import Path


def _print_error(msg: str) -> None:
    print(f"\033[0;31mError: {msg}\033[0m", file=sys.stderr)


def _print_success(msg: str) -> None:
    print(f"\033[0;32m✓ {msg}\033[0m")


def _print_warning(msg: str) -> None:
    print(f"\033[1;33m{msg}\033[0m")


def _print_header(msg: str) -> None:
    print(f"=== {msg} ===")


def _is_kubernetes_runtime() -> bool:
    """Check if running in Kubernetes environment."""
    return "KUBERNETES_SERVICE_HOST" in os.environ


def _load_env_file(env_file_path: Path) -> dict:
    """Load environment variables from file."""
    creds = {}
    with open(env_file_path) as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                key, value = line.split("=", 1)
                # Remove quotes if present
                value = value.strip().strip('"').strip("'")
                creds[key] = value
    return creds


def _validate_credentials(creds: dict) -> bool:
    """Validate that all required credentials are present."""
    required = ["JIRA_URL", "JIRA_EMAIL", "JIRA_TOKEN", "JIRA_PROJECT_KEY"]
    for field in required:
        if not creds.get(field):
            return False
    return True


def _print_credentials_info(creds: dict) -> None:
    """Print credential information for validation."""
    print("Validating credentials...")
    print(f"  JIRA_URL: {creds.get('JIRA_URL', 'not set')}")
    print(f"  JIRA_EMAIL: {creds.get('JIRA_EMAIL', 'not set')}")
    print(f"  JIRA_PROJECT_KEY: {creds.get('JIRA_PROJECT_KEY', 'not set')}")
    token = creds.get("JIRA_TOKEN", "")
    print(f"  JIRA_TOKEN: {token[:10]}..." if len(token) > 10 else "  JIRA_TOKEN: not set")
    print()


def _export_credentials(creds: dict) -> None:
    """Export credentials to environment."""
    os.environ["JIRA_URL"] = creds["JIRA_URL"]
    os.environ["JIRA_EMAIL"] = creds["JIRA_EMAIL"]
    os.environ["JIRA_TOKEN"] = creds["JIRA_TOKEN"]
    os.environ["JIRA_PROJECT_KEY"] = creds["JIRA_PROJECT_KEY"]


def _print_example_credentials() -> None:
    """Print example credentials file content."""
    print('JIRA_URL="https://your-company.atlassian.net"')
    print('JIRA_EMAIL="your-email@company.com"')
    print('JIRA_TOKEN="<generate-token-at-id.atlassian.com/manage-profile/security/api-tokens>"')
    print('JIRA_PROJECT_KEY="YOUR_PROJECT_KEY"')


def _print_kubernetes_instructions() -> None:
    """Print instructions for Kubernetes runtime."""
    print("For Kubernetes runtime:")
    print("  1. Deploy ZenLock resource: kubectl apply -f deploy/zen-lock/jira-zenlock.yaml")
    print("  2. Zen-Brain will consume credentials from env vars injected by ZenLock")


def _print_host_instructions(env_file_path: Path) -> None:
    """Print instructions for host runtime."""
    print("For host runtime:")
    print(f"  1. Create credential file at: {env_file_path}")
    print("  2. Add the following content:")
    print()
    _print_example_credentials()
    print()
    print(f"IMPORTANT: Never commit {env_file_path} to Git")
    print(f"IMPORTANT: Add {env_file_path} to .gitignore")


def main() -> int:
    _print_header("Zen-Brain Jira Credential Loader")
    print()
    
    # Check runtime mode
    if _is_kubernetes_runtime():
        _print_success("Detected Kubernetes runtime")
        print("  Jira credentials will be consumed from ZenLock-injected env vars")
        print("  No action needed - Zen-Brain will read from env vars")
        print()
        return 0
    
    # Host runtime mode
    _print_warning("Detected host runtime mode")
    print()
    
    # Check for ZenLock-managed env file
    env_file_path = Path.home() / ".zen-brain" / "jira-credentials.env"
    
    if env_file_path.exists():
        _print_success(f"Loading Jira credentials from: {env_file_path}")
        
        # Load credentials
        creds = _load_env_file(env_file_path)
        _print_success("Credentials loaded")
        print()
        
        # Validate credentials
        _print_credentials_info(creds)
        
        if not _validate_credentials(creds):
            _print_error("One or more Jira credentials are missing")
            print()
            print(f"Check {env_file_path} and ensure all required fields are set:")
            print("  JIRA_URL")
            print("  JIRA_EMAIL")
            print("  JIRA_TOKEN")
            print("  JIRA_PROJECT_KEY")
            return 1
        
        _print_success("All credentials present")
        print()
        print("Credentials are now available in environment for Zen-Brain commands")
        print()
        print("Example usage:")
        print("  ./bin/zen-brain office doctor")
        print("  ./bin/zen-brain self-improvement")
        print()
        
        # Export credentials for child processes
        _export_credentials(creds)
        
        return 0
    else:
        _print_warning(f"Jira credential file not found: {env_file_path}")
        print()
        _print_kubernetes_instructions()
        print()
        _print_host_instructions(env_file_path)
        print()
        return 1


if __name__ == "__main__":
    sys.exit(main())
