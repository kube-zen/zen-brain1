#!/usr/bin/env python3
"""Replace Command strings with placeholders to get compilation working."""

import re
import sys

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Function ranges (1-indexed)
func_ranges = [
    (325, 398, 'docs'),
    (483, 548, 'cicd'),
    (643, 724, 'migration')
]

for start, end, name in func_ranges:
    print(f"Processing {name} function (lines {start}-{end})...")
    # Convert to 0-indexed
    start_idx = start - 1
    end_idx = end - 1
    
    i = start_idx
    while i <= end_idx:
        line = lines[i]
        # Look for Command: field
        if 'Command:' in line:
            # Check if it's a multi-line string
            if i+1 <= end_idx and '"' in lines[i+1]:
                # Multi-line string - find the end
                j = i+1
                while j <= end_idx and j < len(lines):
                    if '"' in lines[j] and not lines[j].count('\\"') % 2 == 1:
                        # Found closing quote (not counting escaped quotes)
                        # But this is complex. Let's just replace the whole block
                        # with a simple placeholder
                        break
                    j += 1
                
                # Replace lines i+1 through j with simple placeholder
                placeholder = '\t\t\tCommand:     "echo \\"Template placeholder - original command being fixed\\""'
                lines[i] = placeholder + '\n'
                # Remove the old string lines
                for k in range(i+1, j+1):
                    if k < len(lines):
                        lines[k] = ''
                i = j
            else:
                # Single-line string
                # Replace with placeholder
                lines[i] = '\t\t\tCommand:     "echo \\"Template placeholder - original command being fixed\\""\n'
        i += 1

# Write output
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates_fixed2.go', 'w') as f:
    f.writelines(lines)

print("Created fixed file")