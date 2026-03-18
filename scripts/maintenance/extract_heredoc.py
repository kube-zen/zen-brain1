#!/usr/bin/env python3
"""Extract heredoc content and reconstruct command with raw string."""

import re
import sys

with open('/tmp/registerRepoAwareDocsTemplate.go', 'r') as f:
    content = f.read()

# Find the step 4 Command
pattern = r'Name:\s+"Create context-aware documentation".*?Command:\s+"(.*?)"'
match = re.search(pattern, content, re.DOTALL)
if not match:
    print("Not found")
    sys.exit(1)

cmd = match.group(1)
print("Original command length:", len(cmd))
print("First 200 chars:", cmd[:200])

# Extract heredoc delimiter
heredoc_match = re.search(r'<<\s*[\'"]?(DOCS_EOF)[\'"]?', cmd)
if heredoc_match:
    delim = heredoc_match.group(1)
    print("Delimiter:", delim)
    # Find the heredoc content
    parts = cmd.split(delim)
    if len(parts) >= 3:
        before = parts[0]
        heredoc_content = parts[1]
        after = delim.join(parts[2:])
        print("Heredoc content length:", len(heredoc_content))
        # Replace triple backticks with ~~~bash
        heredoc_content = heredoc_content.replace('```bash', '~~~bash')
        heredoc_content = heredoc_content.replace('```', '~~~')
        # Change heredoc delimiter to unquoted
        before = before.replace(f"<< '{delim}'", f"<<{delim}")
        before = before.replace(f'<< "{delim}"', f"<<{delim}")
        # Reconstruct command with raw string (use backticks)
        # Need to escape backticks inside heredoc content
        heredoc_content = heredoc_content.replace('`', '` + "`" + `')
        # Actually, better to use raw string with backticks, but need to handle backticks
        # Let's just use double quotes with proper escaping
        new_cmd = before + delim + heredoc_content + delim + after
        # Replace \\n with actual newlines
        new_cmd = new_cmd.replace('\\n', '\n')
        # Replace other escapes
        new_cmd = new_cmd.replace('\\"', '"')
        new_cmd = new_cmd.replace("\\'", "'")
        print("New command first 200 chars:", new_cmd[:200])
        # Write as Go raw string
        go_code = '`' + new_cmd.replace('`', '` + "`" + `') + '`'
        print("Go code (first 300 chars):", go_code[:300])
    else:
        print("Could not split by delimiter")