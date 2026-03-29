package template

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunnerDryRun(t *testing.T) {
	tmpl := &Template{
		Name:    "test-dryrun",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "analyze", Type: "ai", Prompt: "Analyze: {{.task}}"},
			{Name: "check", Type: "tool", Tool: "jira_read"},
		},
		PostActions: []PostAction{
			{Type: "enqueue", Target: "factory-fill"},
		},
	}

	runner := NewRunner(RunnerConfig{DryRun: true})
	result := runner.Run(tmpl, map[string]string{"task": "fix bug"})

	if !result.Success {
		t.Errorf("dry run should succeed: %s", result.Error)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].StepType != "ai" {
		t.Errorf("expected ai step, got %s", result.Steps[0].StepType)
	}
	if result.Steps[1].StepType != "tool" {
		t.Errorf("expected tool step, got %s", result.Steps[1].StepType)
	}
	if len(result.PostActions) != 1 {
		t.Fatalf("expected 1 post-action, got %d", len(result.PostActions))
	}
	if result.PostActions[0].Type != "enqueue" {
		t.Errorf("expected enqueue post-action, got %s", result.PostActions[0].Type)
	}
	t.Logf("Dry run result: %+v", result)
}

func TestRunnerAIStep(t *testing.T) {
	// Create a mock llama.cpp server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}

		// Verify enable_thinking=false is set
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		if et, ok := req["enable_thinking"]; !ok || et != false {
			t.Error("expected enable_thinking=false")
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{"content": `{"result": "ok"}`},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tmpl := &Template{
		Name:    "test-ai",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "analyze", Type: "ai", Prompt: "Analyze: {{.input}}"},
		},
	}

	runner := NewRunner(RunnerConfig{
		LLMEndpoint: server.URL,
		Timeout:     5 * time.Second,
	})
	result := runner.Run(tmpl, map[string]string{"input": "test data"})

	if !result.Success {
		t.Fatalf("AI step should succeed: %s", result.Error)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
	if !result.Steps[0].Success {
		t.Fatalf("step should succeed: %s", result.Steps[0].Error)
	}
	if string(result.Steps[0].Output) != `{"result": "ok"}` {
		t.Errorf("unexpected output: %s", string(result.Steps[0].Output))
	}
}

func TestRunnerAIStepFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model overloaded", 503)
	}))
	defer server.Close()

	tmpl := &Template{
		Name:    "test-fail",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "analyze", Type: "ai", Prompt: "do work"},
		},
	}

	runner := NewRunner(RunnerConfig{
		LLMEndpoint: server.URL,
		Timeout:     5 * time.Second,
	})
	result := runner.Run(tmpl, map[string]string{})

	if result.Success {
		t.Error("expected failure on 503")
	}
	if len(result.Steps) != 1 || result.Steps[0].Success {
		t.Error("step should be marked failed")
	}
}

func TestRunnerStepOrderStopsOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tmpl := &Template{
		Name:    "test-stop",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "step1", Type: "ai", Prompt: "first"},
			{Name: "step2", Type: "tool", Tool: "nonexistent_tool"}, // will fail
			{Name: "step3", Type: "ai", Prompt: "third"},
		},
	}

	runner := NewRunner(RunnerConfig{
		LLMEndpoint: server.URL,
		Timeout:     5 * time.Second,
	})
	result := runner.Run(tmpl, map[string]string{})

	if result.Success {
		t.Error("should fail on step2")
	}
	// Only 2 steps executed (step1 succeeded, step2 failed, step3 skipped)
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps executed (stop on failure), got %d", len(result.Steps))
	}
}

func TestRunnerPostActionCondition(t *testing.T) {
	tmpl := &Template{
		Name:    "test-cond",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "step1", Type: "ai", Prompt: "work"},
		},
		PostActions: []PostAction{
			{Type: "spawn", Target: "followup", Cond: "has_issues"},
			{Type: "enqueue", Target: "factory"}, // no condition, always fires
		},
	}

	runner := NewRunner(RunnerConfig{DryRun: true})

	// Condition met
	result := runner.Run(tmpl, map[string]string{"has_issues": "true"})
	if len(result.PostActions) != 2 {
		t.Errorf("expected 2 post-actions, got %d", len(result.PostActions))
	}

	// Condition not met
	result2 := runner.Run(tmpl, map[string]string{})
	if len(result2.PostActions) != 1 {
		t.Errorf("expected 1 post-action (condition not met), got %d", len(result2.PostActions))
	}
	if result2.PostActions[0].Type != "enqueue" {
		t.Errorf("expected enqueue, got %s", result2.PostActions[0].Type)
	}
}

func TestRunnerDurationRecorded(t *testing.T) {
	tmpl := &Template{
		Name:    "test-duration",
		Version: "1.0",
		Role:    "worker",
		Steps: []Step{
			{Name: "step1", Type: "ai", Prompt: "work"},
		},
	}

	runner := NewRunner(RunnerConfig{DryRun: true})
	result := runner.Run(tmpl, map[string]string{})

	// Total duration should be non-negative (may be 0 if very fast)
	if result.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
	// Steps array should be populated
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
}
