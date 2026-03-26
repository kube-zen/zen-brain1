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
		LLMTimeoutSeconds: 120,
		LLMEnableThinking: false,
	}

	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels-local.yaml")

	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("Create runner: %v", err)
	}

	bt := &v1alpha1.BrainTask{
		TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "f24a-single"},
		Spec: v1alpha1.BrainTaskSpec{
			SessionID:   "p24a-session",
			Description: "Write a simple Go function: func Add(a, b int) int { return a + b }. Return only the code, no explanation.",
			WorkType:    "implementation",
			WorkDomain:  "codebase",
			Title:       "Single L1 Test",
			Priority:    "medium",
		},
	}

	log.Printf("Running single task through real foreman path...")
	outcome, err := runner.Run(context.Background(), bt)
	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	out, _ := json.MarshalIndent(outcome, "", "  ")
	fmt.Printf("Outcome:\n%s\n", out)
}
