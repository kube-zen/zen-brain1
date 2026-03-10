package factory

import (
	"fmt"
)

// TemplateManager manages work type templates and template variable expansion.
type TemplateManager struct {
	registry *WorkTypeTemplateRegistry
}

// NewTemplateManager creates a new template manager.
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		registry: NewWorkTypeTemplateRegistry(),
	}
}

// GetTemplate returns template for a given work type and domain.
func (tm *TemplateManager) GetTemplate(workType, workDomain string) (*WorkTypeTemplate, error) {
	return tm.registry.GetTemplate(workType, workDomain)
}

// HasTemplate returns true if a template exists for the given work type and domain.
func (tm *TemplateManager) HasTemplate(workType, workDomain string) bool {
	return tm.registry.HasTemplate(workType, workDomain)
}

// GetTemplateOrNil returns the template for work type and domain, or nil if none.
func (tm *TemplateManager) GetTemplateOrNil(workType, workDomain string) *WorkTypeTemplate {
	return tm.registry.GetTemplateOrNil(workType, workDomain)
}

// ExpandTemplateVariables expands template variables using task spec values.
func (tm *TemplateManager) ExpandTemplateVariables(template *WorkTypeTemplate, spec *FactoryTaskSpec) []*ExecutionStep {
	steps := make([]*ExecutionStep, 0, len(template.Steps))

	for i, stepTemplate := range template.Steps {
		// Build variables map
		variables := tm.buildVariableMap(spec, stepTemplate)

		// Expand command with variables
		command := tm.registry.ExpandVariables(stepTemplate.Command, variables)

		// Create execution step
		step := &ExecutionStep{
			StepID:         fmt.Sprintf("%s-step-%d", spec.ID, i+1),
			TaskID:         spec.ID,
			Name:           stepTemplate.Name,
			Description:    stepTemplate.Description,
			Command:        command,
			Status:         StepStatusPending,
			TimeoutSeconds: int64(stepTemplate.Timeout),
			MaxRetries:     stepTemplate.MaxRetries,
		}

		steps = append(steps, step)
	}

	return steps
}

// buildVariableMap creates a map of template variables from task spec.
func (tm *TemplateManager) buildVariableMap(spec *FactoryTaskSpec, stepTemplate ExecutionStepTemplate) map[string]string {
	variables := make(map[string]string)

	// Standard variables
	variables["title"] = spec.Title
	variables["summary"] = spec.Objective
	variables["objective"] = spec.Objective
	variables["work_item_id"] = spec.WorkItemID
	variables["session_id"] = spec.SessionID
	variables["task_id"] = spec.ID
	variables["work_type"] = string(spec.WorkType)
	variables["work_domain"] = string(spec.WorkDomain)
	variables["priority"] = string(spec.Priority)

	// Add step-specific variables
	for key, value := range stepTemplate.Variables {
		variables[key] = value
	}

	return variables
}

// HasTemplateForType checks if a template exists for given work type.
func (tm *TemplateManager) HasTemplateForType(workType string) bool {
	_, err := tm.registry.GetTemplate(workType, "")
	return err == nil
}
