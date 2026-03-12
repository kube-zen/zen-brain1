#!/usr/bin/env python3
"""Fix repo_aware_templates.go by replacing Command strings with fixed versions."""

import re
import os

def escape_for_go_string(s):
    """Escape a string for use in a Go double-quoted string."""
    # Escape backslashes
    s = s.replace('\\', '\\\\')
    # Escape double quotes
    s = s.replace('"', '\\"')
    # Escape newlines (convert to \n)
    s = s.replace('\n', '\\n')
    # Escape tabs
    s = s.replace('\t', '\\t')
    # Escape carriage returns
    s = s.replace('\r', '\\r')
    return s

def load_template(prefix, step_num):
    """Load a template file."""
    path = f'/home/neves/zen/zen-brain1/internal/factory/templates/{prefix}/step_{step_num}.sh.tmpl'
    with open(path, 'r') as f:
        return f.read()

def fix_function(content, start_line, end_line, prefix, num_steps):
    """Fix a function by replacing its Command strings."""
    lines = content.split('\n')
    # Convert to 0-indexed
    start = start_line - 1
    end = end_line - 1
    
    # First, uncomment the function
    for i in range(start, end + 1):
        if i < len(lines) and lines[i].strip().startswith('//'):
            lines[i] = lines[i][2:]  # Remove leading '// '
    
    # Now replace Command strings
    # We'll find each Command: field and replace the string
    # This is a bit naive but should work for our structured file
    current_step = 0
    i = start
    while i <= end and current_step < num_steps:
        line = lines[i]
        if 'Command:' in line:
            current_step += 1
            # Load the fixed template
            template = load_template(prefix, current_step)
            # Escape for Go string
            escaped = escape_for_go_string(template)
            # Replace the Command line
            # The line looks like: Command:     "original..."
            # We need to replace everything after Command:
            lines[i] = re.sub(r'Command:\s+".*"', f'Command:     "{escaped}"', line)
            # If the regex didn't match (might have backticks or multi-line)
            # We'll handle that case
            if lines[i] == line:
                # Try different pattern
                lines[i] = re.sub(r'Command:\s+`.*`', f'Command:     "{escaped}"', line)
        i += 1
    
    # Join back
    return '\n'.join(lines)

def main():
    input_file = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go'
    output_file = '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates_fixed_final.go'
    
    with open(input_file, 'r') as f:
        content = f.read()
    
    # Fix the three functions
    # Function ranges (1-indexed)
    content = fix_function(content, 325, 398, 'docs', 8)
    content = fix_function(content, 483, 548, 'cicd', 7)
    content = fix_function(content, 643, 724, 'migration', 9)
    
    # Uncomment registrations
    content = content.replace('// r.registerRepoAwareDocsTemplate()', 'r.registerRepoAwareDocsTemplate()')
    content = content.replace('// r.registerRepoAwareCICDTemplate()', 'r.registerRepoAwareCICDTemplate()')
    content = content.replace('// r.registerRepoAwareMigrationTemplate()', 'r.registerRepoAwareMigrationTemplate()')
    
    with open(output_file, 'w') as f:
        f.write(content)
    
    print(f"Generated {output_file}")
    
    # Test compilation
    print("Testing compilation...")
    os.system(f'cd /home/neves/zen/zen-brain1 && go build ./internal/factory 2>&1 | head -20')

if __name__ == '__main__':
    main()