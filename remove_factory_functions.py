#!/usr/bin/env python3
"""Remove problematic factory template functions entirely"""

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Find and remove the three problematic functions
import re

# Pattern for each function: from func definition through closing brace at same indentation
# This is complex, so let's use a simpler marker approach

# Add a marker at the start and end of each function to remove
patterns = [
    (r'// registerRepoAwareDocsTemplate creates a truly repo-aware documentation template\.',
     r'// registerRepoAwareCICDTemplate creates a truly repo-aware CI/CD template\.',
     r'// registerRepoAwareMigrationTemplate creates a truly repo-aware migration template\.')
]

for start_pattern in patterns:
    if start_pattern in content:
        # Find the function start and end
        # Find the index of the comment
        start_idx = content.find(start_pattern)
        if start_idx != -1:
            # Find the closing brace at the same indentation level as func
            # Look backward for "func (r"
            func_start = content.rfind('func (r *WorkTypeTemplateRegistry)', 0, start_idx)
            if func_start != -1:
                # Find the closing brace that matches the function
                # Count braces from func_start
                pos = func_start
                brace_level = 0
                while pos < len(content):
                    if content[pos] == '{':
                        brace_level += 1
                    elif content[pos] == '}':
                        brace_level -= 1
                        if brace_level == 0:
                            # Found the end
                            content = content[:func_start] + content[pos+1:]
                            break
                    pos += 1
                break

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(content)

print("Removed problematic template functions")