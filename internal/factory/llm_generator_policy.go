// Package factory provides LLM-powered code generation with policy integration (ZB-024).
//
// This module uses the LLM to generate actual implementation code based on work item
// details, existing codebase context, project structure, and POLICY YAML configuration.
package factory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
	"github.com/kube-zen/zen-brain1/src/config/policy"
)

// LLMGeneratorConfig configures the LLM code generator.
type LLMGeneratorConfig struct {
	// Provider is the LLM provider to use for generation
	Provider llm.Provider

	// Model override (if empty, uses provider default)
	Model string

	// Temperature for generation (0.0-1.0, lower = more deterministic)
	Temperature float64

	// MaxTokens limits response size
	MaxTokens int

	// EnableThinking enables chain-of-thought reasoning
	EnableThinking bool

	// Timeout for LLM requests
	Timeout time.Duration

	// ZB-024: Policy configuration for provider/model selection
	PolicyConfig *policy.Config

	// ZB-024: Task class for policy-based routing
	TaskClass string

	// ZB-024: Role for policy-based provider selection
	Role string

	// ZB-024: Enforce local Ollama clamp to qwen3.5:0.8b
	EnforceOllamaClamp bool
}

// DefaultLLMGeneratorConfig returns sensible defaults.
func DefaultLLMGeneratorConfig(provider llm.Provider) *LLMGeneratorConfig {
	return &LLMGeneratorConfig{
		Provider:           provider,
		Model:              "", // Use provider default
		Temperature:        0.3, // Lower for code generation
		MaxTokens:          4096,
		EnableThinking:     true,
		Timeout:            120 * time.Second,
		EnforceOllamaClamp: true, // ZB-024: Always enforce clamp
	}
}

// LLMGenerator generates code using LLM based on work item context and policy.
type LLMGenerator struct {
	config *LLMGeneratorConfig
}

// NewLLMGenerator creates a new LLM-powered code generator with policy integration.
func NewLLMGenerator(config *LLMGeneratorConfig) (*LLMGenerator, error) {
	if config.Provider == nil {
		return nil, fmt.Errorf("LLM provider is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	if config.Temperature == 0 {
		config.Temperature = 0.3
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}

	// ZB-024: Enforce Ollama clamp if using local Ollama
	if config.EnforceOllamaClamp && isOllamaProvider(config.Provider) {
		log.Printf("[LLMGenerator] Enforcing Ollama clamp to qwen3.5:0.8b (local CPU-only inference)")
		config.Model = "qwen3.5:0.8b"
	}

	// ZB-024: Use policy-based model selection if PolicyConfig is set
	if config.PolicyConfig != nil && config.TaskClass != "" {
		if err := config.applyPolicyModelSelection(); err != nil {
			log.Printf("[LLMGenerator] Policy model selection failed: %v, using defaults", err)
		}
	}

	return &LLMGenerator{config: config}, nil
}

// applyPolicyModelSelection selects provider/model based on policy YAML (ZB-024).
func (c *LLMGeneratorConfig) applyPolicyModelSelection() error {
	// Get task routing from policy
	taskRouting := c.PolicyConfig.GetTaskRouting(c.TaskClass)
	if taskRouting == nil {
		log.Printf("[PolicySelection] No task routing for class %s, using defaults", c.TaskClass)
		return nil
	}

	// Get preferred provider from routing
	preferredProvider := taskRouting.PreferredProvider
	if preferredProvider == "" {
		// Use default provider from routing config
		preferredProvider = c.PolicyConfig.Routing.DefaultStrategy
	}

	log.Printf("[PolicySelection] Task class %s -> provider %s", c.TaskClass, preferredProvider)

	// Get provider config from policy
	providerConfig := c.PolicyConfig.GetProvider(preferredProvider)
	if providerConfig == nil {
		return fmt.Errorf("provider %s not found in policy", preferredProvider)
	}

	// Get default model for provider
	if len(providerConfig.Models) > 0 {
		// Find default model
		for _, model := range providerConfig.Models {
			if model.Default || c.Model == "" {
				c.Model = model.Name
				log.Printf("[PolicySelection] Selected model %s for provider %s", c.Model, preferredProvider)
				break
			}
		}
	}

	// Apply role-based provider selection if role is set
	if c.Role != "" {
		roleConfig := c.PolicyConfig.GetRole(c.Role)
		if roleConfig != nil && roleConfig.DefaultProvider != "" {
			// Override provider based on role
			roleProvider := c.PolicyConfig.GetProvider(roleConfig.DefaultProvider)
			if roleProvider != nil {
				preferredProvider = roleConfig.DefaultProvider
				log.Printf("[PolicySelection] Role %s -> provider %s", c.Role, preferredProvider)
			}
		}
	}

	// ZB-024: Enforce Ollama clamp (fail-closed)
	if c.EnforceOllamaClamp && preferredProvider == "ollama" {
		c.Model = "qwen3.5:0.8b"
		log.Printf("[PolicySelection] Ollama clamp enforced: model set to qwen3.5:0.8b")
	}

	return nil
}

// GenerateImplementation generates Go implementation code for a work item.
func (g *LLMGenerator) GenerateImplementation(ctx context.Context, req *GenerationRequest) (*GenerationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, g.config.Timeout)
	defer cancel()

	// Build prompt based on work type
	var prompt string
	switch req.WorkType {
	case "implementation", "feature":
		prompt = g.buildImplementationPrompt(req)
	case "bugfix", "debug":
		prompt = g.buildBugFixPrompt(req)
	case "refactor":
		prompt = g.buildRefactorPrompt(req)
	case "test":
		prompt = g.buildTestPrompt(req)
	case "migration":
		prompt = g.buildMigrationPrompt(req)
	default:
		prompt = g.buildGenericPrompt(req)
	}

	// Get system prompt from policy if available
	systemPrompt := g.getSystemPrompt(req)

	// Create LLM request
	llmReq := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:        g.config.Model,
		Temperature:  g.config.Temperature,
		MaxTokens:    g.config.MaxTokens,
		Thinking:     g.config.EnableThinking,
	}

	// ZB-024: Log provider/model selection
	log.Printf("[LLMGenerator] Generating code for task %s (provider: %T, model: %s)",
		req.WorkItemID, g.config.Provider, g.config.Model)

	// Generate code
	resp, err := g.config.Provider.Chat(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	if resp.Content == "" {
		return nil, fmt.Errorf("LLM returned empty response")
	}

	// Extract code from response
	code, language := g.extractCode(resp.Content)

	// Get token count
	tokensUsed := 0
	if resp.Usage != nil {
		tokensUsed = int(resp.Usage.TotalTokens)
	}

	log.Printf("[LLMGenerator] Generated %d lines of %s code (%d tokens)",
		strings.Count(code, "\n")+1, language, tokensUsed)

	return &GenerationResult{
		Code:         code,
		Language:     language,
		FullResponse: resp.Content,
		Model:        resp.Model,
		TokensUsed:   tokensUsed,
	}, nil
}

// isOllamaProvider checks if a provider is Ollama.
func isOllamaProvider(provider llm.Provider) bool {
	// Check provider type/name
	if provider == nil {
		return false
	}
	// Ollama providers typically have "ollama" in their name
	return strings.Contains(strings.ToLower(fmt.Sprintf("%T", provider)), "ollama")
}

// GenerationRequest contains all context needed for code generation.
type GenerationRequest struct {
	// Work item details
	WorkItemID string
	Title      string
	Objective  string
	WorkType   string
	WorkDomain string

	// Project context
	ProjectType   string // "go", "python", "node", etc.
	ModuleName    string // Go module name (if Go project)
	PackageName   string // Target package name
	TargetPath    string // Target file path

	// Code context
	ExistingCode   string // Existing code in target file (if any)
	RelatedFiles   map[string]string // Related files for context
	Imports        []string // Required imports

	// Generation constraints
	Constraints    []string // Additional constraints
	Style          string // Code style preferences
}

// GenerationResult contains generated code and metadata.
type GenerationResult struct {
	Code         string            // Extracted code (without markdown)
	Language     string            // Detected language (go, python, etc.)
	FullResponse string            // Full LLM response (including reasoning)
	Model        string            // Model used
	TokensUsed   int               // Token count
	Metadata     map[string]string // Additional metadata
}

// getSystemPrompt returns the system prompt for code generation.
func (g *LLMGenerator) getSystemPrompt(req *GenerationRequest) string {
	// ZB-024: Use policy-based prompts if available
	if g.config.PolicyConfig != nil && g.config.Role != "" {
		rolePrompt := g.config.PolicyConfig.GetPrompt(g.config.Role, req.WorkType)
		if rolePrompt != "" {
			log.Printf("[LLMGenerator] Using policy-based system prompt for role %s", g.config.Role)
			return rolePrompt
		}
	}

	// Default system prompt
	var sb strings.Builder

	sb.WriteString("You are an expert software engineer generating production-quality code.\n\n")

	// Project context
	if req.ProjectType != "" {
		sb.WriteString(fmt.Sprintf("**Project Type:** %s\n", req.ProjectType))
	}
	if req.ModuleName != "" {
		sb.WriteString(fmt.Sprintf("**Module:** %s\n", req.ModuleName))
	}
	if req.PackageName != "" {
		sb.WriteString(fmt.Sprintf("**Package:** %s\n", req.PackageName))
	}

	// Code style guidelines
	sb.WriteString("\n**Code Quality Requirements:**\n")
	sb.WriteString("- Write clean, readable, idiomatic code\n")
	sb.WriteString("- Include proper error handling\n")
	sb.WriteString("- Add godoc comments for exported functions/types\n")
	sb.WriteString("- Follow standard project structure\n")

	return sb.String()
}

// buildImplementationPrompt builds prompt for implementation tasks.
func (g *LLMGenerator) buildImplementationPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate implementation code for the following task:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Objective:** %s\n\n", req.Objective))

	if len(req.Constraints) > 0 {
		sb.WriteString("**Constraints:**\n")
		for _, c := range req.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	if req.TargetPath != "" {
		sb.WriteString(fmt.Sprintf("**Target File:** %s\n\n", req.TargetPath))
	}

	if req.ExistingCode != "" {
		sb.WriteString("**Existing Code:**\n```go\n")
		sb.WriteString(req.ExistingCode)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Generate the implementation code. Include all necessary imports and ensure it compiles.\n")

	return sb.String()
}

// buildBugFixPrompt builds prompt for bug fix tasks.
func (g *LLMGenerator) buildBugFixPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate a bug fix for the following issue:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Issue:** %s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Current Code:**\n```go\n")
		sb.WriteString(req.ExistingCode)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Analyze the issue and generate the corrected code. Explain the fix.\n")

	return sb.String()
}

// buildRefactorPrompt builds prompt for refactor tasks.
func (g *LLMGenerator) buildRefactorPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate refactored code for the following:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Goal:** %s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Current Code:**\n```go\n")
		sb.WriteString(req.ExistingCode)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Refactor the code to improve clarity, performance, or maintainability while preserving behavior.\n")

	return sb.String()
}

// buildTestPrompt builds prompt for test generation tasks.
func (g *LLMGenerator) buildTestPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate tests for the following:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Test Goal:** %s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Code to Test:**\n```go\n")
		sb.WriteString(req.ExistingCode)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("Generate comprehensive tests including edge cases and error conditions.\n")

	return sb.String()
}

// buildMigrationPrompt builds prompt for migration tasks.
func (g *LLMGenerator) buildMigrationPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate migration code for the following:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Migration Goal:** %s\n\n", req.Objective))

	if len(req.Constraints) > 0 {
		sb.WriteString("**Constraints:**\n")
		for _, c := range req.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Generate the migration code. Ensure backward compatibility where required.\n")

	return sb.String()
}

// buildGenericPrompt builds a generic prompt for unknown work types.
func (g *LLMGenerator) buildGenericPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString("Generate code for the following task:\n\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Objective:** %s\n\n", req.Objective))

	if req.TargetPath != "" {
		sb.WriteString(fmt.Sprintf("**Target File:** %s\n\n", req.TargetPath))
	}

	sb.WriteString("Generate the appropriate code based on the objective.\n")

	return sb.String()
}

// extractCode extracts code blocks from LLM response.
func (g *LLMGenerator) extractCode(response string) (string, string) {
	// Look for code blocks with language specification
	codeBlockStart := strings.Index(response, "```")
	if codeBlockStart == -1 {
		// No code blocks, return entire response as code
		return strings.TrimSpace(response), "text"
	}

	// Find language
	langStart := codeBlockStart + 3
	langEnd := strings.Index(response[langStart:], "\n")
	if langEnd == -1 {
		langEnd = strings.Index(response[langStart:], "```")
	}

	language := "text"
	if langEnd > 0 {
		language = strings.TrimSpace(response[langStart : langStart+langEnd])
	}

	// Find code end
	codeStart := langStart + langEnd + 1
	codeEnd := strings.Index(response[codeStart:], "```")
	if codeEnd == -1 {
		// No closing code block
		return strings.TrimSpace(response[codeStart:]), language
	}

	code := response[codeStart : codeStart+codeEnd]
	return strings.TrimSpace(code), language
}
