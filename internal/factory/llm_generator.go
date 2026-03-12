// Package factory provides LLM-powered code generation for templates.
//
// Instead of hardcoded shell scripts with placeholder code, this module
// uses the LLM to generate actual implementation code based on work item
// details, existing codebase context, and project structure.
package factory

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/llm"
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
}

// DefaultLLMGeneratorConfig returns sensible defaults.
func DefaultLLMGeneratorConfig(provider llm.Provider) *LLMGeneratorConfig {
	return &LLMGeneratorConfig{
		Provider:       provider,
		Model:          "", // Use provider default
		Temperature:    0.3, // Lower for code generation
		MaxTokens:      4096,
		EnableThinking: true,
		Timeout:        120 * time.Second,
	}
}

// LLMGenerator generates code using LLM based on work item context.
type LLMGenerator struct {
	config *LLMGeneratorConfig
}

// NewLLMGenerator creates a new LLM-powered code generator.
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

	return &LLMGenerator{config: config}, nil
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

	// Create LLM request
	llmReq := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: g.getSystemPrompt(req),
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

	return &GenerationResult{
		Code:         code,
		Language:     language,
		FullResponse: resp.Content,
		Model:        resp.Model,
		TokensUsed:   tokensUsed,
	}, nil
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
	sb.WriteString("- No TODO placeholders - generate complete implementations\n")
	sb.WriteString("- Include necessary imports\n")

	// Language-specific guidelines
	if req.ProjectType == "go" {
		sb.WriteString("\n**Go Guidelines:**\n")
		sb.WriteString("- Use proper Go idioms and conventions\n")
		sb.WriteString("- Return errors as last return value\n")
		sb.WriteString("- Use context.Context for cancellation\n")
		sb.WriteString("- Prefer small, focused functions\n")
		sb.WriteString("- Include table-driven tests when appropriate\n")
	}

	// Output format
	sb.WriteString("\n**Output Format:**\n")
	sb.WriteString("Return the code in a markdown code block with language identifier.\n")
	sb.WriteString("Example:\n")
	sb.WriteString("```go\n")
	sb.WriteString("package example\n\n")
	sb.WriteString("// Code here\n")
	sb.WriteString("```\n")

	return sb.String()
}

// buildImplementationPrompt builds prompt for feature implementation.
func (g *LLMGenerator) buildImplementationPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Implement %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Objective:**\n%s\n\n", req.Objective))

	if len(req.Constraints) > 0 {
		sb.WriteString("**Constraints:**\n")
		for _, c := range req.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	if req.ExistingCode != "" {
		sb.WriteString("**Existing Code (modify/extend this):**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	if len(req.RelatedFiles) > 0 {
		sb.WriteString("**Related Code (for context):**\n")
		for path, code := range req.RelatedFiles {
			sb.WriteString(fmt.Sprintf("\nFile: `%s`\n```%s\n%s\n```\n", path, req.ProjectType, code))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Generate complete, working implementation\n")
	sb.WriteString("2. Include all necessary imports\n")
	sb.WriteString("3. Add proper error handling\n")
	sb.WriteString("4. Include godoc comments\n")
	sb.WriteString("5. No TODOs or placeholders\n\n")

	sb.WriteString("Generate the implementation code:\n")

	return sb.String()
}

// buildBugFixPrompt builds prompt for bug fixes.
func (g *LLMGenerator) buildBugFixPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Fix Bug - %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Bug Description:**\n%s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Code with Bug:**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Identify and fix the bug\n")
	sb.WriteString("2. Maintain existing functionality\n")
	sb.WriteString("3. Add regression test if applicable\n")
	sb.WriteString("4. Explain the fix in comments\n\n")

	sb.WriteString("Generate the fixed code:\n")

	return sb.String()
}

// buildRefactorPrompt builds prompt for refactoring.
func (g *LLMGenerator) buildRefactorPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Refactor - %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Refactoring Goal:**\n%s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Code to Refactor:**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Preserve existing behavior\n")
	sb.WriteString("2. Improve code quality/readability\n")
	sb.WriteString("3. Follow project conventions\n")
	sb.WriteString("4. Maintain backwards compatibility\n\n")

	sb.WriteString("Generate the refactored code:\n")

	return sb.String()
}

// buildTestPrompt builds prompt for test generation.
func (g *LLMGenerator) buildTestPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Write Tests - %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Test Objective:**\n%s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Code to Test:**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Cover main functionality\n")
	sb.WriteString("2. Include edge cases\n")
	sb.WriteString("3. Use table-driven tests (if Go)\n")
	sb.WriteString("4. Test error conditions\n")
	sb.WriteString("5. Clear test names describing what is tested\n\n")

	sb.WriteString("Generate the test code:\n")

	return sb.String()
}

// buildMigrationPrompt builds prompt for database migrations.
func (g *LLMGenerator) buildMigrationPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Create Migration - %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Migration Description:**\n%s\n\n", req.Objective))

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Generate both UP and DOWN migrations\n")
	sb.WriteString("2. Use transaction-safe SQL\n")
	sb.WriteString("3. Include rollback logic\n")
	sb.WriteString("4. Add comments explaining the migration\n\n")

	sb.WriteString("Generate the migration SQL (UP and DOWN in separate code blocks):\n")

	return sb.String()
}

// buildGenericPrompt builds a generic prompt for unknown work types.
func (g *LLMGenerator) buildGenericPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Objective:**\n%s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Existing Code:**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	sb.WriteString("Generate the implementation:\n")

	return sb.String()
}

// extractCode extracts code from markdown code blocks.
func (g *LLMGenerator) extractCode(content string) (code string, language string) {
	// Look for markdown code blocks
	// ```language
	// code
	// ```

	// Find code block
	start := strings.Index(content, "```")
	if start == -1 {
		// No code block, return as-is
		return strings.TrimSpace(content), "text"
	}

	// Find language identifier
	afterStart := content[start+3:]
	endLang := strings.Index(afterStart, "\n")
	if endLang == -1 {
		return strings.TrimSpace(content), "text"
	}

	language = strings.TrimSpace(afterStart[:endLang])
	if language == "" {
		language = "text"
	}

	// Find end of code block
	codeStart := start + 3 + endLang + 1
	codeEnd := strings.Index(content[codeStart:], "```")
	if codeEnd == -1 {
		// Unclosed code block
		return strings.TrimSpace(content[codeStart:]), language
	}

	code = content[codeStart : codeStart+codeEnd]
	return strings.TrimSpace(code), language
}

// GenerateDocumentation generates documentation using LLM.
func (g *LLMGenerator) GenerateDocumentation(ctx context.Context, req *GenerationRequest) (*GenerationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, g.config.Timeout)
	defer cancel()

	prompt := g.buildDocumentationPrompt(req)

	llmReq := llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "system",
				Content: "You are a technical writer generating clear, comprehensive documentation.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Model:       g.config.Model,
		Temperature: 0.5, // Slightly higher for documentation
		MaxTokens:   g.config.MaxTokens,
	}

	resp, err := g.config.Provider.Chat(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM documentation generation failed: %w", err)
	}

	if resp.Content == "" {
		return nil, fmt.Errorf("LLM returned empty response")
	}

	// Get token count
	tokensUsed := 0
	if resp.Usage != nil {
		tokensUsed = int(resp.Usage.TotalTokens)
	}

	return &GenerationResult{
		Code:         resp.Content,
		Language:     "markdown",
		FullResponse: resp.Content,
		Model:        resp.Model,
		TokensUsed:   tokensUsed,
	}, nil
}

// buildDocumentationPrompt builds prompt for documentation generation.
func (g *LLMGenerator) buildDocumentationPrompt(req *GenerationRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Task: Document - %s\n\n", req.Title))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** %s\n\n", req.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Documentation Goal:**\n%s\n\n", req.Objective))

	if req.ExistingCode != "" {
		sb.WriteString("**Code to Document:**\n")
		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", req.ProjectType, req.ExistingCode))
	}

	sb.WriteString("**Requirements:**\n")
	sb.WriteString("1. Write clear, comprehensive documentation\n")
	sb.WriteString("2. Include usage examples\n")
	sb.WriteString("3. Document parameters and return values\n")
	sb.WriteString("4. Add troubleshooting section if applicable\n")
	sb.WriteString("5. Use markdown formatting\n\n")

	sb.WriteString("Generate the documentation:\n")

	return sb.String()
}

// Close cleans up resources.
func (g *LLMGenerator) Close() error {
	// No-op for now
	return nil
}

// logGeneration logs generation details.
func (g *LLMGenerator) logGeneration(req *GenerationRequest, result *GenerationResult, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = fmt.Sprintf("error: %v", err)
	}

	log.Printf("[LLMGenerator] Generated code for %s (%s/%s) - status=%s model=%s tokens=%d duration=%v",
		req.WorkItemID,
		req.WorkType,
		req.WorkDomain,
		status,
		result.Model,
		result.TokensUsed,
		duration,
	)
}
