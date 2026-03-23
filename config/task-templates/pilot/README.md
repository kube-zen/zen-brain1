# Pilot Templates

Templates for the zen-brain 5-worker pilot.

## Purpose

These templates support the initial pilot of zen-brain with real Jira integration and bounded work execution.

## Templates

### success-docs.yaml
**Type**: Success path
**Timeout**: normal-45m
**Purpose**: Update or create system documentation

Use this template for:
- Adding missing documentation
- Updating outdated sections
- Improving clarity or completeness
- Adding examples or troubleshooting guides

### success-runbook.yaml
**Type**: Success path
**Timeout**: normal-45m
**Purpose**: Create or update operational runbooks

Use this template for:
- Documenting deployment procedures
- Creating troubleshooting guides
- Adding rollback procedures
- Operational checklists

### failure-timeout.yaml
**Type**: Controlled failure path
**Timeout**: short-test
**Purpose**: Test timeout behavior and error handling

**WARNING**: This template deliberately uses short timeouts. DO NOT use for normal work.

Use this template ONLY for:
- Validating timeout handling
- Testing error recovery
- Verifying session state cleanup
- Confirming graceful degradation

## Pilot Execution Flow

1. Planner selects template based on work context
2. Checks for duplicates (existing Jira tickets, docs)
3. Creates Jira ticket with labels and acceptance criteria
4. Worker executes bounded task using template parameters
5. Verifies against acceptance criteria
6. Updates Jira with status and proof-of-work

## Success vs Failure Paths

**Success Path (docs, runbooks)**:
- Long timeout (45m) - allows real work to complete
- Create/update real artifacts
- Verify acceptance criteria
- Mark Jira as Done

**Failure Path (timeout testing)**:
- Short timeout (5m) - intentionally fails
- Verify error handling works
- Confirm no crashes or resource leaks
- Mark Jira as Failed (expected)

## Tracking

Label all pilot tickets with:
- `pilot-success` for success path tickets
- `pilot-validation` for controlled failure tickets
- Additional labels by work type (documentation, operations, testing)
