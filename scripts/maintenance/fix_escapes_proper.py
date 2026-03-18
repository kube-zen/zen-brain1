#!/usr/bin/env python3
import re

def fix_go_string(match):
    """Fix escape sequences in a Go string literal."""
    s = match.group(0)
    # Replace \\" with \" (escaped quote)
    # But we must not replace \\" that's part of \\n, \\t, etc.
    # Actually \\" is backslash followed by quote - we want escaped quote
    # Use negative lookbehind to ensure not preceded by another backslash
    # Actually pattern is \\" which is two backslashes then quote
    # We want to replace with \" (one backslash, escaped quote)
    # In the string, \\" appears as \\" (backslash backslash quote)
    # We need to find this pattern and replace with \" (backslash quote)
    # But careful with raw strings...
    # Let's do simple replace: '\\"' -> '\"'
    # In Python literal: s.replace('\\"', '\"')
    # But we need to escape backslashes
    result = s.replace('\\\\"', '\\"')
    # Also fix \\` -> \` (though \` is valid escape)
    result = result.replace('\\\\`', '\\`')
    return result

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Apply to all double-quoted strings
# This regex matches double-quoted strings with possible escaped quotes
# Too complex. Let's just apply to entire content
content = content.replace('\\\\"', '\\"')
content = content.replace('\\\\`', '\\`')

# Write back
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(content)

print("Fixed escape sequences")