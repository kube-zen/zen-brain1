#!/usr/bin/env python3
import re

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Fix 1: Replace \\" with \\\" (properly escaped quote)
# But careful: we don't want to replace in comments or outside strings
# Simple approach: replace in the entire file, should be safe
# Actually \\" appears only in Command strings
content = content.replace('\\"', '\\\"')

# Fix 2: Replace \\` with \\\` (escape backticks too)
content = content.replace('\\`', '\\\`')

# Fix 3: The $ issue might be due to string termination, fixed by above
# Also fix any \\' with \\\' (though \' is valid escape)
content = content.replace("\\'", "\\\'")

# Write back
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(content)

print("Fixed escape sequences")