#!/usr/bin/env python3
import re

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Find function ranges
functions = [
    'registerRepoAwareDocsTemplate',
    'registerRepoAwareCICDTemplate', 
    'registerRepoAwareMigrationTemplate'
]

i = 0
while i < len(lines):
    line = lines[i]
    for fn in functions:
        if f'func (r *WorkTypeTemplateRegistry) {fn}()' in line:
            # Found start
            # Comment out from this line until matching closing brace
            # Simple brace counting
            brace = 0
            j = i
            while j < len(lines):
                brace += lines[j].count('{')
                brace -= lines[j].count('}')
                if brace == 0 and j > i:
                    # Found closing brace
                    # Replace lines[i:j+1] with commented block
                    # But keep the function signature for reference?
                    # Actually replace with "// TEMP: commented out due to compilation issues"
                    # Let's just prefix each line with //
                    for k in range(i, j+1):
                        lines[k] = '// ' + lines[k]
                    i = j
                    break
                j += 1
            break
    i += 1

# Write back
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.writelines(lines)

print("Commented out problematic template functions")