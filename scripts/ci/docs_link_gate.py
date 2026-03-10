#!/usr/bin/env python3
"""
Gate: Validate internal markdown links.

Check that internal relative links (within the repository) point to existing files.
Skip external URLs, anchor‑only links (#something), and mailto: links.
"""

import os
import re
import sys
from urllib.parse import urlparse


def _repo_root() -> str:
    return os.path.abspath(
        os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    )


# Regex to capture markdown links: [text](url "optional title")
LINK_RE = re.compile(r'\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)')


def is_external_url(link: str) -> bool:
    """Return True if link is an external URL (http, https, ftp, mailto)."""
    parsed = urlparse(link)
    return bool(parsed.scheme) and parsed.scheme not in {"", "file"}


def normalize_link(src_path: str, link: str) -> str:
    """
    Convert a relative link to an absolute path relative to repo root.
    Handles directory traversal (../) and anchors (#).
    """
    # Strip anchor
    if "#" in link:
        link = link.split("#")[0]
        if not link:  # link was just an anchor
            return ""
    
    # If link is empty after stripping anchor, skip
    if not link:
        return ""
    
    # If link starts with /, treat as relative to repo root
    if link.startswith("/"):
        return link[1:]
    
    # Relative link: resolve relative to src_path's directory
    src_dir = os.path.dirname(src_path)
    resolved = os.path.normpath(os.path.join(src_dir, link))
    # Remove leading ./ if any
    if resolved.startswith("./"):
        resolved = resolved[2:]
    return resolved


def find_markdown_files(root: str) -> list[str]:
    """Return list of .md files relative to root. Skip .git and vendor."""
    md_files = []
    for dirpath, _, filenames in os.walk(root):
        if ".git" in dirpath or "vendor" in dirpath:
            continue
        for f in filenames:
            if f.endswith(".md"):
                full = os.path.join(dirpath, f)
                rel = os.path.relpath(full, root)
                md_files.append(rel)
    return md_files


def check_links(root: str) -> tuple[list[str], list[str]]:
    """
    Return (errors, warnings) for broken internal links.
    """
    errors = []
    warnings = []
    
    md_files = find_markdown_files(root)
    # Build set of existing files (relative to root)
    existing_files = set(md_files)
    
    for src in md_files:
        src_path = os.path.join(root, src)
        try:
            with open(src_path, "r", encoding="utf-8") as f:
                content = f.read()
        except (OSError, UnicodeDecodeError):
            warnings.append(f"Cannot read {src}")
            continue
        
        for match in LINK_RE.finditer(content):
            link_text = match.group(1)
            link_url = match.group(2)
            
            # Skip external URLs
            if is_external_url(link_url):
                continue
            
            # Skip anchor‑only links
            if link_url.startswith("#"):
                continue
            
            # Normalize link to repo‑relative path
            target = normalize_link(src, link_url)
            if not target:  # empty after anchor strip
                continue
            
            # Check if target exists
            target_path = os.path.join(root, target)
            if not os.path.exists(target_path):
                errors.append(
                    f"Broken link in {src}: [{link_text}]({link_url}) → {target} (file not found)"
                )
            elif target not in existing_files:
                # File exists but not a markdown file (maybe image, etc.)
                # That's okay, but we could warn if it's a .md link expecting markdown
                if target.endswith(".md"):
                    warnings.append(
                        f"Link to non‑markdown file in {src}: [{link_text}]({link_url}) → {target}"
                    )
    
    return errors, warnings


def main() -> int:
    root = _repo_root()
    errors, warnings = check_links(root)
    
    if warnings:
        for w in warnings:
            print(f"WARNING: {w}", file=sys.stderr)
    
    if errors:
        print("ERROR: Broken internal markdown links:", file=sys.stderr)
        for e in errors:
            print(f"  • {e}", file=sys.stderr)
        print(file=sys.stderr)
        print("Run a link‑fixing script or update the links manually.", file=sys.stderr)
        return 1
    
    print("✓ Docs link gate: pass")
    return 0


if __name__ == "__main__":
    sys.exit(main())