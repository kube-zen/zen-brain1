package promptbuilder

import (
	"bytes"
	"fmt"
	"text/template"
)

// TaskPacket represents a structured prompt for qwen3.5:0.8b execution.
// Based on zen-brain 0.1 task-templates structure.
type TaskPacket struct {
	// Task Identity
	JiraKey    string
	Summary    string
	WorkType   string
	TimeoutSec int

	// Scope
	AllowedPaths    []string
	ForbiddenPaths  []string
	ContextFiles    []string // Files to read first
	TargetFiles     []string // Files to modify

	// Architecture Constraints
	ExistingTypes    []string // Types/interfaces to use
	ExistingPackages []string // Packages to import
	WiringPoints     []string // Where integration belongs
	DoNotModify      []string // Must not be touched

	// Phased Execution
	Phases []Phase

	// Verification
	CompileCmd    string
	TestCmd       string
	VerifyCmds    []string
	StaticChecks  []string

	// Output Contract
	OutputFormat    string
	ReportFiles     bool
	ReportBlockers  bool
	NoCodeExamples  bool // For planning prompts
	NoFakeArtifacts bool
}

// Phase represents a single execution phase with Requirements/Behavior/Verification.
type Phase struct {
	Name             string
	Requirements     []string
	ExpectedBehavior []string
	Verification     []string
}

// BuildPrompt constructs a structured prompt packet from the TaskPacket.
// This is the 0.1-style structured prompt that qwen3.5:0.8b needs.
func BuildPrompt(packet TaskPacket) (string, error) {
	tmpl := `You are executing Jira issue {{.JiraKey}}.

=== TASK IDENTITY ===
Goal: {{.Summary}}
Work type: {{.WorkType}}
Timeout: {{.TimeoutSec}} seconds

=== SCOPE ===
Allowed paths:
{{range .AllowedPaths}}- {{.}}
{{end}}

{{if .ForbiddenPaths}}Forbidden paths:
{{range .ForbiddenPaths}}- DO NOT touch: {{.}}
{{end}}
{{end}}

Read these files first:
{{range .ContextFiles}}- {{.}}
{{end}}

=== ARCHITECTURE CONSTRAINTS ===
{{if .ExistingTypes}}Use these existing types/interfaces:
{{range .ExistingTypes}}- {{.}}
{{end}}
{{end}}

{{if .ExistingPackages}}Use these existing packages:
{{range .ExistingPackages}}- {{.}}
{{end}}
{{end}}

{{if .WiringPoints}}Integration points:
{{range .WiringPoints}}- {{.}}
{{end}}
{{end}}

{{if .DoNotModify}}CRITICAL - Do NOT modify:
{{range .DoNotModify}}- {{.}}
{{end}}
{{end}}

=== PHASED EXECUTION ===
{{range $i, $phase := .Phases}}
Phase {{$i}}: {{$phase.Name}}

Requirements:
{{range $phase.Requirements}}- {{.}}
{{end}}

Expected Behavior:
{{range $phase.ExpectedBehavior}}- {{.}}
{{end}}

Verification:
{{range $phase.Verification}}- {{.}}
{{end}}

{{end}}

=== VERIFICATION COMMANDS ===
{{if .CompileCmd}}Compile: {{.CompileCmd}}{{end}}
{{if .TestCmd}}Tests: {{.TestCmd}}{{end}}
{{range .VerifyCmds}}- {{.}}
{{end}}

{{if .StaticChecks}}Static checks:
{{range .StaticChecks}}- {{.}}
{{end}}
{{end}}

=== OUTPUT CONTRACT ===
Format: {{.OutputFormat}}

{{if .NoCodeExamples}}CRITICAL: Do NOT include code examples.{{end}}
{{if .NoFakeArtifacts}}CRITICAL: Do NOT create fake artifacts or self-referential code.{{end}}
{{if .ReportFiles}}Report exact files changed.{{end}}
{{if .ReportBlockers}}Report blockers honestly - do not claim success if verification fails.{{end}}

FORBIDDEN:
- Do NOT invent new packages/imports not in existing packages list
- Do NOT create fake modules or placeholder code
- Do NOT modify files outside allowed paths
- Do NOT claim success if compile/test commands fail
- If a required type is missing, report blocker instead of hallucinating

OUTPUT:
Files changed:
- <list exact files>

Verification run:
- <exact output of verification commands>

Result: SUCCESS | FAILURE

Blockers (if any):
- <list what blocked you>
`

	t, err := template.New("taskPacket").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, packet); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// RescueTaskTemplate returns a TaskPacket for rescue tasks from 0.1.
// This template is specifically for porting code from zen-brain 0.1 to 1.0.
func RescueTaskTemplate(jiraKey, summary string, source01, target10 string, allowedPaths []string) TaskPacket {
	return TaskPacket{
		JiraKey:    jiraKey,
		Summary:    summary,
		WorkType:   "implementation",
		TimeoutSec: 2700,

		AllowedPaths: allowedPaths,
		ContextFiles: []string{
			source01,      // Read 0.1 source first
			target10,      // Read 1.0 target structure
			"internal/llm/provider.go",    // Existing interfaces
			"pkg/llm/types.go",            // Existing types
		},

		ExistingTypes: []string{
			"github.com/kube-zen/zen-brain1/pkg/llm.Provider",
			"github.com/kube-zen/zen-brain1/pkg/llm.ChatRequest",
			"github.com/kube-zen/zen-brain1/pkg/llm.ChatResponse",
		},

		ExistingPackages: []string{
			"github.com/kube-zen/zen-brain1/internal/llm",
			"github.com/kube-zen/zen-brain1/pkg/llm",
			"github.com/kube-zen/zen-brain1/internal/mlq",
		},

		NoCodeExamples:  false, // Execution can have code
		NoFakeArtifacts: true,
		ReportFiles:     true,
		ReportBlockers:  true,
		OutputFormat:    "structured",

		Phases: []Phase{
			{
				Name: "Analyze 0.1 Source",
				Requirements: []string{
					"Read " + source01,
					"Identify core abstraction (types, interfaces, functions)",
					"Identify dependencies on other 0.1 packages",
				},
				ExpectedBehavior: []string{
					"List the key types/interfaces to port",
					"List the functions to implement",
					"Note any dependencies that need adaptation",
				},
				Verification: []string{
					"Confirm source file exists and was read",
					"List extracted types/functions",
				},
			},
			{
				Name: "Analyze 1.0 Target",
				Requirements: []string{
					"Read " + target10,
					"Identify where to integrate (wiring points)",
					"Identify existing types to reuse",
				},
				ExpectedBehavior: []string{
					"List integration points in 1.0",
					"List existing types that match 0.1 types",
					"Identify gaps that need new types",
				},
				Verification: []string{
					"Confirm target file exists and was read",
					"List integration points",
				},
			},
			{
				Name: "Port with Adaptation",
				Requirements: []string{
					"Port core abstraction shape, not blind copy",
					"Adapt to 1.0 existing types where possible",
					"Create new types only if needed",
					"Use only allowed paths",
				},
				ExpectedBehavior: []string{
					"Code compiles against 1.0 architecture",
					"No fake imports or invented packages",
					"Reuses existing types from pkg/llm",
					"Minimal changes, bounded scope",
				},
				Verification: []string{
					"Run: go build ./...",
					"Confirm no compilation errors",
					"List exact files modified",
				},
			},
			{
				Name: "Report Results",
				Requirements: []string{
					"List exact files changed",
					"Report compilation result",
					"Report test result (if tests exist)",
					"Report any blockers honestly",
				},
				ExpectedBehavior: []string{
					"Structured output with Files changed / Verification / Result / Blockers",
					"Do not claim success if compile fails",
					"Do not claim success if tests fail",
				},
				Verification: []string{
					"Confirm all sections present in output",
					"Confirm Result is SUCCESS or FAILURE",
				},
			},
		},

		CompileCmd:   "go build ./...",
		TestCmd:      "go test ./...",
		VerifyCmds:   []string{"grep -r 'type Provider interface' pkg/llm/"},
		StaticChecks: []string{"No fake imports", "No invented packages", "Stays in allowed paths"},
	}
}
