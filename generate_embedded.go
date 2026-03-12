#!/usr/bin/env python3
"""Generate new repo_aware_templates.go with embedded templates."""

import os
import re
import sys

def read_file(path):
    with open(path, 'r') as f:
        return f.read()

def write_file(path, content):
    with open(path, 'w') as f:
        f.write(content)

def add_imports(content):
    """Add embed import to file content."""
    # Find package declaration
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
    return '\n'.join(lines)

def add_embed_variables(content):
    """Add embed.FS variables after imports."""
    # Find where to insert - after imports or before first function
    lines = content.split('\n')
    
    # Look for first function after imports
    for i, line in enumerate(lines):
        if line.strip().startswith('func ') and 'registerRepoAwareTemplates' in line:
            # Insert before this function
            embed_vars = '''//go:embed templates/docs/*.sh.tmpl
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
            lines.insert(i, embed_vars)
            lines.insert(i, '')
            break
    
    return '\n'.join(lines)

def generate_docs_function():
    """Generate new registerRepoAwareDocsTemplate function."""
    # Load template contents
    template_dir = '/home/neves/zen/zen-brain1/internal/factory/templates/docs'
    steps = []
    for i in range(1, 9):  # 8 steps
        path = f'{template_dir}/step_{i}.sh.tmpl'
        with open(path, 'r') as f:
            content = f.read()
        # Escape backticks and quotes for Go string
        # Use raw string with backticks if no backticks in content
        if '`' in content:
            # Use double quotes with escaping
            content = content.replace('\\', '\\\\')
            content = content.replace('"', '\\"')
            content = content.replace('\n', '\\n')
            cmd = f'"{content}"'
        else:
            # Use raw string
            cmd = f'`{content}`'
        
        # Create step - we need the original step structure
        # For now, use placeholder - we'll extract from original function
        steps.append(f'\t\t\tCommand:     {cmd}')
    
    # This is simplified - need original step structures
    # Actually, better to extract from original function file
    return '''
// registerRepoAwareDocsTemplate creates a truly repo-aware documentation template.
func (r *WorkTypeTemplateRegistry) registerRepoAwareDocsTemplate() {
\t// TODO: Implement with embedded templates
\ttemplate := &WorkTypeTemplate{
\t\tWorkType:   "docs",
\t\tWorkDomain: "real",
\t\tDescription: "Repo-native documentation: detects docs structure, writes to actual paths, context-aware content",
\t\tSteps: []ExecutionStepTemplate{
\t\t\t{
\t\t\t\tName:        "Validate git repository",
\t\t\t\tDescription: "Require git repository for documentation tracking",
\t\t\t\tCommand:     loadTemplate(docsTemplates, "templates/docs/step_1.sh.tmpl"),
\t\t\t\tVariables:   map[string]string{},
\t\t\t\tTimeout:     15,
\t\t\t\tMaxRetries:  1,
\t\t\t},
\t\t\t// ... more steps
\t\t},
\t}
\tr.registerTemplate(template)
}'''

def main():
    input_file = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go'
    output_file = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates_embedded.go'
    
    content = read_file(input_file)
    
    # 1. Add imports
    content = add_imports(content)
    
    # 2. Add embed variables and helper function
    content = add_embed_variables(content)
    
    # 3. Uncomment registrations
    content = content.replace('// r.registerRepoAwareDocsTemplate()', 'r.registerRepoAwareDocsTemplate()')
    content = content.replace('// r.registerRepoAwareCICDTemplate()', 'r.registerRepoAwareCICDTemplate()')
    content = content.replace('// r.registerRepoAwareMigrationTemplate()', 'r.registerRepoAwareMigrationTemplate()')
    
    write_file(output_file, content)
    print(f"Generated {output_file}")
    
    # Now we need to replace the three function bodies
    # This is more complex - we need to parse and replace each function
    # For now, let's just compile to test imports

if __name__ == '__main__':
    main()