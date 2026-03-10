// Package llm provides prompt management for LLM interactions.
package llm

import (
	"fmt"
	"strings"
	"text/template"
)

// PromptRole defines the role for which a prompt is designed.
type PromptRole string

const (
	// RolePlanner is for strategic planning and task breakdown.
	RolePlanner PromptRole = "planner"
	// RoleImplementer is for code implementation and execution.
	RoleImplementer PromptRole = "implementer"
	// RoleReviewer is for code review and quality assessment.
	RoleReviewer PromptRole = "reviewer"
	// RoleOps is for operations and deployment tasks.
	RoleOps PromptRole = "ops"
	// RoleAnalyzer is for work item analysis and classification.
	RoleAnalyzer PromptRole = "analyzer"
)

// PromptTemplate defines a reusable prompt template.
type PromptTemplate struct {
	// Name identifies the prompt (e.g., "classification", "requirements")
	Name string `yaml:"name" json:"name"`
	
	// Role specifies which role this prompt is for.
	Role PromptRole `yaml:"role" json:"role"`
	
	// SystemPrompt is the system message defining the AI's role.
	SystemPrompt string `yaml:"system_prompt" json:"system_prompt"`
	
	// UserTemplate is the user message template with {{.variable}} placeholders.
	UserTemplate string `yaml:"user_template" json:"user_template"`
	
	// Temperature recommendation for this prompt.
	Temperature float64 `yaml:"temperature" json:"temperature"`
	
	// MaxTokens recommendation for this prompt.
	MaxTokens int `yaml:"max_tokens" json:"max_tokens"`
	
	// Version for tracking prompt iterations.
	Version string `yaml:"version" json:"version"`
	
	// Variables lists expected template variables.
	Variables []string `yaml:"variables" json:"variables"`
	
	// compiled template cache
	compiled *template.Template
}

// PromptManager manages prompt templates and rendering.
type PromptManager struct {
	templates map[string]*PromptTemplate
}

// NewPromptManager creates a new prompt manager.
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*PromptTemplate),
	}
}

// RegisterTemplate registers a prompt template.
func (pm *PromptManager) RegisterTemplate(template *PromptTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("prompt template name cannot be empty")
	}
	
	// Compile the template
	compiled, err := template.compile()
	if err != nil {
		return fmt.Errorf("failed to compile template %s: %w", template.Name, err)
	}
	
	template.compiled = compiled
	pm.templates[template.Name] = template
	
	return nil
}

// GetTemplate returns a prompt template by name.
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	template, exists := pm.templates[name]
	if !exists {
		return nil, fmt.Errorf("prompt template not found: %s", name)
	}
	return template, nil
}

// GetTemplatesByRole returns all templates for a specific role.
func (pm *PromptManager) GetTemplatesByRole(role PromptRole) []*PromptTemplate {
	var result []*PromptTemplate
	for _, template := range pm.templates {
		if template.Role == role {
			result = append(result, template)
		}
	}
	return result
}

// Render renders a prompt template with the given variables.
func (pt *PromptTemplate) Render(variables map[string]string) (systemMsg, userMsg string, err error) {
	if pt.compiled == nil {
		if _, err := pt.compile(); err != nil {
			return "", "", err
		}
	}
	
	// Render user template
	var userBuilder strings.Builder
	if err := pt.compiled.Execute(&userBuilder, variables); err != nil {
		return "", "", fmt.Errorf("failed to render template: %w", err)
	}
	
	return pt.SystemPrompt, userBuilder.String(), nil
}

// compile compiles the template for rendering.
func (pt *PromptTemplate) compile() (*template.Template, error) {
	// Create template with delimiters that won't conflict with other systems
	tmpl, err := template.New(pt.Name).Delims("{{.", "}}").Parse(pt.UserTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	return tmpl, nil
}

// DefaultTemplates returns the default prompt templates.
func DefaultTemplates() []*PromptTemplate {
	return []*PromptTemplate{
		{
			Name:         "work_item_analysis",
			Role:         RoleAnalyzer,
			SystemPrompt: "You are a technical analyst. Provide structured JSON responses.",
			UserTemplate: `Analyze this work item and provide a structured assessment:

Title: {{.title}}
Summary: {{.summary}}
Type: {{.work_type}}
Priority: {{.priority}}

Provide:
1. Complexity assessment (low/medium/high)
2. Estimated effort (e.g., "1-2 hours", "half day", "1 day")
3. Recommended approach
4. Key risks
5. Dependencies

Format your response as JSON:
{
  "complexity": "...",
  "estimated_effort": "...",
  "recommended_approach": "...",
  "risks": ["..."],
  "dependencies": ["..."]
}`,
			Temperature: 0.1,
			MaxTokens:   1500,
			Version:     "1.0",
			Variables:   []string{"title", "summary", "work_type", "priority"},
		},
		{
			Name:         "classification",
			Role:         RoleAnalyzer,
			SystemPrompt: "You are a software engineering classifier. Be precise and consistent in your classifications.",
			UserTemplate: `Analyze the following work item and classify it:

Title: {{.title}}
Description: {{.description}}
Source: {{.source_system}} (Issue: {{.issue_key}})

Please classify this work item by answering the following questions:

1. What is the primary work type? (research, design, implementation, debug, refactor, documentation, analysis, operations, security, testing)
2. What is the work domain? (office, factory, sdk, policy, memory, observability, infrastructure, integration, core)
3. What is the appropriate priority? (critical, high, medium, low, background)
4. What knowledge base scopes are relevant? (comma-separated list)
5. Confidence in classification (0.0-1.0)

Format your response as:
WorkType: <type>
WorkDomain: <domain>
Priority: <priority>
KBScopes: <scope1, scope2, ...>
Confidence: <confidence>

Example:
WorkType: implementation
WorkDomain: core
Priority: medium
KBScopes: api-gateway, rate-limiting
Confidence: 0.85`,
			Temperature: 0.1,
			MaxTokens:   500,
			Version:     "1.0",
			Variables:   []string{"title", "description", "source_system", "issue_key"},
		},
		{
			Name:         "requirements",
			Role:         RoleAnalyzer,
			SystemPrompt: "You are a requirements analyst. Extract clear, actionable requirements from work descriptions.",
			UserTemplate: `Extract requirements and constraints from this work item:

Title: {{.title}}
Description: {{.description}}

Please extract:
1. Clear objective (what needs to be done)
2. Acceptance criteria (list of conditions for success)
3. Constraints (technical, time, resource, or other limitations)
4. Dependencies (other work items or systems this depends on)

Format your response as:
Objective: <clear objective statement>
AcceptanceCriteria: <criterion 1>; <criterion 2>; ...
Constraints: <constraint 1>; <constraint 2>; ...
Dependencies: <dependency 1>; <dependency 2>; ...`,
			Temperature: 0.1,
			MaxTokens:   1000,
			Version:     "1.0",
			Variables:   []string{"title", "description"},
		},
		{
			Name:         "planner_complex",
			Role:         RolePlanner,
			SystemPrompt: "You are a strategic planning agent. Break down complex problems into executable steps with clear success criteria.",
			UserTemplate: `Plan the execution for this task:

Title: {{.title}}
Objective: {{.objective}}
Work Type: {{.work_type}}
Priority: {{.priority}}

Create a detailed execution plan with:
1. Phased approach (analysis, design, implementation, review)
2. Estimated time per phase
3. Key risks and mitigation strategies
4. Success criteria for each phase
5. Required tools and resources

Format your response as a structured plan with clear phases and deliverables.`,
			Temperature: 0.3,
			MaxTokens:   2000,
			Version:     "1.0",
			Variables:   []string{"title", "objective", "work_type", "priority"},
		},
		{
			Name:         "implementer_code",
			Role:         RoleImplementer,
			SystemPrompt: "You are a software engineer. Write correct, tested code following best practices. Include appropriate error handling and documentation.",
			UserTemplate: `Implement the following task:

Title: {{.title}}
Objective: {{.objective}}
Language: {{.language}}
Requirements: {{.requirements}}

Write complete, production-ready code that:
1. Solves the stated problem
2. Includes appropriate error handling
3. Has clear documentation/comments
4. Follows language-specific best practices
5. Is testable and maintainable

Provide the complete implementation with any necessary imports/dependencies.`,
			Temperature: 0.1,
			MaxTokens:   4000,
			Version:     "1.0",
			Variables:   []string{"title", "objective", "language", "requirements"},
		},
		{
			Name:         "reviewer_code",
			Role:         RoleReviewer,
			SystemPrompt: "You are a code reviewer. Identify bugs, style issues, security vulnerabilities, and opportunities for improvement. Be constructive and specific.",
			UserTemplate: `Review this code implementation:

Task: {{.title}}
Objective: {{.objective}}
Code:
{{.code}}

Provide a thorough code review covering:
1. Functional correctness (does it solve the problem?)
2. Code quality (style, readability, maintainability)
3. Security considerations
4. Performance implications
5. Test coverage adequacy
6. Documentation completeness

Format as a structured review with specific, actionable feedback.`,
			Temperature: 0.3,
			MaxTokens:   2000,
			Version:     "1.0",
			Variables:   []string{"title", "objective", "code"},
		},
		{
			Name:         "ops_deployment",
			Role:         RoleOps,
			SystemPrompt: "You are an operations specialist. Focus on safety, reliability, and rollback considerations. Always consider approval gates for risky actions.",
			UserTemplate: `Plan the deployment/operations for:

Task: {{.title}}
Description: {{.description}}
Environment: {{.environment}}
Risk Level: {{.risk_level}}

Create a safe deployment plan with:
1. Pre-deployment checks and validations
2. Deployment steps with rollback procedures
3. Monitoring and alerting requirements
4. Post-deployment verification
5. Risk mitigation strategies

Highlight any actions that require explicit approval.`,
			Temperature: 0.1,
			MaxTokens:   1500,
			Version:     "1.0",
			Variables:   []string{"title", "description", "environment", "risk_level"},
		},
		{
			Name:         "breakdown",
			Role:         RoleAnalyzer,
			SystemPrompt: "You are a project planner. Break down work into logical, executable subtasks.",
			UserTemplate: `Break down this work item into subtasks:

Title: {{.title}}
Description: {{.description}}
Work Type: {{.work_type}}

Break this down into 2-5 logical subtasks that could be executed independently.
For each subtask, provide a brief description.

Format your response as:
Subtasks:
1. <subtask 1 description>
2. <subtask 2 description>
...`,
			Temperature: 0.1,
			MaxTokens:   800,
			Version:     "1.0",
			Variables:   []string{"title", "description", "work_type"},
		},
	}
}

// InitializeDefaultManager creates and registers all default templates.
func InitializeDefaultManager() *PromptManager {
	manager := NewPromptManager()
	templates := DefaultTemplates()
	
	for _, template := range templates {
		if err := manager.RegisterTemplate(template); err != nil {
			// Log error but continue (shouldn't happen with default templates)
			continue
		}
	}
	
	return manager
}