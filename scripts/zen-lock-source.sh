#!/usr/bin/env bash
# ██████╗  ██████╗ ██╗     ██╗     ██╗███╗   ██╗ ██████╗ 
# ██╔══██╗██╔═══██╗██║     ██║     ██║████╗  ██║██╔════╝ 
# ██████╔╝██║   ██║██║     ██║     ██║██╔██╗ ██║██║  ███╗
# ██╔═══╝ ██║   ██║██║     ██║     ██║██║╚██╗██║██║   ██║
# ██║     ╚██████╔╝███████╗███████╗██║██║ ╚████║╚██████╔╝
# ╚═╝      ╚═════╝ ╚══════╝╚══════╝╚═╝╚═╝  ╚═══╝ ╚═════╝ 
#                                                        
# Credential Rails Enforcement - Layer 3
#
# QUARANTINED: This script is DEPRECATED and must not be used.
#
# This script exports credentials as environment variables, which violates
# the canonical credential model:
#   - Cluster mode: MUST use /zen-lock/secrets/* mount (no env vars)
#   - Local mode: MUST use internal/secrets/jira.go resolver
#
# CANONICAL PATH (USE THIS INSTEAD):
#   Cluster: /zen-lock/secrets/* (mounted by ZenLock, read-only)
#   Local:   internal/secrets/jira.go:ResolveJira() with DirPath
#
# For local development:
#   Use: deploy/zen-lock/bootstrap-jira-zenlock-from-local.sh
#
# CI ENFORCEMENT:
#   - scripts/ci/canonical_credential_access_gate.py blocks direct env access
#   - scripts/ci/zenlock_mount_only_gate.py enforces mount-only in K8s
#
# DO NOT USE THIS SCRIPT. It is quarantined for historical reference only.

set -euo pipefail

# HARD-FAIL: Prevent any execution of this deprecated script
echo "╔═══════════════════════════════════════════════════════════╗" >&2
echo "║  QUARANTINED SCRIPT - EXECUTION BLOCKED                     ║" >&2
echo "╠═══════════════════════════════════════════════════════════╣" >&2
echo "║  scripts/zen-lock-source.sh is DEPRECATED                   ║" >&2
echo "║                                                             ║" >&2
echo "║  This script exports credentials as environment variables,  ║" >&2
echo "║  which violates the canonical credential model.             ║" >&2
echo "║                                                             ║" >&2
echo "║  CANONICAL PATH:                                            ║" >&2
echo "║    Cluster: /zen-lock/secrets/* (ZenLock mount)             ║" >&2
echo "║    Local:   internal/secrets/jira.go:ResolveJira()          ║" >&2
echo "║                                                             ║" >&2
echo "║  See docs/05-OPERATIONS/CREDENTIAL_RAILS.md for details     ║" >&2
echo "╚═══════════════════════════════════════════════════════════╝" >&2
echo "" >&2
echo "Credential Rails Enforcement - Layer 3" >&2
echo "Exit code 1 - Quarantined script execution blocked" >&2
exit 1
