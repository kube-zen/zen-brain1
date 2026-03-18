#!/usr/bin/env python3
"""Fix step extraction in factory template functions."""

import re
import sys

def extract_steps_braces(func_text):
    """Extract steps using brace counting."""
    # Find Steps block
    start = func_text.find('Steps: []ExecutionStepTemplate{')
    if start == -1:
        return []
    
    # Find matching closing brace for Steps block
    brace = 0
    i = start
    while i < len(func_text):
        if func_text[i] == '{':
            brace += 1
        elif func_text[i] == '}':
            brace -= 1
            if brace == 0:
                steps_block = func_text[start:i+1]
                break
        i += 1
    else:
        return []
    
    # Extract step substrings: each step starts with '{' and ends with '},' (except last)
    # We'll split by '},' but careful about nested braces
    steps = []
    pos = 0
    while pos < len(steps_block):
        # Find next '{'
        next_brace = steps_block.find('{', pos)
        if next_brace == -1:
            break
        
        # Find matching '}'
        brace = 0
        j = next_brace
        while j < len(steps_block):
            if steps_block[j] == '{':
                brace += 1
            elif steps_block[j] == '}':
                brace -= 1
                if brace == 0:
                    # Check if followed by comma (or end of block)
                    step_end = j + 1
                    if step_end < len(steps_block) and steps_block[step_end] == ',':
                        step_end += 1
                    step_text = steps_block[next_brace:step_end]
                    steps.append(step_text)
                    pos = step_end
                    break
            j += 1
        else:
            break
    
    # Parse each step
    parsed = []
    for step_text in steps:
        step = {}
        # Name
        name_match = re.search(r'Name:\s+"(.*?)"', step_text, re.DOTALL)
        step['name'] = name_match.group(1).replace('\n', ' ').strip() if name_match else ''
        # Description
        desc_match = re.search(r'Description:\s+"(.*?)"', step_text, re.DOTALL)
        step['description'] = desc_match.group(1).replace('\n', ' ').strip() if desc_match else ''
        # Variables - capture the whole line
        vars_match = re.search(r'Variables:\s+(map\[string\]string\{.*?\})', step_text, re.DOTALL)
        step['variables'] = vars_match.group(1) if vars_match else 'map[string]string{}'
        # Timeout
        timeout_match = re.search(r'Timeout:\s+(\d+)', step_text)
        step['timeout'] = timeout_match.group(1) if timeout_match else '30'
        # MaxRetries
        retries_match = re.search(r'MaxRetries:\s+(\d+)', step_text)
        step['max_retries'] = retries_match.group(1) if retries_match else '1'
        parsed.append(step)
    
    return parsed

def main():
    # Read current file
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
        content = f.read()
    
    # Process each function
    funcs = [
        ('registerRepoAwareDocsTemplate', 'docs', 8),
        ('registerRepoAwareCICDTemplate', 'cicd', 7),
        ('registerRepoAwareMigrationTemplate', 'migration', 9)
    ]
    
    for func_name, prefix, expected_steps in funcs:
        # Read original function file
        func_file = f'/tmp/{func_name}.go'
        with open(func_file, 'r') as f:
            func_text = f.read()
        
        steps = extract_steps_braces(func_text)
        print(f"{func_name}: extracted {len(steps)} steps (expected {expected_steps})")
        
        if len(steps) != expected_steps:
            print(f"  Warning: step count mismatch")
            # Use dummy steps for missing ones
            while len(steps) < expected_steps:
                steps.append({
                    'name': f'Step {len(steps)+1}',
                    'description': '',
                    'variables': 'map[string]string{}',
                    'timeout': '30',
                    'max_retries': '1'
                })
        
        # Generate new function
        # Template metadata
        if prefix == 'docs':
            work_type = 'docs'
            desc = 'Repo-native documentation: detects docs structure, writes to actual paths, context-aware content'
        elif prefix == 'cicd':
            work_type = 'cicd'
            desc = 'Repo-native CI/CD: detects CI platform, creates workflow files, generates deployment documentation'
        else:
            work_type = 'migration'
            desc = 'Repo-native database migrations: creates migration files, Go migrator, documentation'
        
        lines = []
        lines.append(f'// {func_name} creates a truly repo-aware {prefix} template.')
        lines.append(f'func (r *WorkTypeTemplateRegistry) {func_name}() {{')
        lines.append('\ttemplate := &WorkTypeTemplate{')
        lines.append(f'\t\tWorkType:   "{work_type}",')
        lines.append('\t\tWorkDomain: "real",')
        lines.append(f'\t\tDescription: "{desc}",')
        lines.append('\t\tSteps: []ExecutionStepTemplate{')
        
        for i, step in enumerate(steps[:expected_steps]):
            lines.append('\t\t\t{')
            lines.append(f'\t\t\t\tName:        "{step["name"]}",')
            lines.append(f'\t\t\t\tDescription: "{step["description"]}",')
            lines.append(f'\t\t\t\tCommand:     loadTemplate({prefix}Templates, "templates/{prefix}/step_{i+1}.sh.tmpl"),')
            lines.append(f'\t\t\t\tVariables:   {step["variables"]},')
            lines.append(f'\t\t\t\tTimeout:     {step["timeout"]},')
            lines.append(f'\t\t\t\tMaxRetries:  {step["max_retries"]},')
            lines.append('\t\t\t},')
        
        lines.append('\t\t},')
        lines.append('\t}')
        lines.append('\tr.registerTemplate(template)')
        lines.append('}')
        
        new_func = '\n'.join(lines)
        
        # Replace in content
        # Find old function using regex
        pattern = f'func \\(r \\*WorkTypeTemplateRegistry\\) {func_name}\\(\\) {{.*?^}}'
        import re
        old_func_match = re.search(pattern, content, re.MULTILINE | re.DOTALL)
        if old_func_match:
            content = content[:old_func_match.start()] + new_func + content[old_func_match.end():]
            print(f"  Replaced function")
        else:
            print(f"  Could not find function, appending")
            content += '\n\n' + new_func
    
    # Write back
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
        f.write(content)
    
    print("File updated")
    
    # Quick compilation test
    import subprocess
    result = subprocess.run(
        ['go', 'build', './internal/factory'],
        cwd='/home/neves/zen/zen-brain1',
        capture_output=True,
        text=True
    )
    if result.returncode == 0:
        print("✅ Factory still compiles")
    else:
        print("❌ Compilation failed:")
        print(result.stderr[:500])

if __name__ == '__main__':
    main()