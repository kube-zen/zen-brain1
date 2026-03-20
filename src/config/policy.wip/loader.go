// Package policy provides file-based configuration loading for zen-brain
// TEMPORARY STUB - Policy system is under development (ZB-022C Phase 1)
// This stub provides minimal interfaces to allow zen-brain CLI to compile

package policy

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the complete policy configuration (stub)
type Config struct {
	Roles      []*Role    `yaml:"roles"`
	Tasks       []*Task    `yaml:"tasks"`
	Providers   []*Provider `yaml:"providers"`
	Routing     *Routing    `yaml:"routing"`
	Prompts     []*Prompt   `yaml:"prompts"`
	Chains      []*Chain    `yaml:"chains"`

	// Meta
	DefaultRole string             `yaml:"default_role"`
}

// Role defines an AI agent role with capabilities and constraints (stub)
type Role struct {
	Name                string   `yaml:"name"`
	Description         string   `yaml:"description"`
	Capabilities        []string `yaml:"capabilities"`
	AllowedProviders    []string `yaml:"allowed_providers"`
	DefaultProvider     string   `yaml:"default_provider"`
	MaxTokensPerRequest int      `yaml:"max_tokens_per_request"`
	SystemPromptOverride string  `yaml:"system_prompt_override"`
}

// Task defines a specific task that can be executed (stub)
type Task struct {
	Name         string   `yaml:"name"`
	Class        string   `yaml:"class"`
	Description  string   `yaml:"description"`
	WorkType     string   `yaml:"work_type"`
	WorkDomain   string   `yaml:"work_domain"`
	Priority     string   `yaml:"priority"`
	DefaultModel string   `yaml:"default_model"`
}

// Provider defines an AI provider configuration (stub)
type Provider struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	BaseURL string   `yaml:"base_url"`
	Models  []*Model `yaml:"models"`
}

// Model defines an AI model configuration (stub)
type Model struct {
	Name    string `yaml:"name"`
	Default bool   `yaml:"default"`
}

// Routing defines task routing rules (stub)
type Routing struct {
	DefaultStrategy string         `yaml:"default_strategy"`
	TaskRouting     []*TaskRouting `yaml:"task_routing"`
}

// TaskRouting defines routing for a specific task class (stub)
type TaskRouting struct {
	Class             string `yaml:"class"`
	PreferredProvider string `yaml:"preferred_provider"`
}

// Prompt defines system prompts for different roles (stub)
type Prompt struct {
	Role    string `yaml:"role"`
	Type    string `yaml:"type"`
	Content string `yaml:"content"`
}

// Chain defines prompt chains (stub)
type Chain struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Steps       []*Step  `yaml:"steps"`
}

// Step defines a step in a prompt chain (stub)
type Step struct {
	Name    string `yaml:"name"`
	Role    string `yaml:"role"`
	Prompt  string `yaml:"prompt"`
}

// LoadConfig loads policy configuration from directory (stub - returns empty config)
func LoadConfig(configDir string) (*Config, []error) {
	cfg := &Config{
		Roles:      []*Role{},
		Tasks:       []*Task{},
		Providers:   []*Provider{},
		Routing:     &Routing{},
		Prompts:     []*Prompt{},
		Chains:      []*Chain{},
		DefaultRole: "",
	}

	// Try to load actual files if they exist
	rolesFile := filepath.Join(configDir, "roles.yaml")
	if _, err := os.Stat(rolesFile); err == nil {
		data, err := os.ReadFile(rolesFile)
		if err == nil {
			yaml.Unmarshal(data, &cfg.Roles)
		}
	}

	tasksFile := filepath.Join(configDir, "tasks.yaml")
	if _, err := os.Stat(tasksFile); err == nil {
		data, err := os.ReadFile(tasksFile)
		if err == nil {
			yaml.Unmarshal(data, &cfg.Tasks)
		}
	}

	providersFile := filepath.Join(configDir, "providers.yaml")
	if _, err := os.Stat(providersFile); err == nil {
		data, err := os.ReadFile(providersFile)
		if err == nil {
			yaml.Unmarshal(data, &cfg.Providers)
		}
	}

	routingFile := filepath.Join(configDir, "routing.yaml")
	if _, err := os.Stat(routingFile); err == nil {
		data, err := os.ReadFile(routingFile)
		if err == nil {
			yaml.Unmarshal(data, &cfg.Routing)
		}
	}

	return cfg, nil
}

// GetDefaultRole returns the default role configuration (stub)
func (c *Config) GetDefaultRole() *Role {
	if len(c.Roles) > 0 {
		return c.Roles[0]
	}
	return nil
}

// GetProvider returns provider by name (stub)
func (c *Config) GetProvider(name string) *Provider {
	for _, p := range c.Providers {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// GetRole returns role by name (stub)
func (c *Config) GetRole(name string) *Role {
	for _, r := range c.Roles {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// GetTaskRouting returns routing for a task class (stub)
func (c *Config) GetTaskRouting(class string) *TaskRouting {
	for _, tr := range c.Routing.TaskRouting {
		if tr.Class == class {
			return tr
		}
	}
	return nil
}

// GetPrompt returns a prompt for a role and type (stub)
func (c *Config) GetPrompt(role string, promptType string) string {
	for _, p := range c.Prompts {
		if p.Role == role && p.Type == promptType {
			return p.Content
		}
	}
	return ""
}

// Validate validates the policy configuration (stub - no-op)
func (c *Config) Validate() []error {
	return nil
}

// String returns string representation (stub)
func (c *Config) String() string {
	return fmt.Sprintf("PolicyConfig{Roles:%d, Tasks:%d, Providers:%d}",
		len(c.Roles), len(c.Tasks), len(c.Providers))
}
