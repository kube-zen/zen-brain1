#!/usr/bin/env python3
"""Extract commands from template function and create embedded template files."""

import re
import os
import sys

def extract_commands_from_function(func_text, func_name):
    """Extract all Command strings from a function."""
    # Find the Steps array
    steps_match = re.search(r'Steps:\s*\[\]ExecutionStepTemplate\s*{([^}]+(?:\{[^}]*\}[^}]*)*)}', func_text, re.DOTALL)
    if not steps_match:
        print(f"No Steps found in {func_name}")
        return []
    
    steps_text = steps_match.group(1)
    
    # Split into individual step blocks
    # Look for pattern: { ... },
    steps = []
    brace_level = 0
    current_step = []
    in_step = False
    
    for line in steps_text.split('\n'):
        stripped = line.strip()
        if not in_step and stripped.startswith('{'):
            in_step = True
            brace_level = 0
        
        if in_step:
            current_step.append(line)
            brace_level += line.count('{')
            brace_level -= line.count('}')
            if brace_level <= 0 and '}' in line:
                steps.append('\n'.join(current_step))
                current_step = []
                in_step = False
    
    # Extract Command from each step
    commands = []
    for i, step in enumerate(steps):
        # Find Command field
        cmd_match = re.search(r'Command:\s+"(.*?)"(?=\s*,)', step, re.DOTALL)
        if not cmd_match:
            # Try with backticks
            cmd_match = re.search(r'Command:\s+`(.*?)`(?=\s*,)', step, re.DOTALL)
        if cmd_match:
            cmd = cmd_match.group(1)
            # Remove line continuations
            cmd = cmd.replace('\\\n', '')
            commands.append((i+1, cmd))
        else:
            print(f"Warning: No Command found in step {i+1}")
    
    return commands

def fix_shell_template(cmd):
    """Fix shell template issues."""
    # 1. Fix heredoc delimiters when heredoc contains shell expansions
    # Find << 'EOF' or << "EOF"
    lines = cmd.split('\n')
    output = []
    for line in lines:
        # Check for heredoc start with quotes
        match = re.search(r'<<\s*[\'"]([A-Za-z_][A-Za-z0-9_]*)[\'"]', line)
        if match:
            delim = match.group(1)
            # Check if heredoc contains expansions
            # Simple heuristic: look ahead for $(
            idx = cmd.find(line)
            heredoc_start = idx + len(line)
            # Find the end delimiter
            end_idx = cmd.find(delim, heredoc_start)
            if end_idx != -1:
                heredoc_content = cmd[heredoc_start:end_idx]
                if '$(' in heredoc_content or re.search(r'\$[A-Za-z_][A-Za-z0-9_]', heredoc_content):
                    # Has expansions, use unquoted delimiter
                    line = line.replace(f"<< '{delim}'", f"<<{delim}")
                    line = line.replace(f'<< "{delim}"', f"<<{delim}")
        output.append(line)
    cmd = '\n'.join(output)
    
    # 2. Replace triple backticks with ~~~bash
    cmd = cmd.replace('```bash', '~~~bash')
    cmd = cmd.replace('```', '~~~')
    
    # 3. Fix GitLab CI bug
    cmd = cmd.replace('[ -d .gitlab-ci.yml ]', '[ -f .gitlab-ci.yml ]')
    
    return cmd

def main():
    # Read the three function files
    func_files = [
        ('registerRepoAwareDocsTemplate', 'docs'),
        ('registerRepoAwareCICDTemplate', 'cicd'),
        ('registerRepoAwareMigrationTemplate', 'migration')
    ]
    
    for func_name, prefix in func_files:
        with open(f'/tmp/{func_name}.go', 'r') as f:
            func_text = f.read()
        
        commands = extract_commands_from_function(func_text, func_name)
        print(f"\n{func_name}: {len(commands)} commands")
        
        # Create directory for this template
        template_dir = f'/home/neves/zen/zen-brain1/internal/factory/templates/{prefix}'
        os.makedirs(template_dir, exist_ok=True)
        
        for i, (step_num, cmd) in enumerate(commands):
            # Fix the command
            fixed = fix_shell_template(cmd)
            # Save as template file
            template_file = f'{template_dir}/step_{step_num}.sh.tmpl'
            with open(template_file, 'w') as f:
                f.write(fixed)
            print(f"  Step {step_num}: {len(fixed)} chars -> {template_file}")

if __name__ == '__main__':
    main()