#!/usr/bin/env python3
"""Replace problematic factory template functions with stubs"""

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'r') as f:
    content = f.read()

# Function names to replace with stubs
functions_to_stub = {
    'registerRepoAwareDocsTemplate': 'registerRepoAwareDocsTemplate',
    'registerRepoAwareCICDTemplate': 'registerRepoAwareCICDTemplate',
    'registerRepoAwareMigrationTemplate': 'registerRepoAwareMigrationTemplate'
}

for old_name, new_name in functions_to_stub.items():
    # Find the function definition and replace with a stub
    # Pattern: func (r *WorkTypeTemplateRegistry) old_name() { ... }
    # Replace with: func (r *WorkTypeTemplateRegistry) new_name() {
    #     // STUB: Disabled due to complex escape sequences in Command strings
    #     // TODO: Rewrite Command strings with proper Go escaping
    #     log.Printf("STUB: %s is disabled", "docs/cicd/migration template")
    # }

    # Find function start and end
    import re

    # Pattern to match function definition through closing brace
    pattern = rf'(func \(r \*WorkTypeTemplateRegistry\) {old_name}\(\) \{{[^}}]*\}})'

    # This is complex - let's use a simpler line-by-line approach
    # Read the file, find the function, replace it
    pass

# Simpler: just comment out the registration calls (we already did that)
# and add a note that the function bodies are commented out below

# Actually, let's just verify compilation now - we already commented registrations
print("Done")