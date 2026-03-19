// Package policy provides file-based configuration loading for zen-brain
// Loads roles.yaml, tasks.yaml, providers.yaml, routing.yaml, prompts.yaml, chains.yaml
// from config/policy/ directory and validates cross-references

package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete policy configuration
type Config struct {
	Roles      []*Role    `yaml:"roles"`
	Tasks       []*Task    `yaml:"tasks"`
	Providers   []*Provider `yaml:"providers"`
	Routing     *Routing    `yaml:"routing"`
	Prompts     []*Prompt   `yaml:"prompts"`
	Chains      []*Chain    `yaml:"chains"`

	// Meta
	DefaultRole    string            `yaml:"default_role"`
	Execution      *ExecutionConstraints `yaml:"execution_constraints"`
	Classes        map[string]*TaskClass `yaml:"task_classes"`
	Scheduling      *Scheduling `yaml:"scheduling"`
	Policies       *PromptPolicies  `yaml:"policies"`
	ChainPolicies  *ChainPolicies     `yaml:"policies"`
	Constraints     *ChainConstraints  `yaml:"constraints"`
	Observability  *Observability     `yaml:"observability"`
}

// Role defines an AI agent role with capabilities and constraints
type Role struct {
	Name                 string            `yaml:"name"`
	Description           string            `yaml:"description"`
	Capabilities          []string           `yaml:"capabilities"`
	AllowedProviders      []string           `yaml:"allowed_providers"`
	DefaultProvider      string            `yaml:"default_provider"`
	MaxTokensPerRequest  int                `yaml:"max_tokens_per_request"`
	SystemPromptOverride string            `yaml:"system_prompt_override"`
}

// Task defines a specific task that can be executed
type Task struct {
	Name                string             `yaml:"name"`
	Class               string             `yaml:"class"`
	Description          string             `yaml:"description"`
	RequiredRole        string             `yaml:"required_role"`
	TimeoutSeconds       int                `yaml:"timeout_seconds"`
	MaxTokens           int                `yaml:"max_tokens"`
	RequiredFields      []string            `yaml:"required_fields"`
	OutputSchema        *OutputSchema      `yaml:"output_schema"`
	ExternalSourcesReq  []string            `yaml:"external_sources_required"`
}

// Provider defines an AI provider and its models
type Provider struct {
	Name         string       `yaml:"name"`
	DisplayName   string       `yaml:"display_name"`
	Class        string       `yaml:"class"`
	Enabled      bool         `yaml:"enabled"`
	ProviderType string       `yaml:"provider_type"`
	APIEndpoint  string       `yaml:"api_endpoint"`
	Models       []*Model     `yaml:"models"`
}

// Model defines a specific AI model
type Model struct {
	Name                    string   `yaml:"name"`
	DisplayName              string   `yaml:"display_name"`
	Class                   string   `yaml:"class"`
	ContextWindow           int      `yaml:"context_window"`
	MaxOutputTokens         int      `yaml:"max_output_tokens"`
	CostPer1MInputTokens   float64  `yaml:"cost_per_1m_input_tokens"`
	CostPer1MOutputTokens  float64  `yaml:"cost_per_1m_output_tokens"`
	SupportsStreaming       bool     `yaml:"supports_streaming"`
	SupportsFunctions       bool     `yaml:"supports_functions"`
	SupportsJSONMode       bool     `yaml:"supports_json_mode"`
	MaxBatchSize           int      `yaml:"max_batch_size"`
	RateLimitRPM          int      `yaml:"rate_limit_rpm"`
	RateLimitRPD          int      `yaml:"rate_limit_rpd"`
	RecommendedForTasks     []string `yaml:"recommended_for_tasks"`
}

// Routing defines request routing policies
type Routing struct {
	DefaultStrategy     string          `yaml:"default_strategy"`
	ModelSelection     *ModelSelection `yaml:"model_selection"`
	TaskRouting        []TaskRouting    `yaml:"task_routing"`
	ProviderRouting    []ProviderRouting `yaml:"provider_routing"`
	TenantOverrides    []TenantOverride `yaml:"tenant_overrides"`
	ModelFallback      []ModelFallback  `yaml:"model_fallback"`
	Arbitration       *Arbitration    `yaml:"arbitration"`
	Heuristics       *Heuristics     `yaml:"heuristics"`
}

// Prompt defines a system prompt template
type Prompt struct {
	Role                string          `yaml:"role"`
	Name                string          `yaml:"name"`
	Version              string          `yaml:"version"`
	Template            string          `yaml:"template"`
	TaskOverrides        []TaskOverride  `yaml:"task_overrides"`
}

// Chain defines a task execution chain
type Chain struct {
	Name                string         `yaml:"name"`
	Description          string         `yaml:"description"`
	Tasks               []ChainTask    `yaml:"tasks"`
	OutputAggregation    *Aggregation    `yaml:"output_aggregation"`
}

// ChainTask is a task within a chain
type ChainTask struct {
	Name         string   `yaml:"name"`
	TaskClass    string   `yaml:"task_class"`
	Role         string   `yaml:"role"`
	TimeoutSec   int      `yaml:"timeout_seconds"`
	MaxRetries   int      `yaml:"max_retries"`
	DependsOn    []string `yaml:"depends_on"`
	InputFrom    []string `yaml:"input_from"`
	ParallelWith  []string `yaml:"parallel_with"`
}

// Supporting structures for detailed config
type OutputSchema struct {
	Type       string                 `yaml:"type"`
	Properties map[string]interface{}   `yaml:"properties"`
}

type ModelSelection struct {
	PriorityOrder []string `yaml:"priority_order"`
	UnavailabilityHandling string `yaml:"unavailability_handling"`
	FailoverChain []string `yaml:"failover_chain"`
}

type TaskRouting struct {
	TaskClass          string          `yaml:"task_class"`
	Strategy           string          `yaml:"strategy"`
	PreferredProviders  []string         `yaml:"preferred_providers"`
	FallbackProviders   []string         `yaml:"fallback_providers"`
	ModelConstraints    *RouteConstraints `yaml:"model_constraints"`
	QualityRequirements *QualityReqs     `yaml:"quality_requirements"`
}

type ProviderRouting struct {
	Provider            string  `yaml:"provider"`
	MaxRPM              int     `yaml:"max_rpm"`
	MaxRPD              int     `yaml:"max_rpd"`
	CostPer1MTokens     float64 `yaml:"cost_cents_per_1m_tokens"`
	QualityWeight       float64 `yaml:"quality_weight"`
	TimeoutSeconds      int     `yaml:"timeout_seconds"`
}

type TenantOverride struct {
	TenantID   string `yaml:"tenant_id"`
	TaskClass   string `yaml:"task_class"`
	Strategy    string `yaml:"strategy"`
	Providers   []string `yaml:"preferred_providers"`
}

type ModelFallback struct {
	FromModel string `yaml:"from_model"`
	ToModel   string `yaml:"to_model"`
	Reason     string `yaml:"reason"`
}

type Arbitration struct {
	DefaultStrategy   string            `yaml:"default_strategy"`
	Weights          map[string]float64 `yaml:"weights"`
	MajorityThreshold float64           `yaml:"majority_threshold"`
	TimeoutSeconds   int                `yaml:"arbitration_timeout_seconds"`
}

type Heuristics struct {
	UseCacheIfFresh         bool `yaml:"use_cache_if_fresh"`
	CacheFreshnessSeconds   int  `yaml:"cache_freshness_seconds"`
	CheckRateLimits        bool `yaml:"check_rate_limits"`
	RateLimitBuffer       float64 `yaml:"rate_limit_buffer"`
	UseHistoricalSuccess   bool `yaml:"use_historical_success"`
	SuccessRateWindowHours int     `yaml:"success_rate_window_hours"`
	ConsiderCost           bool `yaml:"consider_cost"`
	CostThresholdTokens   int     `yaml:"cost_threshold_tokens"`
}

type TaskOverride struct {
	Task                 string `yaml:"task"`
	PatchSystemPrompt     bool   `yaml:"cve_patch_system_prompt"`
	TrackCustomerKeys     bool   `yaml:"track_customer_keys"`
	CostAwareRouting      bool   `yaml:"cost_aware_routing"`
	PromptTemplate       string `yaml:"prompt_template"`
}

type RouteConstraints struct {
	MaxContextTokens         int     `yaml:"max_context_tokens"`
	MinContextTokens         int     `yaml:"min_context_tokens"`
	MaxResponseTimeMs       int     `yaml:"max_response_time_ms"`
	MinResponseTimeMs       int     `yaml:"min_response_time_ms"`
	RequireStreaming        bool    `yaml:"require_streaming"`
	RequireJSONMode        bool    `yaml:"require_json_mode"`
	RequireCodeExamples    bool    `yaml:"require_code_examples"`
	MaxBatchSize           int     `yaml:"max_batch_size"`
	MaxConcurrentRequests  int     `yaml:"max_concurrent_requests"`
}

type QualityReqs struct {
	MinConfidence       float64 `yaml:"min_confidence"`
	RequireCitations    bool    `yaml:"require_citations"`
	RequireSources      bool    `yaml:"require_sources"`
}

type ExecutionConstraints struct {
	TimeoutSeconds          int `yaml:"timeout_seconds"`
	MaxConcurrentRequests  int `yaml:"max_concurrent_requests"`
	RetryPolicy          *RetryPolicy `yaml:"retry_policy"`
	BudgetCentsPerMinute  int `yaml:"budget_cents_per_minute"`
}

type RetryPolicy struct {
	MaxAttempts     int `yaml:"max_attempts"`
	BackoffSeconds  int `yaml:"backoff_seconds"`
}

type TaskClass struct {
	Priority       int      `yaml:"priority"`
	CostWeight     float64  `yaml:"cost_weight"`
	AllowedModels  []string `yaml:"allowed_models"`
}

type Scheduling struct {
	MaxConcurrentTasksPerRole int             `yaml:"max_concurrent_tasks_per_role"`
	TaskQueueSizeLimit        int             `yaml:"task_queue_size_limit"`
	PriorityClasses           map[string][]string `yaml:"priority_classes"`
}

type PromptPolicies struct {
	MaxPromptTokens  map[string]int `yaml:"max_prompt_tokens"`
	OutputFormat      []string       `yaml:"output_format_requirements"`
	SafetyPolicies   *SafetyPolicies `yaml:"safety_policies"`
}

type SafetyPolicies struct {
	NeverRefuseReasoningRequests bool     `yaml:"never_refuse_reasoning_requests"`
	AlwaysIncludeConfidenceScores bool   `yaml:"always_include_confidence_scores"`
	RequireSourceCitation           bool   `yaml:"require_source_citation"`
	MaxHallucinationRisk          float64 `yaml:"max_hallucination_risk"`
}

type Aggregation struct {
	MergeOutputs         bool   `yaml:"merge_outputs"`
	OutputSchema         *OutputSchema `yaml:"output_schema"`
}

type ChainPolicies struct {
	Dependencies       *ChainDeps `yaml:"dependencies"`
	Execution        *ChainExec `yaml:"execution"`
	Aggregation       *ChainAgg  `yaml:"aggregation"`
}

type ChainDeps struct {
	MissingDependencyHandling string `yaml:"missing_dependency_handling"`
	MaxWaitSeconds          int     `yaml:"max_wait_seconds"`
	AllowPartialCompletion  bool    `yaml:"allow_partial_completion"`
}

type ChainExec struct {
	MaxConcurrentTasks int        `yaml:"max_concurrent_tasks"`
	DefaultTaskTimeout  int        `yaml:"default_task_timeout_seconds"`
	RetryPolicy        *RetryPolicy `yaml:"retry_policy"`
}

type ChainAgg struct {
	MergeStrategy          string `yaml:"merge_strategy"`
	ConflictResolution    string `yaml:"conflict_resolution"`
	ValidateSchema        bool   `yaml:"validate_schema"`
	FailFastOnSchemaError bool  `yaml:"fail_fast_on_schema_error"`
}

type Observability struct {
	LogChainExecution    bool `yaml:"log_chain_execution"`
	LogTaskStart        bool `yaml:"log_task_start"`
	LogTaskCompletion   bool `yaml:"log_task_completion"`
	LogDependencies     bool `yaml:"log_dependencies"`
	Metrics            []string `yaml:"metrics"`
}

// LoadConfig loads all policy files from configDir (default: ./config/policy/)
func LoadConfig(configDir string) (*Config, []error) {
	if configDir == "" {
		configDir = "./config/policy/"
	}

	var errors []error
	config := &Config{}

	// Load roles.yaml
	roles, err := loadRoles(filepath.Join(configDir, "roles.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load roles.yaml: %w", err))
	} else {
		config.Roles = roles
	}

	// Load tasks.yaml
	tasks, err := loadTasks(filepath.Join(configDir, "tasks.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load tasks.yaml: %w", err))
	} else {
		config.Tasks = tasks
	}

	// Load providers.yaml
	providers, err := loadProviders(filepath.Join(configDir, "providers.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load providers.yaml: %w", err))
	} else {
		config.Providers = providers
	}

	// Load routing.yaml
	routing, err := loadRouting(filepath.Join(configDir, "routing.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load routing.yaml: %w", err))
	} else {
		config.Routing = routing
	}

	// Load prompts.yaml
	prompts, err := loadPrompts(filepath.Join(configDir, "prompts.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load prompts.yaml: %w", err))
	} else {
		config.Prompts = prompts
	}

	// Load chains.yaml
	chains, err := loadChains(filepath.Join(configDir, "chains.yaml"))
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to load chains.yaml: %w", err))
	} else {
		config.Chains = chains
	}

	// Validate cross-references
	validationErrors := validateConfig(config)
	if len(validationErrors) > 0 {
		for _, verr := range validationErrors {
			errors = append(errors, verr)
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return config, nil
}

// loadRoles loads roles.yaml
func loadRoles(path string) ([]*Role, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rolesFile struct {
		Roles     []*Role `yaml:"roles"`
		DefaultRole string `yaml:"default_role"`
		Execution *ExecutionConstraints `yaml:"execution_constraints"`
	}

	if err := yaml.Unmarshal(data, &rolesFile); err != nil {
		return nil, err
	}

	return rolesFile.Roles, nil
}

// loadTasks loads tasks.yaml
func loadTasks(path string) ([]*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tasksFile struct {
		Tasks   []*Task    `yaml:"tasks"`
		Classes map[string]*TaskClass `yaml:"task_classes"`
		Scheduling *Scheduling `yaml:"scheduling"`
	}

	if err := yaml.Unmarshal(data, &tasksFile); err != nil {
		return nil, err
	}

	return tasksFile.Tasks, nil
}

// loadProviders loads providers.yaml
func loadProviders(path string) ([]*Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var providersFile struct {
		Providers []*Provider `yaml:"providers"`
		FallbackChain []string  `yaml:"fallback_chain"`
		Constraints   *ProviderConstraints `yaml:"provider_constraints"`
	}

	if err := yaml.Unmarshal(data, &providersFile); err != nil {
		return nil, err
	}

	return providersFile.Providers, nil
}

// loadRouting loads routing.yaml
func loadRouting(path string) (*Routing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var routing Routing
	if err := yaml.Unmarshal(data, &routing); err != nil {
		return nil, err
	}

	return &routing, nil
}

// loadPrompts loads prompts.yaml
func loadPrompts(path string) ([]*Prompt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var promptsFile struct {
		Prompts  []*Prompt  `yaml:"prompts"`
		Policies *PromptPolicies `yaml:"policies"`
	}

	if err := yaml.Unmarshal(data, &promptsFile); err != nil {
		return nil, err
	}

	return promptsFile.Prompts, nil
}

// loadChains loads chains.yaml
func loadChains(path string) ([]*Chain, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var chainsFile struct {
		Chains     []*Chain `yaml:"chains"`
		Policies   *ChainPolicies `yaml:"policies"`
		Templates   []*Chain  `yaml:"templates"`
		Constraints  *ChainConstraints `yaml:"constraints"`
		Observability *Observability `yaml:"observability"`
	}

	if err := yaml.Unmarshal(data, &chainsFile); err != nil {
		return nil, err
	}

	return chainsFile.Chains, nil
}

// validateConfig validates cross-references between policy files
func validateConfig(config *Config) []error {
	var errors []error

	// Create maps for lookup
	roleMap := make(map[string]bool)
	for _, role := range config.Roles {
		roleMap[role.Name] = true
	}

	taskMap := make(map[string]bool)
	for _, task := range config.Tasks {
		taskMap[task.Name] = true
	}

	providerMap := make(map[string]bool)
	for _, provider := range config.Providers {
		providerMap[provider.Name] = true
	}

	// Validate tasks reference valid roles
	for _, task := range config.Tasks {
		if task.RequiredRole != "" && !roleMap[task.RequiredRole] {
			errors = append(errors, fmt.Errorf("task '%s' references unknown role '%s'", task.Name, task.RequiredRole))
		}
	}

	// Validate chains reference valid tasks
	for _, chain := range config.Chains {
		for _, chainTask := range chain.Tasks {
			if chainTask.Role != "" && !roleMap[chainTask.Role] {
				errors = append(errors, fmt.Errorf("chain '%s' task '%s' references unknown role '%s'", chain.Name, chainTask.Name, chainTask.Role))
			}
		}
	}

	// Validate roles reference valid providers
	for _, role := range config.Roles {
		for _, provider := range role.AllowedProviders {
			if !providerMap[provider] {
				errors = append(errors, fmt.Errorf("role '%s' references unknown provider '%s'", role.Name, provider))
			}
		}
		if role.DefaultProvider != "" && !providerMap[role.DefaultProvider] {
			errors = append(errors, fmt.Errorf("role '%s' default provider '%s' not found", role.Name, role.DefaultProvider))
		}
	}

	// Detect circular dependencies in chains
	for _, chain := range config.Chains {
		cycles := detectCycles(chain.Tasks)
		if len(cycles) > 0 {
			errors = append(errors, fmt.Errorf("chain '%s' has circular dependencies: %v", chain.Name, cycles))
		}
	}

	// Validate routing references valid providers
	if config.Routing != nil {
		for _, providerRouting := range config.Routing.ProviderRouting {
			if !providerMap[providerRouting.Provider] {
				errors = append(errors, fmt.Errorf("routing references unknown provider '%s'", providerRouting.Provider))
			}
		}
		for _, taskRouting := range config.Routing.TaskRouting {
			for _, provider := range taskRouting.PreferredProviders {
				if !providerMap[provider] {
					errors = append(errors, fmt.Errorf("task routing for '%s' references unknown provider '%s'", taskRouting.TaskClass, provider))
				}
			}
		}
	}

	return errors
}

// detectCycles detects circular dependencies using DFS
func detectCycles(tasks []ChainTask) []string {
	// Build adjacency list
	adj := make(map[string][]string)
	taskNames := make(map[string]bool)
	for _, task := range tasks {
		taskNames[task.Name] = true
		for _, dep := range task.DependsOn {
			adj[task.Name] = append(adj[task.Name], dep)
		}
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var cycles []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				// Cycle detected
				cycles = append(cycles, node)
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for task := range tasks {
		if !visited[task.Name] {
			if dfs(task.Name) {
				break
			}
		}
	}

	return cycles
}

// Helper functions for policy queries

// GetRole returns a role by name
func (c *Config) GetRole(name string) *Role {
	for _, role := range c.Roles {
		if role.Name == name {
			return role
		}
	}
	return nil
}

// GetTask returns a task by name
func (c *Config) GetTask(name string) *Task {
	for _, task := range c.Tasks {
		if task.Name == name {
			return task
		}
	}
	return nil
}

// GetProvider returns a provider by name
func (c *Config) GetProvider(name string) *Provider {
	for _, provider := range c.Providers {
		if provider.Name == name {
			return provider
		}
	}
	return nil
}

// GetChain returns a chain by name
func (c *Config) GetChain(name string) *Chain {
	for _, chain := range c.Chains {
		if chain.Name == name {
			return chain
		}
	}
	return nil
}

// GetPrompt returns a prompt for a role
func (c *Config) GetPrompt(role string) *Prompt {
	for _, prompt := range c.Prompts {
		if prompt.Role == role {
			return prompt
		}
	}
	return nil
}

// GetDefaultRole returns the default role
func (c *Config) GetDefaultRole() *Role {
	if c.DefaultRole == "" {
		// Fallback to first role
		if len(c.Roles) > 0 {
			return c.Roles[0]
		}
		return nil
	}
	return c.GetRole(c.DefaultRole)
}
