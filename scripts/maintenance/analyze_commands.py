#!/usr/bin/env python3
"""Analyze Command strings in template functions."""

import re
import os

def analyze_commands(func_file, func_name):
    """Analyze all Command strings in a function."""
    with open(func_file, 'r') as f:
        content = f.read()
    
    # Find all Command: fields
    # Pattern: Command:\s+"(.*?)"(?=\s*,)
    # But need to handle multi-line strings
    # Simpler: find lines with Command: then collect until closing quote
    lines = content.split('\n')
    commands = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if 'Command:' in line:
            # Found a command
            cmd_lines = []
            j = i
            # Find the start of the string
            while j < len(lines) and '"' not in lines[j]:
                j += 1
            if j < len(lines):
                # Start collecting string lines
                k = j
                quote_count = 0
                while k < len(lines):
                    cmd_lines.append(lines[k])
                    quote_count += lines[k].count('"')
                    # Count escaped quotes
                    # Remove escaped quotes from count
                    escaped_quotes = lines[k].count('\\"')
                    quote_count -= escaped_quotes
                    if quote_count >= 2:
                        break
                    k += 1
                cmd_text = '\n'.join(cmd_lines)
                # Extract just the string content
                # Find first " and last "
                first = cmd_text.find('"')
                last = cmd_text.rfind('"')
                if first != -1 and last != -1 and last > first:
                    cmd = cmd_text[first+1:last]
                    # Remove line continuations
                    cmd = cmd.replace('\\\n', '')
                    commands.append((i+1, cmd))
                i = k
        i += 1
    
    print(f"\n=== {func_name} ===")
    for line_num, cmd in commands:
        print(f"\nCommand at line {line_num}:")
        print(f"  Length: {len(cmd)} chars")
        print(f"  Lines: {cmd.count(chr(10))+1}")
        print(f"  Has heredoc: {'<<' in cmd}")
        print(f"  Has triple backticks: {'```' in cmd}")
        print(f"  Has shell expansions: {'$(' in cmd or ('$' in cmd and re.search(r'\\$[A-Za-z_]', cmd))}")
        # Show first 100 chars
        preview = cmd[:100].replace('\n', '\\n')
        print(f"  Preview: {preview}...")

# Analyze all three functions
for name in ['registerRepoAwareDocsTemplate', 'registerRepoAwareCICDTemplate', 'registerRepoAwareMigrationTemplate']:
    analyze_commands(f'/tmp/{name}.go', name)