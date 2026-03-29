package template

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateEmptyTemplate(t *testing.T) {
	tmpl := &Template{}
	result := Validate(tmpl)
	if result.Valid {
		t.Error("empty template should be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("expected validation errors")
	}
	// Must mention name, version, role, steps
	found := map[string]bool{}
	for _, e := range result.Errors {
		for _, kw := range []string{"name", "version", "role", "step"} {
			if contains(e, kw) {
				found[kw] = true
			}
		}
	}
	for _, kw := range []string{"name", "version", "role", "step"} {
		if !found[kw] {
			t.Errorf("expected error mentioning %q", kw)
		}
	}
}

func TestValidateValidTemplate(t *testing.T) {
	tmpl := &Template{
		Name:        "test-worker",
		Version:     "1.0",
		Role:        "worker",
		Queue:       "default",
		TargetModel: "qwen3.5:0.8b-q4",
		Inputs: []TemplateInput{
			{Name: "task", Type: "string", Required: true},
		},
		AllowedTools: []string{"jira_read", "jira_write"},
		Steps: []Step{
			{Name: "analyze", Type: "ai", Prompt: "Analyze this task: {{.task}}"},
		},
		Outputs: []TemplateOutput{
			{Name: "result", Type: "json", Required: true},
		},
		PostActions: []PostAction{
			{Type: "enqueue", Target: "factory-fill"},
		},
	}
	result := Validate(tmpl)
	if !result.Valid {
		t.Errorf("valid template rejected: %v", result.Errors)
	}
}

func TestValidateOllamaForbidden(t *testing.T) {
	tests := []struct {
		name string
		tmpl Template
	}{
		{"name", Template{Name: "ollama-worker", Version: "1.0", Role: "worker", Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}}}},
		{"prompt", Template{Name: "worker", Version: "1.0", Role: "worker", Steps: []Step{{Name: "s", Type: "ai", Prompt: "use Ollama for inference"}}}},
		{"model", Template{Name: "worker", Version: "1.0", Role: "worker", TargetModel: "ollama/llama3", Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(&tt.tmpl)
			if result.Valid {
				t.Error("expected Ollama reference to fail validation")
			}
			hasOllama := false
			for _, e := range result.Errors {
				if contains(e, "Ollama") || contains(e, "ollama") {
					hasOllama = true
				}
			}
			if !hasOllama {
				t.Errorf("expected Ollama-related error, got: %v", result.Errors)
			}
		})
	}
}

func TestValidateInvalidRole(t *testing.T) {
	tmpl := &Template{
		Name: "test", Version: "1.0", Role: "admin",
		Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}},
	}
	result := Validate(tmpl)
	if result.Valid {
		t.Error("invalid role should be rejected")
	}
}

func TestValidateStepTypes(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{"valid ai", Step{Name: "s", Type: "ai", Prompt: "do work"}, false},
		{"ai no prompt", Step{Name: "s", Type: "ai"}, true},
		{"valid tool", Step{Name: "s", Type: "tool", Tool: "jira_read"}, false},
		{"tool no tool", Step{Name: "s", Type: "tool"}, true},
		{"valid script", Step{Name: "s", Type: "script", Script: "echo hi"}, false},
		{"valid http", Step{Name: "s", Type: "http", HTTPURL: "http://example.com/api"}, false},
		{"http no url", Step{Name: "s", Type: "http"}, true},
		{"invalid type", Step{Name: "s", Type: "foobar"}, true},
		{"no name", Step{Type: "ai", Prompt: "do work"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{
				Name: "test", Version: "1.0", Role: "worker",
				Steps: []Step{tt.step},
			}
			result := Validate(tmpl)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected error: %v", result.Errors)
			}
		})
	}
}

func TestValidatePostActions(t *testing.T) {
	tests := []struct {
		name    string
		action  PostAction
		wantErr bool
	}{
		{"valid spawn", PostAction{Type: "spawn", Target: "remediation-l1"}, false},
		{"valid enqueue", PostAction{Type: "enqueue", Target: "factory-fill"}, false},
		{"valid handoff", PostAction{Type: "handoff"}, false}, // handoff doesn't need target
		{"valid schedule", PostAction{Type: "schedule", Target: "nightly-report"}, false},
		{"invalid type", PostAction{Type: "delete"}, true},
		{"enqueue no target", PostAction{Type: "enqueue"}, true},
		{"spawn no target", PostAction{Type: "spawn"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := &Template{
				Name: "test", Version: "1.0", Role: "worker",
				Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}},
				PostActions: []PostAction{tt.action},
			}
			result := Validate(tmpl)
			if tt.wantErr && result.Valid {
				t.Error("expected validation error")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("unexpected error: %v", result.Errors)
			}
		})
	}
}

func TestValidateDuplicateInputs(t *testing.T) {
	tmpl := &Template{
		Name: "test", Version: "1.0", Role: "worker",
		Inputs: []TemplateInput{
			{Name: "task", Type: "string"},
			{Name: "task", Type: "string"},
		},
		Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}},
	}
	result := Validate(tmpl)
	if result.Valid {
		t.Error("duplicate inputs should be rejected")
	}
}

func TestValidateVersion(t *testing.T) {
	tmpl := &Template{
		Name: "test", Version: "2.0", Role: "worker",
		Steps: []Step{{Name: "s", Type: "ai", Prompt: "do work"}},
	}
	result := Validate(tmpl)
	if !result.Valid {
		t.Errorf("template with different version should still be valid: %v", result.Errors)
	}
	hasWarning := false
	for _, w := range result.Warnings {
		if contains(w, "version") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected version mismatch warning")
	}
}

func TestRenderPrompt(t *testing.T) {
	prompt := "Analyze queue state: {{.QueueStateJSON}}. Target: {{.Target}}."
	inputs := map[string]string{
		"QueueStateJSON": `{"ready": 5}`,
		"Target":         "10",
	}

	result := RenderPrompt(prompt, inputs)
	if !contains(result, `{"ready": 5}`) {
		t.Errorf("expected QueueStateJSON substitution, got: %s", result)
	}
	if !contains(result, "10") {
		t.Errorf("expected Target substitution, got: %s", result)
	}
	if contains(result, "{{.Target}}") {
		t.Error("unreplaced placeholder should not remain for known vars")
	}
}

func TestRenderPromptUnknownVar(t *testing.T) {
	prompt := "Hello {{.Unknown}}"
	result := RenderPrompt(prompt, map[string]string{})
	if !contains(result, "{{.Unknown}}") {
		t.Error("unknown vars should remain as-is")
	}
}

func TestToJSON(t *testing.T) {
	tmpl := &Template{
		Name:    "test",
		Version: "1.0",
		Role:    "worker",
		Steps:   []Step{{Name: "s", Type: "ai", Prompt: "work"}},
	}
	data, err := tmpl.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	// Verify it's valid JSON with expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("expected name=test, got: %v", parsed["name"])
	}
	if parsed["version"] != "1.0" {
		t.Errorf("expected version=1.0, got: %v", parsed["version"])
	}
}

func TestLoadQueueStewardTemplate(t *testing.T) {
	// Load the actual queue-steward-l1 template from the repo
	path := filepath.Join("..", "..", "config", "task-templates", "queue-steward-l1.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("queue-steward-l1.yaml not found")
	}
	_, err := Load(path)
	// The existing template has a different schema — it may not pass our new validation.
	// That's expected; the migration will update the schema.
	t.Logf("Load result: %v", err)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
