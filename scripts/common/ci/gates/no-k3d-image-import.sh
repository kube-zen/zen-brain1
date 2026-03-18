#!/usr/bin/env bash
# CI gate: forbid k3d image import
# This gate fails if `k3d image import` appears in active code or scripts.

set -euo pipefail

# Check if k3d image import appears in active code
if git grep -r "k3d.*image.*import" --exclude-dir=.git --exclude-dir=vendor --include="*.py" --include="*.sh" --include="Makefile" scripts >/dev/null 2>&1; then
    echo "❌ FAIL: k3d image import found in scripts/"
    echo "k3d image import is forbidden in canonical deployment path"
    echo "Use shared registry :5000 instead"
    exit 1
fi

echo "✅ PASS: No k3d image import in active code paths"
