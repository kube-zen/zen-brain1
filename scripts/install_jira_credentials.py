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
  deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh

WHAT CHANGED:
  - Old: ~/.zen-brain/secrets/jira.yaml (FORBIDDEN)
  - Old: ~/.zen-lock/private-key.age (FORBIDDEN)
  - New: ~/zen/keys/zen-brain/credentials.key (canonical)
  - New: ~/zen/keys/zen-brain/secrets.d/jira.enc (canonical)
  - New: /zen-lock/secrets/* (cluster runtime ONLY)

If you need to rotate credentials:
  1. Place token in ~/zen/DONOTASKMOREFORTHISSHIT.txt
  2. Run: deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
  3. Delete plaintext token file
  4. Verify: zen-brain office doctor

CI ENFORCEMENT:
  - scripts/ci/canonical_credential_access_gate.py blocks new usage
  - scripts/ci/no_alt_credential_rails_gate.py blocks alternate paths
  - scripts/ci/docs_drift_credential_rails_gate.py ensures doc consistency

DO NOT MODIFY THIS SCRIPT. It is quarantined for historical reference only.
"""

import sys

# HARD-FAIL: Prevent any execution of this deprecated script
def main() -> int:
    print("\033[0;31m╔═══════════════════════════════════════════════════════════╗\033[0m", file=sys.stderr)
    print("\033[0;31m║  QUARANTINED SCRIPT - EXECUTION BLOCKED                     ║\033[0m", file=sys.stderr)
    print("\033[0;31m╠═══════════════════════════════════════════════════════════╣\033[0m", file=sys.stderr)
    print("\033[0;31m║  scripts/install_jira_credentials.py is DEPRECATED          ║\033[0m", file=sys.stderr)
    print("\033[0;31m║                                                             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║  This script uses forbidden credential paths:               ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    - ~/.zen-brain/secrets/jira.yaml (FORBIDDEN)             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    - ~/.zen-lock/private-key.age (FORBIDDEN)                ║\033[0m", file=sys.stderr)
    print("\033[0;31m║                                                             ║\033[0m", file=sys.stderr)
    print("\033[0;31m║  CANONICAL PATH:                                            ║\033[0m", file=sys.stderr)
    print("\033[0;31m║    deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh     ║\033[0m", file=sys.stderr)
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
