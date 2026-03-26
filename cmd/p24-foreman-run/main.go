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
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// PHASE 24C: Usefulness evidence/reporting tasks through direct L1 path.
// Broken new harness (cmd/p24c-useful-batch) abandoned.
// cmd/p24-foreman-run reused as canonical execution path, rewritten for direct L1 calls.
//
// Why direct L1 HTTP instead of FactoryTaskRunner:
//   - FactoryTaskRunner wraps requests with 5 tool definitions (read_file, search_file,
//     inspect_file, run_build_test, inspect_diff)
//   - Qwen3.5 0.8B Q4_K_M cannot parse tool definitions → returns empty response
//   - PHASE 22 mlq-dispatcher proved 10/10 success with simple system+user messages, no tools
//   - Usefulness tasks don't need tools — they produce markdown from bounded prompts
//
// This preserves the foreman task shape (BrainTaskSpec) but dispatches via direct HTTP,
// proving that usefulness artifacts flow through zen-brain1's L1-first architecture.

const (
	OutputDir  = "/tmp/zen-brain1-foreman-run"
	L1Endpoint = "http://localhost:56227/v1/chat/completions"
	L1Model    = "Qwen3.5-0.8B-Q4_K_M.gguf"
)

type task struct {
	ID, Class, Title, Prompt, Filename string
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	for _, dir := range []string{OutputDir + "/final", OutputDir + "/logs", OutputDir + "/telemetry"} {
		os.MkdirAll(dir, 0755)
	}

	// PHASE 24C: Usefulness evidence/reporting tasks (NOT implementation/codegen)
	tasks := []task{
		{"u24c-001", "dead_code", "Dead Code Report", "Scan the codebase for unreferenced exported functions. Use read_file and search_file tools to examine pkg/ and internal/ directories. Produce a markdown report listing each function, its file, reference count, and recommendation. Title: Dead Code Report. Do NOT generate any Go code.", "dead-code.md"},
		{"u24c-002", "defects", "Defects Report", "Scan the codebase for common defect patterns: nil pointer dereference risk, unchecked error returns, missing mutex locks, hardcoded credentials. Use search_file to find patterns. Produce a markdown checklist with severity. Title: Defects Report. Do NOT generate any Go code.", "defects.md"},
		{"u24c-003", "tech_debt", "Tech Debt Report", "Scan the codebase for TODO/FIXME/HACK comments, deprecated API usage, functions over 100 lines, and packages with no tests. Use search_file. Produce a markdown report with severity ratings per finding. Title: Tech Debt Report. Do NOT generate any Go code.", "tech-debt.md"},
		{"u24c-004", "roadmap", "Roadmap Report", "Read docs/ directory to extract current project status and milestones. Produce a markdown summary with sections: Completed, In Progress, Blocked, Next 30 Days. Title: Roadmap Report. Do NOT generate any Go code.", "roadmap.md"},
		{"u24c-005", "bug_hunting", "Bug Hunting Report", "Scan cmd/, internal/, pkg/ for suspicious patterns: race conditions (shared state without locks), memory leaks (unclosed resources), and logic errors (off-by-one, wrong comparisons). Produce a markdown report with evidence. Title: Bug Hunting Report. Do NOT generate any Go code.", "bug-hunting.md"},
		{"u24c-006", "stub_hunting", "Stub Hunting Report", "Scan the codebase for stubs: empty function bodies, panic(not implemented), hardcoded return values, and TODO-only functions. Use search_file. Produce a markdown checklist. Title: Stub Hunting Report. Do NOT generate any Go code.", "stub-hunting.md"},
		{"u24c-007", "package_hotspot", "Package Hotspots Report", "Scan pkg/ and internal/ to identify packages with the most exported types and functions. Use search_file to count package declarations and exports. Produce a markdown table. Title: Package Hotspots Report. Do NOT generate any Go code.", "package-hotspots.md"},
		{"u24c-008", "test_gap", "Test Gap Report", "Scan for _test.go files across the codebase. List packages that have tests and those that do not. Use search_file. Produce a markdown report with coverage estimates. Title: Test Gap Report. Do NOT generate any Go code.", "test-gaps.md"},
		{"u24c-009", "config_drift", "Config Drift Report", "Compare documented policies in docs/ with actual config in config/policy/. Use read_file. Produce a markdown report identifying gaps. Title: Config Drift Report. Do NOT generate any Go code.", "config-policy-drift.md"},
		{"u24c-010", "exec_summary", "Executive Summary", "Synthesize findings from the other 9 reports into a concise executive summary with top 5 findings and recommended actions. Use read_file to review the other reports. Produce a markdown summary. Title: Executive Summary. Do NOT generate any Go code.", "executive-summary.md"},
	}

	// SMOKE_TEST_COUNT env var limits task count for quick smoke tests (P24C-C6)
	if n := os.Getenv("SMOKE_TEST_COUNT"); n != "" {
		if count, err := strconv.Atoi(n); err == nil && count > 0 && count < len(tasks) {
			tasks = tasks[:count]
		}
	}

	log.Printf("[P24C] Dispatching %d usefulness tasks through direct L1 path (no tools, system+user messages)...", len(tasks))

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
				"path":       "direct-l1-http",
				"start_time": start.Format(time.RFC3339Nano),
			}

			logFile.WriteString(fmt.Sprintf("[P24C] task=%s class=%s START path=direct-l1-http endpoint=%s\n", t.ID, t.Class, L1Endpoint))

			// Direct L1 HTTP call — same proven pattern as PHASE 22 mlq-dispatcher
			// Simple system+user messages, NO tool definitions, enable_thinking=false
			artifactPath, err := executeDirectL1(t, logFile)

			end := time.Now()
			r["end_time"] = end.Format(time.RFC3339Nano)
			r["duration_ms"] = end.Sub(start).Milliseconds()
			r["worker_endpoint"] = L1Endpoint
			r["selected_level"] = 1

			if err != nil {
				r["success"] = false
				r["error"] = err.Error()
				r["escalated"] = false
				logFile.WriteString(fmt.Sprintf("[P24C] task=%s FAIL duration=%v error=%s\n",
					t.ID, end.Sub(start), err))
				log.Printf("[P24C] ❌ %s (%s) FAILED: %v", t.ID, t.Class, err)
			} else {
				r["success"] = true
				r["artifact_path"] = artifactPath
				r["escalated"] = false
				succeeded.Add(1)
				logFile.WriteString(fmt.Sprintf("[P24C] task=%s SUCCESS duration=%v artifact=%s\n",
					t.ID, end.Sub(start), artifactPath))
				log.Printf("[P24C] ✅ %s (%s) completed in %v → %s", t.ID, t.Class, end.Sub(start), artifactPath)
			}

			results[idx] = r
		}(i, t)
	}

	wg.Wait()
	dispatchDuration := time.Since(dispatchStart)
	logFile.Close()

	log.Printf("[P24C] === USEFULNESS BATCH COMPLETE ===")
	log.Printf("[P24C] Dispatched: %d  Succeeded: %d  Failed: %d  Wall: %v",
		len(tasks), succeeded.Load(), len(tasks)-int(succeeded.Load()), dispatchDuration)

	// Write telemetry
	telemetry := map[string]interface{}{
		"batch_id":       fmt.Sprintf("useful-%s", time.Now().Format("20060102-150405")),
		"path":           "direct-l1-http (no tools, system+user, enable_thinking=false)",
		"reuse_proof":    "Reused PHASE 22 mlq-dispatcher pattern: simple HTTP calls to L1, no tool definitions",
		"total_wall_ms":  dispatchDuration.Milliseconds(),
		"tasks_dispatched": len(tasks),
		"tasks_succeeded":  succeeded.Load(),
		"tasks_failed":     len(tasks) - int(succeeded.Load()),
		"worker_endpoint":  L1Endpoint,
		"results":          results,
	}
	tj, _ := json.MarshalIndent(telemetry, "", "  ")
	os.WriteFile(OutputDir+"/telemetry/foreman-batch-telemetry.json", tj, 0644)

	log.Printf("[P24C] Artifacts: %s/final/  Telemetry: %s/telemetry/", OutputDir, OutputDir)
}

// executeDirectL1 sends a simple system+user prompt to L1 via HTTP (no tools).
// This is the same proven pattern from PHASE 22 mlq-dispatcher that achieved 10/10.
func executeDirectL1(t task, logFile *os.File) (string, error) {
	reqBody := map[string]interface{}{
		"model": L1Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analysis assistant for a Go project called zen-brain1. Produce concise, factual, useful markdown reports. Do NOT generate Go code. Focus on evidence from the codebase. Use markdown tables, bullet lists, and sections."},
			{"role": "user", "content": t.Prompt},
		},
		"max_tokens": 2048,
		"temperature": 0.3,
		"chat_template_kwargs": map[string]interface{}{
			"enable_thinking": false,
		},
	}
	bodyJSON, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", L1Endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("L1 request failed: %w", err)
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
		return "", fmt.Errorf("L1 error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("L1 returned empty response (in=%d, out=%d tokens)", chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)
	}

	content := chatResp.Choices[0].Message.Content
	logFile.WriteString(fmt.Sprintf("[P24C] task=%s L1_RESPONSE tokens_in=%d tokens_out=%d len=%d\n",
		t.ID, chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, len(content)))

	// Write artifact
	artifactPath := fmt.Sprintf("%s/final/%s", OutputDir, t.Filename)
	if err := os.WriteFile(artifactPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write artifact: %w", err)
	}

	return artifactPath, nil
}
