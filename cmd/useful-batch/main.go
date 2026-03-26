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

// zen-brain1 continuous useful-task batch launcher.
// Dispatches usefulness reporting tasks to L1 via direct HTTP (proven pattern).
// Records telemetry and produces indexed artifacts.
//
// ENV VARS:
//   BATCH_NAME       — batch identifier (default: "adhoc")
//   OUTPUT_ROOT      — artifact root (default: /tmp/zen-brain1-runs)
//   TASKS            — comma-separated task class names (default: all 10)
//   TIMEOUT          — per-task timeout in seconds (default: 300)
//   WORKERS          — max concurrent requests (default: 5)
//   L1_ENDPOINT      — L1 chat completions URL (default: http://localhost:56227/v1/chat/completions)
//   L1_MODEL         — L1 model name (default: Qwen3.5-0.8B-Q4_K_M.gguf)

const (
	defaultEndpoint = "http://localhost:56227/v1/chat/completions"
	defaultModel    = "Qwen3.5-0.8B-Q4_K_M.gguf"
)

type TaskClass struct {
	Title  string `yaml:"title" json:"title"`
	Prompt string `yaml:"prompt" json:"prompt"`
	Output string `yaml:"output" json:"output"`
}

// Built-in task classes (same as workload-schedule.yaml for standalone use)
var taskClasses = map[string]TaskClass{
	"dead_code":        {"Dead Code Report", "Scan the codebase for unreferenced exported functions in pkg/ and internal/. Produce a markdown report listing each function, its file, reference count, and recommendation. Do NOT generate Go code.", "dead-code.md"},
	"defects":          {"Defects Report", "Scan cmd/, internal/, pkg/ for common defect patterns: nil pointer dereference risk, unchecked error returns, missing mutex locks, hardcoded credentials. Produce a markdown checklist with severity. Do NOT generate Go code.", "defects.md"},
	"tech_debt":        {"Tech Debt Report", "Scan for TODO/FIXME/HACK comments, deprecated API usage, functions over 100 lines, packages with no tests. Produce a markdown report with severity ratings. Do NOT generate Go code.", "tech-debt.md"},
	"roadmap":          {"Roadmap Report", "Read docs/ directory to extract current project status and milestones. Produce a markdown summary: Completed, In Progress, Blocked, Next 30 Days. Do NOT generate Go code.", "roadmap.md"},
	"bug_hunting":      {"Bug Hunting Report", "Scan cmd/, internal/, pkg/ for race conditions (shared state without locks), memory leaks (unclosed resources), logic errors. Produce a markdown report. Do NOT generate Go code.", "bug-hunting.md"},
	"stub_hunting":     {"Stub Hunting Report", "Scan for empty function bodies, panic(not implemented), hardcoded return values, TODO-only functions. Produce a markdown checklist. Do NOT generate Go code.", "stub-hunting.md"},
	"package_hotspots": {"Package Hotspots Report", "Scan pkg/ and internal/ for packages with most exported types and functions. Produce a markdown table. Do NOT generate Go code.", "package-hotspots.md"},
	"test_gaps":        {"Test Gap Report", "Scan for _test.go files. List packages with and without tests. Produce a markdown report with coverage estimates. Do NOT generate Go code.", "test-gaps.md"},
	"config_drift":     {"Config Drift Report", "Compare documented policies in docs/ with actual config in config/policy/. Produce a markdown report identifying gaps. Do NOT generate Go code.", "config-policy-drift.md"},
	"executive_summary": {"Executive Summary", "Synthesize findings into a concise executive summary with top 5 findings and recommended actions. Produce a markdown summary. Do NOT generate Go code.", "executive-summary.md"},
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	batchName := envOr("BATCH_NAME", "adhoc")
	outputRoot := envOr("OUTPUT_ROOT", "/tmp/zen-brain1-runs")
	endpoint := envOr("L1_ENDPOINT", defaultEndpoint)
	model := envOr("L1_MODEL", defaultModel)
	timeoutSec := envInt("TIMEOUT", 300)
	maxWorkers := envInt("WORKERS", 5)

	// Resolve task list
	taskNames := allTaskNames()
	if t := os.Getenv("TASKS"); t != "" {
		taskNames = splitCSV(t)
	}

	tasks := make([]string, 0, len(taskNames))
	for _, name := range taskNames {
		if _, ok := taskClasses[name]; ok {
			tasks = append(tasks, name)
		} else {
			log.Printf("WARNING: unknown task class '%s', skipping", name)
		}
	}

	// Create run directory
	ts := time.Now().Format("20060102-150405")
	runDir := fmt.Sprintf("%s/%s/%s", outputRoot, batchName, ts)
	for _, sub := range []string{"final", "logs", "telemetry"} {
		os.MkdirAll(fmt.Sprintf("%s/%s", runDir, sub), 0755)
	}

	log.Printf("[BATCH] %s: dispatching %d tasks (workers=%d, timeout=%ds)", batchName, len(tasks), maxWorkers, timeoutSec)

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	var completed, succeeded atomic.Int64
	results := make([]map[string]interface{}, len(tasks))
	logFile, _ := os.Create(fmt.Sprintf("%s/logs/dispatch.log", runDir))
	start := time.Now()

	for i, taskName := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, name string) {
			defer wg.Done()
			defer func() { <-sem }()
			defer completed.Add(1)

			tc := taskClasses[name]
			taskStart := time.Now()
			r := map[string]interface{}{
				"task_id":    name,
				"title":      tc.Title,
				"lane":       "L1",
				"endpoint":   endpoint,
				"start_time": taskStart.Format(time.RFC3339Nano),
			}

			logFile.WriteString(fmt.Sprintf("[BATCH] task=%s START lane=L1\n", name))

			artifactPath := fmt.Sprintf("%s/final/%s", runDir, tc.Output)
			err := dispatchTask(endpoint, model, tc.Prompt, artifactPath, timeoutSec)

			taskEnd := time.Now()
			r["end_time"] = taskEnd.Format(time.RFC3339Nano)
			r["duration_ms"] = taskEnd.Sub(taskStart).Milliseconds()

			if err != nil {
				r["success"] = false
				r["error"] = err.Error()
				r["escalated"] = false
				log.Printf("[BATCH] ❌ %s (%s): %v", name, tc.Title, err)
				logFile.WriteString(fmt.Sprintf("[BATCH] task=%s FAIL duration=%v error=%s\n", name, taskEnd.Sub(taskStart), err))
			} else {
				r["success"] = true
				r["artifact_path"] = artifactPath
				r["escalated"] = false
				succeeded.Add(1)
				log.Printf("[BATCH] ✅ %s (%s): %v → %s", name, tc.Title, taskEnd.Sub(taskStart), tc.Output)
				logFile.WriteString(fmt.Sprintf("[BATCH] task=%s SUCCESS duration=%v artifact=%s\n", name, taskEnd.Sub(taskStart), tc.Output))
			}

			results[idx] = r
		}(i, taskName)
	}

	wg.Wait()
	wall := time.Since(start)
	logFile.Close()

	// Write artifact index
	index := map[string]interface{}{
		"batch_id":       fmt.Sprintf("%s-%s", batchName, ts),
		"batch_name":     batchName,
		"lane":           "L1",
		"total":          len(tasks),
		"succeeded":      succeeded.Load(),
		"failed":         len(tasks) - int(succeeded.Load()),
		"wall_ms":        wall.Milliseconds(),
		"run_dir":        runDir,
		"results":        results,
		"timestamp":      start.UTC().Format(time.RFC3339),
	}
	idxJSON, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(fmt.Sprintf("%s/telemetry/batch-index.json", runDir), idxJSON, 0644)

	log.Printf("[BATCH] === %s COMPLETE: %d/%d OK, wall=%v ===", batchName, succeeded.Load(), len(tasks), wall)
	log.Printf("[BATCH] Run dir: %s", runDir)

	if succeeded.Load() == 0 {
		os.Exit(1)
	}
}

func dispatchTask(endpoint, model, prompt, artifactPath string, timeoutSec int) error {
	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analysis assistant for a Go project called zen-brain1. Produce concise, factual, useful markdown reports. Do NOT generate Go code. Use markdown tables, bullet lists, and sections."},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 2048,
		"temperature": 0.3,
		"chat_template_kwargs": map[string]interface{}{"enable_thinking": false},
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("L1 request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var chatResp struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct{ Message string `json:"message"` } `json:"error"`
	}
	json.Unmarshal(respBody, &chatResp)

	if chatResp.Error != nil {
		return fmt.Errorf("L1 error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 || chatResp.Choices[0].Message.Content == "" {
		return fmt.Errorf("L1 empty response (in=%d, out=%d)", chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens)
	}

	return os.WriteFile(artifactPath, []byte(chatResp.Choices[0].Message.Content), 0644)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := parseInt(v); err == nil {
			return n
		}
	}
	return fallback
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func allTaskNames() []string {
	names := make([]string, 0, len(taskClasses))
	for k := range taskClasses {
		names = append(names, k)
	}
	return names
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range splitString(s, ',') {
		if part := trimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitString(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}
