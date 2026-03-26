package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/foreman"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	runtimeDir := "/tmp/zen-brain-factory"
	workspaceHome := "/tmp/zen-brain-workspaces"
	os.MkdirAll(runtimeDir, 0755)
	os.MkdirAll(workspaceHome, 0755)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:         runtimeDir,
		WorkspaceHome:      workspaceHome,
		PreferRealTemplates: true,
		EnableFactoryLLM:   true,
		LLMBaseURL:        "http://localhost:11434",
		LLMModel:          "qwen3.5:0.8b",
		LLMTimeoutSeconds:   120,
		LLMEnableThinking:   false,
	}

	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels-local.yaml")

	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("Create runner: %v", err)
	}

	// Task designed to FAIL on L1 (too complex for 0.8B) but succeed on L2 (2B)
	bt := &v1alpha1.BrainTask{
		TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "escalation-test-24b"},
		Spec: v1alpha1.BrainTaskSpec{
			SessionID: "p24b-session",
			// More complex task: requires cross-file analysis
			Description: `Analyze the zen-brain1 codebase and write a report:
1. Identify the top 3 most frequently used Go packages across all internal/ directories
2. For each package, list the top 2 exported functions by name
3. Write the result as a markdown table with columns: Package | Exported Functions

Only output the final markdown table, no explanations.`,
			WorkType:   "implementation",
			WorkDomain: "codebase",
			Title:      "Escalation Test — L1 Fail, L2 Pass",
			Priority:   "medium",
		},
	}

	log.Printf("Running escalation test task through real foreman path...")
	outcome, err := runner.Run(context.Background(), bt)
	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	out, _ := json.MarshalIndent(outcome, "", "  ")
	fmt.Printf("Outcome:\n%s\n", out)
}
