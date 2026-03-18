#!/usr/bin/env python3
"""Fix Command strings in extracted template functions."""

import re
import sys

def fix_command(cmd):
    """Fix a Command string."""
    # Replace << 'EOF' with <<EOF when heredoc contains shell expansions
    # Simple heuristic: if heredoc contains $( or $VAR, change delimiter
    lines = cmd.split('\n')
    in_heredoc = False
    heredoc_delimiter = None
    output = []
    for line in lines:
        # Detect heredoc start
        match = re.search(r'<<\s*[\'"]?([A-Za-z_][A-Za-z0-9_]*)[\'"]?', line)
        if match and not in_heredoc:
            delim = match.group(1)
            quoted = "'" in line or '"' in line
            # Check if line contains expansions after the heredoc start? Hard.
            # We'll just always use unquoted delimiter for now
            if quoted:
                # Remove quotes
                line = line.replace(f"<< '{delim}'", f"<<{delim}")
                line = line.replace(f'<< "{delim}"', f"<<{delim}")
            in_heredoc = True
            heredoc_delimiter = delim
        elif in_heredoc and line.strip() == heredoc_delimiter:
            in_heredoc = False
            heredoc_delimiter = None
        output.append(line)
    cmd = '\n'.join(output)
    
    # Replace triple backticks with ~~~bash
    cmd = re.sub(r'```bash', '~~~bash', cmd)
    cmd = re.sub(r'```', '~~~', cmd)
    
    # Fix GitLab CI detection bug
    cmd = cmd.replace('[ -d .gitlab-ci.yml ]', '[ -f .gitlab-ci.yml ]')
    
    return cmd

def process_function(func_text):
    """Process a function and fix all Command fields."""
    # Split into lines
    lines = func_text.split('\n')
    output = []
    i = 0
    while i < len(lines):
        line = lines[i]
        # Look for Command: field
        if 'Command:' in line and i+1 < len(lines) and lines[i+1].strip().startswith('"'):
            # Multi-line string starting with double quote
            output.append(line)
            # Collect the string lines
            cmd_lines = []
            j = i+1
            while j < len(lines) and (lines[j].strip().startswith('"') or lines[j].strip().startswith('\\')):
                cmd_lines.append(lines[j])
                j += 1
            cmd_text = '\n'.join(cmd_lines)
            # Fix the command
            fixed = fix_command(cmd_text)
            output.append(fixed)
            i = j
            continue
        output.append(line)
        i += 1
    return '\n'.join(output)

if __name__ == '__main__':
    for name in ['registerRepoAwareDocsTemplate', 'registerRepoAwareCICDTemplate', 'registerRepoAwareMigrationTemplate']:
        with open(f'/tmp/{name}.go', 'r') as f:
            func = f.read()
        fixed = process_function(func)
        with open(f'/tmp/{name}_fixed.go', 'w') as f:
            f.write(fixed)
        print(f"Fixed {name}")