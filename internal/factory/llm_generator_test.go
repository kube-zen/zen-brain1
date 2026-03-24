package factory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kube-zen/zen-brain1/pkg/llm"
)

// MockLLMProvider is a mock LLM provider for testing.
type MockLLMProvider struct {
	Response *llm.ChatResponse
	Error    error
}

func (m *MockLLMProvider) Name() string {
	return "mock"
}

func (m *MockLLMProvider) SupportsTools() bool {
	return false
}

func (m *MockLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Response, nil
}

func (m *MockLLMProvider) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) (*llm.ChatResponse, error) {
	return m.Chat(ctx, req)
}

func (m *MockLLMProvider) Embed(ctx context.Context, req llm.EmbeddingRequest) (*llm.EmbeddingResponse, error) {
	return nil, llm.ErrEmbeddingNotSupported
}

func TestLLMGenerator_GenerateImplementation(t *testing.T) {
	tests := []struct {
		name     string
		request  *GenerationRequest
		response *llm.ChatResponse
		wantErr  bool
		wantLang string
	}{
		{
			name: "generate go implementation",
			request: &GenerationRequest{
				WorkItemID:  "TEST-001",
				Title:       "Add user service",
				Objective:   "Create a UserService with Create, Get, Update, Delete methods",
				WorkType:    "implementation",
				WorkDomain:  "core",
				ProjectType: "go",
				PackageName: "service",
			},
			response: &llm.ChatResponse{
				Content: "```go\npackage service\n\nimport \"errors\"\n\ntype UserService struct {\n\tdb DB\n}\n\nfunc NewUserService(db DB) *UserService {\n\treturn &UserService{db: db}\n}\n\nfunc (s *UserService) Create(user *User) error {\n\treturn s.db.Create(user)\n}\n```\n",
				Model:   "mock-model",
				Usage:   &llm.TokenUsage{TotalTokens: 150},
			},
			wantErr:  false,
			wantLang: "go",
		},
		{
			name: "generate bug fix",
			request: &GenerationRequest{
				WorkItemID:  "BUG-001",
				Title:       "Fix nil pointer in auth",
				Objective:   "Fix nil pointer dereference in auth middleware",
				WorkType:    "bugfix",
				WorkDomain:  "auth",
				ProjectType: "go",
				PackageName: "middleware",
				ExistingCode: "func AuthMiddleware() {\n\t// bug here\n}",
			},
			response: &llm.ChatResponse{
				Content: "```go\npackage middleware\n\nfunc AuthMiddleware() {\n\tif user == nil {\n\t\treturn\n\t}\n\t// fixed\n}\n```\n",
				Model:   "mock-model",
				Usage:   &llm.TokenUsage{TotalTokens: 100},
			},
			wantErr:  false,
			wantLang: "go",
		},
		{
			name: "generate migration",
			request: &GenerationRequest{
				WorkItemID: "MIG-001",
				Title:      "Add users table",
				Objective:  "Create users table with id, name, email, created_at columns",
				WorkType:   "migration",
				WorkDomain: "database",
			},
			response: &llm.ChatResponse{
				Content: "```sql\n-- UP\nCREATE TABLE users (\n\tid SERIAL PRIMARY KEY,\n\tname VARCHAR(255) NOT NULL,\n\temail VARCHAR(255) UNIQUE NOT NULL,\n\tcreated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP\n);\n```\n\n```sql\n-- DOWN\nDROP TABLE IF EXISTS users;\n```\n",
				Model:   "mock-model",
				Usage:   &llm.TokenUsage{TotalTokens: 120},
			},
			wantErr:  false,
			wantLang: "sql",
		},
		{
			name: "empty response",
			request: &GenerationRequest{
				WorkItemID: "TEST-002",
				Title:      "Empty test",
				Objective:  "Should fail",
				WorkType:   "implementation",
			},
			response: &llm.ChatResponse{
				Content: "",
				Model:   "mock-model",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider
			mockProvider := &MockLLMProvider{
				Response: tt.response,
			}

			// Create generator config
			config := DefaultLLMGeneratorConfig(mockProvider)
			config.EnableThinking = false // Disable for tests

			// Create generator
			generator, err := NewLLMGenerator(config)
			if err != nil {
				t.Fatalf("NewLLMGenerator() error = %v", err)
			}

			// Generate implementation
			result, err := generator.GenerateImplementation(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateImplementation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateImplementation() error = %v", err)
				return
			}

			if result == nil {
				t.Fatal("GenerateImplementation() returned nil result")
			}

			// Verify language detection
			if result.Language != tt.wantLang {
				t.Errorf("GenerateImplementation() language = %v, want %v", result.Language, tt.wantLang)
			}

			// Verify code was extracted
			if result.Code == "" {
				t.Error("GenerateImplementation() returned empty code")
			}

			// Verify metadata
			if result.Model == "" {
				t.Error("GenerateImplementation() returned empty model")
			}

			// Verify token count
			if tt.response.Usage != nil && result.TokensUsed != int(tt.response.Usage.TotalTokens) {
				t.Errorf("GenerateImplementation() tokens = %v, want %v", result.TokensUsed, tt.response.Usage.TotalTokens)
			}

			t.Logf("Generated code (lang=%s, tokens=%d):\n%s", result.Language, result.TokensUsed, result.Code)
		})
	}
}

func TestLLMGenerator_Documentation(t *testing.T) {
	mockProvider := &MockLLMProvider{
		Response: &llm.ChatResponse{
			Content: "# UserService\n\n## Overview\n\nThe UserService provides CRUD operations for users.\n\n## Usage\n\n```go\nsvc := service.NewUserService(db)\nerr := svc.Create(user)\n```\n",
			Model:   "mock-model",
			Usage:   &llm.TokenUsage{TotalTokens: 80},
		},
	}

	config := DefaultLLMGeneratorConfig(mockProvider)
	config.EnableThinking = false

	generator, err := NewLLMGenerator(config)
	if err != nil {
		t.Fatalf("NewLLMGenerator() error = %v", err)
	}

	req := &GenerationRequest{
		WorkItemID:   "DOC-001",
		Title:        "Document UserService",
		Objective:    "Add documentation for UserService with usage examples",
		WorkType:     "documentation",
		ExistingCode: "type UserService struct { db DB }",
	}

	result, err := generator.GenerateDocumentation(context.Background(), req)
	if err != nil {
		t.Fatalf("GenerateDocumentation() error = %v", err)
	}

	if result.Language != "markdown" {
		t.Errorf("GenerateDocumentation() language = %v, want markdown", result.Language)
	}

	if result.Code == "" {
		t.Error("GenerateDocumentation() returned empty code")
	}

	t.Logf("Generated documentation:\n%s", result.Code)
}

func TestLLMGenerator_ExtractCode(t *testing.T) {
	generator := &LLMGenerator{}

	tests := []struct {
		name      string
		input     string
		wantCode  string
		wantLang  string
		wantErr   bool
	}{
		{
			name:     "go code block",
			input:    "Here's the code:\n```go\npackage main\n\nfunc main() {}\n```\nThat's it.",
			wantCode: "package main\n\nfunc main() {}",
			wantLang: "go",
		},
		{
			name:     "python code block",
			input:    "```python\ndef hello():\n    print('hello')\n```",
			wantCode: "def hello():\n    print('hello')",
			wantLang: "python",
		},
		{
			name:     "no code block - narrative rejected",
			input:    "Just plain text without code blocks",
			wantCode: "",
			wantLang: "",
			wantErr:  true,
		},
		{
			name:     "unclosed code block",
			input:    "```go\npackage main",
			wantCode: "package main",
			wantLang: "go",
		},
		{
			name:     "code block with no language",
			input:    "```\nsome code\n```",
			wantCode: "some code",
			wantLang: "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, lang, err := generator.extractCode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if code != tt.wantCode {
				t.Errorf("extractCode() code = %q, want %q", code, tt.wantCode)
			}

			if lang != tt.wantLang {
				t.Errorf("extractCode() lang = %v, want %v", lang, tt.wantLang)
			}
		})
	}
}

func TestLLMTemplateExecutor_BuildGenerationRequest(t *testing.T) {
	// Create temp workspace
	workspacePath := t.TempDir()

	// Create a go.mod file
	goMod := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(workspacePath, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an existing Go file
	existingCode := "package test\n\nfunc Existing() {}"
	if err := os.WriteFile(filepath.Join(workspacePath, "existing.go"), []byte(existingCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create mock generator
	mockProvider := &MockLLMProvider{
		Response: &llm.ChatResponse{Content: "```go\npackage test\n```"},
	}
	config := DefaultLLMGeneratorConfig(mockProvider)
	generator, err := NewLLMGenerator(config)
	if err != nil {
		t.Fatal(err)
	}

	// Create executor
	execConfig := &LLMTemplateConfig{
		Type:       LLMTemplateImplementation,
		WorkType:   "implementation",
		WorkDomain: "core",
	}
	executor, err := NewLLMTemplateExecutor(generator, execConfig)
	if err != nil {
		t.Fatal(err)
	}

	// Build request
	spec := &FactoryTaskSpec{
		ID:         "task-001",
		WorkItemID: "TEST-001",
		Title:      "Add feature",
		Objective:  "Implement new feature",
		WorkType:   "implementation",
		WorkDomain: "core",
	}

	req, err := executor.buildGenerationRequest(context.Background(), spec, workspacePath)
	if err != nil {
		t.Fatalf("buildGenerationRequest() error = %v", err)
	}

	// Verify project type detection
	if req.ProjectType != "go" {
		t.Errorf("ProjectType = %v, want go", req.ProjectType)
	}

	// Verify module name detection
	if req.ModuleName != "example.com/test" {
		t.Errorf("ModuleName = %v, want example.com/test", req.ModuleName)
	}

	// Verify package name detection
	if req.PackageName != "test" {
		t.Errorf("PackageName = %v, want test", req.PackageName)
	}
}
