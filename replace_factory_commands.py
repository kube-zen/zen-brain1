#!/usr/bin/env python3
"""Replace problematic Command strings with placeholders"""

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Replace three specific problematic Command lines
replacements = {
    # Line ~358: Create context-aware documentation
    'Command:     "if [ -f .zen-project-info ]; then . .zen-project-info; fi && if [ -f .zen-target-info ]; then . .zen-target-info; fi && cat > \\"$TARGET_PATH\\" << \'DOCS_EOF\'':
        'Command:     "echo \\"Template disabled pending escape sequence fix\\""',
    
    # Line ~700: Create migration documentation  
    'Command:     "mkdir -p docs && if [ -f .zen-project-info ]; then . .zen-project-info; fi && cat > docs/MIGRATIONS.md << \'MIGR_DOC_EOF\'':
        'Command:     "echo \\"Template disabled pending escape sequence fix\\""',
}

for old, new in replacements.items():
    content = content.replace(old, new, 1)  # Replace first occurrence only

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(content)

print("Replaced problematic Command strings")