// Package main: worker subcommands (remediate, batch, ticketize).
// Consolidated from cmd/remediation-worker, cmd/useful-batch, cmd/finding-ticketizer.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kube-zen/zen-brain1/internal/worktree"
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
// WORKER: REMEDIATE (bounded git-backed execution)
// ═══════════════════════════════════════════════════════════════════════════════

// runWorkerRemediate implements PHASE 1 bounded git-backed execution.
// This is the canonical remediation path - no fallback to legacy binaries.
func runWorkerRemediate() {
	// Parse --task-key flag
	taskKey := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--task-key" && i+1 < len(os.Args) {
			taskKey = os.Args[i+1]
			break
		}
	}
	if taskKey == "" {
		// Fallback: read from PILOT_KEYS env (legacy subprocess mode)
		taskKey = strings.Split(envOr("PILOT_KEYS", ""), ",")[0]
	}
	if taskKey == "" {
		log.Fatal("[REMEDIATE] --task-key required or PILOT_KEYS must be set")
	}

	log.Printf("[REMEDIATE] Starting bounded execution for %s", taskKey)

	// Load config from env
	cfg := loadRemediationConfig()

	// Fetch ticket from Jira
	ticket, err := fetchJiraTicket(cfg, taskKey)
	if err != nil {
		log.Printf("[REMEDIATE] Failed to fetch ticket: %v", err)
		writeTerminalResultFail(cfg.ResultDir, taskKey, fmt.Sprintf("fetch failed: %v", err))
		os.Exit(1)
	}

	// Determine if git-backed execution is possible
	if cfg.RepoRoot == "" || !isGitRepo(cfg.RepoRoot) {
		log.Printf("[REMEDIATE] No git repo available — proposal-only mode")
		runProposalOnly(cfg, ticket)
		return
	}

	// Git-backed execution path
	err = runBoundedExecution(cfg, ticket)
	if err != nil {
		log.Printf("[REMEDIATE] Bounded execution failed: %v", err)
		os.Exit(1)
	}
}

// loadRemediationConfig reads configuration from environment.
func loadRemediationConfig() RemediationConfig {
	return RemediationConfig{
		JiraURL:        envOr("JIRA_URL", ""),
		JiraEmail:      envOr("JIRA_EMAIL", ""),
		JiraToken:      envOr("JIRA_API_TOKEN", ""),
		JiraProject:    envOr("JIRA_PROJECT_KEY", "ZB"),
		L1Endpoint:     envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:        envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		RepoRoot:       envOr("REPO_ROOT", ""),
		ResultDir:      envOr("RESULT_DIR", "/tmp/zen-brain1-worker-results"),
		EvidenceRoot:   envOr("EVIDENCE_ROOT", "/var/lib/zen-brain1/evidence"),
		TimeoutSec:     envIntOr("REMEDIATION_TIMEOUT", 120),
		GitAuthorName:  envOr("GIT_AUTHOR_NAME", "zen-brain1"),
		GitAuthorEmail: envOr("GIT_AUTHOR_EMAIL", "zen-brain1@kube-zen.io"),
		GitPushEnabled: envOr("ZEN_GIT_PUSH_ENABLED", "false") == "true",
	}
}

// RemediationConfig holds runtime configuration.
type RemediationConfig struct {
	JiraURL, JiraEmail, JiraToken, JiraProject string
	L1Endpoint, L1Model                        string
	RepoRoot, ResultDir, EvidenceRoot          string
	TimeoutSec                                 int
	GitAuthorName, GitAuthorEmail              string
	GitPushEnabled                             bool
}

// TerminalResult is the outcome of bounded execution.
type TerminalResult struct {
	JiraKey          string   `json:"jira_key"`
	TerminalClass    string   `json:"terminal_class"`
	ExecutionMode    string   `json:"execution_mode"`
	QualityPassed    bool     `json:"quality_passed"`
	GitBranch        string   `json:"git_branch,omitempty"`
	RemoteBranch     string   `json:"remote_branch,omitempty"`
	GitCommit        string   `json:"git_commit,omitempty"`
	FilesChanged     []string `json:"files_changed,omitempty"`
	ValidationReport string   `json:"validation_report_path,omitempty"`
	ProofOfWorkPath  string   `json:"proof_of_work_path,omitempty"`
	BlockerReason    string   `json:"blocker_reason,omitempty"`
	JiraState        string   `json:"jira_state"`
	Timestamp        string   `json:"timestamp"`
}

// runBoundedExecution implements the full git-backed remediation flow.
func runBoundedExecution(cfg RemediationConfig, ticket *JiraTicket) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSec)*time.Second)
	defer cancel()

	resultDir := cfg.ResultDir
	os.MkdirAll(resultDir, 0755)

	// 1. Create git branch
	branchName := worktree.GenerateBranchName(ticket.Key)
	log.Printf("[REMEDIATE] Creating branch: %s", branchName)

	publisher, err := worktree.NewPublisher(cfg.RepoRoot, cfg.GitAuthorName, cfg.GitAuthorEmail, cfg.GitPushEnabled)
	if err != nil {
		return fmt.Errorf("create publisher: %w", err)
	}

	if err := publisher.CreateBranch(ctx, branchName, "HEAD"); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}

	// 2. Determine target file (bounded: single file only)
	targetFile := determineTargetFile(ticket)
	if targetFile == "" {
		return fmt.Errorf("no target file determined")
	}

	absTargetPath := filepath.Join(cfg.RepoRoot, targetFile)
	log.Printf("[REMEDIATE] Target file: %s", targetFile)

	// 3. Read existing file content
	existingContent, err := ioutil.ReadFile(absTargetPath)
	if err != nil {
		return fmt.Errorf("read target file: %w", err)
	}

	// 4. Call L1 for full replacement content
	log.Printf("[REMEDIATE] Calling L1 for bounded fix...")
	newContent, err := callL1ForBoundedFix(ctx, cfg, ticket.Key, targetFile, string(existingContent))
	if err != nil {
		return fmt.Errorf("L1 call failed: %w", err)
	}

	// 5. Write file atomically
	tmpPath := absTargetPath + ".tmp"
	if err := ioutil.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, absTargetPath); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	log.Printf("[REMEDIATE] File written: %s", targetFile)

	// 6. Run validation
	validationCmd := determineValidationCommand(targetFile)
	validationPassed := false
	validationOutput := ""
	if validationCmd != "" {
		log.Printf("[REMEDIATE] Running validation: %s", validationCmd)
		cmd := exec.CommandContext(ctx, "sh", "-c", validationCmd)
		cmd.Dir = cfg.RepoRoot
		output, err := cmd.CombinedOutput()
		validationOutput = string(output)
		validationPassed = err == nil
		log.Printf("[REMEDIATE] Validation: passed=%v output=%s", validationPassed, truncate(validationOutput, 200))
	} else {
		// No validation command — accept the change
		validationPassed = true
	}

	// 7. Commit and push
	commitSHA := ""
	remoteBranch := ""
	if validationPassed {
		commitSHA, err = publisher.CommitChanges(ctx, []string{targetFile}, fmt.Sprintf("%s: bounded remediation via zen-brain1", ticket.Key))
		if err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
		log.Printf("[REMEDIATE] Committed: %s", commitSHA[:7])

		if cfg.GitPushEnabled {
			remoteBranch, err = publisher.PushBranch(ctx, branchName)
			if err != nil {
				log.Printf("[REMEDIATE] Push failed: %v (continuing)", err)
			} else {
				log.Printf("[REMEDIATE] Pushed: %s", remoteBranch)
			}
		}
	}

	// 8. Write diff stat
	diffStatPath := filepath.Join(resultDir, ticket.Key+"-diff.txt")
	publisher.WriteDiffStat(ctx, "HEAD~1", diffStatPath)

	// 9. Write terminal result
	result := TerminalResult{
		JiraKey:          ticket.Key,
		TerminalClass:    "needs_review",
		ExecutionMode:    "git_backed_execution",
		QualityPassed:    validationPassed,
		GitBranch:        branchName,
		RemoteBranch:     remoteBranch,
		GitCommit:        commitSHA,
		FilesChanged:     []string{targetFile},
		ValidationReport: validationOutput,
		ProofOfWorkPath:  diffStatPath,
		JiraState:        "Needs Review",
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	if !validationPassed {
		result.TerminalClass = "paused"
		result.BlockerReason = "validation failed"
		result.JiraState = "PAUSED"
	}

	writeTerminalResult(resultDir, result)

	// 10. Post Jira comment
	postJiraComment(cfg, ticket.Key, buildProofComment(result))

	log.Printf("[REMEDIATE] Complete: class=%s commit=%s", result.TerminalClass, shortSHA(commitSHA))
	return nil
}

// runProposalOnly executes when no git repo is available.
func runProposalOnly(cfg RemediationConfig, ticket *JiraTicket) {
	// Fall back to standalone remediation-worker binary for proposal generation
	bin := findImplementationBinary("remediation-worker")
	cmd := exec.Command(bin)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		log.Fatalf("[REMEDIATE] failed to run %s: %v", bin, err)
	}
}

// JiraTicket represents a Jira issue.
type JiraTicket struct {
	Key         string
	Summary     string
	Description string
	Priority    string
	Labels      []string
}

// fetchJiraTicket retrieves ticket details from Jira.
func fetchJiraTicket(cfg RemediationConfig, key string) (*JiraTicket, error) {
	if cfg.JiraURL == "" || cfg.JiraEmail == "" || cfg.JiraToken == "" {
		return nil, fmt.Errorf("Jira not configured")
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,description,priority,labels", cfg.JiraURL, key)
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jira returned %d", resp.StatusCode)
	}

	var data struct {
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
	json.NewDecoder(resp.Body).Decode(&data)

	return &JiraTicket{
		Key:         data.Key,
		Summary:     data.Fields.Summary,
		Description: data.Fields.Description,
		Priority:    data.Fields.Priority.Name,
		Labels:      data.Fields.Labels,
	}, nil
}

// determineTargetFile extracts the target file path from ticket context.
func determineTargetFile(ticket *JiraTicket) string {
	// Look for file references in description
	desc := ticket.Description
	// Common patterns: "config/", "internal/", "cmd/", "docs/"
	patterns := []string{"config/", "internal/", "cmd/", "docs/", "scripts/"}
	for _, p := range patterns {
		if idx := strings.Index(desc, p); idx >= 0 {
			// Extract until whitespace or end
			rest := desc[idx:]
			end := strings.IndexAny(rest, " \n\t,;)")
			if end > 0 {
				return rest[:end]
			}
			return rest
		}
	}
	return ""
}

// determineValidationCommand returns validation command based on file type.
func determineValidationCommand(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".go":
		return "go build ./..."
	case ".yaml", ".yml":
		return fmt.Sprintf("python3 -c \"import yaml; yaml.safe_load(open('%s'))\"", filePath)
	case ".md":
		return "" // No validation for markdown
	default:
		return ""
	}
}

// callL1ForBoundedFix requests full file replacement content.
func callL1ForBoundedFix(ctx context.Context, cfg RemediationConfig, jiraKey, targetFile, existingContent string) (string, error) {
	systemPrompt := fmt.Sprintf(`You are a bounded remediation worker. Your task is to produce the COMPLETE replacement content for exactly ONE file.

TARGET FILE: %s

RULES:
- Return ONLY the complete file content, nothing else
- No markdown fences, no explanations
- Preserve existing structure and style where appropriate
- Apply ONLY the specific fix needed
- Do NOT add new imports or dependencies
- Do NOT change package declarations
- If you cannot determine the fix, return the existing content unchanged

Return the full file content only.`, targetFile)

	userPrompt := fmt.Sprintf(`Ticket: %s
Summary: Fix the issue described in the ticket

EXISTING CONTENT:
%s

Return the complete modified file content only.`, jiraKey, truncate(existingContent, 6000))

	payload := map[string]interface{}{
		"model": cfg.L1Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.3,
		"max_tokens":  4096,
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST",
		cfg.L1Endpoint+"/v1/chat/completions",
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("L1 returned %d: %s", resp.StatusCode, string(body))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(body, &llmResp)
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("empty L1 response")
	}

	content := llmResp.Choices[0].Message.Content
	// Strip markdown fences if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```go")
	content = strings.TrimPrefix(content, "```yaml")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	return content, nil
}

// writeTerminalResult writes the outcome to JSON for factory-fill.
func writeTerminalResult(dir string, result TerminalResult) {
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, result.JiraKey+".json")
	data, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(path, data, 0644)
	log.Printf("[REMEDIATE] Terminal result written: %s", path)
}

// writeTerminalResultFail writes a failure terminal result.
func writeTerminalResultFail(dir, jiraKey, reason string) {
	writeTerminalResult(dir, TerminalResult{
		JiraKey:       jiraKey,
		TerminalClass: "failed",
		ExecutionMode: "proposal_only",
		QualityPassed: false,
		BlockerReason: reason,
		JiraState:     "RETRYING",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// postJiraComment posts a comment to the Jira ticket.
func postJiraComment(cfg RemediationConfig, key, comment string) error {
	if cfg.JiraURL == "" || cfg.JiraEmail == "" || cfg.JiraToken == "" {
		return nil // Jira not configured
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", cfg.JiraURL, key)
	body := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": comment},
					},
				},
			},
		},
	}
	bodyBytes, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// buildProofComment creates the Jira comment with evidence.
func buildProofComment(result TerminalResult) string {
	return fmt.Sprintf(`[zen-brain1 bounded remediation]

Execution Mode: %s
Validation: %v
Branch: %s
Commit: %s
Remote: %s
Files: %s

Status: %s`,
		result.ExecutionMode,
		result.QualityPassed,
		result.GitBranch,
		shortSHA(result.GitCommit),
		result.RemoteBranch,
		strings.Join(result.FilesChanged, ", "),
		result.TerminalClass,
	)
}

// Helper functions

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return fallback
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// ═══════════════════════════════════════════════════════════════════════════════
// WORKER: BATCH (from cmd/useful-batch)
// ═══════════════════════════════════════════════════════════════════════════════

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
