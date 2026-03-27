package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	defaultEndpoint  = "http://localhost:56227/v1/chat/completions"
	defaultModel     = "Qwen3.5-0.8B-Q4_K_M.gguf"
	evidenceMaxLines = 150 // bound evidence to ~150 lines per task
)

// repoRoot resolves the repo root directory (default: parent of binary location).
func repoRoot() string {
	if r := os.Getenv("REPO_ROOT"); r != "" {
		return r
	}
	exe, _ := os.Executable()
	return filepath.Dir(filepath.Dir(exe))
}

// runGather executes a shell command in the repo root and returns output trimmed to maxLines.
func runGather(repoRoot, cmd string, maxLines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir = repoRoot
	out, err := c.CombinedOutput()
	if err != nil {
		log.Printf("[EVIDENCE] gather command failed: %s — %v", cmd, err)
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, fmt.Sprintf("... (truncated to %d lines)", maxLines))
	}
	return strings.Join(lines, "\n")
}

// gatherEvidence produces real repo evidence for each task class.
func gatherEvidence(taskClass, root string) string {
	switch taskClass {
	case "dead_code":
		return runGather(root, "grep -rn '^func ' pkg/ internal/ 2>/dev/null | head -50", evidenceMaxLines)
	case "defects":
		return runGather(root, "find cmd/ internal/ pkg/ -name '*.go' 2>/dev/null | head -40", evidenceMaxLines)
	case "tech_debt":
		todo := runGather(root, "grep -rn 'TODO\\|FIXME\\|HACK\\|XXX\\|DEPRECATED' cmd/ internal/ pkg/ 2>/dev/null | head -60", 80)
		longFuncs := runGather(root, "find cmd/ internal/ pkg/ -name '*.go' -exec wc -l {} \\; 2>/dev/null | sort -rn | head -15", 20)
		return todo + "\n\n## Long files (by line count)\n" + longFuncs
	case "roadmap":
		ls := runGather(root, "ls docs/ 2>/dev/null", 20)
		progress := runGather(root, "cat docs/01-ARCHITECTURE/PROGRESS.md 2>/dev/null | head -60", 60)
		changelog := runGather(root, "cat CHANGELOG.md 2>/dev/null | head -30", 30)
		return "## docs/ listing\n" + ls + "\n\n## PROGRESS.md\n" + progress + "\n\n## CHANGELOG.md\n" + changelog
	case "bug_hunting":
		return runGather(root, "find cmd/ internal/ pkg/ -name '*.go' 2>/dev/null | head -40", evidenceMaxLines)
	case "stub_hunting":
		funcs := runGather(root, "grep -rn '^func ' cmd/ internal/ pkg/ 2>/dev/null | head -40", 60)
		panics := runGather(root, "grep -rn 'panic(' cmd/ internal/ pkg/ 2>/dev/null | head -20", 25)
		return "## Exported functions\n" + funcs + "\n\n## Panic calls\n" + panics
	case "package_hotspots":
		return runGather(root, "find pkg/ internal/ -name '*.go' -exec dirname {} \\; 2>/dev/null | sort | uniq -c | sort -rn | head -25", evidenceMaxLines)
	case "test_gaps":
		withTest := runGather(root, "find . -name '*_test.go' 2>/dev/null | sed 's|/[^/]*$||' | sort -u | head -25", 30)
		withoutTest := runGather(root, "find cmd/ internal/ pkg/ -name '*.go' ! -name '*_test.go' 2>/dev/null | sed 's|/[^/]*$||' | sort -u | head -25", 30)
		return "## Packages WITH tests\n" + withTest + "\n\n## Packages WITHOUT tests\n" + withoutTest
	case "config_drift":
		configFiles := runGather(root, "find config/ -name '*.yaml' 2>/dev/null | head -20", 25)
		docsFiles := runGather(root, "ls docs/05-OPERATIONS/ 2>/dev/null | head -20", 25)
		return "## Config files\n" + configFiles + "\n\n## Operations docs\n" + docsFiles
	case "executive_summary":
		state := runGather(root, "cat CURRENT_STATE.md 2>/dev/null | head -50", 50)
		return state
	default:
		return runGather(root, "ls cmd/ internal/ pkg/ 2>/dev/null | head -30", 30)
	}
}

// ValidationResult captures the outcome of output validation.
type ValidationResult struct {
	Status string   // success, success-needs-review, artifact-fail, context-fail
	Issues []string // human-readable reasons
}

// validateReport checks that report output is grounded and non-trivial.
func validateReport(content, taskClass, root string) ValidationResult {
	vr := ValidationResult{}

	// Check 1: non-empty
	if len(content) < 200 {
		vr.Status = "artifact-fail"
		vr.Issues = append(vr.Issues, fmt.Sprintf("output too short: %d bytes (< 200)", len(content)))
		return vr
	}

	// Check 2: has markdown structure
	if !strings.Contains(content, "#") {
		vr.Status = "artifact-fail"
		vr.Issues = append(vr.Issues, "no markdown headings found")
		return vr
	}

	// Check 3: repetition detection — if any line appears 5+ times, it's template-only hallucination
	lineCounts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCounts[line]++
	}
	for line, count := range lineCounts {
		if count >= 5 {
			vr.Status = "context-fail"
			vr.Issues = append(vr.Issues, fmt.Sprintf("repeated line %dx: %q", count, line))
		}
	}

	// Check 4: file reference grounding — extract paths that look like Go files, check if they exist
	if taskClass != "executive_summary" && taskClass != "roadmap" {
		refCount := 0
		existCount := 0
		for _, field := range strings.Fields(content) {
			field = strings.Trim(field, "`*_[](){}:")
			if looksLikeGoPath(field) {
				refCount++
				fullPath := filepath.Join(root, field)
				if _, err := os.Stat(fullPath); err == nil {
					existCount++
				}
			}
		}
		if refCount > 0 {
			ratio := float64(existCount) / float64(refCount)
			if ratio < 0.3 && refCount >= 3 {
				vr.Issues = append(vr.Issues, fmt.Sprintf("file grounding low: %d/%d refs exist (%.0f%%)", existCount, refCount, ratio*100))
				if vr.Status == "" {
					vr.Status = "success-needs-review"
				}
			}
		}
	}

	if vr.Status == "" {
		vr.Status = "success"
	}
	return vr
}

// looksLikeGoPath checks if a string looks like a Go file reference.
func looksLikeGoPath(s string) bool {
	if !strings.Contains(s, ".go") {
		return false
	}
	// Must have at least one path separator or be a bare .go filename
	if strings.Contains(s, "/") || (strings.HasPrefix(s, "internal/") || strings.HasPrefix(s, "pkg/") || strings.HasPrefix(s, "cmd/")) {
		return strings.HasSuffix(s, ".go")
	}
	return false
}

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

	// Resolve repo root for evidence gathering
	root := repoRoot()
	log.Printf("[BATCH] repo_root=%s", root)

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

			// P1: Gather real repo evidence for this task
			evidence := gatherEvidence(name, root)
			if evidence != "" {
				log.Printf("[EVIDENCE] %s: gathered %d bytes of real repo context", name, len(evidence))
			} else {
				log.Printf("[EVIDENCE] %s: WARNING — no evidence gathered", name)
			}

			// Build enriched prompt with evidence
			enrichedPrompt := tc.Prompt
			if evidence != "" {
				enrichedPrompt = fmt.Sprintf("## Codebase Evidence (real, from live repo scan)\n```\n%s\n```\n\n## Your Task\n%s\n\nIMPORTANT: Only reference files and paths shown in the evidence above. Do NOT invent file paths.", evidence, tc.Prompt)
			}

			artifactPath := fmt.Sprintf("%s/final/%s", runDir, tc.Output)
			err := dispatchTask(endpoint, model, name, enrichedPrompt, artifactPath, timeoutSec)

			taskEnd := time.Now()
			r["end_time"] = taskEnd.Format(time.RFC3339Nano)
			r["duration_ms"] = taskEnd.Sub(taskStart).Milliseconds()

			if err != nil {
				r["success"] = false
				r["error"] = err.Error()
				r["escalated"] = false
				r["validation"] = "dispatch-fail"
				log.Printf("[BATCH] ❌ %s (%s): %v", name, tc.Title, err)
				logFile.WriteString(fmt.Sprintf("[BATCH] task=%s FAIL duration=%v error=%s\n", name, taskEnd.Sub(taskStart), err))
			} else {
				// P2: Validate report output
				artifactContent, readErr := os.ReadFile(artifactPath)
				var vr ValidationResult
				if readErr != nil {
					vr = ValidationResult{Status: "artifact-fail", Issues: []string{"cannot read artifact: " + readErr.Error()}}
				} else {
					vr = validateReport(string(artifactContent), name, root)
				}
				r["validation_status"] = vr.Status
				r["validation_issues"] = vr.Issues

				if vr.Status == "success" {
					r["success"] = true
					r["artifact_path"] = artifactPath
					r["escalated"] = false
					succeeded.Add(1)
					log.Printf("[BATCH] ✅ %s (%s): %v → %s [valid=%s]", name, tc.Title, taskEnd.Sub(taskStart), tc.Output, vr.Status)
				} else if vr.Status == "success-needs-review" {
					r["success"] = true
					r["artifact_path"] = artifactPath
					r["escalated"] = false
					succeeded.Add(1)
					log.Printf("[BATCH] ⚠️ %s (%s): %v → %s [valid=%s issues=%v]", name, tc.Title, taskEnd.Sub(taskStart), tc.Output, vr.Status, vr.Issues)
				} else {
					r["success"] = false
					r["artifact_path"] = artifactPath
					r["escalated"] = false
					r["validation_fail"] = true
					log.Printf("[BATCH] ❌ %s (%s): validation-fail status=%s issues=%v", name, tc.Title, vr.Status, vr.Issues)
				}
				logFile.WriteString(fmt.Sprintf("[BATCH] task=%s DONE validation=%s issues=%v duration=%v\n", name, vr.Status, vr.Issues, taskEnd.Sub(taskStart)))
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

func dispatchTask(endpoint, model, taskClass, prompt, artifactPath string, timeoutSec int) error {
	// P3: Log no-think status (enable_thinking:false is already in the request body below)
	log.Printf("[NO-THINK] enable_thinking=false active for %s", taskClass)

	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analysis assistant for a Go project called zen-brain1. Produce concise, factual, useful markdown reports. Do NOT generate Go code. Use markdown tables, bullet lists, and sections. IMPORTANT: Only reference files and paths provided in the evidence. Do NOT invent or fabricate file paths."},
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
