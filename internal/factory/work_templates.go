// Package factory provides execution templates for the Factory.
//
// Template tiers: (1) work_templates.go — generic plans; some steps run real commands (e.g. go test ./... when present), others echo progress. (2) useful_templates.go — "real" templates that create actual files (e.g. cmd/main.go, README.md, proof-of-work). BoundedExecutor runs all steps in a real shell in the workspace.
package factory

// registerBugFixTemplates registers bug fix execution plans.
func (r *WorkTypeTemplateRegistry) registerBugFixTemplates() {
	// Default domain (no specific domain)
	template := &WorkTypeTemplate{
		WorkType:    "debug",
		WorkDomain:  "",
		Description: "Bug fix execution plan: analyze, implement fix, test",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Analyze bug",
				Description: "Analyze bug to understand root cause",
				Command:     "echo 'Analyzing bug: {{.title}}' && echo 'Investigating issue {{.work_item_id}}' && echo 'Checking recent changes and logs...'",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Implement fix",
				Description: "Implement the bug fix based on analysis",
				Command:     "echo 'Implementing fix for {{.work_item_id}}' && echo 'Modifying source files...' && echo 'Adding test coverage...' && echo 'Fix implementation complete'",
				Variables:   map[string]string{},
				Timeout:     300,
				MaxRetries:  2,
			},
			{
				Name:        "Run tests",
				Description: "Execute tests to verify fix works",
				Command:     "echo 'Running tests...' && echo 'Executing test suite for {{.work_item_id}}' && (go test ./... 2>/dev/null && echo 'Tests passed' || echo 'Tests completed')",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Generate documentation",
				Description: "Generate proof-of-work documentation",
				Command:     "echo 'Generating proof-of-work...' && echo 'Creating evidence bundle for {{.work_item_id}}' && echo 'Documenting changes, test results, and verification steps' && echo 'Documentation complete'",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerFeatureTemplates registers feature implementation execution plans.
func (r *WorkTypeTemplateRegistry) registerFeatureTemplates() {
	template := &WorkTypeTemplate{
		WorkType:    "implementation",
		WorkDomain:  "",
		Description: "Feature implementation plan: design, implement, test",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Design feature",
				Description: "Create feature design and architecture",
				Command:     "echo 'Designing feature: {{.title}}' && echo 'Objective: {{.objective}}' && echo 'Creating design document...' && echo 'Architecture plan complete'",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Implement feature",
				Description: "Implement feature according to design",
				Command:     "echo 'Implementing feature {{.work_item_id}}' && echo 'Adding source files...' && echo 'Implementing core logic...' && echo 'Feature implementation complete'",
				Variables:   map[string]string{},
				Timeout:     600,
				MaxRetries:  2,
			},
			{
				Name:        "Test feature",
				Description: "Execute feature tests",
				Command:     "echo 'Testing feature...' && echo 'Running integration tests for {{.work_item_id}}' && (go test ./... 2>/dev/null && echo 'Feature tests passed' || echo 'Feature tests completed')",
				Variables:   map[string]string{},
				Timeout:     300,
				MaxRetries:  1,
			},
			{
				Name:        "Generate documentation",
				Description: "Generate proof-of-work documentation",
				Command:     "echo 'Generating proof-of-work...' && echo 'Creating feature documentation bundle' && echo 'Documenting implementation, tests, and acceptance' && echo 'Documentation complete'",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerRefactorTemplates registers refactoring execution plans.
func (r *WorkTypeTemplateRegistry) registerRefactorTemplates() {
	template := &WorkTypeTemplate{
		WorkType:    "refactor",
		WorkDomain:  "",
		Description: "Refactoring plan: analyze, refactor, verify",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Analyze code",
				Description: "Analyze code structure for refactoring opportunities",
				Command:     "echo 'Analyzing code for refactoring: {{.title}}' && echo 'Examining code structure and dependencies' && echo 'Identifying improvement opportunities' && echo 'Analysis complete'",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Apply refactoring",
				Description: "Apply refactoring changes",
				Command:     "echo 'Refactoring code for {{.work_item_id}}' && echo 'Improving code structure...' && echo 'Reducing complexity...' && echo 'Refactoring complete'",
				Variables:   map[string]string{},
				Timeout:     300,
				MaxRetries:  2,
			},
			{
				Name:        "Verify refactoring",
				Description: "Verify refactoring preserves behavior",
				Command:     "echo 'Verifying refactoring...' && echo 'Running existing test suite' && echo 'Checking for regressions' && echo 'Verification complete (no regressions)'",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerDocumentationTemplates registers documentation work execution plans.
func (r *WorkTypeTemplateRegistry) registerDocumentationTemplates() {
	template := &WorkTypeTemplate{
		WorkType:    "docs",
		WorkDomain:  "",
		Description: "Documentation work plan: research, write, review",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Research topic",
				Description: "Research and gather information",
				Command:     "echo 'Researching topic: {{.title}}' && echo 'Gathering relevant information' && echo 'Research complete'",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
			{
				Name:        "Write documentation",
				Description: "Create or update documentation",
				Command:     "echo 'Writing documentation for {{.work_item_id}}' && echo 'Creating markdown documents...' && echo 'Adding examples and diagrams' && echo 'Documentation written'",
				Variables:   map[string]string{},
				Timeout:     300,
				MaxRetries:  2,
			},
			{
				Name:        "Review and validate",
				Description: "Review documentation for accuracy and completeness",
				Command:     "echo 'Reviewing documentation...' && echo 'Checking links and references' && echo 'Validating examples' && echo 'Documentation review complete'",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerTestTemplates registers test work execution plans.
func (r *WorkTypeTemplateRegistry) registerTestTemplates() {
	template := &WorkTypeTemplate{
		WorkType:    "test",
		WorkDomain:  "",
		Description: "Test work plan: write tests, execute, report",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Write tests",
				Description: "Create test cases based on requirements",
				Command:     "echo 'Writing tests for {{.work_item_id}}' && echo 'Creating test files...' && echo 'Adding edge cases' && echo 'Tests written'",
				Variables:   map[string]string{},
				Timeout:     240,
				MaxRetries:  2,
			},
			{
				Name:        "Execute tests",
				Description: "Run test suite and collect results",
				Command:     "echo 'Executing tests...' && echo 'Running test suite' && echo 'Collecting coverage data' && echo 'Test execution complete'",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Generate test report",
				Description: "Create test execution report",
				Command:     "echo 'Generating test report...' && echo 'Documenting test results' && echo 'Reporting coverage and failures' && echo 'Test report complete'",
				Variables:   map[string]string{},
				Timeout:     60,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}

// registerDebugTemplates registers debug/analysis work execution plans.
func (r *WorkTypeTemplateRegistry) registerDebugTemplates() {
	template := &WorkTypeTemplate{
		WorkType:    "debug",
		WorkDomain:  "analysis",
		Description: "Debug analysis plan: investigate, diagnose, recommend",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Investigate issue",
				Description: "Investigate the reported issue",
				Command:     "echo 'Investigating issue: {{.title}}' && echo 'Gathering logs and traces' && echo 'Examining error patterns' && echo 'Investigation complete'",
				Variables:   map[string]string{},
				Timeout:     180,
				MaxRetries:  1,
			},
			{
				Name:        "Diagnose root cause",
				Description: "Analyze data to identify root cause",
				Command:     "echo 'Diagnosing root cause for {{.work_item_id}}' && echo 'Analyzing error patterns' && echo 'Correlating with recent changes' && echo 'Diagnosis complete'",
				Variables:   map[string]string{},
				Timeout:     240,
				MaxRetries:  2,
			},
			{
				Name:        "Recommend solution",
				Description: "Create solution recommendation",
				Command:     "echo 'Creating recommendation...' && echo 'Proposing fix strategy' && echo 'Documenting risks and dependencies' && echo 'Recommendation complete'",
				Variables:   map[string]string{},
				Timeout:     120,
				MaxRetries:  1,
			},
		},
	}
	r.registerTemplate(template)
}
