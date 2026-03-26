package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kube-zen/zen-brain1/api/v1alpha1"
	"github.com/kube-zen/zen-brain1/internal/foreman"
)

// PHASE 24: Prove real foreman path.
// Uses FactoryTaskRunner.Run() → Factory.ExecuteTask() → executeWithLLM() → TaskExecutor.ExecuteWithRetry()
// This is the exact same path the real k8s foreman uses.

const (
	OutputDir = "/tmp/zen-brain1-foreman-run"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	for _, dir := range []string{OutputDir + "/final", OutputDir + "/logs", OutputDir + "/telemetry"} {
		os.MkdirAll(dir, 0755)
	}

	runtimeDir := "/tmp/zen-brain-factory"
	workspaceHome := "/tmp/zen-brain-workspaces"
	os.MkdirAll(runtimeDir, 0755)
	os.MkdirAll(workspaceHome, 0755)

	cfg := foreman.FactoryTaskRunnerConfig{
		RuntimeDir:           runtimeDir,
		WorkspaceHome:        workspaceHome,
		PreferRealTemplates:  true,
		EnableFactoryLLM:     true,
		LLMBaseURL:          "http://localhost:56227",
		LLMModel:            "Qwen3.5-0.8B-Q4_K_M.gguf",
		LLMTimeoutSeconds:   120,
		LLMEnableThinking:   false,
	}

	// Point to real MLQ config (enables TaskExecutor with retry/escalation)
	os.Setenv("ZEN_BRAIN_MLQ_CONFIG", "/home/neves/zen/zen-brain1/config/policy/mlq-levels.yaml")

	log.Printf("[P24] Creating FactoryTaskRunner (real foreman config)...")
	runner, err := foreman.NewFactoryTaskRunner(cfg)
	if err != nil {
		log.Fatalf("[P24] Failed to create FactoryTaskRunner: %v", err)
	}
	log.Printf("[P24] FactoryTaskRunner created — real Factory.ExecuteTask path active")

	type task struct {
		ID, Class, Title, Prompt, Filename string
	}

	tasks := []task{
		{"f24-001", "dead_code", "Dead Code Report", "List unreferenced exported functions in a Go codebase. Use markdown format with function name, file, and why unused.", "dead-code-report.md"},
		{"f24-002", "defects", "Defects Report", "Analyze Go defect patterns: nil pointer dereference, unchecked errors, missing mutex locks. Markdown checklist.", "defects-report.md"},
		{"f24-003", "tech_debt", "Tech Debt Report", "Identify technical debt: TODO/FIXME comments, deprecated APIs, large functions, missing tests. Severity ratings.", "tech-debt-report.md"},
		{"f24-004", "roadmap", "Roadmap Report", "Suggest a 30-day Go project roadmap covering testing, docs, dependencies, and code quality.", "roadmap-report.md"},
		{"f24-005", "bug_hunting", "Bug Hunting Guide", "Describe Go bug-hunting techniques: race detection, memory profiling, API contract testing.", "bug-hunting-guide.md"},
		{"f24-006", "stub_hunting", "Stub Hunting Guide", "Define stub identification criteria: empty function bodies, panic(not implemented), hardcoded returns.", "stub-hunting-guide.md"},
		{"f24-007", "package_hotspot", "Package Hotspot Guide", "Explain Go package hotspot analysis: import frequency, dependency graph metrics, coupling scores.", "package-hotspot-guide.md"},
		{"f24-008", "test_gap", "Test Gap Analysis", "Describe Go test gap analysis: untested functions, missing edge case coverage, integration test gaps.", "test-gap-analysis.md"},
		{"f24-009", "config_drift", "Config Drift Guide", "Define config/policy drift detection for Go: schema validation, env var consistency, deployment parity.", "config-drift-guide.md"},
		{"f24-010", "exec_summary", "Executive Summary", "Write a Go project health summary template: build status, test coverage, dependency health, code quality.", "executive-summary.md"},
	}

	log.Printf("[P24] Dispatching %d tasks through real foreman path (FactoryTaskRunner.Run)...", len(tasks))

	var wg sync.WaitGroup
	var completed, succeeded atomic.Int64
	results := make([]map[string]interface{}, len(tasks))
	logFile, _ := os.Create(OutputDir + "/logs/foreman-dispatch.log")

	dispatchStart := time.Now()

	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, t task) {
			defer wg.Done()
			defer completed.Add(1)

			start := time.Now()
			r := map[string]interface{}{
				"task_id":    t.ID,
				"task_class": t.Class,
				"title":      t.Title,
				"path":       "real-foreman-via-FactoryTaskRunner",
				"start_time": start.Format(time.RFC3339Nano),
			}

			// Create a BrainTask matching what the k8s foreman would ingest
			bt := &v1alpha1.BrainTask{
				TypeMeta:   metav1.TypeMeta{Kind: "BrainTask", APIVersion: "zen.kube-zen.io/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{Name: t.ID},
				Spec: v1alpha1.BrainTaskSpec{
					SessionID:   "p24-session",
					Description: t.Prompt,
					WorkType:    "implementation",
					WorkDomain:  "codebase",
					Title:       t.Title,
					Priority:    "medium",
				},
			}

			logFile.WriteString(fmt.Sprintf("[P24] task=%s class=%s START path=foreman-FactoryTaskRunner.Run\n", t.ID, t.Class))

			// THIS IS THE REAL FOREMAN PATH:
			// FactoryTaskRunner.Run() → brainTaskToFactorySpec() → Factory.ExecuteTask()
			// → executeWithLLM() → executeWithLLMRetry() → TaskExecutor.ExecuteWithRetry()
			outcome, execErr := runner.Run(context.Background(), bt)

			end := time.Now()
			r["end_time"] = end.Format(time.RFC3339Nano)
			r["duration_ms"] = end.Sub(start).Milliseconds()

			if execErr != nil {
				r["success"] = false
				r["error"] = execErr.Error()
				r["disposition"] = classifyError(execErr)
				logFile.WriteString(fmt.Sprintf("[P24] task=%s FAIL duration=%v error=%s disposition=%s\n",
					t.ID, end.Sub(start), execErr, r["disposition"]))
				log.Printf("[P24] ❌ %s (%s) FAILED: %v", t.ID, t.Class, execErr)
			} else {
				r["success"] = true
				r["disposition"] = "L1-success"
				succeeded.Add(1)
				log.Printf("[P24] ✅ %s (%s) completed in %v", t.ID, t.Class, end.Sub(start))
				logFile.WriteString(fmt.Sprintf("[P24] task=%s SUCCESS duration=%v workspace=%s template=%s files=%d mode=%s\n",
					t.ID, end.Sub(start), outcome.WorkspacePath, outcome.TemplateKey,
					outcome.FilesChanged, outcome.ExecutionMode))

				// Copy artifact from workspace
				if outcome != nil && outcome.WorkspacePath != "" {
					artifactSrc := outcome.WorkspacePath
					data, readErr := os.ReadFile(artifactSrc)
					if readErr == nil && len(data) > 0 {
						artifactDest := fmt.Sprintf("%s/final/%s", OutputDir, t.Filename)
						os.WriteFile(artifactDest, data, 0644)
						r["artifact_path"] = artifactDest
						r["artifact_bytes"] = len(data)
					} else {
						// Try reading generated Go files from workspace
						files, _ := os.ReadDir(outcome.WorkspacePath)
						for _, f := range files {
							if !f.IsDir() && len(f.Name()) > 0 {
								data, err2 := os.ReadFile(outcome.WorkspacePath + "/" + f.Name())
								if err2 == nil && len(data) > 100 {
									artifactDest := fmt.Sprintf("%s/final/%s", OutputDir, t.Filename)
									os.WriteFile(artifactDest, data, 0644)
									r["artifact_path"] = artifactDest
									r["artifact_bytes"] = len(data)
									break
								}
							}
						}
					}
				}
			}

			results[idx] = r
		}(i, t)
	}

	wg.Wait()
	dispatchDuration := time.Since(dispatchStart)
	logFile.Close()

	log.Printf("[P24] === FOREMAN BATCH COMPLETE ===")
	log.Printf("[P24] Dispatched: %d  Succeeded: %d  Failed: %d  Wall: %v",
		len(tasks), succeeded.Load(), len(tasks)-int(succeeded.Load()), dispatchDuration)

	// Write telemetry
	telemetry := map[string]interface{}{
		"batch_id":        fmt.Sprintf("foreman-%s", time.Now().Format("20060102-150405")),
		"path":            "real-foreman (FactoryTaskRunner.Run → Factory.ExecuteTask → executeWithLLM → TaskExecutor)",
		"total_wall_ms":   dispatchDuration.Milliseconds(),
		"tasks_dispatched": len(tasks),
		"tasks_succeeded":  succeeded.Load(),
		"tasks_failed":     len(tasks) - int(succeeded.Load()),
		"results":         results,
	}
	tj, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile(OutputDir+"/telemetry/foreman-batch-telemetry.json", tj, 0644)

	log.Printf("[P24] Artifacts: %s/  Telemetry: %s/telemetry/", OutputDir, OutputDir)
}

func classifyError(err error) string {
	s := err.Error()
	switch {
	case contains(s, "provider health") || contains(s, "connection refused") || contains(s, "timeout"):
		return "infra-fail"
	case contains(s, "LLM execution") || contains(s, "generation"):
		return "model-fail"
	case contains(s, "preflight") || contains(s, "workspace"):
		return "infra-fail"
	default:
		return "unknown"
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
