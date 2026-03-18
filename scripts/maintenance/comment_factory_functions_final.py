#!/usr/bin/env python3
"""Comment out three problematic template function bodies"""

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Find the three problematic functions and comment out their bodies
functions_to_disable = [
    'registerRepoAwareDocsTemplate',
    'registerRepoAwareCICDTemplate',
    'registerRepoAwareMigrationTemplate'
]

result = []
i = 0
in_function = None
while i < len(lines):
    line = lines[i]
    
    # Check if we're entering one of our target functions
    for func_name in functions_to_disable:
        if f'func (r *WorkTypeTemplateRegistry) {func_name}()' in line:
            in_function = func_name
            result.append(line)
            i += 1
            break
    
    if in_function:
        # We're inside a function to disable
        # Comment out the line if it's not already commented
        stripped = line.strip()
        if not stripped.startswith('//'):
            result.append('// ' + line)
        else:
            result.append(line)
        
        # Check if function ends (closing brace at column 0)
        if '}' in line and line.strip() == '}':
            in_function = None
    else:
        result.append(line)
    i += 1

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.writelines(result)

print("Commented out problematic function bodies")