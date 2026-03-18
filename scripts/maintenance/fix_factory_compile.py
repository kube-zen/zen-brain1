#!/usr/bin/env python3
"""Fix factory template functions by uncommenting and replacing Command strings."""

import re
import sys

def uncomment_range(lines, start, end):
    """Uncomment lines in range (0-indexed inclusive)."""
    for i in range(start, end + 1):
        if i < len(lines) and lines[i].strip().startswith('//'):
            lines[i] = lines[i][2:]  # Remove '// '
    return lines

def replace_commands_in_function(lines, start, end, template_prefix, num_steps):
    """Replace Command strings with loadTemplate calls."""
    # Find Command fields in this range
    step_counter = 0
    i = start
    while i <= end and step_counter < num_steps:
        line = lines[i]
        # Look for Command: field
        if 'Command:' in line:
            step_counter += 1
            # Replace the entire Command line
            # Pattern: Command:\s+"..." or Command:\s+`...` or Command:\s+loadTemplate(...)
            # We'll replace with loadTemplate call
            new_line = re.sub(
                r'Command:\s+(?:"[^"]*"|`[^`]*`|loadTemplate\([^)]+\))',
                f'Command:     loadTemplate({template_prefix}Templates, "templates/{template_prefix}/step_{step_counter}.sh.tmpl")',
                line
            )
            lines[i] = new_line
        i += 1
    return lines

def main():
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
        lines = f.readlines()
    
    # Uncomment three function ranges (1-indexed from earlier analysis)
    # Convert to 0-indexed
    ranges = [
        (325-1, 398-1, 'docs', 8),
        (483-1, 548-1, 'cicd', 7),
        (643-1, 724-1, 'migration', 9)
    ]
    
    for start, end, prefix, num_steps in ranges:
        print(f"Processing {prefix} function (lines {start+1}-{end+1})...")
        # First uncomment
        lines = uncomment_range(lines, start, end)
        # Then replace Command strings
        lines = replace_commands_in_function(lines, start, end, prefix, num_steps)
    
    # Write back
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
        f.writelines(lines)
    
    print("Fixed file written")
    
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
    else:
        print("❌ Compilation failed:")
        print(result.stderr[:500])

if __name__ == '__main__':
    main()