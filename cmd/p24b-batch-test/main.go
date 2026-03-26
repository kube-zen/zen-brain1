package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/foreman"
)

type TaskResult struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Files  int    `json:"files_changed"`
	Error  string `json:"error,omitempty"`
}

// Read a file's contents for context injection
func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Truncate to first 3000 chars to keep prompt within budget
	s := string(data)
	if len(s) > 3000 {
		s = s[:3000] + "\n// ... (truncated)"
	}
	return s
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	runtimeDir := "/tmp/zen-brain-factory"
	workspaceHome := "/tmp/zen-brain-workspaces"
	os.MkdirAll(runtimeDir, 0755)
	os.MkdirAll(workspaceHome, 0755)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:          runtimeDir,
		WorkspaceHome:       workspaceHome,
		PreferRealTemplates: true,
		EnableFactoryLLM:    true,
		LLMBaseURL:         "http://localhost:11434",
		LLMModel:           "qwen3.5:0.8b",
		LLMTimeoutSeconds:    120,
		LLMEnableThinking:    false,
	}

	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels-local.yaml")

	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("Create runner: %v", err)
	}

	// Read existing code for context injection (quickwin-l1 pattern)
	existingCode := readFile("internal/factory/interface.go")
	adjacentCode := readFile("internal/factory/factory.go")
	if len(adjacentCode) > 1500 {
		adjacentCode = adjacentCode[:1500]
	}
	// Safety: skip adjacent context if empty
	if adjacentCode == "" {
		adjacentCode = "// (context file not available)"
	}

	// Properly shaped L1 tasks following quickwin-l1.yaml template rules:
	// - Single target file
	// - Existing code injected
	// - Edit-in-place goal (not greenfield)
	// - "Do not invent" constraint
	tasks := []v1alpha1.BrainTask{
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "qw-001", Labels: map[string]string{"workItemID": "qw-001"}},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "p24b-qw",
				WorkItemID:  "qw-001",
				Title:       "Add String() method to FactoryResult",
				Description: `GOAL: Add a String() string method to the ExecutionResult struct in internal/factory/interface.go.
TARGET FILE: internal/factory/interface.go
PACKAGE: factory

EXISTING CODE (target file):
` + existingCode + `

CONTEXT FILE: internal/factory/factory.go
` + adjacentCode + `

RULES:
- Modify the target file in place. Package must remain "factory".
- Use ONLY symbols already defined in the existing code or context file.
- Do NOT invent new types, interfaces, or imports beyond fmt.
- Do NOT create new files.
- The String() method should return a JSON-like summary of ExecutionResult fields.

OUTPUT: Return only the complete modified file content.`,
				WorkType:    "implementation",
				WorkDomain:  "codebase",
				Priority:    "medium",
			},
		},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "qw-002", Labels: map[string]string{"workItemID": "qw-002"}},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "p24b-qw",
				WorkItemID:  "qw-002",
				Title:       "Add IsEmpty() method to WorkspaceMetadata",
				Description: `GOAL: Add an IsEmpty() bool method to the WorkspaceMetadata struct in internal/factory/interface.go.
TARGET FILE: internal/factory/interface.go
PACKAGE: factory

EXISTING CODE:
` + existingCode + `

RULES:
- Modify the target file in place. Package must remain "factory".
- Use ONLY symbols already defined in the existing code.
- Do NOT invent new types, interfaces, or imports.
- IsEmpty() should return true if Path is empty.
- Do NOT create new files.

OUTPUT: Return only the complete modified file content.`,
				WorkType:   "implementation",
				WorkDomain: "codebase",
				Priority:   "medium",
			},
		},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "qw-003", Labels: map[string]string{"workItemID": "qw-003"}},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "p24b-qw",
				WorkItemID:  "qw-003",
				Title:       "Add Duration() method to ExecutionResult",
				Description: `GOAL: Add a Duration() time.Duration method to the ExecutionResult struct in internal/factory/interface.go that computes the elapsed time from StartedAt to CompletedAt.
TARGET FILE: internal/factory/interface.go
PACKAGE: factory

EXISTING CODE:
` + existingCode + `

RULES:
- Modify the target file in place. Package must remain "factory".
- Use ONLY symbols already defined in the existing code (time.Time, time.Duration).
- Do NOT invent new types, interfaces, or imports beyond what already exists.
- Return 0 if StartedAt is zero.
- Do NOT create new files.

OUTPUT: Return only the complete modified file content.`,
				WorkType:   "implementation",
				WorkDomain: "codebase",
				Priority:   "medium",
			},
		},
	}

	var wg sync.WaitGroup
	results := make(chan TaskResult, len(tasks))
	startTime := time.Now()

	log.Printf("Dispatching %d quickwin-l1 tasks through real foreman path...", len(tasks))

	for i := range tasks {
		wg.Add(1)
		go func(bt v1alpha1.BrainTask) {
			defer wg.Done()
			result := TaskResult{TaskID: bt.Name}
			outcome, err := runner.Run(context.Background(), &bt)
			if err != nil {
				result.Status = "FAIL"
				result.Error = err.Error()
			} else if outcome != nil && (outcome.ResultStatus == "SUCCESS" || outcome.ResultStatus == "COMPLETED") {
				result.Status = "OK"
				result.Files = outcome.FilesChanged
			} else {
				result.Status = "FAIL"
				if outcome != nil {
					result.Error = outcome.Recommendation
				}
			}
			results <- result
			log.Printf("Task %s finished: %s", bt.Name, result.Status)
		}(tasks[i])
	}

	wg.Wait()
	close(results)
	wallTime := time.Since(startTime)

	var ok, failed int
	for r := range results {
		if r.Status == "OK" {
			ok++
			log.Printf("✅ %s: %d files changed", r.TaskID, r.Files)
		} else {
			failed++
			errMsg := r.Error
			if len(errMsg) > 120 {
				errMsg = errMsg[:120] + "..."
			}
			log.Printf("❌ %s: %s", r.TaskID, errMsg)
		}
	}

	log.Printf("Batch complete: %d OK, %d FAIL, wall: %v", ok, failed, wallTime)

	telemetry := map[string]interface{}{
		"tasks_total":    len(tasks),
		"tasks_ok":       ok,
		"tasks_failed":    failed,
		"wall_ms":        wallTime.Milliseconds(),
		"timestamp":      startTime.UTC().Format(time.RFC3339),
		"task_shape":     "quickwin-l1 (bounded single-file, edit-in-place)",
		"config": map[string]int{
			"l1_slots":        10,
			"l1_ctx_total":    65536,
			"l1_ctx_per_slot": 6656,
		},
	}
	telemetryJSON, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile("/tmp/p24b-qw-telemetry.json", telemetryJSON, 0644)
}
