#!/usr/bin/env python3
import re

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Fix 1: Documentation template command (around line 358)
# Replace the entire Command string with a simpler version that avoids complex escaping
# We'll keep the functionality but simplify the escaping
# The problematic part seems to be the echo statements with backticks and backslashes
# We'll rewrite using a different approach: use cat with simpler content

# Find the documentation template command
# Pattern: from "Command:" to the closing quote before Variables
# This is hacky but works for this file structure
lines = content.split('\n')
new_lines = []
i = 0
while i < len(lines):
    line = lines[i]
    if 'Name:        "Create context-aware documentation"' in line:
        # Found the start of this step
        # Keep lines until we find the closing quote of Command
        new_lines.append(line)
        i += 1
        while i < len(lines):
            line2 = lines[i]
            new_lines.append(line2)
            if 'Command:' in line2:
                # This is the Command line - we need to replace it
                # But the command spans multiple lines
                # Actually the command is all on one line (very long)
                # We'll replace the entire line
                new_lines.pop()  # Remove the old command line
                # Create a simpler command
                simple_cmd = '''\t\t\tCommand:     "if [ -f .zen-project-info ]; then . .zen-project-info; fi && if [ -f .zen-target-info ]; then . .zen-target-info; fi && cat > \\"$TARGET_PATH\\" << 'DOCS_EOF'
# {{.title}}

> **Work Item:** {{.work_item_id}}
> **Created:** $(date -u +%Y-%m-%dT%H:%M:%SZ)

## Overview

{{.objective}}

## Project Context

$(if [ \\"$PROJECT_TYPE\\" = 'go' ]; then echo 'This documentation applies to the Go module **$MODULE_NAME**.'; elif [ \\"$PROJECT_TYPE\\" = 'node' ]; then echo 'This documentation applies to the Node.js package.'; else echo 'This documentation applies to this project.'; fi)

## Getting Started

### Prerequisites
$(if [ \\"$PROJECT_TYPE\\" = 'go' ]; then echo '- Go 1.21+ installed'; echo '- GOPATH/bin in your PATH'; elif [ \\"$PROJECT_TYPE\\" = 'node' ]; then echo '- Node.js 18+ installed'; echo '- npm or yarn package manager'; else echo '- Appropriate development environment for this project type'; fi)

### Installation
$(if [ \\"$PROJECT_TYPE\\" = 'go' ]; then echo 'Install with: go get ./...'; elif [ \\"$PROJECT_TYPE\\" = 'node' ]; then echo 'Install with: npm install'; else echo 'Follow standard installation procedures for this project type.'; fi)

### Quick Start
1. Clone the repository
2. Navigate to project directory
3. $(if [ \\"$PROJECT_TYPE\\" = 'go' ]; then echo 'Run: go run main.go'; elif [ \\"$PROJECT_TYPE\\" = 'node' ]; then echo 'Run: npm start'; else echo 'Run the main application'; fi)
4. Verify the service is running

## Usage

### Basic Usage
The {{.title}} provides the following capabilities:

$(echo '{{.objective}}' | sed 's/\\. /\\\\n- /g' | sed 's/^/- /')

### Configuration
Configuration can be provided via environment variables or configuration files.

## Troubleshooting

### Common Issues
- Service fails to start: Check port availability and required environment variables
- Database connection errors: Verify database credentials and network connectivity
- Performance degradation: Monitor resource usage and adjust scaling

## See Also
- Project README
- Architecture Documentation

*Documented as part of work item {{.work_item_id}}*
DOCS_EOF
echo \\"$TARGET_PATH\\" >> .zen-repo-files-changed && echo \\"Created: $TARGET_PATH\\""'''
                new_lines.append(simple_cmd)
                # Skip until we're past Variables
                i += 1
                while i < len(lines) and 'Variables:' not in lines[i]:
                    i += 1  # Skip any continuation lines (shouldn't be any)
                new_lines.append(lines[i])  # Add Variables line
                i += 1
                break
            else:
                i += 1
    elif 'Name:        "Create migration documentation"' in line:
        # Found migration documentation step
        new_lines.append(line)
        i += 1
        while i < len(lines):
            line2 = lines[i]
            if 'Command:' in line2:
                # Replace this command with simpler version
                new_lines.pop()  # Remove old command
                simple_migr_cmd = '''\t\t\tCommand:     "mkdir -p docs && if [ -f .zen-project-info ]; then . .zen-project-info; fi && cat > docs/MIGRATIONS.md << 'MIGR_DOC_EOF'
# Database Migrations

> **Work Item:** {{.work_item_id}}
> **Framework:** $MIGRATION_TYPE

## Overview

This project uses **$MIGRATION_TYPE** for database migrations.

## Running Migrations

### Apply Migrations

\\`\\`\\`bash
# Apply all pending migrations
zen-brain migrate up

# Apply specific migration
zen-brain migrate up 1
\\`\\`\\`

### Rollback Migrations

\\`\\`\\`bash
# Rollback last migration
zen-brain migrate down

# Rollback specific migration
zen-brain migrate down 1
\\`\\`\\`

### Check Status

\\`\\`\\`bash
# Show current version
zen-brain migrate version
\\`\\`\\`

## Migration Files

$(if [ -d migrations ]; then echo '### Applied Migrations'; ls -1t migrations/*.up.sql 2>/dev/null | head -20 | while read f; do NAME=\\$(basename \\"$f\\" .up.sql); echo \\"- [\\$NAME](../migrations/\\$NAME.up.sql) - Created: \\$(stat -c %y \\"migrations/\\$f.up.sql\\" 2>/dev/null || stat -f %Sm \\"migrations/\\$f.up.sql\\" 2>/dev/null)\\"; done 2>/dev/null; else echo 'No migrations created yet.'; fi)

## Creating New Migrations

1. Create migration files in \\`migrations/\\` directory
2. Use timestamp naming convention: \\`YYYYMMDDHHMMSS_description.up.sql\\`
3. Create corresponding rollback: \\`YYYYMMDDHHMMSS_description.down.sql\\`

MIGR_DOC_EOF
echo 'docs/MIGRATIONS.md' >> .zen-repo-files-changed && echo 'Created: docs/MIGRATIONS.md'"'''
                new_lines.append(simple_migr_cmd)
                i += 1
                while i < len(lines) and 'Variables:' not in lines[i]:
                    i += 1
                new_lines.append(lines[i])
                i += 1
                break
            else:
                new_lines.append(line2)
                i += 1
    else:
        new_lines.append(line)
        i += 1

new_content = '\n'.join(new_lines)

# Write back
with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'w') as f:
    f.write(new_content)

print("Applied fixes to factory template")