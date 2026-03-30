// Package main: worker subcommands (remediate, batch, ticketize).
// Consolidated from cmd/remediation-worker, cmd/useful-batch, cmd/finding-ticketizer.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

type batchConfig struct {
	WorkspaceHome string
	LLMProvider   string
	Model         string
	BatchSize     int
	Once          bool
	DryRun        bool
}

func runWorkerBatch() {
	cfg := batchConfig{
		WorkspaceHome: "/tmp/zen-brain-workspaces",
		LLMProvider:   "ollama",
		Model:         "qwen3.5:0.8b",
		BatchSize:     5,
		Once:          false,
		DryRun:        os.Getenv("DRY_RUN") != "",
	}

	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--once":
			cfg.Once = true
		case "--dry-run":
			cfg.DryRun = true
		case "--batch-size":
			if i+1 < len(os.Args) {
				cfg.BatchSize, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		case "--model":
			if i+1 < len(os.Args) {
				cfg.Model = os.Args[i+1]
				i++
			}
		}
	}

	log.Printf("[BATCH] Starting useful-batch worker (model=%s, batchSize=%d, once=%v)",
		cfg.Model, cfg.BatchSize, cfg.Once)

	if cfg.Once {
		runBatchOnce(cfg)
		return
	}

	// Continuous loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		runBatchOnce(cfg)
	}
}

func runBatchOnce(cfg batchConfig) {
	// Find useful tasks to execute
	tasks := findUsefulTasks(cfg)
	if len(tasks) == 0 {
		log.Printf("[BATCH] No useful tasks found")
		return
	}

	log.Printf("[BATCH] Found %d useful tasks", len(tasks))

	var wg sync.WaitGroup
	var successCount atomic.Int32
	var failCount atomic.Int32

	for i := 0; i < cfg.BatchSize && i < len(tasks); i++ {
		wg.Add(1)
		go func(task string) {
			defer wg.Done()
			if err := executeBatchTask(cfg, task); err != nil {
				log.Printf("[BATCH] Task failed: %v", err)
				failCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}(tasks[i])
	}

	wg.Wait()
	log.Printf("[BATCH] Complete: %d success, %d failed", successCount.Load(), failCount.Load())
}

func findUsefulTasks(cfg batchConfig) []string {
	// Scan for discovery tasks (placeholder)
	var tasks []string
	entries, err := os.ReadDir("/var/lib/zen-brain1/discovery")
	if err != nil {
		return tasks
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			tasks = append(tasks, e.Name())
		}
	}
	return tasks
}

func executeBatchTask(cfg batchConfig, taskFile string) error {
	if cfg.DryRun {
		log.Printf("[BATCH] DRY RUN: would execute %s", taskFile)
		return nil
	}

	// TODO: Actual batch task execution
	// Read task definition, build prompt, call LLM, write artifact
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: TICKETIZE (from cmd/finding-ticketizer)
// ═══════════════════════════════════════════════════════════════════════════════

type ticketizeConfig struct {
	ArtifactsDir string
	JiraBaseURL  string
	JiraEmail    string
	JiraAPIToken string
	ProjectKey   string
	DryRun       bool
	MaxFindings  int
}

func runWorkerTicketize() {
	cfg := ticketizeConfig{
		ArtifactsDir: "/var/lib/zen-brain1/artifacts",
		JiraBaseURL:  os.Getenv("JIRA_URL"),
		JiraEmail:    os.Getenv("JIRA_EMAIL"),
		JiraAPIToken: os.Getenv("JIRA_TOKEN"),
		ProjectKey:   os.Getenv("JIRA_PROJECT"),
		DryRun:       os.Getenv("DRY_RUN") != "",
		MaxFindings:  10,
	}

	for i := 3; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--dir":
			if i+1 < len(os.Args) {
				cfg.ArtifactsDir = os.Args[i+1]
				i++
			}
		case "--dry-run":
			cfg.DryRun = true
		case "--max":
			if i+1 < len(os.Args) {
				cfg.MaxFindings, _ = strconv.Atoi(os.Args[i+1])
				i++
			}
		}
	}

	log.Printf("[TICKETIZE] Scanning %s for findings (max=%d, dry-run=%v)",
		cfg.ArtifactsDir, cfg.MaxFindings, cfg.DryRun)

	findings, err := scanForFindings(cfg)
	if err != nil {
		log.Fatalf("Failed to scan findings: %v", err)
	}

	log.Printf("[TICKETIZE] Found %d actionable findings", len(findings))

	if len(findings) == 0 {
		fmt.Println("No actionable findings found.")
		return
	}

	if cfg.DryRun {
		fmt.Printf("DRY RUN: Would create %d tickets\n", len(findings))
		for _, f := range findings {
			fmt.Printf("  - [%s] %s\n", f.Type, truncate(f.Description, 80))
		}
		return
	}

	// Create Jira tickets
	created := 0
	for _, f := range findings {
		if err := createJiraTicket(cfg, f); err != nil {
			log.Printf("[TICKETIZE] Failed to create ticket: %v", err)
		} else {
			created++
		}
	}

	fmt.Printf("Created %d/%d tickets\n", created, len(findings))
}

type Finding struct {
	Type        string
	Path        string
	Description string
	Severity    string
	Source      string
}

func scanForFindings(cfg ticketizeConfig) ([]Finding, error) {
	var findings []Finding

	entries, err := os.ReadDir(cfg.ArtifactsDir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		path := filepath.Join(cfg.ArtifactsDir, e.Name())
		fileFindings := extractFindingsFromFile(path)
		findings = append(findings, fileFindings...)
	}

	// Sort by severity and limit
	findings = sortFindingsBySeverity(findings)
	if len(findings) > cfg.MaxFindings {
		findings = findings[:cfg.MaxFindings]
	}

	return findings, nil
}

func extractFindingsFromFile(path string) []Finding {
	var findings []Finding

	f, err := os.Open(path)
	if err != nil {
		return findings
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	findingType := detectFindingType(path)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) < 20 || len(line) > 200 {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") ||
			strings.HasPrefix(line, "|") || strings.HasPrefix(line, "```") {
			continue
		}

		sev := "medium"
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "critical") {
			sev = "critical"
		} else if strings.Contains(lowerLine, "high") {
			sev = "high"
		} else if strings.Contains(lowerLine, "low") {
			sev = "low"
		}

		findings = append(findings, Finding{
			Type:        findingType,
			Path:        path,
			Description: truncate(line, 120),
			Severity:    sev,
			Source:      filepath.Base(path),
		})
	}

	return findings
}

func detectFindingType(path string) string {
	name := strings.ToLower(path)
	switch {
	case strings.Contains(name, "defect") || strings.Contains(name, "bug"):
		return "defect"
	case strings.Contains(name, "dead_code") || strings.Contains(name, "dead-code"):
		return "dead-code"
	case strings.Contains(name, "tech_debt") || strings.Contains(name, "tech-debt"):
		return "tech-debt"
	case strings.Contains(name, "stub"):
		return "stub"
	case strings.Contains(name, "test_gap") || strings.Contains(name, "test-gaps"):
		return "test-gap"
	case strings.Contains(name, "config") || strings.Contains(name, "drift"):
		return "config-drift"
	default:
		return "finding"
	}
}

func sortFindingsBySeverity(findings []Finding) []Finding {
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

	for i := 0; i < len(findings)-1; i++ {
		for j := i + 1; j < len(findings); j++ {
			if severityOrder[findings[i].Severity] > severityOrder[findings[j].Severity] {
				findings[i], findings[j] = findings[j], findings[i]
			}
		}
	}

	return findings
}

func createJiraTicket(cfg ticketizeConfig, f Finding) error {
	url := fmt.Sprintf("%s/rest/api/2/issue", cfg.JiraBaseURL)

	summary := fmt.Sprintf("[%s] %s: %s", f.Type, f.Source, truncate(f.Description, 80))

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":     map[string]string{"key": cfg.ProjectKey},
			"summary":     summary,
			"description": fmt.Sprintf("Finding from %s:\n\n%s", f.Source, f.Description),
			"issuetype":   map[string]string{"name": "Task"},
			"labels":      []string{"zen-brain", "finding", f.Type},
			"priority":    map[string]string{"name": severityToPriority(f.Severity)},
		},
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("Jira returned %d", resp.StatusCode)
	}

	log.Printf("[TICKETIZE] Created ticket: %s", summary)
	return nil
}

func severityToPriority(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "low":
		return "Low"
	default:
		return "Medium"
	}
}
