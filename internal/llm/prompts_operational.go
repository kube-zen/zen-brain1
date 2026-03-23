// Package llm provides operational-style prompt templates for zen-brain 0.1 pattern.
// ZB-024: Operational planner/worker prompts - bounded work, blueprint outputs, no code examples in planning.
package llm

// InitializeOperationalManager creates a prompt manager with operational templates only.
// Used by DefaultTemplates() to register operational prompts alongside default ones.
func InitializeOperationalManager() *PromptManager {

import (
	"fmt"
	"strings"
	"text/template"
)

// OperationalTemplates returns zen-brain 0.1 style prompt templates.
// These prompts follow the operational pattern: bounded work, clear requirements, structured outputs.
func OperationalTemplates() []*PromptTemplate {
	return []*PromptTemplate{
		// ============================================================
		// PLANNER PROMPT - Operational Style
		// ============================================================
		{
			Name:         "planner_operational",
			Role:         RolePlanner,
			SystemPrompt: `You are an operational planner for zen-brain. Your job is to identify the NEXT BOUNDED work item, check for duplicates, and produce a clear blueprint.

CRITICAL RULES:
1. Pick ONE bounded work item - do not take on multiple items at once
2. Check for existing tickets/content first - AVOID DUPLICATES
3. Produce a BLUEPRINT, not vague fluff
4. Structure output with: Requirements, Expected Behavior, Verification
5. DO NOT include code examples in the planner output
6. Create real Jira tickets with labels and acceptance criteria
7. Keep work bounded and system-improving
8. Avoid self-referential artifacts

When checking for duplicates:
- Search for similar tickets by title keywords
- Check documentation for overlapping content
- Identify if this is a new issue or an enhancement to existing work

Your output must be actionable by a worker with clear success criteria.`,
			UserTemplate: `Plan the NEXT bounded work item for zen-brain:

WORK CONTEXT:
{{.work_context}}

AVAILABLE TEMPLATES:
{{.available_templates}}

TASK INSTRUCTIONS:
1. Identify ONE bounded work item from the context above
2. Check for duplicates (existing tickets, docs, related work)
3. Select the appropriate template (success-docs, success-runbook, failure-timeout)
4. Define file/path scope explicitly
5. Set clear acceptance criteria
6. Define verification steps
7. Determine if this is success-path or controlled-failure-path
8. Create real Jira ticket details (summary, labels, acceptance criteria)

OUTPUT FORMAT (MUST follow exactly):

WORK_ITEM: <clear, concise title>
TEMPLATE: <template name>
PATH_SCOPE: <exact files/directories to work on>

REQUIREMENTS:
- <requirement 1>
- <requirement 2>
- ...

EXPECTED_BEHAVIOR:
- <behavior 1>
- <behavior 2>
- ...

VERIFICATION:
- <step 1 to verify success>
- <step 2 to verify success>
- ...

JIRA_TICKET:
Summary: <Jira ticket title>
Labels: <label1,label2,label3>
Acceptance_Criteria: <criterion 1; criterion 2; ...>

IS_SUCCESS_PATH: <true|false>

NOTES: <any important context, risks, or dependencies>`,
			Temperature: 0.2,
			MaxTokens:   3000,
			Version:     "0.1-operational",
			Variables:   []string{"work_context", "available_templates"},
		},

		// ============================================================
		// WORKER PROMPT - Operational Style
		// ============================================================
		{
			Name:         "worker_operational",
			Role:         RoleImplementer,
			SystemPrompt: `You are a bounded task executor for zen-brain. Your job is to execute the EXACT bounded task, respect acceptance criteria, and report results.

CRITICAL RULES:
1. Execute ONLY the bounded task specified - do not expand scope
2. Respect ALL acceptance criteria - success depends on meeting them
3. Create real proof-of-work artifacts ONLY when relevant to the task
4. Avoid junk files - every file must serve a purpose
5. Report EXACT files changed - no vague "some files were modified"
6. Report verification result - did acceptance criteria pass?
7. Update Jira/feedback through normal flow
8. If using controlled-failure path, expect short timeout and handle gracefully

Proof-of-work artifacts:
- Documentation updates: Show before/after or clear summary
- Runbook creation: Include final runbook with test results
- Code changes: Show diff and test output
- Failure testing: Show timeout occurred, error handling worked

Your output must be verifiable and actionable.`,
			UserTemplate: `Execute this bounded task:

WORK_ITEM: {{.work_item}}
TEMPLATE: {{.template}}
PATH_SCOPE: {{.path_scope}}

REQUIREMENTS:
{{.requirements}}

EXPECTED_BEHAVIOR:
{{.expected_behavior}}

VERIFICATION_STEPS:
{{.verification}}

ACCEPTANCE_CRITERIA:
{{.acceptance_criteria}}

IS_SUCCESS_PATH: {{.is_success_path}}

EXECUTION INSTRUCTIONS:
1. Work ONLY within the specified path scope
2. Execute the bounded task step by step
3. Create/update files as needed (do not create junk)
4. Test your changes against the verification steps
5. Ensure all acceptance criteria are met

OUTPUT FORMAT (MUST follow exactly):

EXECUTION_STATUS: <success|failed|timeout>
FILES_CHANGED:
- <file1> (<action: created|updated|deleted>)
- <file2> (<action>)
- ...

VERIFICATION_RESULT: <PASS|FAIL>
VERIFICATION_NOTES:
- <verification step 1 result>
- <verification step 2 result>
- ...

ACCEPTANCE_CRITERIA_STATUS: <all met|partial|none>
ACCEPTANCE_NOTES:
- <criteria 1 status>
- <criteria 2 status>
- ...

PROOF_OF_WORK:
{{.proof_instructions}}

JIRA_UPDATE:
Status: <Done|Failed|In Progress|Blocked>
Comment: <clear status update for Jira>

NOTES: <any issues, blockers, or dependencies encountered>`,
			Temperature: 0.1,
			MaxTokens:   4000,
			Version:     "0.1-operational",
			Variables:   []string{"work_item", "template", "path_scope", "requirements", "expected_behavior", "verification", "acceptance_criteria", "is_success_path", "proof_instructions"},
		},
	}
}

// InitializeOperationalManager creates a prompt manager with operational templates.
func InitializeOperationalManager() *PromptManager {
	manager := NewPromptManager()
	templates := OperationalTemplates()

	for _, template := range templates {
		if err := manager.RegisterTemplate(template); err != nil {
			continue
		}
	}

	return manager
}

// RenderPlannerContext creates work context string for planner prompt.
func RenderPlannerContext(availableTemplates []string, backlog []string, activeSession string) string {
	var sb strings.Builder
	sb.WriteString("=== ACTIVE SESSION ===\n")
	if activeSession != "" {
		sb.WriteString(fmt.Sprintf("Session ID: %s\n", activeSession))
	} else {
		sb.WriteString("No active session\n")
	}
	sb.WriteString("\n")

	sb.WriteString("=== BACKLOG / PENDING WORK ===\n")
	if len(backlog) > 0 {
		for _, item := range backlog {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
	} else {
		sb.WriteString("No pending items\n")
	}
	sb.WriteString("\n")

	sb.WriteString("=== AVAILABLE TEMPLATES ===\n")
	if len(availableTemplates) > 0 {
		for _, tmpl := range availableTemplates {
			sb.WriteString(fmt.Sprintf("- %s\n", tmpl))
		}
	} else {
		sb.WriteString("No templates configured\n")
	}
	sb.WriteString("\n")

	sb.WriteString("=== OPERATIONAL CONTEXT ===\n")
	sb.WriteString("System: zen-brain (v1.0)\n")
	sb.WriteString("Target: bounded system improvement tasks\n")
	sb.WriteString("Profiles:\n")
	sb.WriteString("  - normal-45m: Success path (documentation, runbooks)\n")
	sb.WriteString("  - short-test: Controlled failure path (timeout testing)\n")
	sb.WriteString("\n")

	return sb.String()
}

// RenderWorkerProofInstructions creates proof-of-work instructions for worker prompt.
func RenderWorkerProofInstructions(workType string, pathScope string) string {
	var sb strings.Builder
	sb.WriteString("Based on work type: ")
	sb.WriteString(workType)
	sb.WriteString("\n\n")

	if strings.Contains(workType, "documentation") || strings.Contains(workType, "docs") {
		sb.WriteString("Proof-of-work requirements:\n")
		sb.WriteString("- Show documentation file(s) updated/created\n")
		sb.WriteString("- Include summary of changes\n")
		sb.WriteString("- Verify markdown renders correctly\n")
		sb.WriteString("- Confirm no TODO/placeholder text remains\n")
	} else if strings.Contains(workType, "operations") || strings.Contains(workType, "runbook") {
		sb.WriteString("Proof-of-work requirements:\n")
		sb.WriteString("- Show runbook file(s) created/updated\n")
		sb.WriteString("- Test each procedure step\n")
		sb.WriteString("- Document test results\n")
		sb.WriteString("- Verify rollback procedures work\n")
	} else if strings.Contains(workType, "testing") || strings.Contains(workType, "failure") {
		sb.WriteString("Proof-of-work requirements:\n")
		sb.WriteString("- Show timeout occurred at expected time\n")
		sb.WriteString("- Verify error handling worked\n")
		sb.WriteString("- Confirm no panic or crash\n")
		sb.WriteString("- Check session state updated correctly\n")
	} else {
		sb.WriteString("Proof-of-work requirements:\n")
		sb.WriteString("- List all files changed\n")
		sb.WriteString("- Show diff for code changes\n")
		sb.WriteString("- Include test output\n")
		sb.WriteString("- Verify acceptance criteria met\n")
	}

	sb.WriteString("\n")
	sb.WriteString("Proof-of-work artifacts will be attached to Jira ticket.\n")

	return sb.String()
}
