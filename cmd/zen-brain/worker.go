// Package main: worker subcommands (remediate, batch, ticketize).
// Consolidated from cmd/remediation-worker, cmd/useful-batch, cmd/finding-ticketizer.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func runWorkerCommand() {
	if len(os.Args) < 3 {
		printWorkerUsage()
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "remediate":
		runWorkerRemediate()
	case "batch":
		runWorkerBatch()
	case "ticketize":
		runWorkerTicketize()
	case "-h", "--help":
		printWorkerUsage()
	default:
		fmt.Printf("Unknown worker subcommand: %s\n", sub)
		printWorkerUsage()
		os.Exit(1)
	}
}

func printWorkerUsage() {
	fmt.Println("Usage: zen-brain worker <subcommand> [options]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  remediate   Run remediation worker (fix tasks from Jira)")
	fmt.Println("  batch       Run useful-batch worker (continuous discovery tasks)")
	fmt.Println("  ticketize   Run finding-ticketizer (convert findings to Jira tickets)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  zen-brain worker remediate --task-key ZB-123")
	fmt.Println("  zen-brain worker batch --once")
	fmt.Println("  zen-brain worker ticketize --dir /var/lib/zen-brain1/artifacts")
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: REMEDIATE (from cmd/remediation-worker)
// ═══════════════════════════════════════════════════════════════════════════════

type remediationConfig struct {
	JiraBaseURL   string
	JiraEmail     string
	JiraAPIToken  string
	ModelProvider string
	ModelName     string
	WorkDir       string
	EvidenceDir   string
	MaxRetries    int
	TaskTimeout   time.Duration
	DryRun        bool
}

func runWorkerRemediate() {
	// Parse flags
	cfg := remediationConfig{
		JiraBaseURL:   os.Getenv("JIRA_URL"),
		JiraEmail:     os.Getenv("JIRA_EMAIL"),
		JiraAPIToken:  os.Getenv("JIRA_TOKEN"),
		ModelProvider: os.Getenv("MODEL_PROVIDER"),
		ModelName:     os.Getenv("MODEL_NAME"),
		WorkDir:       "/tmp/zen-brain-worker",
		EvidenceDir:   "/var/lib/zen-brain1/evidence",
		MaxRetries:    3,
		TaskTimeout:   30 * time.Minute,
		DryRun:        os.Getenv("DRY_RUN") != "",
	}

	var taskKey string
	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--task-key":
			if i+1 < len(os.Args) {
				taskKey = os.Args[i+1]
				i++
			}
		case "--dry-run":
			cfg.DryRun = true
		case "--work-dir":
			if i+1 < len(os.Args) {
				cfg.WorkDir = os.Args[i+1]
				i++
			}
		}
	}

	if taskKey == "" {
		fmt.Println("Error: --task-key is required")
		os.Exit(1)
	}

	if cfg.JiraBaseURL == "" || cfg.JiraEmail == "" || cfg.JiraAPIToken == "" {
		log.Fatal("Jira credentials required: JIRA_URL, JIRA_EMAIL, JIRA_TOKEN")
	}

	log.Printf("[REMEDIATE] Starting remediation for %s (dry-run=%v)", taskKey, cfg.DryRun)

	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		log.Fatalf("Failed to create work dir: %v", err)
	}

	// Fetch task from Jira
	task, err := fetchJiraTask(cfg, taskKey)
	if err != nil {
		log.Fatalf("Failed to fetch task: %v", err)
	}

	log.Printf("[REMEDIATE] Task: %s - %s", task.Key, task.Fields.Summary)

	// Execute remediation
	ctx, cancel := context.WithTimeout(context.Background(), cfg.TaskTimeout)
	defer cancel()

	result, err := executeRemediation(ctx, cfg, task)
	if err != nil {
		log.Fatalf("Remediation failed: %v", err)
	}

	log.Printf("[REMEDIATE] Complete: %s", result.Summary)
	fmt.Printf("Result: %s\n", result.Summary)
}

type jiraTask struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Priority    struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels []string `json:"labels"`
	} `json:"fields"`
}

type remediationResult struct {
	TaskKey  string
	Summary  string
	Duration time.Duration
	Success  bool
}

func fetchJiraTask(cfg remediationConfig, taskKey string) (*jiraTask, error) {
	url := fmt.Sprintf("%s/rest/api/2/issue/%s", cfg.JiraBaseURL, taskKey)

	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jira returned %d", resp.StatusCode)
	}

	var task jiraTask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &task, nil
}

func executeRemediation(ctx context.Context, cfg remediationConfig, task *jiraTask) (*remediationResult, error) {
	start := time.Now()

	// Create worktree
	worktreePath := filepath.Join(cfg.WorkDir, task.Key)
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return nil, err
	}

	// Write task context
	contextPath := filepath.Join(worktreePath, "context.json")
	contextData, _ := json.MarshalIndent(task, "", "  ")
	if err := os.WriteFile(contextPath, contextData, 0644); err != nil {
		return nil, err
	}

	if cfg.DryRun {
		return &remediationResult{
			TaskKey:  task.Key,
			Summary:  "DRY RUN - no changes made",
			Duration: time.Since(start),
			Success:  true,
		}, nil
	}

	// TODO: Actual remediation logic (LLM call, code changes, etc.)
	// For now, just mark as complete

	return &remediationResult{
		TaskKey:  task.Key,
		Summary:  "Remediation placeholder complete",
		Duration: time.Since(start),
		Success:  true,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: BATCH (from cmd/useful-batch)
// ═══════════════════════════════════════════════════════════════════════════════

// runWorkerBatch exec's into the real useful-batch binary.
// The scheduler passes context via env vars (TASKS, BATCH_NAME, OUTPUT_ROOT,
// WORKERS, TIMEOUT) which the real binary reads directly.
// This makes `zen-brain worker batch` the canonical invocation contract
// while the implementation binary provides the actual logic.
func runWorkerBatch() {
	bin := findImplementationBinary("useful-batch")
	args := []string{}
	for _, a := range os.Args[3:] {
		args = append(args, a)
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatalf("[BATCH] failed to run %s: %v", bin, err)
	}
}

// findImplementationBinary locates a runtime implementation binary.
// Search order: same directory as current binary, then PATH.
func findImplementationBinary(name string) string {
	// Same directory as current executable
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// PATH lookup
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	log.Fatalf("[CANONICAL] implementation binary %q not found (searched same-dir and PATH)", name)
	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: TICKETIZE (from cmd/finding-ticketizer)
// ═══════════════════════════════════════════════════════════════════════════════

// runWorkerTicketize exec's into the real finding-ticketizer binary.
// Passes through all args; the real binary reads env vars for Jira config.
func runWorkerTicketize() {
	bin := findImplementationBinary("finding-ticketizer")
	args := os.Args[3:]

	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatalf("[TICKETIZE] failed to run %s: %v", bin, err)
	}
}
