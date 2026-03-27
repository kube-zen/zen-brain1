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

// zen-brain1 continuous useful-task batch launcher (PHASE 34 — 0.1-style rewrite).
//
// Design principles (copied from zen-brain 0.1):
//   - Code does deterministic prep (evidence gathering, clustering, shaping)
//   - Model gets a bounded, structured packet with real evidence
//   - Code does post-flight validation (grounding checks, repetition detection)
//   - Each task has a typed packet like 0.1's output_template
//
// ENV VARS:
//   BATCH_NAME       — batch identifier (default: "adhoc")
//   OUTPUT_ROOT      — artifact root (default: /tmp/zen-brain1-runs)
//   TASKS            — comma-separated task class names (default: all 10)
//   TIMEOUT          — per-task timeout in seconds (default: 300)
//   WORKERS          — max concurrent requests (default: 5)
//   L1_ENDPOINT      — L1 chat completions URL (default: http://localhost:56227/v1/chat/completions)
//   L1_MODEL         — L1 model name (default: Qwen3.5-0.8B-Q4_K_M.gguf)
//   REPO_ROOT        — repository root for evidence gathering (default: auto-detect)

const (
	defaultEndpoint = "http://localhost:56227/v1/chat/completions"
	defaultModel    = "Qwen3.5-0.8B-Q4_K_M.gguf"
)

// ─── Task Packet (0.1-style structured template) ───────────────────────

// ReportTaskDef defines a useful report task with 0.1-style explicit structure.
type ReportTaskDef struct {
	Name        string // task class key
	Title       string // human-readable title
	OutputFile  string // artifact filename
	Scope       string // what subsystems/files this task covers
	EvidenceCmd []EvidenceCmd
	OutputSpec  OutputSpec // required output sections
	Prompt      string     // the actual instruction (short, bounded)
}

// EvidenceCmd is a deterministic evidence-gathering command.
type EvidenceCmd struct {
	Label string // section header for this evidence
	Cmd   string // shell command to run
	Lines int    // max lines to keep (bounded)
}

// OutputSpec defines the required output structure (like 0.1's output_template).
type OutputSpec struct {
	RequiredSections []string // markdown headings that must appear
	MaxFindings      int      // max findings/items in report
	Format           string   // "table" | "checklist" | "bullets"
}

// ─── Task Definitions (0.1-style canonical packets) ───────────────────

var taskDefs = map[string]ReportTaskDef{
	"dead_code": {
		Name: "dead_code", Title: "Dead Code Report", OutputFile: "dead-code.md",
		Scope: "pkg/ and internal/ exported functions",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Exported functions in pkg/", Cmd: "grep -rn '^func [A-Z]' pkg/ 2>/dev/null | head -25", Lines: 30},
			{Label: "Exported functions in internal/", Cmd: "grep -rn '^func [A-Z]' internal/factory/ internal/foreman/ internal/llm/ internal/mlq/ internal/scheduler/ 2>/dev/null | head -25", Lines: 30},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 10, Format: "table"},
		Prompt: "Based on the evidence below, identify potentially unreferenced exported functions. For each, state the function name, file, and whether it appears unused. Produce at most 10 findings. Use only functions shown in the evidence.",
	},
	"defects": {
		Name: "defects", Title: "Defects Report", OutputFile: "defects.md",
		Scope: "cmd/, internal/, pkg/ code patterns",
		EvidenceCmd: []EvidenceCmd{
			{Label: "admission-gate code", Cmd: "head -50 cmd/admission-gate/main.go 2>/dev/null", Lines: 55},
			{Label: "factory ExecuteTask code", Cmd: "grep -A5 'func.*ExecuteTask\\|func.*Dispatch' internal/factory/factory.go 2>/dev/null | head -30", Lines: 35},
			{Label: "foreman reconciler error handling", Cmd: "grep -A3 'if err' internal/foreman/reconciler.go 2>/dev/null | head -20", Lines: 25},
			{Label: "ollama provider error handling", Cmd: "grep -A3 'if err' internal/llm/ollama_provider.go 2>/dev/null | head -20", Lines: 25},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 8, Format: "table"},
		Prompt: "Based on the code evidence below, find defect patterns: nil pointer risk, unchecked errors, hardcoded secrets, missing error handling. For each finding state: file, line pattern, severity (HIGH/MEDIUM/LOW), issue description. Produce at most 8 findings. Use only code shown in the evidence.",
	},
	"tech_debt": {
		Name: "tech_debt", Title: "Tech Debt Report", OutputFile: "tech-debt.md",
		Scope: "TODO/FIXME/HACK comments and file complexity",
		EvidenceCmd: []EvidenceCmd{
			{Label: "TODO/FIXME/HACK comments", Cmd: "grep -rn 'TODO\\|FIXME\\|HACK\\|XXX\\|DEPRECATED' cmd/ internal/ pkg/ 2>/dev/null | head -30", Lines: 35},
			{Label: "Largest source files", Cmd: "find cmd/ internal/ pkg/ -name '*.go' -exec wc -l {} \\; 2>/dev/null | sort -rn | head -10", Lines: 15},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 10, Format: "table"},
		Prompt: "Based on the evidence below, list TODO/FIXME/HACK items and overly large files. For each: file, line, type (TODO/FIXME/HACK), description. Produce at most 10 findings. Use only items shown in the evidence.",
	},
	"roadmap": {
		Name: "roadmap", Title: "Roadmap Report", OutputFile: "roadmap.md",
		Scope: "docs/ directory project status",
		EvidenceCmd: []EvidenceCmd{
			{Label: "docs/ structure", Cmd: "ls docs/ 2>/dev/null", Lines: 20},
			{Label: "Current state", Cmd: "cat CURRENT_STATE.md 2>/dev/null | head -40", Lines: 45},
			{Label: "Architecture progress", Cmd: "cat docs/01-ARCHITECTURE/PROGRESS.md 2>/dev/null | head -40", Lines: 45},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Completed", "In Progress", "Next Steps"}, MaxFindings: 5, Format: "bullets"},
		Prompt: "Based on the evidence below, summarize project status into three sections: Completed, In Progress, Next Steps. Use bullet points. Produce at most 5 items per section. Use only information from the evidence.",
	},
	"bug_hunting": {
		Name: "bug_hunting", Title: "Bug Hunting Report", OutputFile: "bug-hunting.md",
		Scope: "internal/ concurrency and error handling patterns",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Goroutine patterns with context", Cmd: "grep -B1 -A5 'go func' internal/factory/factory.go internal/foreman/reconciler.go internal/foreman/worker.go 2>/dev/null | head -30", Lines: 35},
			{Label: "Mutex/sync usage with context", Cmd: "grep -B1 -A3 'sync\\.Mutex\\|sync\\.RWMutex\\|\\.Lock()\\|\\.Unlock()' internal/ 2>/dev/null | head -25", Lines: 30},
			{Label: "Error handling in foreman dispatch", Cmd: "grep -A3 'if err' internal/foreman/reconciler.go 2>/dev/null | head -20", Lines: 25},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 8, Format: "table"},
		Prompt: "Based on the code evidence below, find potential bugs: race conditions, missing defer, unchecked errors, goroutine leaks. For each: file, pattern found, risk level (HIGH/MEDIUM/LOW), description. Produce at most 8 findings. Use only code shown in the evidence.",
	},
	"stub_hunting": {
		Name: "stub_hunting", Title: "Stub Hunting Report", OutputFile: "stub-hunting.md",
		Scope: "panic calls and short function bodies",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Panic calls with context", Cmd: "grep -B2 -A2 'panic(' cmd/ internal/ 2>/dev/null | head -25", Lines: 30},
			{Label: "Short exported functions", Cmd: "for f in internal/factory/*.go internal/foreman/*.go internal/llm/*.go; do awk -v f=\"$f\" '/^func [A-Z]/{name=$0; start=NR; body=\"\"} /^}/{n=NR-start; if(n>0 && n<=3) printf \"%s:%d %s (%d lines)\\n\",f,start,name,n}' \"$f\" 2>/dev/null; done | head -15", Lines: 20},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 10, Format: "table"},
		Prompt: "Based on the evidence below, find panic calls and very short functions (potential stubs). For each: file, function, type (panic/short-stub), description. Produce at most 10 findings. Use only items shown in the evidence.",
	},
	"package_hotspots": {
		Name: "package_hotspots", Title: "Package Hotspots Report", OutputFile: "package-hotspots.md",
		Scope: "pkg/ and internal/ package sizes",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Package file counts", Cmd: "find pkg/ internal/ -name '*.go' -exec dirname {} \\; 2>/dev/null | sort | uniq -c | sort -rn | head -15", Lines: 20},
			{Label: "Exported function count per file", Cmd: "find pkg/ internal/ -name '*.go' -exec sh -c 'echo $(grep -c \"^func [A-Z]\" \"$1\" 2>/dev/null) $1' _ {} \\; 2>/dev/null | sort -rn | head -15", Lines: 20},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 10, Format: "table"},
		Prompt: "Based on the evidence below, rank the top packages by complexity (file count and exported function count). For each: package path, file count, estimated complexity. Produce at most 10 entries. Use only packages shown in the evidence.",
	},
	"test_gaps": {
		Name: "test_gaps", Title: "Test Gap Report", OutputFile: "test-gaps.md",
		Scope: "packages with and without tests",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Packages WITH tests", Cmd: "find . -name '*_test.go' 2>/dev/null | sed 's|/[^/]*$||' | sort -u | grep -v vendor | grep -v '.artifacts' | head -15", Lines: 20},
			{Label: "Packages WITHOUT tests", Cmd: "comm -23 <(find cmd/ internal/ pkg/ -name '*.go' ! -name '*_test.go' 2>/dev/null | sed 's|/[^/]*$||' | sort -u) <(find . -name '*_test.go' 2>/dev/null | sed 's|/[^/]*$||' | sort -u) | head -15", Lines: 20},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Has Tests", "Missing Tests", "Summary"}, MaxFindings: 10, Format: "bullets"},
		Prompt: "Based on the evidence below, list packages with tests and packages missing tests. Two sections. Use bullet points. Produce at most 10 items total. Use only packages shown in the evidence.",
	},
	"config_drift": {
		Name: "config_drift", Title: "Config Drift Report", OutputFile: "config-policy-drift.md",
		Scope: "config/policy/ vs docs/05-OPERATIONS/",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Policy config files", Cmd: "ls -la config/ config/policy/ 2>/dev/null", Lines: 15},
			{Label: "Policy content", Cmd: "cat config/policy/default.yaml 2>/dev/null | head -30", Lines: 35},
			{Label: "Operations docs", Cmd: "ls docs/05-OPERATIONS/ 2>/dev/null | head -15", Lines: 20},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Top Findings", "Summary"}, MaxFindings: 8, Format: "bullets"},
		Prompt: "Based on the evidence below, compare the policy config with available operations docs. List any gaps: documented policy not in config, or config not documented. Produce at most 8 findings. Use only files shown in the evidence.",
	},
	"executive_summary": {
		Name: "executive_summary", Title: "Executive Summary", OutputFile: "executive-summary.md",
		Scope: "project state and current status",
		EvidenceCmd: []EvidenceCmd{
			{Label: "Current state", Cmd: "cat CURRENT_STATE.md 2>/dev/null | head -40", Lines: 45},
			{Label: "Recent git activity", Cmd: "git log --oneline -10 2>/dev/null", Lines: 15},
		},
		OutputSpec: OutputSpec{RequiredSections: []string{"Key Facts", "Recommended Actions"}, MaxFindings: 5, Format: "bullets"},
		Prompt: "Based on the evidence below, write a concise summary with exactly 2 sections: Key Facts (5 bullets) and Recommended Actions (3 bullets). Use only information from the evidence.",
	},
}

// ─── Evidence Gathering (deterministic prep) ──────────────────────────

// repoRoot resolves the repo root directory.
func repoRoot() string {
	if r := os.Getenv("REPO_ROOT"); r != "" {
		return r
	}
	exe, _ := os.Executable()
	return filepath.Dir(filepath.Dir(exe))
}

// runGather executes a shell command and returns bounded output.
func runGather(root, cmd string, maxLines int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	c.Dir = root
	out, err := c.CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

// buildEvidenceBundle runs all evidence commands for a task and produces
// a structured evidence bundle — code does the prep, model summarizes.
func buildEvidenceBundle(def ReportTaskDef, root string) string {
	var parts []string
	totalLines := 0
	maxTotal := 80 // keep total evidence under 80 lines for 0.8B context

	for _, ec := range def.EvidenceCmd {
		output := runGather(root, ec.Cmd, ec.Lines)
		if output == "" {
			continue
		}
		lines := strings.Split(output, "\n")
		// Reserve room for label
		remaining := maxTotal - totalLines
		if remaining <= 2 {
			break
		}
		if len(lines) > remaining-2 {
			lines = lines[:remaining-2]
		}
		part := fmt.Sprintf("### %s\n%s", ec.Label, strings.Join(lines, "\n"))
		parts = append(parts, part)
		totalLines += len(lines) + 2
	}

	if len(parts) == 0 {
		return "(no evidence gathered)"
	}
	return strings.Join(parts, "\n\n")
}

// ─── Prompt Builder (0.1-style bounded packet) ────────────────────────

// buildTaskPacket constructs the full prompt as a structured task packet.
// Model gets: evidence bundle → scope → output spec → instruction.
func buildTaskPacket(def ReportTaskDef, evidence string) string {
	var sb strings.Builder

	// Section 1: Evidence bundle (what code already found)
	sb.WriteString("# Evidence Bundle\n\n")
	sb.WriteString(evidence)
	sb.WriteString("\n\n")

	// Section 2: Scope
	sb.WriteString("# Scope\n\n")
	sb.WriteString(def.Scope)
	sb.WriteString("\n\n")

	// Section 3: Output Requirements
	sb.WriteString("# Output Requirements\n\n")
	sb.WriteString(fmt.Sprintf("Format: %s\n", def.OutputSpec.Format))
	sb.WriteString(fmt.Sprintf("Max findings: %d\n", def.OutputSpec.MaxFindings))
	sb.WriteString("Required sections: ")
	for i, s := range def.OutputSpec.RequiredSections {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(s)
	}
	sb.WriteString("\n\n")

	// Section 4: Instruction (short, bounded)
	sb.WriteString("# Task\n\n")
	sb.WriteString(def.Prompt)
	sb.WriteString("\n\n")

	// Grounding constraint
	sb.WriteString("# Constraints\n\n")
	sb.WriteString("- Only reference files and data shown in the Evidence Bundle above\n")
	sb.WriteString("- Do not invent file paths, function names, or findings\n")
	sb.WriteString("- If evidence is empty or insufficient, say so explicitly\n")

	return sb.String()
}

// ─── MaxFindings Enforcement ──────────────────────────────────────────

// enforceMaxFindings trims a report to keep only the first N findings.
// Findings are identified by markdown table rows (lines starting with |).
// The header row and separator row are preserved.
func enforceMaxFindings(content string, maxFindings int) (string, bool) {
	lines := strings.Split(content, "\n")
	var result []string
	tableRows := 0
	trimmed := false

	for _, line := range lines {
		// Count table data rows (skip header and separator)
		if strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "| ---") && !strings.HasPrefix(line, "|---") && !strings.HasPrefix(line, "| #") {
			// Check if this looks like a header row (e.g., "| # | File |")
			cells := strings.Count(line, "|")
			if cells >= 3 && (strings.Contains(line, "#") || strings.Contains(line, "File") || strings.Contains(line, "Finding")) && tableRows == 0 {
				result = append(result, line)
				continue
			}
			tableRows++
			if tableRows > maxFindings {
				trimmed = true
				continue
			}
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n"), trimmed
}

// ─── Output Validation (fail-closed) ──────────────────────────────────

type ValidationResult struct {
	Status string   // success, success-needs-review, artifact-fail, context-fail
	Issues []string
}

func validateReport(content, taskClass, root string) ValidationResult {
	vr := ValidationResult{}

	// Check 1: minimum size
	if len(content) < 150 {
		vr.Status = "artifact-fail"
		vr.Issues = append(vr.Issues, fmt.Sprintf("too short: %d bytes (< 150)", len(content)))
		return vr
	}

	// Check 2: has markdown structure
	if !strings.Contains(content, "#") {
		vr.Status = "artifact-fail"
		vr.Issues = append(vr.Issues, "no markdown headings")
		return vr
	}

	// Check 3: required sections present
	if def, ok := taskDefs[taskClass]; ok {
		for _, section := range def.OutputSpec.RequiredSections {
			// Check for section as heading or bold text
			if !strings.Contains(content, section) {
				vr.Issues = append(vr.Issues, fmt.Sprintf("missing required section: %q", section))
			}
		}
	}

	// Check 4: repetition detection
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
			if vr.Status == "" || vr.Status == "success-needs-review" {
				vr.Status = "context-fail"
			}
			vr.Issues = append(vr.Issues, fmt.Sprintf("line repeated %dx: %q", count, line))
		}
	}

	// Check 5: file reference grounding (for code analysis tasks)
	if taskClass != "executive_summary" && taskClass != "roadmap" {
		refCount := 0
		existCount := 0
		for _, field := range strings.Fields(content) {
			field = strings.Trim(field, "`*_[](){}:")
			if looksLikeGoPath(field) {
				refCount++
				if _, err := os.Stat(filepath.Join(root, field)); err == nil {
					existCount++
				}
			}
		}
		if refCount >= 3 {
			ratio := float64(existCount) / float64(refCount)
			if ratio < 0.3 {
				vr.Issues = append(vr.Issues, fmt.Sprintf("file grounding: %d/%d refs exist (%.0f%%)", existCount, refCount, ratio*100))
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

func looksLikeGoPath(s string) bool {
	return strings.Contains(s, ".go") && strings.HasPrefix(s, "internal/") || strings.HasPrefix(s, "pkg/") || strings.HasPrefix(s, "cmd/")
}

// ─── Main ──────────────────────────────────────────────────────────────

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	batchName := envOr("BATCH_NAME", "adhoc")
	outputRoot := envOr("OUTPUT_ROOT", "/tmp/zen-brain1-runs")
	endpoint := envOr("L1_ENDPOINT", defaultEndpoint)
	model := envOr("L1_MODEL", defaultModel)
	timeoutSec := envInt("TIMEOUT", 300)
	maxWorkers := envInt("WORKERS", 5)
	root := repoRoot()

	log.Printf("[BATCH] %s: repo=%s dispatching useful tasks (workers=%d, timeout=%ds)", batchName, root, maxWorkers, timeoutSec)

	// Resolve task list
	taskNames := allTaskNames()
	if t := os.Getenv("TASKS"); t != "" {
		taskNames = splitCSV(t)
	}
	var tasks []string
	for _, name := range taskNames {
		if _, ok := taskDefs[name]; ok {
			tasks = append(tasks, name)
		} else {
			log.Printf("[BATCH] WARNING: unknown task class %q, skipping", name)
		}
	}

	// Create run directory
	ts := time.Now().Format("20060102-150405")
	runDir := fmt.Sprintf("%s/%s/%s", outputRoot, batchName, ts)
	for _, sub := range []string{"final", "logs", "telemetry"} {
		os.MkdirAll(fmt.Sprintf("%s/%s", runDir, sub), 0755)
	}

	log.Printf("[BATCH] %s: %d tasks queued, dispatching...", batchName, len(tasks))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	var succeeded atomic.Int64
	results := make([]map[string]interface{}, len(tasks))
	logFile, _ := os.Create(fmt.Sprintf("%s/logs/dispatch.log", runDir))
	start := time.Now()

	for i, taskName := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, name string) {
			defer wg.Done()
			defer func() { <-sem }()

			def := taskDefs[name]
			workItemID := fmt.Sprintf("%s-%s-%d", batchName, ts, idx)
			taskStart := time.Now()
			r := map[string]interface{}{
				"work_item_id": workItemID,
				"task_class":   name,
				"title":        def.Title,
				"lane":         "L1",
				"state":        "in_progress",
				"start_time":   taskStart.Format(time.RFC3339Nano),
			}

			logFile.WriteString(fmt.Sprintf("[TASK] %s START state=in_progress lane=L1\n", workItemID))

			// ── Step 1: Deterministic evidence prep (code, not AI) ──
			evidence := buildEvidenceBundle(def, root)
			evidenceBytes := len(evidence)
			log.Printf("[EVIDENCE] %s: %d bytes structured evidence gathered", workItemID, evidenceBytes)

			// ── Step 2: Build bounded task packet (0.1-style) ──
			packet := buildTaskPacket(def, evidence)
			log.Printf("[PACKET] %s: task packet built (%d bytes)", workItemID, len(packet))

			// ── Step 3: Dispatch to L1 ──
			artifactPath := fmt.Sprintf("%s/final/%s", runDir, def.OutputFile)
			err := dispatchTask(endpoint, model, workItemID, packet, artifactPath, timeoutSec)
			taskEnd := time.Now()
			r["end_time"] = taskEnd.Format(time.RFC3339Nano)
			r["duration_ms"] = taskEnd.Sub(taskStart).Milliseconds()

			if err != nil {
				r["success"] = false
				r["state"] = "failed"
				r["error"] = err.Error()
				log.Printf("[BATCH] ❌ %s: %v", workItemID, err)
				logFile.WriteString(fmt.Sprintf("[TASK] %s FAIL error=%s\n", workItemID, err))
			} else {
				// ── Step 4: Post-flight MaxFindings enforcement ──
			if def.OutputSpec.MaxFindings > 0 {
				artifactContent, readErr := os.ReadFile(artifactPath)
				if readErr == nil {
					enforced, wasTrimmed := enforceMaxFindings(string(artifactContent), def.OutputSpec.MaxFindings)
					if wasTrimmed {
						if writeErr := os.WriteFile(artifactPath, []byte(enforced), 0644); writeErr == nil {
							log.Printf("[MAXFINDINGS] %s: trimmed to %d findings", workItemID, def.OutputSpec.MaxFindings)
							r["maxfindings_trimmed"] = true
						}
					}
				}
			}

			// ── Step 5: Fail-closed validation ──
				artifactContent, readErr := os.ReadFile(artifactPath)
				var vr ValidationResult
				if readErr != nil {
					vr = ValidationResult{Status: "artifact-fail", Issues: []string{readErr.Error()}}
				} else {
					vr = validateReport(string(artifactContent), name, root)
				}
				r["validation_status"] = vr.Status
				r["validation_issues"] = vr.Issues

				switch vr.Status {
				case "success":
					r["success"] = true
					r["state"] = "done"
					succeeded.Add(1)
					log.Printf("[BATCH] ✅ %s: %v → %s [valid=success]", workItemID, taskEnd.Sub(taskStart), def.OutputFile)
				case "success-needs-review":
					r["success"] = true
					r["state"] = "needs_review"
					succeeded.Add(1)
					log.Printf("[BATCH] ⚠️ %s: %v → %s [valid=needs-review issues=%v]", workItemID, taskEnd.Sub(taskStart), def.OutputFile, vr.Issues)
				default:
					r["success"] = false
					r["state"] = "validation_fail"
					log.Printf("[BATCH] ❌ %s: validation-fail status=%s issues=%v", workItemID, vr.Status, vr.Issues)
				}
				logFile.WriteString(fmt.Sprintf("[TASK] %s DONE state=%s validation=%s duration=%v\n", workItemID, r["state"], vr.Status, taskEnd.Sub(taskStart)))
			}

			results[idx] = r
		}(i, taskName)
	}

	wg.Wait()
	wall := time.Since(start)
	logFile.Close()

	// Write telemetry
	index := map[string]interface{}{
		"batch_id":  fmt.Sprintf("%s-%s", batchName, ts),
		"lane":      "L1",
		"total":     len(tasks),
		"succeeded": succeeded.Load(),
		"failed":    len(tasks) - int(succeeded.Load()),
		"wall_ms":   wall.Milliseconds(),
		"run_dir":   runDir,
		"results":   results,
		"timestamp": start.UTC().Format(time.RFC3339),
	}
	idxJSON, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(fmt.Sprintf("%s/telemetry/batch-index.json", runDir), idxJSON, 0644)

	log.Printf("[BATCH] === %s: %d/%d OK, wall=%v ===", batchName, succeeded.Load(), len(tasks), wall)
	if succeeded.Load() == 0 {
		os.Exit(1)
	}
}

// ─── L1 Dispatch ──────────────────────────────────────────────────────

func dispatchTask(endpoint, model, workItemID, packet, artifactPath string, timeoutSec int) error {
	log.Printf("[NO-THINK] enable_thinking=false active for %s", workItemID)

	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a code analyst. Write short factual markdown. Only use files and data from the Evidence Bundle. Do not invent anything. Do not generate Go code."},
			{"role": "user", "content": packet},
		},
		"max_tokens":             2048,
		"temperature":            0.3,
		"chat_template_kwargs":   map[string]interface{}{"enable_thinking": false},
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

// ─── Utility ──────────────────────────────────────────────────────────

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
	names := make([]string, 0, len(taskDefs))
	for k := range taskDefs {
		names = append(names, k)
	}
	return names
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part := strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
