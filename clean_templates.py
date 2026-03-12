#!/usr/bin/env python3
"""Clean up template files."""

import os
import re
import glob

def clean_template_file(filepath):
    """Clean up a template file."""
    with open(filepath, 'r') as f:
        content = f.read()
    
    original = content
    
    # 1. Remove trailing backslashes that are not shell line continuations
    # Shell line continuations: backslash followed by newline
    # But in our case, we have actual newlines, not \n escapes
    # So trailing backslash at end of line should be removed unless it's a continuation
    # Actually, looking at the files, backslashes appear at end of lines inside heredoc
    # Those should be removed because heredoc content doesn't need line continuations
    lines = content.split('\n')
    cleaned = []
    for i, line in enumerate(lines):
        # Check if line ends with backslash
        if line.endswith('\\'):
            # Check if this is a heredoc delimiter line
            if '<<' in line:
                # Remove the backslash
                line = line.rstrip('\\')
            # Check if next line exists and is not empty
            elif i+1 < len(lines) and lines[i+1].strip():
                # Might be shell line continuation, keep it
                pass
            else:
                # Remove trailing backslash
                line = line.rstrip('\\')
        cleaned.append(line)
    
    content = '\n'.join(cleaned)
    
    # 2. Ensure heredoc delimiters are unquoted when they contain expansions
    # Already done in extraction, but double-check
    lines = content.split('\n')
    for i, line in enumerate(lines):
        if '<<' in line:
            # Check if quoted
            match = re.search(r'<<\s*[\'"]([A-Za-z_][A-Za-z0-9_]*)[\'"]', line)
            if match:
                delim = match.group(1)
                # Check if heredoc has expansions
                # Find heredoc content
                heredoc_start = content.find(line)
                content_start = heredoc_start + len(line)
                end_pos = content.find(delim, content_start)
                if end_pos != -1:
                    heredoc_content = content[content_start:end_pos]
                    if '$(' in heredoc_content or re.search(r'\$[A-Za-z_][A-Za-z0-9_]', heredoc_content):
                        # Has expansions, unquote
                        lines[i] = line.replace(f"<< '{delim}'", f"<<{delim}")
                        lines[i] = lines[i].replace(f'<< "{delim}"', f"<<{delim}")
    
    content = '\n'.join(lines)
    
    # 3. Final pass: remove any standalone backslash lines
    content = re.sub(r'^\\$', '', content, flags=re.MULTILINE)
    
    if content != original:
        with open(filepath, 'w') as f:
            f.write(content)
        return True
    return False

def main():
    template_dirs = [
        '/home/neves/zen/zen-brain1/internal/factory/templates/docs',
        '/home/neves/zen/zen-brain1/internal/factory/templates/cicd',
        '/home/neves/zen/zen-brain1/internal/factory/templates/migration'
    ]
    
    cleaned_count = 0
    for dir_path in template_dirs:
        for filepath in glob.glob(os.path.join(dir_path, '*.sh.tmpl')):
            if clean_template_file(filepath):
                print(f"Cleaned: {filepath}")
                cleaned_count += 1
    
    print(f"Cleaned {cleaned_count} files")

if __name__ == '__main__':
    main()