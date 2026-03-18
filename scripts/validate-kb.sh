#!/bin/bash
# Validate real KB integration (optional)
# This script tests real KB/qmd when configured
# Keeps stub mode as fallback

set -e

echo "=== KB Integration Validation ==="
echo ""

# Check if qmd binary is available
echo "[1/3] Checking for qmd binary..."
if command -v qmd &> /dev/null; then
  echo "✓ PASS: qmd binary found ($(qmd --version 2>&1 | head -1))"
else
  echo "⚠️  WARN: qmd binary not found - stub mode required"
  echo "   To enable real KB: install qmd and add to PATH"
  echo "   Current: Stub mode will be used"
  exit 0
fi

# Check for docs repo configuration
echo "[2/3] Checking KB configuration..."
if [ -z "$ZEN_BRAIN_DOCS_REPO" ]; then
  echo "⚠️  WARN: ZEN_BRAIN_DOCS_REPO not set - stub mode will be used"
  echo "   To enable real KB: set ZEN_BRAIN_DOCS_REPO path"
  echo "   Current: Stub mode will be used"
  exit 0
fi

# Check if docs repo exists
echo "[3/3] Checking docs repo accessibility..."
if [ -d "$ZEN_BRAIN_DOCS_REPO" ]; then
  echo "✓ PASS: Docs repo exists at $ZEN_BRAIN_DOCS_REPO"
  DOCS_COUNT=$(find "$ZEN_BRAIN_DOCS_REPO" -name "*.md" 2>/dev/null | wc -l || echo "0")
  echo "   Found $DOCS_COUNT markdown files"
else
  echo "✗ FAIL: Docs repo not found at $ZEN_BRAIN_DOCS_REPO"
  echo "   To enable real KB: create or point to valid docs repo"
  exit 1
fi

echo ""
echo "=== KB Integration Ready ==="
echo "✓ Real KB can be enabled with:"
echo "   export ZEN_BRAIN_DOCS_REPO=$ZEN_BRAIN_DOCS_REPO"
echo "   export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=0"
echo ""
echo "✓ Stub mode available with:"
echo "   export ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1"
echo ""
echo "✓ Default: Stub mode (ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1)"
