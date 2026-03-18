#!/usr/bin/env python3
"""Comment out three problematic functions."""

import sys

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Function ranges (start line, end line) - 1-indexed
ranges = [(325, 398), (483, 548), (643, 724)]

# Convert to 0-indexed
for start, end in ranges:
    start -= 1
    end -= 1
    for i in range(start, end+1):
        if not lines[i].strip().startswith('//'):
            lines[i] = '// ' + lines[i]

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.writelines(lines)

print("Commented out three functions")