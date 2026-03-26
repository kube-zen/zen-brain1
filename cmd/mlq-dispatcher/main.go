package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// MLQTaskDispatcher dispatches 10 reporting tasks across a single llama.cpp
// endpoint with 10 parallel slots. Each task is a useful bounded report.
//
// PHASE 22 P6-P9: Feed useful tasks through MLQ, prove parallelism, produce artifacts.

const (
	L1Endpoint = "http://localhost:56227/v1/chat/completions"
	L1Model    = "Qwen3.5-0.8B-Q4_K_M.gguf"
	OutputDir  = "/tmp/zen-brain1-mlq-run"
)

// ReportingTask defines a useful bounded reporting task for L1.
type ReportingTask struct {
	ID       string `json:"id"`
	Class    string `json:"class"`
	Prompt   string `json:"prompt"`
	Filename string `json:"filename"`
}

// TaskResult captures the outcome of a single task.
type TaskResult struct {
	TaskID         string        `json:"task_id"`
	TaskClass      string        `json:"task_class"`
	WorkerEndpoint string        `json:"worker_endpoint"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration_ms"`
	Success        bool          `json:"success"`
	Error          string        `json:"error,omitempty"`
	ArtifactPath   string        `json:"artifact_path"`
	TokensPrompt   int           `json:"tokens_prompt"`
	TokensCompletion int         `json:"tokens_completion"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Create output directories
	for _, dir := range []string{OutputDir + "/final", OutputDir + "/logs", OutputDir + "/telemetry"} {
		os.MkdirAll(dir, 0755)
	}

	tasks := []ReportingTask{
		{ID: "mlq-001", Class: "dead_code", Prompt: "List unreferenced exported functions in a Go codebase. Focus on pkg/ and internal/ directories. Return a markdown report with function name, file path, and why it might be unused. Title: Dead Code Report.", Filename: "dead-code-report.md"},
		{ID: "mlq-002", Class: "defects", Prompt: "Analyze common Go defect patterns: nil pointer dereference risks, unchecked errors, missing mutex locks, and resource leaks. Return a markdown checklist-style report. Title: Defects Report.", Filename: "defects-report.md"},
		{ID: "mlq-003", Class: "tech_debt", Prompt: "Identify technical debt indicators in a Go project: TODO/FIXME/HACK comments, deprecated API usage, large functions (>100 lines), god objects, and missing tests. Return a markdown report with severity ratings. Title: Tech Debt Report.", Filename: "tech-debt-report.md"},
		{ID: "mlq-004", Class: "roadmap", Prompt: "Based on common patterns in Go backend projects, suggest a 30-day roadmap covering: testing improvements, documentation gaps, dependency updates, and code quality tooling. Return markdown. Title: Roadmap Report.", Filename: "roadmap-report.md"},
		{ID: "mlq-005", Class: "bug_hunting", Prompt: "Describe systematic bug-hunting techniques for Go codebases: race condition detection, memory profiling, API contract testing, and edge case analysis. Return a markdown guide. Title: Bug Hunting Guide.", Filename: "bug-hunting-guide.md"},
		{ID: "mlq-006", Class: "stub_hunting", Prompt: "Define criteria for identifying stubs and incomplete implementations in Go: empty function bodies, panic(\"not implemented\"), hardcoded return values, and TODO-only functions. Return a markdown checklist. Title: Stub Hunting Guide.", Filename: "stub-hunting-guide.md"},
		{ID: "mlq-007", Class: "package_hotspot", Prompt: "Explain how to identify package hotspots in a Go project: import frequency analysis, dependency graph metrics, coupling/cohesion scoring, and change frequency tracking. Return a markdown guide. Title: Package Hotspot Analysis Guide.", Filename: "package-hotspot-guide.md"},
		{ID: "mlq-008", Class: "test_gap", Prompt: "Describe a test gap analysis approach for Go: untested exported functions, missing edge case coverage, integration test gaps, and benchmark absence. Return a markdown report template. Title: Test Gap Analysis.", Filename: "test-gap-analysis.md"},
		{ID: "mlq-009", Class: "config_drift", Prompt: "Define config/policy drift detection for a Go project with YAML configs: schema validation, default value auditing, environment variable consistency, and deployment parity checks. Return a markdown guide. Title: Config/Policy Drift Guide.", Filename: "config-drift-guide.md"},
		{ID: "mlq-010", Class: "executive_summary", Prompt: "Write a template for a Go project executive health summary covering: build status, test coverage, dependency health, code quality metrics, known issues, and recommended actions. Return markdown. Title: Executive Health Summary Template.", Filename: "executive-summary.md"},
	}

	log.Printf("[Dispatcher] Dispatching %d reporting tasks to L1 (%s)", len(tasks), L1Endpoint)
	log.Printf("[Dispatcher] All tasks route to L1 first. Escalation only after repeated L1 failure.")

	var wg sync.WaitGroup
	var completed atomic.Int64
	var succeeded atomic.Int64
	results := make([]TaskResult, len(tasks))
	logFile, _ := os.Create(OutputDir + "/logs/dispatch.log")
	defer logFile.Close()

	dispatchStart := time.Now()

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t ReportingTask) {
			defer wg.Done()
			defer completed.Add(1)

			result := executeTask(t, logFile)
			results[idx] = result

			if result.Success {
				succeeded.Add(1)
				log.Printf("[Dispatcher] ✅ %s (%s) completed in %v → %s", t.ID, t.Class, result.Duration, result.ArtifactPath)
			} else {
				log.Printf("[Dispatcher] ❌ %s (%s) failed in %v: %s", t.ID, t.Class, result.Duration, result.Error)
			}
		}(i, task)
	}

	wg.Wait()
	dispatchDuration := time.Since(dispatchStart)

	// Summary
	log.Printf("[Dispatcher] === BATCH COMPLETE ===")
	log.Printf("[Dispatcher] Tasks: %d dispatched, %d succeeded, %d failed", len(tasks), succeeded.Load(), len(tasks)-int(succeeded.Load()))
	log.Printf("[Dispatcher] Total wall time: %v", dispatchDuration)

	// Write telemetry
	telemetry := map[string]interface{}{
		"batch_id":         fmt.Sprintf("mlq-batch-%s", time.Now().Format("20060102-150405")),
		"dispatch_start":   dispatchStart.Format(time.RFC3339),
		"dispatch_end":     time.Now().Format(time.RFC3339),
		"total_wall_ms":    dispatchDuration.Milliseconds(),
		"tasks_dispatched": len(tasks),
		"tasks_succeeded":  succeeded.Load(),
		"tasks_failed":     len(tasks) - int(succeeded.Load()),
		"parallel_workers": 10,
		"l1_endpoint":      L1Endpoint,
		"l1_model":         L1Model,
		"results":          results,
	}
	telemetryJSON, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile(OutputDir+"/telemetry/batch-telemetry.json", telemetryJSON, 0644)

	// Write concurrency report
	var concurrency bytes.Buffer
	fmt.Fprintf(&concurrency, "# MLQ Concurrency Report\n\n")
	fmt.Fprintf(&concurrency, "## Batch Summary\n\n")
	fmt.Fprintf(&concurrency, "- **Tasks dispatched:** %d\n", len(tasks))
	fmt.Fprintf(&concurrency, "- **Tasks succeeded:** %d\n", succeeded.Load())
	fmt.Fprintf(&concurrency, "- **Tasks failed:** %d\n", len(tasks)-int(succeeded.Load()))
	fmt.Fprintf(&concurrency, "- **Total wall time:** %v\n", dispatchDuration)
	fmt.Fprintf(&concurrency, "- **L1 endpoint:** %s\n", L1Endpoint)
	fmt.Fprintf(&concurrency, "- **L1 model:** %s\n", L1Model)
	fmt.Fprintf(&concurrency, "- **Parallel slots:** 10\n\n")
	fmt.Fprintf(&concurrency, "## Per-Task Results\n\n")
	fmt.Fprintf(&concurrency, "| Task ID | Class | Duration | Status | Artifact |\n")
	fmt.Fprintf(&concurrency, "|---------|-------|----------|--------|----------|\n")
	for _, r := range results {
		status := "✅ success"
		if !r.Success {
			status = "❌ failed"
		}
		fmt.Fprintf(&concurrency, "| %s | %s | %v | %s | %s |\n", r.TaskID, r.TaskClass, r.Duration.Round(time.Millisecond), status, r.ArtifactPath)
	}
	os.WriteFile(OutputDir+"/final/concurrency-report.md", concurrency.Bytes(), 0644)

	log.Printf("[Dispatcher] Artifacts written to %s/", OutputDir)
}

func executeTask(task ReportingTask, logFile *os.File) TaskResult {
	start := time.Now()
	result := TaskResult{
		TaskID:         task.ID,
		TaskClass:      task.Class,
		WorkerEndpoint: L1Endpoint,
		StartTime:      start,
	}

	// Build request
	reqBody := map[string]interface{}{
		"model": L1Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analysis assistant. Produce concise, useful reports. Use markdown format."},
			{"role": "user", "content": task.Prompt},
		},
		"max_tokens": 2048,
		"temperature": 0.3,
		// PHASE 23 P003: Disable thinking for llama.cpp useful tasks
		// Prevents 0.8B model from burning tokens on internal reasoning
		"chat_template_kwargs": map[string]interface{}{
			"enable_thinking": false,
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", L1Endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		result.Error = err.Error()
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	logEntry := fmt.Sprintf("[MLQ] task=%s class=%s level=1 endpoint=%s start=%s\n",
		task.ID, task.Class, L1Endpoint, start.Format(time.RFC3339Nano))
	logFile.WriteString(logEntry)

	resp, err := http.DefaultClient.Do(req)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)

	if err != nil {
		result.Error = err.Error()
		logFile.WriteString(fmt.Sprintf("[MLQ] task=%s FAIL: %s duration=%v\n", task.ID, err, result.Duration))
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(respBody, &chatResp)

	if chatResp.Error != nil {
		result.Error = chatResp.Error.Message
		logFile.WriteString(fmt.Sprintf("[MLQ] task=%s ERROR: %s duration=%v\n", task.ID, result.Error, result.Duration))
		return result
	}

	content := chatResp.Choices[0].Message.Content
	result.Success = true
	result.TokensPrompt = chatResp.Usage.PromptTokens
	result.TokensCompletion = chatResp.Usage.CompletionTokens

	// Write artifact
	artifactPath := fmt.Sprintf("%s/final/%s", OutputDir, task.Filename)
	os.WriteFile(artifactPath, []byte(content), 0644)
	result.ArtifactPath = artifactPath

	logFile.WriteString(fmt.Sprintf("[MLQ] task=%s SUCCESS level=1 endpoint=%s tokens=%d+%d duration=%v artifact=%s\n",
		task.ID, L1Endpoint, result.TokensPrompt, result.TokensCompletion, result.Duration, artifactPath))

	return result
}
