#!/usr/bin/env python3
"""Fix Command strings in repo_aware_templates.go."""

import re
import sys

def fix_command_string(cmd):
    """Fix a Command string."""
    # 1. Replace triple backticks with ~~~bash
    cmd = cmd.replace('```bash', '~~~bash')
    cmd = cmd.replace('```', '~~~')
    
    # 2. Fix GitLab CI detection bug
    cmd = cmd.replace('[ -d .gitlab-ci.yml ]', '[ -f .gitlab-ci.yml ]')
    
    # 3. Fix heredoc delimiters when heredoc contains shell expansions
    # Find heredoc patterns: << 'EOF' or << "EOF" or <<EOF
    lines = cmd.split('\n')
    output = []
    in_heredoc = False
    heredoc_delimiter = None
    heredoc_has_expansion = False
    
    for i, line in enumerate(lines):
        # Check for heredoc start
        match = re.search(r'<<\s*[\'"]?([A-Za-z_][A-Za-z0-9_]*)[\'"]?', line)
        if match and not in_heredoc:
            delim = match.group(1)
            quoted = "'" in line or '"' in line
            in_heredoc = True
            heredoc_delimiter = delim
            heredoc_has_expansion = False
            # Check next lines for expansions
            for j in range(i+1, min(i+20, len(lines))):
                if lines[j].strip() == delim:
                    break
                if '$(' in lines[j] or re.search(r'\$[A-Za-z_][A-Za-z0-9_]*', lines[j]):
                    heredoc_has_expansion = True
                    break
            # If heredoc has expansions and delimiter is quoted, unquote it
            if quoted and heredoc_has_expansion:
                line = line.replace(f"<< '{delim}'", f"<<{delim}")
                line = line.replace(f'<< "{delim}"', f"<<{delim}")
            output.append(line)
        elif in_heredoc and line.strip() == heredoc_delimiter:
            in_heredoc = False
            heredoc_delimiter = None
            heredoc_has_expansion = False
            output.append(line)
        else:
            output.append(line)
    
    cmd = '\n'.join(output)
    
    # 4. Convert escape sequences (if we keep double quotes)
    # Actually, we'll convert to raw string (backticks) later
    return cmd

def process_file(input_file, output_file):
    """Process the Go file."""
    with open(input_file, 'r') as f:
        content = f.read()
    
    # We need to parse the Go structs. This is complex.
    # Instead, let's find the three function blocks and process them
    functions = [
        ('registerRepoAwareDocsTemplate', 325, 398),
        ('registerRepoAwareCICDTemplate', 483, 548),
        ('registerRepoAwareMigrationTemplate', 643, 724)
    ]
    
    lines = content.split('\n')
    
    # Process each function
    for func_name, start_line, end_line in functions:
        print(f"Processing {func_name}...")
        # Convert to 0-indexed
        start = start_line - 1
        end = end_line - 1
        
        # Find Command: fields in this range
        i = start
        while i <= end:
            line = lines[i]
            if 'Command:' in line:
                # Found a Command field
                # Check if it's a multi-line string
                if i+1 < len(lines) and lines[i+1].strip().startswith('"'):
                    # Multi-line string
                    string_start = i
                    # Find the end of the string
                    j = i+1
                    quote_count = 0
                    while j <= end and j < len(lines):
                        quote_count += lines[j].count('"')
                        # Adjust for escaped quotes
                        escaped = lines[j].count('\\"')
                        quote_count -= escaped
                        if quote_count >= 2:
                            break
                        j += 1
                    
                    # Extract string lines
                    string_lines = lines[string_start+1:j+1]
                    string_text = '\n'.join(string_lines)
                    
                    # Extract the actual string content (between quotes)
                    # Find first and last quote
                    first_quote = string_text.find('"')
                    last_quote = string_text.rfind('"')
                    if first_quote != -1 and last_quote != -1 and last_quote > first_quote:
                        cmd = string_text[first_quote+1:last_quote]
                        # Remove line continuations
                        cmd = cmd.replace('\\\n', '')
                        # Fix the command
                        fixed = fix_command_string(cmd)
                        # Convert to raw string (backticks)
                        # But need to handle backticks in the command
                        if '`' in fixed:
                            # Can't use raw string, keep double quotes with proper escaping
                            # Escape double quotes and backslashes
                            fixed = fixed.replace('\\', '\\\\')
                            fixed = fixed.replace('"', '\\"')
                            fixed = fixed.replace('\n', '\\n')
                            # Reconstruct as double-quoted string
                            new_string = '"' + fixed + '"'
                        else:
                            # Use raw string
                            # Need to preserve newlines
                            new_string = '`' + fixed + '`'
                        
                        # Replace in lines
                        # We need to replace the string lines with new string
                        # Actually, we should replace from string_start to j
                        # Let's just do a simple replacement for now
                        print(f"  Fixed command at line {i+1}")
                    
                    i = j
                else:
                    # Single-line string
                    # Extract string between quotes
                    match = re.search(r'Command:\s+"(.*?)"', line)
                    if match:
                        cmd = match.group(1)
                        fixed = fix_command_string(cmd)
                        if '`' in fixed:
                            fixed = fixed.replace('\\', '\\\\')
                            fixed = fixed.replace('"', '\\"')
                            new_line = re.sub(r'Command:\s+"(.*?)"', f'Command:     "{fixed}"', line)
                        else:
                            new_line = re.sub(r'Command:\s+"(.*?)"', f'Command:     `{fixed}`', line)
                        lines[i] = new_line
                        print(f"  Fixed single-line command at line {i+1}")
            i += 1
    
    # Write output
    with open(output_file, 'w') as f:
        f.write('\n'.join(lines))
    
    print(f"Written to {output_file}")

if __name__ == '__main__':
    process_file(
        '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go',
        '/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates_fixed.go'
    )