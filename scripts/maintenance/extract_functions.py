#!/usr/bin/env python3
"""Extract the three problematic template functions from repo_aware_templates.go"""

import sys

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Find function start lines
func_starts = {}
func_names = [
    'registerRepoAwareDocsTemplate',
    'registerRepoAwareCICDTemplate', 
    'registerRepoAwareMigrationTemplate'
]

for i, line in enumerate(lines):
    for name in func_names:
        if f'func (r *WorkTypeTemplateRegistry) {name}()' in line:
            func_starts[name] = i
            print(f"Found {name} at line {i+1}: {line.strip()}")

# Find function end lines (matching braces)
for name, start in func_starts.items():
    brace_level = 0
    for i in range(start, len(lines)):
        line = lines[i]
        brace_level += line.count('{')
        brace_level -= line.count('}')
        if brace_level == 0:
            print(f"  Ends at line {i+1}: {line.strip()}")
            break

print("\nNow extracting each function...")

# Extract each function
for name, start in func_starts.items():
    brace_level = 0
    for i in range(start, len(lines)):
        line = lines[i]
        brace_level += line.count('{')
        brace_level -= line.count('}')
        if brace_level == 0:
            end = i
            break
    func_lines = lines[start:end+1]
    with open(f'/tmp/{name}.go', 'w') as f:
        f.writelines(func_lines)
    print(f"  Saved {name} ({len(func_lines)} lines) to /tmp/{name}.go")