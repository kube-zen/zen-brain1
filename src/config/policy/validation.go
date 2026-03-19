// Package policy provides validation for policy configuration
// Extends validation.go to support policy-based configuration

package policy

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateConfig performs comprehensive validation on loaded policy
func ValidateConfig(config *Config) []error {
	var errors []error

	// Run basic validation
	basicErrors := validateConfig(config)
	errors = append(errors, basicErrors...)

	// Validate model availability
	modelErrors := validateModels(config)
	errors = append(errors, modelErrors...)

	// Validate routing consistency
	routingErrors := validateRouting(config)
	errors = append(errors, routingErrors...)

	// Validate prompt templates
	promptErrors := validatePrompts(config)
	errors = append(errors, promptErrors...)

	// Validate chain dependencies
	chainErrors := validateChains(config)
	errors = append(errors, chainErrors...)

	// Validate cost constraints
	costErrors := validateCosts(config)
	errors = append(errors, costErrors...)

	return errors
}

// validateModels checks that all referenced models exist in provider config
func validateModels(config *Config) []error {
	var errors []error

	// Build model map
	modelMap := make(map[string]bool)
	for _, provider := range config.Providers {
		if provider.Enabled {
			for _, model := range provider.Models {
				modelMap[model.Name] = true
			}
		}
	}

	// Check task classes reference valid models
	for className, taskClass := range config.Classes {
		if taskClass != nil && taskClass.AllowedModels != nil {
			for _, modelName := range taskClass.AllowedModels {
				if !modelMap[modelName] {
					errors = append(errors, fmt.Errorf("task class '%s' references unknown model '%s'", className, modelName))
				}
			}
		}
	}

	// Check routing references valid models
	if config.Routing != nil {
		for _, providerRouting := range config.Routing.ProviderRouting {
			provider := config.GetProvider(providerRouting.Provider)
			if provider != nil && provider.Models != nil {
				for _, model := range provider.Models {
					if !modelMap[model.Name] {
						errors = append(errors, fmt.Errorf("provider routing for '%s' references unknown model '%s'", providerRouting.Provider, model.Name))
					}
				}
			}
		}
	}

	return errors
}

// validateRouting checks routing configuration consistency
func validateRouting(config *Config) []error {
	var errors []error

	if config.Routing == nil {
		return errors
	}

	// Validate default strategy
	validStrategies := map[string]bool{
		"fastest":        true,
		"lowest_cost":    true,
		"highest_quality": true,
		"smart":          true,
	}
	if !validStrategies[config.Routing.DefaultStrategy] {
		errors = append(errors, fmt.Errorf("invalid default routing strategy: '%s'", config.Routing.DefaultStrategy))
	}

	// Validate task routing references valid task classes
	taskClassMap := make(map[string]bool)
	for _, task := range config.Tasks {
		taskClassMap[task.Class] = true
	}

	for _, taskRouting := range config.Routing.TaskRouting {
		if !taskClassMap[taskRouting.TaskClass] {
			errors = append(errors, fmt.Errorf("task routing references unknown task class '%s'", taskRouting.TaskClass))
		}
	}

	// Validate fallback chain references valid providers
	providerMap := make(map[string]bool)
	for _, provider := range config.Providers {
		providerMap[provider.Name] = true
	}

	if config.Routing.FallbackChain != nil {
		for _, providerName := range config.Routing.FallbackChain {
			if !providerMap[providerName] {
				errors = append(errors, fmt.Errorf("fallback chain references unknown provider '%s'", providerName))
			}
		}
	}

	// Validate model fallback references valid models
	if config.Routing.ModelFallback != nil {
		modelMap := make(map[string]bool)
		for _, provider := range config.Providers {
			for _, model := range provider.Models {
				modelMap[model.Name] = true
			}
		}
		for _, fallback := range config.Routing.ModelFallback.Rules {
			if !modelMap[fallback.FromModel] {
				errors = append(errors, fmt.Errorf("model fallback references unknown model '%s'", fallback.FromModel))
			}
			if !modelMap[fallback.ToModel] {
				errors = append(errors, fmt.Errorf("model fallback references unknown model '%s'", fallback.ToModel))
			}
		}
	}

	return errors
}

// validatePrompts checks prompt template configuration
func validatePrompts(config *Config) []error {
	var errors []error

	// Check all prompts reference valid roles
	roleMap := make(map[string]bool)
	for _, role := range config.Roles {
		roleMap[role.Name] = true
	}

	for _, prompt := range config.Prompts {
		if !roleMap[prompt.Role] {
			errors = append(errors, fmt.Errorf("prompt '%s' references unknown role '%s'", prompt.Name, prompt.Role))
		}
	}

	// Validate prompt templates are not empty
	for _, prompt := range config.Prompts {
		if prompt.Template == "" {
			errors = append(errors, fmt.Errorf("prompt '%s' has empty template", prompt.Name))
		}
	}

	// Validate task overrides reference valid tasks
	taskMap := make(map[string]bool)
	for _, task := range config.Tasks {
		taskMap[task.Name] = true
	}

	for _, prompt := range config.Prompts {
		for _, override := range prompt.TaskOverrides {
			if !taskMap[override.Task] {
				errors = append(errors, fmt.Errorf("prompt task override references unknown task '%s'", override.Task))
			}
		}
	}

	return errors
}

// validateChains checks chain configuration
func validateChains(config *Config) []error {
	var errors []error

	// Build task map
	taskMap := make(map[string]bool)
	for _, task := range config.Tasks {
		taskMap[task.Name] = true
	}

	// Validate chain tasks reference valid tasks
	for _, chain := range config.Chains {
		for _, chainTask := range chain.Tasks {
			if !taskMap[chainTask.Name] {
				errors = append(errors, fmt.Errorf("chain '%s' references unknown task '%s'", chain.Name, chainTask.Name))
			}
		}

		// Validate input_from references
		for _, chainTask := range chain.Tasks {
			for _, inputSource := range chainTask.InputFrom {
				// Allow "task.field" format
				found := false
				for _, task := range config.Tasks {
					if inputSource == task.Name {
						found = true
						break
					}
				}
				if !found {
					// Check for "task.field" format
					// For now, just check if the task part exists
					for _, task := range config.Tasks {
						if len(inputSource) > len(task.Name) && inputSource[:len(task.Name)] == task.Name {
							found = true
							break
						}
					}
				}
				if !found {
					errors = append(errors, fmt.Errorf("chain '%s' task '%s' has invalid input_from '%s'", chain.Name, chainTask.Name, inputSource))
				}
			}
		}
	}

	return errors
}

// validateCosts validates cost constraints are reasonable
func validateCosts(config *Config) []error {
	var errors []error

	// Validate provider costs are non-negative
	for _, provider := range config.Providers {
		for _, model := range provider.Models {
			if model.CostPer1MInputTokens < 0 {
				errors = append(errors, fmt.Errorf("model '%s' has negative input cost", model.Name))
			}
			if model.CostPer1MOutputTokens < 0 {
				errors = append(errors, fmt.Errorf("model '%s' has negative output cost", model.Name))
			}
		}
	}

	// Validate budget limits are reasonable
	if config.Execution != nil {
		if config.Execution.BudgetCentsPerMinute < 0 {
			errors = append(errors, fmt.Errorf("budget_cents_per_minute cannot be negative"))
		}
		if config.Execution.BudgetCentsPerMinute > 10000 {
			errors = append(errors, fmt.Errorf("budget_cents_per_minute %d is too high (max: 10000 = $100/min)", config.Execution.BudgetCentsPerMinute))
		}
	}

	// Validate chain constraints
	if config.ChainPolicies != nil && config.ChainPolicies.Constraints != nil {
		if config.ChainPolicies.Constraints.MaxChainCostCents < 0 {
			errors = append(errors, fmt.Errorf("max_chain_cost_cents cannot be negative"))
		}
	}

	return errors
}

// ValidateYAML validates a single YAML file syntax
func ValidateYAML(path string) error {
	data, err := ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read YAML file '%s': %w", path, err)
	}

	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("YAML syntax error in '%s': %w", path, err)
	}

	return nil
}

// ValidateAllYAMLs validates all policy YAML files in configDir
func ValidateAllYAMLs(configDir string) []error {
	var errors []error

	policyFiles := []string{
		"roles.yaml",
		"tasks.yaml",
		"providers.yaml",
		"routing.yaml",
		"prompts.yaml",
		"chains.yaml",
	}

	for _, file := range policyFiles {
		path := configDir + "/" + file
		if err := ValidateYAML(path); err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", file, err))
		}
	}

	return errors
}
