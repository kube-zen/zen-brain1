#!/usr/bin/env python3
"""Extract Command strings from Go template functions and save as template files."""

import re
import os
import sys

def unescape_go_string(s):
    """Unescape a Go string literal."""
    # Replace escaped newlines with actual newlines
    s = s.replace('\\n', '\n')
    # Replace escaped quotes
    s = s.replace('\\"', '"')
    s = s.replace("\\'", "'")
    # Replace escaped backslashes
    s = s.replace('\\\\', '\\')
    # Replace other common escapes
    s = s.replace('\\t', '\t')
    s = s.replace('\\r', '\r')
    return s

def extract_commands_from_go_function(func_text):
    """Extract Command strings from a Go function."""
    # Find all Command: fields
    # Pattern: Command:\s+"(.*?)"(?=\s*,)
    # But need to handle multi-line strings with escaped newlines
    # The strings are single-line with \n escapes
    commands = []
    
    # Split by lines and look for Command:
    lines = func_text.split('\n')
    i = 0
    while i < len(lines):
        line = lines[i]
        if 'Command:' in line:
            # Find the string after Command:
            match = re.search(r'Command:\s+"(.*)"', line)
            if match:
                # Single-line string
                cmd = match.group(1)
                commands.append(unescape_go_string(cmd))
            else:
                # Might be multi-line or have continuation
                # Look for opening quote
                if '"' in line:
                    # Find the start
                    quote_start = line.find('"')
                    # Collect until closing quote
                    cmd_parts = [line[quote_start+1:]]
                    j = i
                    while j < len(lines) and cmd_parts[-1].count('"') - cmd_parts[-1].count('\\"') < 1:
                        j += 1
                        if j < len(lines):
                            cmd_parts.append(lines[j])
                    # Join and extract
                    full_line = '\n'.join(cmd_parts)
                    # Find first and last quote
                    first_quote = full_line.find('"')
                    last_quote = full_line.rfind('"')
                    if first_quote != -1 and last_quote != -1:
                        cmd = full_line[first_quote+1:last_quote]
                        commands.append(unescape_go_string(cmd))
                    i = j
        i += 1
    return commands

def fix_shell_template(cmd):
    """Fix shell template issues."""
    # 1. Replace triple backticks with ~~~bash
    cmd = cmd.replace('```bash', '~~~bash')
    cmd = cmd.replace('```', '~~~')
    
    # 2. Fix GitLab CI detection bug
    cmd = cmd.replace('[ -d .gitlab-ci.yml ]', '[ -f .gitlab-ci.yml ]')
    
    # 3. Fix heredoc delimiters when heredoc contains shell expansions
    lines = cmd.split('\n')
    output = []
    for i, line in enumerate(lines):
        # Check for heredoc start with quotes
        match = re.search(r'<<\s*[\'"]([A-Za-z_][A-Za-z0-9_]*)[\'"]', line)
        if match:
            delim = match.group(1)
            # Check if this heredoc contains expansions
            # Look ahead in cmd for heredoc content
            heredoc_start = cmd.find(line)
            content_start = heredoc_start + len(line)
            # Find the end delimiter
            end_pos = cmd.find(delim, content_start)
            if end_pos != -1:
                heredoc_content = cmd[content_start:end_pos]
                # Check for shell expansions
                if '$(' in heredoc_content or re.search(r'\$[A-Za-z_][A-Za-z0-9_]', heredoc_content):
                    # Has expansions, use unquoted delimiter
                    line = line.replace(f"<< '{delim}'", f"<<{delim}")
                    line = line.replace(f'<< "{delim}"', f"<<{delim}")
        output.append(line)
    
    return '\n'.join(output)

def process_function(func_file, template_dir, prefix):
    """Process a function and save its commands as template files."""
    with open(func_file, 'r') as f:
        func_text = f.read()
    
    commands = extract_commands_from_go_function(func_text)
    print(f"{prefix}: Extracted {len(commands)} commands")
    
    for i, cmd in enumerate(commands):
        step_num = i + 1
        # Fix shell template
        fixed = fix_shell_template(cmd)
        # Save as template file
        template_file = os.path.join(template_dir, f"step_{step_num}.sh.tmpl")
        with open(template_file, 'w') as f:
            f.write(fixed)
        print(f"  Step {step_num}: {len(fixed)} chars -> {template_file}")
    
    return len(commands)

def main():
    # Process each function
    funcs = [
        ('/tmp/registerRepoAwareDocsTemplate.go', 'docs'),
        ('/tmp/registerRepoAwareCICDTemplate.go', 'cicd'),
        ('/tmp/registerRepoAwareMigrationTemplate.go', 'migration')
    ]
    
    total_commands = 0
    for func_file, prefix in funcs:
        template_dir = f'/home/neves/zen/zen-brain1/internal/factory/templates/{prefix}'
        os.makedirs(template_dir, exist_ok=True)
        count = process_function(func_file, template_dir, prefix)
        total_commands += count
    
    print(f"\nTotal commands extracted: {total_commands}")

if __name__ == '__main__':
    main()