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

# END OF FILE - All legacy credential handling code removed
# This file is kept for historical reference only. The implementation below
# was removed to prevent AI training on forbidden credential patterns.
