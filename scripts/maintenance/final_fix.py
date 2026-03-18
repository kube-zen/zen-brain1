#!/usr/bin/env python3
"""Replace three factory template functions with embedded versions."""

import re
import sys

def extract_steps_from_function(func_text):
    """Extract steps from function text."""
    # Find Steps array
    start = func_text.find('Steps: []ExecutionStepTemplate{')
    if start == -1:
        return []
    
    # Parse from start to matching brace
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
    
    # Now extract individual step blocks within Steps block
    # Steps block starts with '{' and ends with '}'
    # We need to find each step block: { ... },
    steps = []
    pos = steps_block.find('{')
    while pos < len(steps_block):
        # Find matching brace for this step
        brace = 0
        j = pos
        while j < len(steps_block):
            if steps_block[j] == '{':
                brace += 1
            elif steps_block[j] == '}':
                brace -= 1
                if brace == 0:
                    step_block = steps_block[pos:j+1]
                    # Parse fields
                    step = {}
                    # Name
                    name_match = re.search(r'Name:\s+"(.*?)"', step_block)
                    step['name'] = name_match.group(1) if name_match else ''
                    # Description
                    desc_match = re.search(r'Description:\s+"(.*?)"', step_block)
                    step['description'] = desc_match.group(1) if desc_match else ''
                    # Variables (keep as is)
                    vars_match = re.search(r'Variables:\s+(map\[string\]string\{.*?\})', step_block, re.DOTALL)
                    step['variables'] = vars_match.group(1) if vars_match else 'map[string]string{}'
                    # Timeout
                    timeout_match = re.search(r'Timeout:\s+(\d+)', step_block)
                    step['timeout'] = timeout_match.group(1) if timeout_match else '30'
                    # MaxRetries
                    retries_match = re.search(r'MaxRetries:\s+(\d+)', step_block)
                    step['max_retries'] = retries_match.group(1) if retries_match else '1'
                    steps.append(step)
                    # Move past this step
                    pos = j + 1
                    # Skip comma and whitespace
                    while pos < len(steps_block) and steps_block[pos] in ' ,\n\t':
                        pos += 1
                    break
            j += 1
        else:
            break
        # Find next '{'
        next_pos = steps_block.find('{', pos)
        if next_pos == -1:
            break
        pos = next_pos
    
    return steps

def generate_function(func_name, steps, prefix, num_steps):
    """Generate Go function with loadTemplate calls."""
    # Template metadata (hardcoded based on prefix)
    if prefix == 'docs':
        work_type = 'docs'
        description = 'Repo-native documentation: detects docs structure, writes to actual paths, context-aware content'
    elif prefix == 'cicd':
        work_type = 'cicd'
        description = 'Repo-native CI/CD: detects CI platform, creates workflow files, generates deployment documentation'
    elif prefix == 'migration':
        work_type = 'migration'
        description = 'Repo-native database migrations: creates migration files, Go migrator, documentation'
    else:
        work_type = prefix
        description = ''
    
    lines = []
    lines.append(f'// {func_name} creates a truly repo-aware {prefix} template.')
    lines.append(f'func (r *WorkTypeTemplateRegistry) {func_name}() {{')
    lines.append('\ttemplate := &WorkTypeTemplate{')
    lines.append(f'\t\tWorkType:   "{work_type}",')
    lines.append('\t\tWorkDomain: "real",')
    lines.append(f'\t\tDescription: "{description}",')
    lines.append('\t\tSteps: []ExecutionStepTemplate{')
    
    for i, step in enumerate(steps[:num_steps]):
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
    
    return '\n'.join(lines)

def main():
    # Read backup file
    backup_path = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go.backup'
    with open(backup_path, 'r') as f:
        lines = f.readlines()
    
    # Add imports after package line
    new_lines = []
    i = 0
    while i < len(lines):
        line = lines[i]
        new_lines.append(line)
        if line.strip() == 'package factory':
            # Insert imports
            new_lines.append('')
            new_lines.append('import (')
            new_lines.append('\t"embed"')
            new_lines.append('\t"strings"')
            new_lines.append(')')
            new_lines.append('')
            # Also need to add embed variables and loadTemplate function
            # We'll add them after imports but before first function
            # We'll insert after this point, but we need to find where to insert
            # Let's insert right after imports, before the next line
            # Actually we'll add after the imports block, before the next line
            # We'll do it after we finish reading the whole file.
        i += 1
    
    # Now we have new_lines with imports added.
    # We'll need to insert embed variables before the first function.
    # Let's join and find the position of first function.
    content = '\n'.join(new_lines)
    # Find first occurrence of 'func (r *WorkTypeTemplateRegistry)'
    func_pos = content.find('func (r *WorkTypeTemplateRegistry)')
    if func_pos != -1:
        # Insert embed variables before that
        embed_code = '''//go:embed templates/docs/*.sh.tmpl
var docsTemplates embed.FS

//go:embed templates/cicd/*.sh.tmpl
var cicdTemplates embed.FS

//go:embed templates/migration/*.sh.tmpl
var migrationTemplates embed.FS

func loadTemplate(fs embed.FS, path string) string {
\tdata, err := fs.ReadFile(path)
\tif err != nil {
\t\treturn "echo \\"ERROR: Failed to load template: " + path + "\\""
\t}
\treturn strings.TrimSpace(string(data))
}'''
        content = content[:func_pos] + embed_code + '\n\n' + content[func_pos:]
    
    # Now we need to replace the three function blocks
    # We'll parse steps from original function files
    funcs = [
        ('registerRepoAwareDocsTemplate', 'docs', 8),
        ('registerRepoAwareCICDTemplate', 'cicd', 7),
        ('registerRepoAwareMigrationTemplate', 'migration', 9)
    ]
    
    for func_name, prefix, num_steps in funcs:
        # Read original function file
        func_file = f'/tmp/{func_name}.go'
        with open(func_file, 'r') as f:
            func_text = f.read()
        steps = extract_steps_from_function(func_text)
        print(f"{func_name}: extracted {len(steps)} steps")
        if len(steps) == 0:
            print("Warning: No steps extracted!")
            # Fallback: create empty steps with correct count
            steps = [{'name': f'Step {i+1}', 'description': '', 'variables': 'map[string]string{}', 'timeout': '30', 'max_retries': '1'} for i in range(num_steps)]
        
        new_func = generate_function(func_name, steps, prefix, num_steps)
        
        # Replace in content
        # Find old function
        pattern = f'func \\(r \\*WorkTypeTemplateRegistry\\) {func_name}\\(\\) {{[^}}]+}}'
        # Use regex with DOTALL
        import re
        old_func_match = re.search(f'func \\(r \\*WorkTypeTemplateRegistry\\) {func_name}\\(\\) {{.*?^}}', content, re.MULTILINE | re.DOTALL)
        if old_func_match:
            content = content[:old_func_match.start()] + new_func + content[old_func_match.end():]
        else:
            print(f"Warning: Could not find {func_name} in content")
            # Append at end
            content += '\n\n' + new_func
    
    # Uncomment registrations
    content = content.replace('// r.registerRepoAwareDocsTemplate()', 'r.registerRepoAwareDocsTemplate()')
    content = content.replace('// r.registerRepoAwareCICDTemplate()', 'r.registerRepoAwareCICDTemplate()')
    content = content.replace('// r.registerRepoAwareMigrationTemplate()', 'r.registerRepoAwareMigrationTemplate()')
    
    # Write final file
    output_path = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go'
    with open(output_path, 'w') as f:
        f.write(content)
    
    print(f"Written to {output_path}")
    
    # Test compilation
    print("Testing compilation...")
    import subprocess
    result = subprocess.run(
        ['go', 'build', './internal/factory'],
        cwd='/home/neves/zen/zen-brain1',
        capture_output=True,
        text=True
    )
    if result.returncode == 0:
        print("✅ Factory package compiles!")
        # Test dependent binaries
        for binary in ['./cmd/zen-brain', './cmd/controller', './cmd/apiserver']:
            result2 = subprocess.run(
                ['go', 'build', binary],
                cwd='/home/neves/zen/zen-brain1',
                capture_output=True,
                text=True
            )
            if result2.returncode == 0:
                print(f"✅ {binary} compiles!")
            else:
                print(f"❌ {binary} failed: {result2.stderr[:200]}")
    else:
        print("❌ Compilation failed:")
        print(result.stderr[:500])

if __name__ == '__main__':
    main()