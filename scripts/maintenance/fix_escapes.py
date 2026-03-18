#!/usr/bin/env python3
import re

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Fix 1: Escape backslashes in the Installation section
# Pattern: echo '\`bash\ngo get /./...\n\`\`\`'
# Need to double backslashes: echo '\\`bash\\ngo get /./...\\n\\`\\`\\`'
# Actually, we need to escape backslashes in Go string: \\ becomes \\\\

# Find the problematic pattern
pattern1 = r"echo '\\`bash\\ngo get /\./\.\.\.\\n\\`\\`\\`'"
pattern2 = r"echo '\\`bash\\nnpm install\\n\\`\\`\\`'"

# Replace with properly escaped version
# For now, simplify: remove the code blocks entirely
# Replace with: echo 'Installation instructions for Go: go get ./...'
# Let's just comment out the entire template for now? No.

# Instead, let's fix by escaping backslashes
# We'll replace the entire Command string? Too complex.

# Let's take a different approach: fix the two specific echo statements
# We'll use regex to find each echo with backticks and escape backslashes

def escape_backslashes(match):
    text = match.group(0)
    # Double all backslashes
    return text.replace('\\', '\\\\')

# Find lines containing echo '\`... and replace
lines = content.split('\n')
new_lines = []
for line in lines:
    if 'echo \'\\`bash' in line:
        # This line contains the problematic echo
        # Replace single backslashes with double backslashes
        line = line.replace('\\`', '\\\\`')
        line = line.replace('\\n', '\\\\n')
        line = line.replace('\\`\\`\\`', '\\\\`\\\\`\\\\`')
    new_lines.append(line)

new_content = '\n'.join(new_lines)

# Write back
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(new_content)

print("Fixed backslash escapes in factory template")