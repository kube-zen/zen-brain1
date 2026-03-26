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
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Artifact string `json:"artifact_path,omitempty"`
	Error   string `json:"error,omitempty"`
	Files   int    `json:"files_changed,omitempty"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	runtimeDir := "/tmp/zen-brain-factory"
	workspaceHome := "/tmp/zen-brain-workspaces"
	os.MkdirAll(runtimeDir, 0755)
	os.MkdirAll(workspaceHome, 0755)
	os.MkdirAll("/tmp/zen-brain1-foreman-run/final", 0755)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:      runtimeDir,
		WorkspaceHome:    workspaceHome,
		PreferRealTemplates: true,
		EnableFactoryLLM: true,
		LLMBaseURL:       "http://localhost:11434",
		LLMModel:         "qwen3.5:0.8b",
		LLMTimeoutSeconds: 120,
		LLMEnableThinking:   false,
	}

	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels-local.yaml")

	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("Create runner: %v", err)
	}

	// 3 achievable usefulness tasks through L1-first path
	tasks := []v1alpha1.BrainTask{
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "uf-001"},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "phase24c-useful",
				WorkItemID:  "uf-001",
				Title:       "Doc Analysis",
				Description: `Scan docs/ directory and count the total number of markdown files. Output a simple markdown report listing all markdown files with their line counts.`,
				WorkType:    "implementation",
				WorkDomain:  "codebase",
				Priority:    "medium",
			},
		},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "uf-002"},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "phase24c-useful",
				WorkItemID:  "uf-002",
				Title:       "File Count Summary",
				Description: `Use search_file tool to find all .go files in cmd/ directory. Count them and create a simple markdown report with file path and line count.`,
				WorkType:    "implementation",
				WorkDomain:  "codebase",
				Priority:    "medium",
			},
		{
			TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
			ObjectMeta: metav1.ObjectMeta{Name: "uf-003"},
			Spec: v1alpha1.BrainTaskSpec{
				SessionID:   "phase24c-useful",
				WorkItemID:  "uf-003",
				Title:       "Simple Report",
				Description: `Create a markdown report summarizing: (1) the total count of Go files in cmd/ directory, (2) the number of packages in pkg/ directory. Output as plain markdown with no formatting requirements.`,
				WorkType:    "implementation",
				WorkDomain:  "codebase",
				Priority:    "medium",
			},
	}

	var wg sync.WaitGroup
	results := make(chan TaskResult, len(tasks))
	startTime := time.Now()

	log.Printf("Dispatching %d usefulness tasks through L1-first foreman path...", len(tasks))

	for i := range tasks {
		wg.Add(1)
		go func(bt v1alpha1.BrainTask) {
			defer wg.Done()
			result := TaskResult{TaskID: bt.Name}
			outcome, err := runner.Run(context.Background(), bt)
			if err != nil {
				result.Status = "FAIL"
				result.Error = err.Error()
			} else if outcome != nil && (outcome.ResultStatus == "SUCCESS" || outcome.ResultStatus == "COMPLETED") {
				result.Status = "OK"
				result.Files = outcome.FilesChanged
				if outcome.ArtifactPath() != "" {
					result.Artifact = outcome.ArtifactPath()
				}
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

	// Count results
	var ok, failed int
	for r := range results {
		if r.Status == "OK" {
			ok++
			log.Printf("✅ %s: %s", r.TaskID, r.Artifact)
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

	// Write telemetry
	telemetry := map[string]interface{}{
		"tasks_total":    len(tasks),
		"tasks_ok":       ok,
		"tasks_failed":    failed,
		"wall_ms":        wallTime.Milliseconds(),
		"timestamp":      startTime.UTC().Format(time.RFC3339),
		"task_shape":     "usefulness-l1-simple (evidence reporting, markdown output, L1-first)",
		"config": map[string]int{
			"l1_slots":        10,
			"l1_ctx_total":    65536,
			"l1_ctx_per_slot": 6656,
		},
		"tasks": []map[string]interface{}{},
	}
	for _, r := range results {
		telemetry["tasks"] = append(telemetry["tasks"].([]map[string]interface{}), map[string]interface{}{
			"task_id": r.TaskID,
			"status":  r.Status,
			"artifact": r.Artifact,
			"files":   r.Files,
			"error":   r.Error,
		})
	}
	telemetryJSON, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile("/tmp/phase24c-useful-telemetry.json", telemetryJSON, 0644)
}
