#!/usr/bin/env python3
"""Comment out problematic factory template functions"""

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    lines = f.readlines()

# Function names to comment out
functions_to_comment = [
    'registerRepoAwareDocsTemplate',
    'registerRepoAwareCICDTemplate',
    'registerRepoAwareMigrationTemplate'
]

result = []
i = 0
while i < len(lines):
    line = lines[i]

    # Check if line starts one of our target functions
    for func_name in functions_to_comment:
        if f'func (r *WorkTypeTemplateRegistry) {func_name}()' in line:
            # Found start of function to comment out
            # Comment out this line
            result.append('// ' + line)
            i += 1
            # Comment out until matching closing brace at column 0
            brace_level = 0
            while i < len(lines):
                curr = lines[i]
                # Count braces
                open_braces = curr.count('{')
                close_braces = curr.count('}')
                brace_level += open_braces - close_braces

                # Check if this is the closing brace at brace_level 0
                if brace_level == 0 and close_braces > 0 and curr.strip().startswith('}'):
                    result.append('// ' + curr)
                    i += 1
                    break

                result.append('// ' + curr)
                i += 1
            break
    else:
        result.append(line)
        i += 1

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.writelines(result)

print("Commented out problematic template functions")