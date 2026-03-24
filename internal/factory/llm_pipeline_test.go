package factory

import (
	"context"
	"strings"
	"testing"
)

// TestBuildImplementationPrompt_WithDescription tests description injection
func TestBuildImplementationPrompt_WithDescription(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID:  "TEST-1",
		Title:       "Test Task",
		Objective:   "Do something",
		Description: "Detailed description of the task",
		WorkType:    "implementation",
		Constraints: []string{"no fake imports"},
	}
	prompt := g.buildImplementationPrompt(req)
	if !strings.Contains(prompt, "Detailed description") {
		t.Error("prompt should contain description")
	}
	if !strings.Contains(prompt, "no fake imports") {
		t.Error("prompt should contain constraints")
	}
}

// TestBuildImplementationPrompt_EmptySections tests empty section omission
func TestBuildImplementationPrompt_EmptySections(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID: "TEST-1",
		Title:      "Test Task",
		Objective:  "Do something",
		WorkType:   "implementation",
	}
	prompt := g.buildImplementationPrompt(req)
	// Should not have empty constraints heading followed immediately by newline
	if strings.Contains(prompt, "**Constraints:**\n\n") {
		t.Error("should not have empty constraints heading")
	}
}

// TestBuildImplementationPrompt_WithAcceptanceCriteria tests acceptance criteria injection
func TestBuildImplementationPrompt_WithAcceptanceCriteria(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID:        "TEST-1",
		Title:             "Test Task",
		Objective:         "Do something",
		WorkType:          "implementation",
		AcceptanceCriteria: []string{"Code compiles", "Tests pass"},
	}
	prompt := g.buildImplementationPrompt(req)
	if !strings.Contains(prompt, "Code compiles") {
		t.Error("prompt should contain acceptance criteria: Code compiles")
	}
	if !strings.Contains(prompt, "Tests pass") {
		t.Error("prompt should contain acceptance criteria: Tests pass")
	}
}

// TestBuildImplementationPrompt_WithTargetFiles tests target file injection
func TestBuildImplementationPrompt_WithTargetFiles(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID:  "TEST-1",
		Title:       "Test Task",
		Objective:   "Do something",
		WorkType:    "implementation",
		TargetFiles: []string{"internal/mlq/selector.go"},
	}
	prompt := g.buildImplementationPrompt(req)
	if !strings.Contains(prompt, "internal/mlq/selector.go") {
		t.Error("prompt should contain target file path")
	}
}

// TestBuildImplementationPrompt_WithAllowedPaths tests allowed paths injection
func TestBuildImplementationPrompt_WithAllowedPaths(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID:   "TEST-1",
		Title:        "Test Task",
		Objective:    "Do something",
		WorkType:     "implementation",
		AllowedPaths: []string{"internal/mlq/*", "pkg/llm/types.go"},
	}
	prompt := g.buildImplementationPrompt(req)
	if !strings.Contains(prompt, "internal/mlq/*") {
		t.Error("prompt should contain allowed paths")
	}
	if !strings.Contains(prompt, "pkg/llm/types.go") {
		t.Error("prompt should contain allowed path pkg/llm/types.go")
	}
}

// TestBuildImplementationPrompt_MultipleTargetFiles tests multiple target files
func TestBuildImplementationPrompt_MultipleTargetFiles(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		WorkItemID:  "TEST-1",
		Title:       "Test Task",
		Objective:   "Do something",
		WorkType:    "implementation",
		TargetFiles: []string{"internal/mlq/selector.go", "pkg/llm/types.go", "internal/foreman/factory_runner.go"},
	}
	prompt := g.buildImplementationPrompt(req)
	for _, tf := range req.TargetFiles {
		if !strings.Contains(prompt, tf) {
			t.Errorf("prompt should contain target file: %s", tf)
		}
	}
}

// TestExtractCode_RejectsEmpty tests empty content rejection
func TestExtractCode_RejectsEmpty(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	_, _, err := g.extractCode("")
	if err == nil {
		t.Error("should reject empty content")
	}
}

// TestExtractCode_RejectsWhitespaceOnly tests whitespace-only rejection
func TestExtractCode_RejectsWhitespaceOnly(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	_, _, err := g.extractCode("   \n\n  \t  ")
	if err == nil {
		t.Error("should reject whitespace-only content")
	}
}

// TestExtractCode_RejectsNarrativeOnly tests narrative-only rejection
func TestExtractCode_RejectsNarrativeOnly(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	_, _, err := g.extractCode("This is a Go implementation.")
	if err == nil {
		t.Error("should reject narrative-only content")
	}
}

// TestExtractCode_SucceedsOnValidCodeBlock tests valid code block extraction
func TestExtractCode_SucceedsOnValidCodeBlock(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	code, lang, err := g.extractCode("```go\npackage main\nfunc main() {}\n```")
	if err != nil {
		t.Errorf("should succeed: %v", err)
	}
	if code != "package main\nfunc main() {}" {
		t.Errorf("wrong code: %q", code)
	}
	if lang != "go" {
		t.Errorf("wrong language: %q", lang)
	}
}

// TestExtractCode_RejectsEmptyCodeBlock tests empty code block rejection
func TestExtractCode_RejectsEmptyCodeBlock(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	_, _, err := g.extractCode("```go\n```")
	if err == nil {
		t.Error("should reject empty code block")
	}
}

// TestCheckFilesCreated_NoFilesHardFailure tests zero-file hard failure
func TestCheckFilesCreated_NoFilesHardFailure(t *testing.T) {
	v := NewPostflightVerifier(nil)
	result := &ExecutionResult{FilesChanged: []string{}}
	spec := &FactoryTaskSpec{ID: "test", SessionID: "test"}
	ctx := context.Background()
	check, err := v.checkFilesCreated(ctx, result, spec)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if check.Passed {
		t.Error("zero files should be a hard failure")
	}
	if !strings.Contains(check.Message, "No files generated") {
		t.Errorf("unexpected message: %s", check.Message)
	}
}

// TestGetSystemPrompt_WithTargetFiles tests structured output format instruction
func TestGetSystemPrompt_WithTargetFiles(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{
		TargetFiles: []string{"internal/mlq/selector.go", "pkg/llm/types.go"},
	}
	prompt := g.getSystemPrompt(req)
	if !strings.Contains(prompt, "FILE:") {
		t.Error("system prompt should instruct FILE: format for multi-file output")
	}
	if !strings.Contains(prompt, "multi-file") {
		t.Error("system prompt should mention multi-file output")
	}
}

// TestGetSystemPrompt_WithoutTargetFiles tests default output format
func TestGetSystemPrompt_WithoutTargetFiles(t *testing.T) {
	g, _ := NewLLMGenerator(DefaultLLMGeneratorConfig(&MockLLMProvider{}))
	req := &GenerationRequest{}
	prompt := g.getSystemPrompt(req)
	if strings.Contains(prompt, "FILE:") {
		t.Error("system prompt should not instruct FILE: format without target files")
	}
}
