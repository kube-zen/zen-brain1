#!/usr/bin/env python3
"""
Gate: Repository layout and documentation structure.

Zen‑Brain rules:
1. Only allowed root‑level markdown files: README.md, INDEX.md, AGENTS.md,
   WORKFLOW.md, CONTRIBUTING.md.
2. All other markdown files must be under docs/.
3. docs/ must be organized into numbered directories:
   - 01‑ARCHITECTURE/
   - 02‑CONTRACTS/
   - 03‑DESIGN/
   - 04‑DEVELOPMENT/
   - 05‑OPERATIONS/
   - 06‑EXAMPLES/
   - 99‑ARCHIVE/
4. Markdown files under docs/ must use UPPER_SNAKE_CASE names (except README.md, INDEX.md).
5. ADR files under docs/01‑ARCHITECTURE/ADR/ must follow 0001_STRUCTURED_TAGS.md pattern.
"""

import os
import re
import sys


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


ALLOWED_ROOT_MD = {
    "README.md",
    "INDEX.md",
    "AGENTS.md",
    "WORKFLOW.md",
    "CONTRIBUTING.md",
}

EXPECTED_DOCS_DIRS = {
    "01-ARCHITECTURE",
    "02-CONTRACTS",
    "03-DESIGN",
    "04-DEVELOPMENT",
    "05-OPERATIONS",
    "06-EXAMPLES",
    "99-ARCHIVE",
}

# Regex for valid uppercase snake case markdown filenames
UPPER_SNAKE_RE = re.compile(r'^[A-Z][A-Z0-9_]*\.md$')
# Regex for ADR filenames: 0001_STRUCTURED_TAGS.md
ADR_FILENAME_RE = re.compile(r'^\d{4}_[A-Z][A-Z0-9_]*\.md$')


def check_root_markdown(root: str) -> tuple[list[str], list[str]]:
    """Return (errors, warnings)."""
    errors = []
    warnings = []
    
    for entry in os.listdir(root):
        if entry.endswith(".md"):
            if entry not in ALLOWED_ROOT_MD:
                errors.append(
                    f"Root‑level markdown file not allowed: {entry}. "
                    f"Allowed: {', '.join(sorted(ALLOWED_ROOT_MD))}. "
                    f"Move to docs/ or rename to an allowed file."
                )
    return errors, warnings


def check_docs_structure(root: str) -> tuple[list[str], list[str]]:
    """Validate docs/ directory layout."""
    errors = []
    warnings = []
    docs_dir = os.path.join(root, "docs")
    
    if not os.path.isdir(docs_dir):
        errors.append("docs/ directory does not exist.")
        return errors, warnings
    
    # Check for unexpected top‑level items under docs/
    for entry in os.listdir(docs_dir):
        entry_path = os.path.join(docs_dir, entry)
        if os.path.isdir(entry_path):
            if entry not in EXPECTED_DOCS_DIRS:
                errors.append(
                    f"Unexpected directory under docs/: {entry}. "
                    f"Expected: {', '.join(sorted(EXPECTED_DOCS_DIRS))}"
                )
        else:
            # Files directly under docs/ should only be README.md, INDEX.md
            if entry not in {"README.md", "INDEX.md"}:
                errors.append(
                    f"File directly under docs/ must be README.md or INDEX.md, found: {entry}"
                )
    
    # Check each numbered directory
    for dir_name in EXPECTED_DOCS_DIRS:
        dir_path = os.path.join(docs_dir, dir_name)
        if not os.path.isdir(dir_path):
            warnings.append(f"Expected directory docs/{dir_name} does not exist (optional).")
            continue
        
        # Special handling for ADR directory
        if dir_name == "01-ARCHITECTURE":
            adr_dir = os.path.join(dir_path, "ADR")
            if os.path.isdir(adr_dir):
                for adr_file in os.listdir(adr_dir):
                    if adr_file.endswith(".md"):
                        # Allow supporting files: README.md, TEMPLATE.md (case‑insensitive)
                        if adr_file.lower() in {"readme.md", "template.md"}:
                            continue
                        if not ADR_FILENAME_RE.match(adr_file):
                            errors.append(
                                f"ADR file must follow 0001_STRUCTURED_TAGS.md pattern: "
                                f"docs/{dir_name}/ADR/{adr_file}"
                            )
        
        # Check markdown files in this directory
        for file_name in os.listdir(dir_path):
            if not file_name.endswith(".md"):
                continue
            file_path = os.path.join(dir_path, file_name)
            if os.path.isdir(file_path):
                continue
            
            # README.md and INDEX.md are allowed with any casing
            if file_name.lower() in {"readme.md", "index.md"}:
                continue
            
            # Check uppercase snake case
            if not UPPER_SNAKE_RE.match(file_name):
                errors.append(
                    f"Markdown file not in UPPER_SNAKE_CASE: docs/{dir_name}/{file_name}. "
                    f"Example: PROJECT_STRUCTURE.md"
                )
    
    return errors, warnings


def main() -> int:
    root = _repo_root()
    errors = []
    warnings = []
    
    root_errs, root_warns = check_root_markdown(root)
    errors.extend(root_errs)
    warnings.extend(root_warns)
    
    docs_errs, docs_warns = check_docs_structure(root)
    errors.extend(docs_errs)
    warnings.extend(docs_warns)
    
    if warnings:
        for w in warnings:
            print(f"WARNING: {w}", file=sys.stderr)
    
    if errors:
        print("ERROR: Repository layout violations:", file=sys.stderr)
        for e in errors:
            print(f"  • {e}", file=sys.stderr)
        print(file=sys.stderr)
        print("Refer to docs/01‑ARCHITECTURE/PROJECT_STRUCTURE.md for layout rules.", file=sys.stderr)
        return 1
    
    print("✓ Repository layout gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())