#!/bin/bash
# Guardrail: prevent compiled binaries from being tracked in git.
#
# This catches binaries that were accidentally staged or committed
# under known build output paths. Run as a pre-commit hook or CI check.
#
# Exit 0 = clean (no forbidden binaries staged)
# Exit 1 = forbidden binaries found
#
# Usage:
#   ./scripts/guardrails/check-no-tracked-binaries.sh [--ci]
#
# --ci mode: check all tracked files (not just staged), for CI pipelines

set -euo pipefail

FORBIDDEN_PATTERNS=(
    "cmd/useful-batch/useful-batch"
    "cmd/scheduler/scheduler"
    "cmd/admission-gate/admission-gate"
    "cmd/jira-ledger/jira-ledger"
    "^scheduler$"
    "^useful-batch$"
)

if [[ "${1:-}" == "--ci" ]]; then
    # CI mode: check all tracked files
    FILES=$(cd "$(git rev-parse --show-toplevel)" && git ls-files)
else
    # Pre-commit mode: check staged files only
    FILES=$(cd "$(git rev-parse --show-toplevel)" && git diff --cached --name-only --diff-filter=ACM)
fi

FOUND=0
for file in $FILES; do
    for pattern in "${FORBIDDEN_PATTERNS[@]}"; do
        if echo "$file" | grep -qE "$pattern"; then
            echo "❌ BLOCKED: binary artifact staged for commit: $file"
            echo "   Pattern: $pattern"
            echo "   Fix: git rm --cached $file && git commit -m 'chore: remove tracked binary'"
            FOUND=$((FOUND + 1))
        fi
    done
done

if [[ $FOUND -gt 0 ]]; then
    echo ""
    echo "Found $FOUND forbidden binary file(s). Commit blocked."
    exit 1
fi

echo "✅ No tracked binary artifacts found."
exit 0
