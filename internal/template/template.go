// Package template implements the template-first workflow foundation for zen-brain.
//
// Templates define workflow intent; the engine enforces safety, leases, retries,
// permissions, audit, and dispatch. Templates must not bypass policy or encode
// backend-specific hacks.
//
// Schema version: 1.0
// Local inference: llama.cpp with enable_thinking=false (the only compliant path).
package template

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
)

// CurrentSchemaVersion is the supported template schema version.
const CurrentSchemaVersion = "1.0"

// Template defines a bounded work item for zen-brain execution.
type Template struct {
	// Identity
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"display_name" json:"display_name,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
	Version     string `yaml:"version" json:"version"`

	// Role & queue
	Role  string `yaml:"role" json:"role"`           // worker, supervisor, steward
	Queue string `yaml:"queue" json:"queue"`         // default MLQ level for execution

	// Execution
	OperationType   string   `yaml:"operation_type" json:"operation_type"`   // provider, manual, hybrid
	TargetModel     string   `yaml:"target_model" json:"target_model"`       // llama.cpp model name
	MaxConcurrent   int      `yaml:"max_concurrent" json:"max_concurrent"`   // max parallel instances
	Priority        int      `yaml:"priority" json:"priority"`               // dispatch priority (lower = higher)
	EstimatedHours  float64  `yaml:"estimated_hours" json:"estimated_hours"` // rough cost estimate
	QueueLevel      int      `yaml:"queue_level" json:"queue_level"`         // MLQ level (0=fallback, 1=L1, 2=L2)
	ControlledFail  bool     `yaml:"controlled_failure" json:"controlled_failure"` // controlled failure test

	// Inputs
	Inputs []TemplateInput `yaml:"inputs" json:"inputs"`

	// Allowed tools — explicit contract of what the AI step can invoke.
	// Empty means no tools allowed.
	AllowedTools []string `yaml:"allowed_tools" json:"allowed_tools"`

	// Steps — the execution sequence
	Steps []Step `yaml:"steps" json:"steps"`

	// Outputs — structured output contract
	Outputs []TemplateOutput `yaml:"outputs" json:"outputs"`

	// Post-actions — what to do after successful execution
	PostActions []PostAction `yaml:"post_actions" json:"post_actions"`

	// Labels for Jira classification
	Labels []string `yaml:"labels" json:"labels,omitempty"`

	// Config overrides
	Config TemplateConfig `yaml:"config" json:"config,omitempty"`
}

// TemplateInput defines a named input for the template.
type TemplateInput struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description,omitempty"`
	Type        string `yaml:"type" json:"type"`           // string, json, file, queue_state
	Required    bool   `yaml:"required" json:"required"`    // true if execution cannot proceed without this
	Default     string `yaml:"default" json:"default,omitempty"`
}

// Step defines a single execution step.
type Step struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description,omitempty"`
	Type        string            `yaml:"type" json:"type"` // ai, tool, script, http, conditional
	Model       string            `yaml:"model" json:"model,omitempty"`
	Prompt      string            `yaml:"prompt" json:"prompt,omitempty"`
	Tool        string            `yaml:"tool" json:"tool,omitempty"`
	Script      string            `yaml:"script" json:"script,omitempty"`
	HTTPMethod  string            `yaml:"http_method" json:"http_method,omitempty"`
	HTTPURL     string            `yaml:"http_url" json:"http_url,omitempty"`
	TimeoutSec  int               `yaml:"timeout_sec" json:"timeout_sec,omitempty"`
	MaxRetries  int               `yaml:"max_retries" json:"max_retries,omitempty"`
	Env         map[string]string `yaml:"env" json:"env,omitempty"`
	Condition   string            `yaml:"condition" json:"condition,omitempty"` // for conditional steps
}

// TemplateOutput defines a named output from template execution.
type TemplateOutput struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description,omitempty"`
	Type        string `yaml:"type" json:"type"` // json, file, jira_transition, artifact
	Required    bool   `yaml:"required" json:"required"`
	Schema      string `yaml:"schema,omitempty" json:"schema,omitempty"` // optional JSON schema for validation
}

// PostAction defines what happens after template execution.
type PostAction struct {
	Type     string            `yaml:"type" json:"type"` // spawn, enqueue, handoff, schedule
	Target   string            `yaml:"target" json:"target"` // template name, queue name, or schedule name
	Filter   string            `yaml:"filter" json:"filter,omitempty"` // output key to pass through
	Params   map[string]string `yaml:"params" json:"params,omitempty"`
	Cond     string            `yaml:"cond" json:"cond,omitempty"` // condition expression
	Priority int               `yaml:"priority" json:"priority,omitempty"`
}

// TemplateConfig holds template-specific config overrides.
type TemplateConfig struct {
	Timeout         int    `yaml:"timeout" json:"timeout,omitempty"`
	MaxPromptChars  int    `yaml:"max_prompt_chars" json:"max_prompt_chars,omitempty"`
	MaxContextBytes int    `yaml:"max_context_bytes" json:"max_context_bytes,omitempty"`
	RequireWarmup   bool   `yaml:"require_warmup" json:"require_warmup,omitempty"`
	RequireTools    bool   `yaml:"require_tools" json:"require_tools,omitempty"`
}

// ValidationResult holds the result of template validation.
type ValidationResult struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// Load reads and validates a template from a YAML file.
func Load(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}

	tmpl := &Template{}
	if err := yaml.Unmarshal(data, tmpl); err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Set source path for error messages
	result := Validate(tmpl)
	if !result.Valid {
		return nil, fmt.Errorf("template validation failed:\n  %s", strings.Join(result.Errors, "\n  "))
	}

	return tmpl, nil
}

// LoadFromDir loads all templates from a directory.
func LoadFromDir(dir string) (map[string]*Template, error) {
	templates := make(map[string]*Template)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read template dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		tmpl, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", e.Name(), err)
		}
		templates[tmpl.Name] = tmpl
	}

	return templates, nil
}

// Validate checks a template against the schema.
func Validate(t *Template) ValidationResult {
	var errs []string
	var warns []string

	// Required fields
	if t.Name == "" {
		errs = append(errs, "name is required")
	}
	if t.Version == "" {
		errs = append(errs, "version is required")
	} else if t.Version != CurrentSchemaVersion {
		warns = append(warns, fmt.Sprintf("version %q != supported %q", t.Version, CurrentSchemaVersion))
	}
	if t.Role == "" {
		errs = append(errs, "role is required")
	}

	// Role must be known
	validRoles := map[string]bool{"worker": true, "supervisor": true, "steward": true}
	if t.Role != "" && !validRoles[t.Role] {
		errs = append(errs, fmt.Sprintf("invalid role %q (must be worker, supervisor, or steward)", t.Role))
	}

	// Steps validation
	if len(t.Steps) == 0 {
		errs = append(errs, "at least one step is required")
	}
	for i, s := range t.Steps {
		if s.Name == "" {
			errs = append(errs, fmt.Sprintf("step[%d]: name is required", i))
		}
		validStepTypes := map[string]bool{"ai": true, "tool": true, "script": true, "http": true, "conditional": true}
		if s.Type != "" && !validStepTypes[s.Type] {
			errs = append(errs, fmt.Sprintf("step[%d]: invalid type %q", i, s.Type))
		}
		if s.Type == "ai" && s.Prompt == "" {
			errs = append(errs, fmt.Sprintf("step[%d]: ai step requires prompt", i))
		}
		if s.Type == "tool" && s.Tool == "" {
			errs = append(errs, fmt.Sprintf("step[%d]: tool step requires tool", i))
		}
		if s.Type == "http" && s.HTTPURL == "" {
			errs = append(errs, fmt.Sprintf("step[%d]: http step requires http_url", i))
		}
	}

	// Post-actions validation
	for i, pa := range t.PostActions {
		validTypes := map[string]bool{"spawn": true, "enqueue": true, "handoff": true, "schedule": true}
		if !validTypes[pa.Type] {
			errs = append(errs, fmt.Sprintf("post_action[%d]: invalid type %q (must be spawn, enqueue, handoff, or schedule)", i, pa.Type))
		}
		if pa.Target == "" && pa.Type != "handoff" {
			errs = append(errs, fmt.Sprintf("post_action[%d]: target is required for %q", i, pa.Type))
		}
	}

	// Inputs validation
	inputNames := make(map[string]bool)
	for _, inp := range t.Inputs {
		if inputNames[inp.Name] {
			errs = append(errs, fmt.Sprintf("duplicate input name %q", inp.Name))
		}
		inputNames[inp.Name] = true
		if inp.Type == "" {
			errs = append(errs, fmt.Sprintf("input %q: type is required", inp.Name))
		}
	}

	// Output validation
	outputNames := make(map[string]bool)
	for _, out := range t.Outputs {
		if outputNames[out.Name] {
			errs = append(errs, fmt.Sprintf("duplicate output name %q", out.Name))
		}
		outputNames[out.Name] = true
	}

	// Policy: no Ollama references (G003)
	if containsOllama(t) {
		errs = append(errs, "Ollama references are forbidden — use llama.cpp only")
	}

	return ValidationResult{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warns,
	}
}

// containsOllama checks for any Ollama references in the template.
func containsOllama(t *Template) bool {
	check := func(s string) bool {
		lower := strings.ToLower(s)
		return strings.Contains(lower, "ollama") || strings.Contains(lower, "localhost:11434")
	}

	if check(t.Name) || check(t.DisplayName) || check(t.Description) {
		return true
	}
	if check(t.TargetModel) || check(t.Queue) {
		return true
	}
	for _, s := range t.Steps {
		if check(s.Prompt) || check(s.Tool) || check(s.Script) {
			return true
		}
	}
	return false
}

// ToJSON serializes the template to JSON.
func (t *Template) ToJSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

// StepOutput holds the result of a single step execution.
type StepOutput struct {
	StepName   string          `json:"step_name"`
	StepType   string          `json:"step_type"`
	Success    bool            `json:"success"`
	Output     json.RawMessage `json:"output,omitempty"`
	Error      string          `json:"error,omitempty"`
	DurationMs int64           `json:"duration_ms"`
}

// ExecutionResult holds the complete result of template execution.
type ExecutionResult struct {
	TemplateName string       `json:"template_name"`
	TemplateVer  string       `json:"template_version"`
	Success      bool         `json:"success"`
	Steps        []StepOutput `json:"steps"`
	Outputs      json.RawMessage `json:"outputs,omitempty"`
	PostActions  []PostActionResult `json:"post_actions,omitempty"`
	Error        string       `json:"error,omitempty"`
	DurationMs   int64        `json:"duration_ms"`
}

// PostActionResult holds the result of a post-action.
type PostActionResult struct {
	Type    string `json:"type"`
	Target  string `json:"target"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// RenderPrompt applies input substitutions to a prompt template.
// Supports {{.InputName}} syntax.
var promptVarRe = regexp.MustCompile(`\{\{\.(\w+)\}\}`)

// RenderPrompt substitutes {{.VarName}} placeholders with values from the inputs map.
func RenderPrompt(prompt string, inputs map[string]string) string {
	return promptVarRe.ReplaceAllStringFunc(prompt, func(match string) string {
		submatch := promptVarRe.FindStringSubmatch(match)
		if len(submatch) >= 2 {
			if val, ok := inputs[submatch[1]]; ok {
				return val
			}
		}
		return match // leave unreplaced if not found
	})
}
