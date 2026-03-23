# Task Templates

This directory contains YAML templates that define bounded work items for zen-brain execution.

## Templates

Each template defines:
- `summary`: Brief description of the work
- `description`: Detailed explanation of what the template is for
- `labels`: Jira labels to apply to tickets
- `work_type`: Type of work (documentation, operations, testing, etc.)
- `allowed_paths`: File/directory scope for the work
- `acceptance_criteria`: Clear conditions for success
- `verification`: Steps to verify the work succeeded
- `timeout_profile`: Which timeout profile to use (normal-45m, short-test)
- `controlled_failure`: Whether this is a controlled failure path (only for failure testing)

## Timeout Profiles

### normal-45m
- Timeout: 2700 seconds (45 minutes)
- keep_alive: 45m
- Use for: Normal success-path work (documentation, runbooks, implementation)
- Model: qwen3.5:0.8b on CPU requires this timeout for reliable execution

### short-test
- Timeout: 300 seconds (5 minutes)
- keep_alive: 5m
- Use for: Controlled failure testing ONLY (validating error handling)
- Purpose: Intentionally trigger timeouts to verify graceful failure handling

**WARNING**: Only use `short-test` timeout profile with templates that have `controlled_failure: true`.

## Pilot Templates

The `pilot/` subdirectory contains templates for the 5-worker pilot:

- `success-docs.yaml`: Documentation improvement work (success path)
- `success-runbook.yaml`: Runbook/operations work (success path)
- `failure-timeout.yaml`: Timeout testing (controlled failure path)

## Adding New Templates

When adding new templates:

1. Follow the template schema above
2. Choose appropriate timeout profile:
   - `normal-45m` for actual work
   - `short-test` only for intentional failure testing
3. Define clear acceptance criteria
4. Include verification steps that can be automated
5. Specify allowed paths to prevent scope creep
6. Set `controlled_failure: false` unless this is explicitly a failure testing template

## ZB-024 Timeout Policy

**Rule**: Any real execution path using qwen3.5:0.8b on the active local CPU lane MUST use:
- timeout = 2700s (45m)
- keep_alive = 45m
- stale threshold > 45m

**Exceptions**: Controlled failure templates with `controlled_failure: true` may use short timeouts.

**Rationale**: qwen3.5:0.8b on CPU is slow but reliable. First token generation can take 10-20 minutes. The 45m timeout profile ensures real work completes while preventing indefinite hangs. Short timeouts (300s, 600s, 1200s) are WRONG for the normal lane and will cause spurious failures.

## Verification

After implementing work using a template:
1. Check all acceptance criteria are met
2. Run all verification steps
3. Ensure files changed are within `allowed_paths`
4. Proof-of-work artifacts must be real and meaningful
5. Update Jira with status and proof

## Duplicates

The planner MUST check for existing tickets/content before creating new work:
- Search Jira for similar titles
- Check documentation for overlapping content
- Identify if work is new vs enhancement to existing

Avoid duplicate tickets at all costs.
