#!/usr/bin/env python3
import sys

with open('/home/neves/zen/zen-brain1/internal/factory/repo_aware_templates.go', 'rb') as f:
    data = f.read()

# Find line 358
lines = data.split(b'\n')
if len(lines) >= 358:
    line = lines[357]  # zero-index
    print("Line 358 raw:", line)
    # Find escape sequences
    for i, ch in enumerate(line):
        if ch == ord('\\'):
            print(f"  Backslash at position {i}, next char: {chr(line[i+1]) if i+1 < len(line) else 'EOF'}")

# Also check the problematic $ at line 700
if len(lines) >= 700:
    line = lines[699]
    print("\nLine 700 raw:", line)
    for i, ch in enumerate(line):
        if ch == ord('$'):
            print(f"  Dollar at position {i}")
            # Show context
            context = line[max(0,i-10):i+10]
            print(f"    Context: {context}")