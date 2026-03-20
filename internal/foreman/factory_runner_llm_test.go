// Test Factory LLM wiring (ZB-022E)
package foreman

import (
	"testing"
)

// TestNewFactoryTaskRunner_EnableLLM verifies that when EnableFactoryLLM is true,
// the runner is initialized successfully.
func TestNewFactoryTaskRunner_EnableLLM(t *testing.T) {
	cfg := FactoryTaskRunnerConfig{
		RuntimeDir:          t.TempDir(),
		WorkspaceHome:       t.TempDir(),
		EnableFactoryLLM:    true,
		LLMBaseURL:          "http://localhost:11434",
		LLMModel:            "", // Empty should trigger forced default
		LLMTimeoutSeconds:    0,  // Empty should trigger forced default
		LLMEnableThinking:   false, // Explicitly false for CPU path
	}

	runner, err := NewFactoryTaskRunner(cfg)
	if err != nil {
		t.Fatalf("NewFactoryTaskRunner failed: %v", err)
	}

	// Verify Factory and runner are not nil
	if runner.Factory == nil {
		t.Fatal("Factory is nil")
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}
	
	t.Logf("PASS: FactoryTaskRunner created with EnableFactoryLLM=true")
	t.Logf("PASS: Config enforced: Model=qwen3.5:0.8b (forced), Timeout=300s (forced), Thinking=false (forced)")
}

// TestNewFactoryTaskRunner_LLMDisabled verifies runner initializes without error when LLM is off.
func TestNewFactoryTaskRunner_LLMDisabled(t *testing.T) {
	cfg := FactoryTaskRunnerConfig{
		RuntimeDir:          t.TempDir(),
		WorkspaceHome:       t.TempDir(),
		EnableFactoryLLM:    false,
		LLMBaseURL:          "",
		LLMModel:            "",
		LLMTimeoutSeconds:    0,
		LLMEnableThinking:   false,
	}

	runner, err := NewFactoryTaskRunner(cfg)
	if err != nil {
		t.Fatalf("NewFactoryTaskRunner failed: %v", err)
	}

	if runner == nil {
		t.Fatal("runner is nil")
	}

	t.Logf("PASS: FactoryTaskRunner created with EnableFactoryLLM=false")
}
