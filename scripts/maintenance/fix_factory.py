#!/usr/bin/env python3
import re
import sys

def fix_go_string_literal(s):
    """Convert a Go double-quoted string literal to a raw string literal if possible."""
    # If the string contains backticks, we can't use raw string literal directly
    # Instead, we can keep it double-quoted but properly escape it
    # For now, just escape backslashes and quotes
    # Actually, we need to ensure the string is valid Go.
    # We'll use str repr and adjust
    return s

def process_file(content):
    # First, fix the specific problematic lines
    # Replace \\n with \n (but in Go source, we need \\n to represent literal backslash-n?)
    # Actually, we want newline characters in the string, not \n literals.
    # Since the string is inside a shell script heredoc, we want literal newlines.
    # The easiest is to replace \\n with actual newline in the Go string.
    # However, that would break line continuation in Go source.
    # Instead, we can keep \\n as is but ensure Go doesn't treat it as invalid escape.
    # The error "unknown escape" suggests Go sees a backslash followed by a character that's not a valid escape.
    # For example, \` is not a valid Go escape sequence. We have \` in the string.
    # Need to escape backticks: \` -> \` (backslash-backtick) is invalid; should be \\`
    # Let's find all backslash sequences and ensure they are valid.
    # Valid Go escapes: \a, \b, \f, \n, \r, \t, \v, \\, \', \", \`
    # Actually \` is not a valid escape; backtick is only used in raw strings.
    # So \` must be represented as \\`
    # Similarly, \$ is not valid, but $ doesn't need escaping.
    # We'll replace \` with \\`
    # Replace \` with \\`
    content = re.sub(r'(?<!\\)\\(`)', r'\\\1', content)
    # Replace \\` with \\\`? Hmm.
    # Let's do a more systematic approach: find all backslash sequences inside double-quoted strings.
    # We'll parse the file token-wise? Too complex.
    # Instead, we'll replace the entire Command strings with properly escaped versions using raw strings.
    # We'll identify each "Command:" field.
    return content

def main():
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
        content = f.read()
    
    # First, fix the $ issue: maybe there's an unclosed string causing $ to appear outside.
    # Let's look for lines with Command: and ensure quotes are balanced.
    lines = content.split('\n')
    in_command = False
    command_start = None
    new_lines = []
    i = 0
    while i < len(lines):
        line = lines[i]
        if 'Command:' in line and not in_command:
            # Find the opening double quote
            idx = line.find('"', line.find('Command:'))
            if idx != -1:
                # Start of string
                in_command = True
                command_lines = [line]
                # Count quotes in this line
                # If the string ends on same line, check
                # Simple heuristic: if number of double quotes after idx is odd, string closes
                # But there are escaped quotes.
                # We'll just collect until we find a line with an unescaped double quote that closes.
                # This is hacky but works for this file.
                j = i
                current = line
                quote_count = 0
                for ch in current:
                    if ch == '"' and (len(current) == 0 or current[-1] != '\\'):
                        quote_count += 1
                while quote_count % 2 == 0 and j < len(lines):
                    j += 1
                    if j >= len(lines):
                        break
                    current = lines[j]
                    for ch in current:
                        if ch == '"' and (len(current) == 0 or current[-1] != '\\'):
                            quote_count += 1
                # Now lines[i:j] contain the full string (including closing quote on line j)
                # Let's extract the whole string and fix it.
                # For simplicity, we'll just keep as is for now.
                pass
        new_lines.append(line)
        i += 1
    
    # Instead of complex parsing, let's just replace the two problematic commands
    # We'll locate the documentation template command (around line 358)
    # and migration documentation command (around line 700)
    # and replace them with corrected versions.
    # We'll write a corrected version using backticks and escaping internal backticks by splitting.
    # Let's create a corrected command for documentation template.
    # We'll read the original command string from the file using regex.
    
    # Pattern: Command:\s*"([^"]*)" but multi-line with escaped quotes.
    # Use re.DOTALL and find all occurrences.
    pattern = r'(Command:\s*")(.*?)("\s*,\s*\n)'
    # Need to match across lines? Use re.DOTALL and non-greedy.
    # Let's just manually replace the two problematic ones by line numbers.
    
    # Write back
    with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
        f.write(content)
    
    print("Attempted fixes")

if __name__ == '__main__':
    main()