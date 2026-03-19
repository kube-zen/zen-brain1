// Package policy provides provider factory integrated with policy configuration
// Replaces hardcoded provider factory with policy-based provider selection

package policy

import (
	"fmt"

	"github.com/kube-zen/zen-brain/src/ai"
	"github.com/kube-zen/zen-brain/src/ai/providers"
)

// ConfiguredProviderFactory creates providers based on policy configuration
type ConfiguredProviderFactory struct {
	policy *Config
}

// NewConfiguredProviderFactory creates a new provider factory from policy config
func NewConfiguredProviderFactory(policy *Config) *ConfiguredProviderFactory {
	return &ConfiguredProviderFactory{
		policy: policy,
	}
}

// BuildRegistry builds an AI registry from policy configuration
func (f *ConfiguredProviderFactory) BuildRegistry(apiKeys map[string]string, enabledProviders []string) (ai.Registry, error) {
	registry := ai.NewRegistry()

	for _, provider := range f.policy.Providers {
		// Check if provider is enabled
		if !provider.Enabled {
			continue
		}

		// Check if provider is in enabled providers list (if specified)
		if len(enabledProviders) > 0 {
			enabled := false
			for _, ep := range enabledProviders {
				if ep == provider.Name {
					enabled = true
					break
				}
			}
			if !enabled {
				continue
			}
		}

		// Check if API key is available (either service key or BYOK)
		apiKey, hasKey := apiKeys[provider.Name]
		if !hasKey {
			// Provider supports BYOK, will be registered later
			if provider.ProviderType != "byok" && provider.ProviderType != "managed|byok" {
				return nil, fmt.Errorf("provider '%s' requires API key", provider.Name)
			}
		}

		// Create provider instance based on provider type
		var aiProvider ai.Provider
		var err error

		switch provider.Name {
		case "openai":
			if apiKey != "" {
				aiProvider, err = providers.NewOpenAIProvider(apiKey)
			}
		case "anthropic":
			if apiKey != "" {
				aiProvider, err = providers.NewAnthropicProvider(apiKey)
			}
		case "deepseek":
			if apiKey != "" {
				aiProvider, err = providers.NewDeepSeekProvider(apiKey)
			}
		default:
			return nil, fmt.Errorf("unknown provider: %s", provider.Name)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create provider %s: %w", provider.Name, err)
		}

		if aiProvider == nil {
			// Provider supports BYOK but no key yet, skip for now
			// BYOK keys will be registered dynamically via API
			continue
		}

		// Register provider
		if err := registry.Register(aiProvider); err != nil {
			return nil, fmt.Errorf("failed to register provider %s: %w", provider.Name, err)
		}
	}

	return registry, nil
}

// GetDefaultProvider returns the default provider based on policy
func (f *ConfiguredProviderFactory) GetDefaultProvider() ai.ProviderName {
	// Use default role's default provider
	defaultRole := f.policy.GetDefaultRole()
	if defaultRole != nil && defaultRole.DefaultProvider != "" {
		return ai.ProviderName(defaultRole.DefaultProvider)
	}

	// Use routing default strategy
	if f.policy.Routing != nil && f.policy.Routing.DefaultStrategy != "" {
		// For now, return first enabled provider as fallback
		for _, provider := range f.policy.Providers {
			if provider.Enabled {
				return ai.ProviderName(provider.Name)
			}
		}
	}

	// Fallback to deepseek
	return "deepseek"
}

// GetProviderForTask returns the recommended provider for a specific task
func (f *ConfiguredProviderFactory) GetProviderForTask(taskName string) ai.ProviderName {
	// Get task from policy
	task := f.policy.GetTask(taskName)
	if task == nil {
		// Fallback to default provider
		return f.GetDefaultProvider()
	}

	// Check if role has specific default provider
	role := f.policy.GetRole(task.RequiredRole)
	if role != nil && role.DefaultProvider != "" {
		return ai.ProviderName(role.DefaultProvider)
	}

	// Check routing policy for task class
	if f.policy.Routing != nil {
		for _, taskRouting := range f.policy.Routing.TaskRouting {
			if taskRouting.TaskClass == task.Class {
				// Use first preferred provider that's enabled
				for _, prefProvider := range taskRouting.PreferredProviders {
					for _, provider := range f.policy.Providers {
						if provider.Name == prefProvider && provider.Enabled {
							return ai.ProviderName(provider.Name)
						}
					}
				}
			}
		}
	}

	// Fallback to default provider
	return f.GetDefaultProvider()
}

// GetModelForTask returns the recommended model for a specific task
func (f *ConfiguredProviderFactory) GetModelForTask(taskName string) string {
	// Get task from policy
	task := f.policy.GetTask(taskName)
	if task == nil {
		// Return empty string (use provider default)
		return ""
	}

	// Get task class definition
	taskClass, ok := f.policy.Classes[task.Class]
	if !ok {
		return ""
	}

	// Return first allowed model for the task class
	if len(taskClass.AllowedModels) > 0 {
		return taskClass.AllowedModels[0]
	}

	return ""
}

// GetPromptForRole returns the system prompt for a role
func (f *ConfiguredProviderFactory) GetPromptForRole(roleName string) string {
	// Get prompt from policy
	prompt := f.policy.GetPrompt(roleName)
	if prompt != nil {
		return prompt.Template
	}

	// Get role for default prompt
	role := f.policy.GetRole(roleName)
	if role != nil && role.SystemPromptOverride != "" {
		return role.SystemPromptOverride
	}

	// Return empty string (use provider default)
	return ""
}

// GetTaskClassForTask returns the task class for a specific task
func (f *ConfiguredProviderFactory) GetTaskClassForTask(taskName string) string {
	task := f.policy.GetTask(taskName)
	if task != nil {
		return task.Class
	}
	return ""
}

// GetChains returns all defined chains
func (f *ConfiguredProviderFactory) GetChains() []*Chain {
	return f.policy.Chains
}

// GetChain returns a specific chain by name
func (f *ConfiguredProviderFactory) GetChain(chainName string) *Chain {
	return f.policy.GetChain(chainName)
}
