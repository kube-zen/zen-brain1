package factory

import (
	"fmt"
	"strings"
)

// WorkTypeTemplate defines execution steps for specific work types.
// This provides standardized execution plans based on work type and domain.
type WorkTypeTemplate struct {
	WorkType    string
	WorkDomain  string
	Steps       []ExecutionStepTemplate
	Description string
}

// ExecutionStepTemplate defines a single execution step with template variables.
type ExecutionStepTemplate struct {
	Name        string
	Description string
	Command     string            // Template with {{.var}} placeholders
	Variables   map[string]string // Key-value pairs for template expansion
	Timeout     int               // seconds
	MaxRetries  int
}

// WorkTypeTemplateRegistry provides access to all templates.
type WorkTypeTemplateRegistry struct {
	templates map[string]map[string]*WorkTypeTemplate // [workType][workDomain] -> template
}

// NewWorkTypeTemplateRegistry creates a new template registry.
func NewWorkTypeTemplateRegistry() *WorkTypeTemplateRegistry {
	registry := &WorkTypeTemplateRegistry{
		templates: make(map[string]map[string]*WorkTypeTemplate),
	}

	// Register all templates
	registry.registerBugFixTemplates()
	registry.registerFeatureTemplates()
	registry.registerRefactorTemplates()
	registry.registerDocumentationTemplates()
	registry.registerTestTemplates()
	registry.registerDebugTemplates()
	registry.registerUsefulTemplates() // Register templates that do real work
	registry.registerDefaultTemplate()

	return registry
}

// HasTemplate returns true if a template exists for the given work type and domain.
func (r *WorkTypeTemplateRegistry) HasTemplate(workType, workDomain string) bool {
	t := r.GetTemplateOrNil(workType, workDomain)
	return t != nil
}

// GetTemplateOrNil returns the template for work type and domain, or nil if none.
func (r *WorkTypeTemplateRegistry) GetTemplateOrNil(workType, workDomain string) *WorkTypeTemplate {
	domainMap, exists := r.templates[workType]
	if !exists {
		if defaultMap, ok := r.templates["default"]; ok && len(defaultMap) > 0 {
			for _, t := range defaultMap {
				return t
			}
		}
		return nil
	}
	if workDomain != "" {
		if template, ok := domainMap[workDomain]; ok {
			return template
		}
	}
	if template, ok := domainMap[""]; ok {
		return template
	}
	for _, t := range domainMap {
		return t
	}
	return nil
}

// GetTemplate returns best matching template for work type and domain.
// If domain-specific template exists, returns that; otherwise returns default for work type.
func (r *WorkTypeTemplateRegistry) GetTemplate(workType, workDomain string) (*WorkTypeTemplate, error) {
	t := r.GetTemplateOrNil(workType, workDomain)
	if t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("no template for work type: %s", workType)
}

// ExpandVariables replaces {{.var}} placeholders with values.
// Simple template engine that replaces variables in command strings.
func (r *WorkTypeTemplateRegistry) ExpandVariables(command string, variables map[string]string) string {
	result := command
	for key, value := range variables {
		placeholder := "{{." + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// registerTemplate adds a template to registry.
func (r *WorkTypeTemplateRegistry) registerTemplate(template *WorkTypeTemplate) {
	if r.templates[template.WorkType] == nil {
		r.templates[template.WorkType] = make(map[string]*WorkTypeTemplate)
	}
	r.templates[template.WorkType][template.WorkDomain] = template
}

// registerDefaultTemplate registers a fallback template for unknown work types.
func (r *WorkTypeTemplateRegistry) registerDefaultTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "default",
		WorkDomain:  "",
		Description: "Default execution plan for unknown work types",
		Steps: []ExecutionStepTemplate{
			{
				Name:        "Generic execution",
				Description: "Execute work using available tools and context",
				Command:     "echo 'Executing: {{.title}}' && echo 'Objective: {{.objective}}' && echo 'Work type: {{.work_type}}' && echo 'Domain: {{.work_domain}}'",
				Variables:   map[string]string{},
				Timeout:     300,
				MaxRetries:  2,
			},
		},
	}
	r.registerTemplate(template)
}
