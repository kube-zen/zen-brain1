package template

import (
	"testing"
)

func TestLoadV2QueueSteward(t *testing.T) {
	tmpl, err := Load("../../config/task-templates/v2/queue-steward-l1.yaml")
	if err != nil {
		t.Fatalf("failed to load v2 queue-steward template: %v", err)
	}
	if tmpl.Name != "queue-steward-l1" {
		t.Errorf("expected name=queue-steward-l1, got %s", tmpl.Name)
	}
	if tmpl.Role != "steward" {
		t.Errorf("expected role=steward, got %s", tmpl.Role)
	}
	if len(tmpl.Steps) < 1 {
		t.Errorf("expected at least 1 step, got %d", len(tmpl.Steps))
	}
	if len(tmpl.PostActions) < 1 {
		t.Errorf("expected at least 1 post_action, got %d", len(tmpl.PostActions))
	}
	if len(tmpl.AllowedTools) < 1 {
		t.Errorf("expected at least 1 allowed_tool, got %d", len(tmpl.AllowedTools))
	}
	if len(tmpl.Inputs) < 1 {
		t.Errorf("expected at least 1 input, got %d", len(tmpl.Inputs))
	}
	// Verify no ollama references
	result := Validate(tmpl)
	for _, e := range result.Errors {
		if contains(e, "Ollama") {
			t.Errorf("template should not have ollama references: %s", e)
		}
	}
	t.Logf("v2 template loaded: %s v%s (%d steps, %d post_actions, %d tools, %d inputs)",
		tmpl.Name, tmpl.Version, len(tmpl.Steps), len(tmpl.PostActions), len(tmpl.AllowedTools), len(tmpl.Inputs))
}
