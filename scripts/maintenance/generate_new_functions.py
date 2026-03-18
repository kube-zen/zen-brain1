#!/usr/bin/env python3
"""Generate new factory template functions with embedded templates."""

import re
import os
import sys

def extract_template_metadata(func_text):
    """Extract template metadata from function."""
    meta = {}
    # WorkType
    match = re.search(r'WorkType:\s+"(.*?)"', func_text)
    if match:
        meta['work_type'] = match.group(1)
    # WorkDomain
    match = re.search(r'WorkDomain:\s+"(.*?)"', func_text)
    if match:
        meta['work_domain'] = match.group(1)
    # Description
    match = re.search(r'Description:\s+"(.*?)"', func_text)
    if match:
        meta['description'] = match.group(1)
    return meta

def extract_steps(func_text):
    """Extract steps from function text."""
    # Find Steps array
    steps_match = re.search(r'Steps:\s*\[\]ExecutionStepTemplate\s*{([^}]+(?:\{[^}]*\}[^}]*)*)}', func_text, re.DOTALL)
    if not steps_match:
        return []
    
    steps_text = steps_match.group(1)
    
    # Split into individual step blocks
    steps = []
    brace_level = 0
    current_step = []
    in_step = False
    
    for line in steps_text.split('\n'):
        stripped = line.strip()
        if not in_step and stripped.startswith('{'):
            in_step = True
            brace_level = 0
        
        if in_step:
            current_step.append(line)
            brace_level += line.count('{')
            brace_level -= line.count('}')
            if brace_level <= 0 and '}' in line:
                steps.append('\n'.join(current_step))
                current_step = []
                in_step = False
    
    # Parse each step
    parsed_steps = []
    for step_text in steps:
        step = {}
        # Name
        match = re.search(r'Name:\s+"(.*?)"', step_text)
        step['name'] = match.group(1) if match else ''
        # Description
        match = re.search(r'Description:\s+"(.*?)"', step_text)
        step['description'] = match.group(1) if match else ''
        # Variables (keep as is)
        match = re.search(r'Variables:\s+(map\[string\]string\{.*?\})', step_text, re.DOTALL)
        step['variables'] = match.group(1) if match else 'map[string]string{}'
        # Timeout
        match = re.search(r'Timeout:\s+(\d+)', step_text)
        step['timeout'] = match.group(1) if match else '30'
        # MaxRetries
        match = re.search(r'MaxRetries:\s+(\d+)', step_text)
        step['max_retries'] = match.group(1) if match else '1'
        parsed_steps.append(step)
    
    return parsed_steps

def generate_function(func_name, meta, steps, prefix, num_steps):
    """Generate Go function code."""
    lines = []
    lines.append(f'// {func_name} creates a truly repo-aware {prefix} template.')
    lines.append(f'func (r *WorkTypeTemplateRegistry) {func_name}() {{')
    lines.append('\ttemplate := &WorkTypeTemplate{')
    lines.append(f'\t\tWorkType:   "{meta.get("work_type", prefix)}",')
    lines.append(f'\t\tWorkDomain: "{meta.get("work_domain", "real")}",')
    lines.append(f'\t\tDescription: "{meta.get("description", "")}",')
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
        content = f.read()
    
    # Add imports if not present
    if 'import (' not in content:
        # Find package line
        lines = content.split('\n')
        for i, line in enumerate(lines):
            if line.strip() == 'package factory':
                # Insert imports after package line
                imports = '''import (
\t"embed"
\t"strings"
)'''
                lines.insert(i + 1, '')
                lines.insert(i + 2, imports)
                lines.insert(i + 3, '')
                break
        content = '\n'.join(lines)
    
    # Add embed variables and helper function if not present
    if '//go:embed templates/docs' not in content:
        # Find where to insert - after imports, before first function
        lines = content.split('\n')
        for i, line in enumerate(lines):
            if line.strip().startswith('func ') and 'registerRepoAwareTemplates' in line:
                # Insert before this function
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
                lines.insert(i, '')
                lines.insert(i, embed_code)
                lines.insert(i, '')
                break
        content = '\n'.join(lines)
    
    # Parse original functions
    funcs = [
        ('registerRepoAwareDocsTemplate', 'docs', 8),
        ('registerRepoAwareCICDTemplate', 'cicd', 7),
        ('registerRepoAwareMigrationTemplate', 'migration', 9)
    ]
    
    new_functions = {}
    for func_name, prefix, num_steps in funcs:
        func_file = f'/tmp/{func_name}.go'
        with open(func_file, 'r') as f:
            func_text = f.read()
        meta = extract_template_metadata(func_text)
        steps = extract_steps(func_text)
        new_func = generate_function(func_name, meta, steps, prefix, num_steps)
        new_functions[func_name] = new_func
        print(f"Generated {func_name} with {len(steps)} steps")
    
    # Replace old function blocks with new ones
    # We'll build new content by replacing from the old file
    # First, find the line ranges of the three old functions (commented or not)
    # Since they might be commented, we'll just replace the entire file from
    # the first function to the end with our new functions plus other existing functions.
    # Simpler: we'll create a new file with:
    # 1. Everything up to registerRepoAwareTemplates function
    # 2. Our new functions
    # 3. All other existing functions (implementation, bugfix, refactor, test, monitoring)
    
    # Let's extract existing other functions
    # This is getting complex. Instead, let's just replace the three function blocks
    # by searching for their signatures.
    
    lines = content.split('\n')
    output = []
    i = 0
    while i < len(lines):
        line = lines[i]
        # Check if this line starts one of our target functions
        replaced = False
        for func_name in new_functions:
            if f'func (r *WorkTypeTemplateRegistry) {func_name}()' in line:
                # Skip the old function block
                # Find the end of the function (matching braces)
                brace_level = 0
                j = i
                while j < len(lines):
                    brace_level += lines[j].count('{')
                    brace_level -= lines[j].count('}')
                    if brace_level == 0:
                        # Replace with new function
                        output.append(new_functions[func_name])
                        i = j
                        replaced = True
                        break
                    j += 1
                break
        if not replaced:
            output.append(line)
        i += 1
    
    content = '\n'.join(output)
    
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
        # Also test dependent binaries
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