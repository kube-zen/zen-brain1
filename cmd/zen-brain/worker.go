// Package main: worker subcommands (remediate, batch, ticketize).
// Consolidated from cmd/remediation-worker, cmd/useful-batch, cmd/finding-ticketizer.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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

	"github.com/kube-zen/zen-brain1/internal/mlq"
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
//
// PRIORITY 1: If git-backed execution is required (ProposalOnlyMode=false)
// and the repo or push credentials are missing, the worker FAILS CLOSED.
// It does NOT silently degrade to proposal-only mode.
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

	// PRIORITY 1: Explicit proposal-only mode
	if cfg.ProposalOnlyMode {
		log.Printf("[REMEDIATE] Proposal-only mode (ZEN_PROPOSAL_ONLY=true)")
		runProposalOnly(cfg, ticket)
		return
	}

	// PRIORITY 1 + 6: Hard runtime preflight
	// If git-backed execution is the default mode, repo + push MUST be available.
	// No silent fallback. No hidden degradation.
	preflight := runRuntimePreflight(cfg)
	if !preflight.Passed() {
		log.Printf("[REMEDIATE] ❌ PREFLIGHT FAILED: %s", preflight.Summary())
		writeTerminalResultPreflightFail(cfg.ResultDir, taskKey, preflight)
		os.Exit(1)
	}
	log.Printf("[REMEDIATE] ✅ Preflight passed: %s", preflight.Summary())

	// Git-backed execution path
	err = runBoundedExecution(cfg, ticket)
	if err != nil {
		log.Printf("[REMEDIATE] Bounded execution failed: %v", err)
		os.Exit(1)
	}
}

// ─── Runtime Preflight (Priority 6) ────────────────────────────────────

// PreflightCheck represents a single preflight check result.
type PreflightCheck struct {
	Name   string
	Passed bool
	Reason string
}

// PreflightResult aggregates all preflight checks.
type PreflightResult struct {
	Checks []PreflightCheck
}

// Passed returns true if all checks passed.
func (pr *PreflightResult) Passed() bool {
	for _, c := range pr.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// Summary returns a human-readable summary of all checks.
func (pr *PreflightResult) Summary() string {
	passed := 0
	failed := []string{}
	for _, c := range pr.Checks {
		if c.Passed {
			passed++
		} else {
			failed = append(failed, c.Name)
		}
	}
	return fmt.Sprintf("%d/%d passed, failed: %v", passed, len(pr.Checks), failed)
}

// runRuntimePreflight validates all prerequisites for git-backed execution.
// Any failure here means the worker CANNOT proceed — no fallback.
func runRuntimePreflight(cfg RemediationConfig) PreflightResult {
	var checks []PreflightCheck

	// 1. Repo mount present and is a git repo
	repoCheck := PreflightCheck{Name: "repo_mount"}
	if cfg.RepoRoot == "" {
		repoCheck.Passed = false
		repoCheck.Reason = "ZEN_EXECUTION_REPO/REPO_ROOT not set"
	} else if !isGitRepo(cfg.RepoRoot) {
		repoCheck.Passed = false
		repoCheck.Reason = "not a git repo: " + cfg.RepoRoot
	} else {
		repoCheck.Passed = true
		repoCheck.Reason = cfg.RepoRoot
	}
	checks = append(checks, repoCheck)

	// 2. Git identity present
	identityCheck := PreflightCheck{Name: "git_identity"}
	if cfg.GitAuthorName == "" || cfg.GitAuthorEmail == "" {
		identityCheck.Passed = false
		identityCheck.Reason = "GIT_AUTHOR_NAME or GIT_AUTHOR_EMAIL not set"
	} else {
		identityCheck.Passed = true
		identityCheck.Reason = cfg.GitAuthorName + " <" + cfg.GitAuthorEmail + ">"
	}
	checks = append(checks, identityCheck)

	// 3. Push enabled
	pushCheck := PreflightCheck{Name: "push_enabled"}
	if !cfg.GitPushEnabled {
		pushCheck.Passed = false
		pushCheck.Reason = "ZEN_GIT_PUSH_ENABLED not set to true"
	} else {
		pushCheck.Passed = true
		pushCheck.Reason = "enabled"
	}
	checks = append(checks, pushCheck)

	// 4. Push credentials present (verify with a non-destructive ls-remote)
	credCheck := PreflightCheck{Name: "push_credentials"}
	if cfg.GitPushEnabled && cfg.RepoRoot != "" && isGitRepo(cfg.RepoRoot) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "ls-remote", "--exit-code", cfg.GitRemote, "HEAD")
		if out, err := cmd.CombinedOutput(); err != nil {
			credCheck.Passed = false
			credCheck.Reason = "ls-remote failed: " + string(out)
		} else {
			credCheck.Passed = true
			credCheck.Reason = "ls-remote OK"
		}
	} else if !cfg.GitPushEnabled {
		credCheck.Passed = false
		credCheck.Reason = "push not enabled"
	} else {
		credCheck.Passed = false
		credCheck.Reason = "repo not available"
	}
	checks = append(checks, credCheck)

	// 5. Results directory writable
	resultsCheck := PreflightCheck{Name: "results_dir"}
	os.MkdirAll(cfg.ResultDir, 0755)
	testFile := filepath.Join(cfg.ResultDir, ".preflight-write-test")
	if err := ioutil.WriteFile(testFile, []byte("test"), 0644); err != nil {
		resultsCheck.Passed = false
		resultsCheck.Reason = "not writable: " + err.Error()
	} else {
		os.Remove(testFile)
		resultsCheck.Passed = true
		resultsCheck.Reason = cfg.ResultDir
	}
	checks = append(checks, resultsCheck)

	// 6. Worktree base writable (if configured)
	worktreeCheck := PreflightCheck{Name: "worktree_base"}
	os.MkdirAll(cfg.WorktreeBase, 0755)
	testFile = filepath.Join(cfg.WorktreeBase, ".preflight-write-test")
	if err := ioutil.WriteFile(testFile, []byte("test"), 0644); err != nil {
		worktreeCheck.Passed = false
		worktreeCheck.Reason = "not writable: " + err.Error()
	} else {
		os.Remove(testFile)
		worktreeCheck.Passed = true
		worktreeCheck.Reason = cfg.WorktreeBase
	}
	checks = append(checks, worktreeCheck)

	return PreflightResult{Checks: checks}
}

// writeTerminalResultPreflightFail writes a terminal result when preflight fails.
func writeTerminalResultPreflightFail(dir, jiraKey string, preflight PreflightResult) {
	failedChecks := []string{}
	for _, c := range preflight.Checks {
		if !c.Passed {
			failedChecks = append(failedChecks, c.Name+": "+c.Reason)
		}
	}
	writeTerminalResult(dir, TerminalResult{
		JiraKey:       jiraKey,
		TerminalClass: "failed",
		ExecutionMode: "none",
		QualityPassed: false,
		BlockerReason: "preflight failed: " + strings.Join(failedChecks, "; "),
		JiraState:     "RETRYING",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	})
}

// loadRemediationConfig reads configuration from environment.
func loadRemediationConfig() RemediationConfig {
	return RemediationConfig{
		JiraURL:          envOr("JIRA_URL", ""),
		JiraEmail:        envOr("JIRA_EMAIL", ""),
		JiraToken:        envOr("JIRA_API_TOKEN", ""),
		JiraProject:      envOr("JIRA_PROJECT_KEY", "ZB"),
		L1Endpoint:       envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:          envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		RepoRoot:         envOr("ZEN_EXECUTION_REPO", envOr("REPO_ROOT", "")),
		WorktreeBase:     envOr("ZEN_WORKTREE_BASE", "/workspace/worktrees"),
		ResultDir:        envOr("ZEN_RESULTS_DIR", envOr("RESULT_DIR", "/tmp/zen-brain1-worker-results")),
		EvidenceRoot:     envOr("EVIDENCE_ROOT", "/var/lib/zen-brain1/evidence"),
		TimeoutSec:       envIntOr("REMEDIATION_TIMEOUT", 300),
		GitAuthorName:    envOr("GIT_AUTHOR_NAME", "zen-brain1"),
		GitAuthorEmail:   envOr("GIT_AUTHOR_EMAIL", "zen-brain1@kube-zen.io"),
		GitPushEnabled:   envOr("ZEN_GIT_PUSH_ENABLED", "false") == "true",
		GitRemote:        envOr("ZEN_GIT_REMOTE", "origin"),
		ProposalOnlyMode: envOr("ZEN_PROPOSAL_ONLY", "false") == "true",
	}
}

// RemediationConfig holds runtime configuration.
type RemediationConfig struct {
	JiraURL, JiraEmail, JiraToken, JiraProject string
	L1Endpoint, L1Model                        string
	RepoRoot, ResultDir, EvidenceRoot          string
	WorktreeBase                               string
	TimeoutSec                                 int
	GitAuthorName, GitAuthorEmail              string
	GitPushEnabled                             bool
	GitRemote                                  string
	// ProposalOnlyMode is an explicit flag. When true, the worker may
	// produce proposals without git-backed execution. When false (default),
	// missing repo or push credentials is a fatal blocker.
	ProposalOnlyMode bool
}

// TerminalResult is the outcome of bounded execution.
type TerminalResult struct {
	JiraKey          string   `json:"jira_key"`
	TerminalClass    string   `json:"terminal_class"`
	ExecutionMode    string   `json:"execution_mode"`
	ProposalOnly     bool     `json:"proposal_only"`
	QualityPassed    bool     `json:"quality_passed"`
	ResultClass      string   `json:"result_class,omitempty"`
	GitBranch        string   `json:"git_branch,omitempty"`
	RemoteBranch     string   `json:"remote_branch,omitempty"`
	GitCommit        string   `json:"git_commit,omitempty"`
	FilesChanged     []string `json:"files_changed,omitempty"`
	ValidationReport string   `json:"validation_report_path,omitempty"`
	ProofOfWorkPath  string   `json:"proof_of_work_path,omitempty"`
	OriginalHash     string   `json:"original_hash,omitempty"`
	NewHash          string   `json:"new_hash,omitempty"`
	BlockerReason    string   `json:"blocker_reason,omitempty"`
	FailureReason    string   `json:"failure_reason,omitempty"`
	JiraState        string   `json:"jira_state"`
	Timestamp        string   `json:"timestamp"`
	// MLQ metadata (populated when ZEN_MLQ_ENABLED=true)
	MLQTaskID         string `json:"mlq_task_id,omitempty"`
	MLQSelectedLevel  int    `json:"mlq_selected_level,omitempty"`
	MLQSelectedModel  string `json:"mlq_selected_model,omitempty"`
	MLQAttemptCount   int    `json:"mlq_attempt_count,omitempty"`
	MLQRetryCount     int    `json:"mlq_retry_count,omitempty"`
	MLQEscalated      bool   `json:"mlq_escalated,omitempty"`
	MLQEscalatedFrom  int    `json:"mlq_escalated_from,omitempty"`
	MLQFinalModel     string `json:"mlq_final_model,omitempty"`
	MLQWorkerEndpoint string `json:"mlq_worker_endpoint,omitempty"`
	MLQFailureClass   string `json:"mlq_failure_class,omitempty"`
}

// fileHash returns a short SHA-256 hash of file content for change detection.
func fileHash(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h[:])[:16]
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

	publisher, err := worktree.NewPublisher(cfg.RepoRoot, cfg.GitAuthorName, cfg.GitAuthorEmail, cfg.GitPushEnabled, cfg.GitRemote)
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
	originalHash := fileHash(existingContent)
	log.Printf("[REMEDIATE] Original hash: %s (%d bytes)", originalHash, len(existingContent))

	// 4. Call L1 for full replacement content (or use deterministic canary injection)
	var newContentStr, intendedDelta string
	var mlqMeta map[string]interface{}

	if os.Getenv("ZEN_DETERMINISTIC_CANARY") != "" {
		// PRIORITY 2A: Deterministic plumbing proof — skip L1, inject known change
		log.Printf("[REMEDIATE] Deterministic canary mode: injecting known change")
		timestamp := time.Now().UTC().Format(time.RFC3339)
		intendedDelta = fmt.Sprintf("deterministic canary injection at %s", timestamp)
		injectLine := fmt.Sprintf("\n## Canary Entry — %s\n\nProof run for %s. Bounded execution pipeline verified.\nTarget file: %s\nTimestamp: %s\n", ticket.Key, ticket.Key, targetFile, timestamp)
		// Append to the last section (before final closing tag, or at end)
		newContentStr = string(existingContent) + injectLine
		mlqMeta = map[string]interface{}{"mlq_enabled": false, "mode": "deterministic_canary"}
	} else {
		// Standard bounded execution with optional MLQ retry/escalation
		log.Printf("[REMEDIATE] Calling L1 for bounded fix (MLQ enabled: %v)...", os.Getenv("ZEN_MLQ_ENABLED") != "")
		var err error
		newContentStr, intendedDelta, mlqMeta, err = executeBoundedWithMLQ(ctx, cfg, ticket, targetFile, string(existingContent))
		if err != nil {
			return fmt.Errorf("L1 call failed: %w", err)
		}
	}
	newContent := []byte(newContentStr)
	newHash := fileHash(newContent)
	log.Printf("[REMEDIATE] Generated hash: %s (%d bytes)", newHash, len(newContent))
	log.Printf("[REMEDIATE] Intended delta: %s", truncate(intendedDelta, 200))

	// 5. No-op detection: if content is identical, stop cleanly
	if originalHash == newHash {
		log.Printf("[REMEDIATE] No effective change detected (hashes match)")
		// Clean up: switch back to main, delete the no-op branch
		exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "checkout", "main").Run()
		exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "branch", "-D", branchName).Run()

		noopResult := TerminalResult{
			JiraKey:       ticket.Key,
			TerminalClass: "retrying",
			ExecutionMode: "git_backed_execution",
			ProposalOnly:  false,
			QualityPassed: false,
			ResultClass:   "no_effective_change",
			FailureReason: "model_output_identical_to_target",
			FilesChanged:  []string{},
			OriginalHash:  originalHash,
			NewHash:       newHash,
			BlockerReason: fmt.Sprintf("no effective change: original=%s new=%s target=%s delta=%q", originalHash, newHash, targetFile, intendedDelta),
			JiraState:     "RETRYING",
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}
		writeTerminalResult(resultDir, noopResult)
		return nil // Clean exit, not a crash
	}

	// 6. Write file atomically
	tmpPath := absTargetPath + ".tmp"
	if err := ioutil.WriteFile(tmpPath, newContent, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, absTargetPath); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	log.Printf("[REMEDIATE] File written: %s", targetFile)

	// 7. Verify with git diff
	diffCmd := exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "diff", "--exit-code", "--", targetFile)
	if diffOut, diffErr := diffCmd.CombinedOutput(); diffErr == nil {
		// exit code 0 means no diff — git confirms no change
		log.Printf("[REMEDIATE] git diff confirms no change, cleaning up branch")
		exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "checkout", "--", targetFile).Run()
		exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "checkout", "main").Run()
		exec.CommandContext(ctx, "git", "-C", cfg.RepoRoot, "branch", "-D", branchName).Run()

		noopResult := TerminalResult{
			JiraKey:       ticket.Key,
			TerminalClass: "retrying",
			ExecutionMode: "git_backed_execution",
			ProposalOnly:  false,
			QualityPassed: false,
			ResultClass:   "no_effective_change",
			FailureReason: "git_diff_confirms_no_change",
			FilesChanged:  []string{},
			OriginalHash:  originalHash,
			NewHash:       newHash,
			BlockerReason: fmt.Sprintf("hashes differed but git diff confirmed no effective change: original=%s new=%s", originalHash, newHash),
			JiraState:     "RETRYING",
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}
		writeTerminalResult(resultDir, noopResult)
		return nil
	} else {
		log.Printf("[REMEDIATE] git diff confirms changes: %s", truncate(string(diffOut), 300))
	}

	// 8. Run validation
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

	// 9. Commit and push
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

	// 10. Write diff stat
	diffStatPath := filepath.Join(resultDir, ticket.Key+"-diff.txt")
	publisher.WriteDiffStat(ctx, "HEAD~1", diffStatPath)

	// 11. Write terminal result
	result := TerminalResult{
		JiraKey:          ticket.Key,
		TerminalClass:    "needs_review",
		ExecutionMode:    "git_backed_execution",
		ProposalOnly:     false,
		QualityPassed:    validationPassed,
		ResultClass:      "remediation_complete",
		GitBranch:        branchName,
		RemoteBranch:     remoteBranch,
		GitCommit:        commitSHA,
		FilesChanged:     []string{targetFile},
		OriginalHash:     originalHash,
		NewHash:          newHash,
		ValidationReport: validationOutput,
		ProofOfWorkPath:  diffStatPath,
		JiraState:        "Needs Review",
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	// Populate MLQ metadata if available
	if mlqMeta != nil {
		if v, ok := mlqMeta["mlq_enabled"].(bool); ok && v {
			result.MLQTaskID, _ = mlqMeta["mlq_task_id"].(string)
			result.MLQSelectedLevel, _ = mlqMeta["mlq_selected_level"].(int)
			result.MLQSelectedModel, _ = mlqMeta["mlq_selected_model"].(string)
			result.MLQAttemptCount, _ = mlqMeta["mlq_attempt_count"].(int)
			result.MLQRetryCount, _ = mlqMeta["mlq_retry_count"].(int)
			result.MLQEscalated, _ = mlqMeta["mlq_escalated"].(bool)
			result.MLQEscalatedFrom, _ = mlqMeta["mlq_escalated_from"].(int)
			result.MLQFinalModel, _ = mlqMeta["mlq_final_model"].(string)
			result.MLQWorkerEndpoint, _ = mlqMeta["mlq_worker_endpoint"].(string)
			result.MLQFailureClass, _ = mlqMeta["mlq_failure_class"].(string)
		}
	}

	if !validationPassed {
		result.TerminalClass = "paused"
		result.ResultClass = "validation_failed"
		result.BlockerReason = "validation failed"
		result.JiraState = "PAUSED"
	}

	writeTerminalResult(resultDir, result)

	// 12. Post Jira comment
	postJiraComment(cfg, ticket.Key, buildProofComment(result))

	log.Printf("[REMEDIATE] Complete: class=%s commit=%s", result.TerminalClass, shortSHA(commitSHA))
	return nil
}

// runProposalOnly executes when explicitly configured as proposal-only mode (ZEN_PROPOSAL_ONLY=true).
// WARNING: This should NEVER be the default path in production.
// If proposal-only is triggered by implicit fallback (missing repo), that is a bug.
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
	Description string // Raw JSON string of ADF description (may contain nested objects)
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

	body, _ := ioutil.ReadAll(resp.Body)

	// Parse manually to handle ADF description (which is a JSON object, not a string)
	var raw struct {
		Key    string          `json:"key"`
		Fields json.RawMessage `json:"fields"`
	}
	json.Unmarshal(body, &raw)

	var fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Priority    struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels []string `json:"labels"`
	}
	json.Unmarshal(raw.Fields, &fields)

	return &JiraTicket{
		Key:         key,
		Summary:     fields.Summary,
		Description: string(fields.Description), // raw JSON bytes as string, searchable for file paths
		Priority:    fields.Priority.Name,
		Labels:      fields.Labels,
	}, nil
}

// determineTargetFile extracts the target file path from ticket context.
// Searches the raw description string for file path patterns.
func determineTargetFile(ticket *JiraTicket) string {
	desc := ticket.Description
	if desc == "" {
		return ""
	}

	// Strategy 1: Look for file paths matching common patterns
	// Match patterns like: docs/01-ARCHITECTURE/PROJECT_STRUCTURE.md
	// The description is often ADF JSON, so file paths are embedded in "text" values.
	// Look for path-like strings: starts with a known prefix, ends with a file extension.
	pathPrefixes := []string{"docs/", "internal/", "cmd/", "config/", "scripts/", "pkg/"}
	fileExts := []string{".go", ".yaml", ".yml", ".md", ".json", ".toml", ".txt", ".sh", ".py"}

	for _, prefix := range pathPrefixes {
		idx := strings.Index(desc, prefix)
		for idx >= 0 {
			// Extract from this position
			rest := desc[idx:]
			// Find end of path (stop at whitespace, quote, comma, backslash, or closing brace)
			end := len(rest)
			for i, c := range rest {
				if c == ' ' || c == '\n' || c == '\t' || c == '"' || c == ',' || c == '\\' || c == '}' || c == ')' || c == ']' {
					end = i
					break
				}
			}
			candidate := rest[:end]
			// Check if it ends with a known file extension
			for _, ext := range fileExts {
				if strings.HasSuffix(candidate, ext) && len(candidate) > len(prefix)+2 {
					// Validate it looks like a path (contains / and has a filename)
					if strings.Count(candidate, "/") >= 1 && !strings.Contains(candidate, " ") {
						return candidate
					}
				}
			}
			// Try next occurrence
			nextIdx := strings.Index(rest[len(prefix):], prefix)
			if nextIdx < 0 {
				break
			}
			idx = idx + len(prefix) + nextIdx
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
// Returns: (fullReplacementContent, intendedDeltaSummary, error)
func callL1ForBoundedFix(ctx context.Context, cfg RemediationConfig, jiraKey, targetFile, existingContent string) (string, string, error) {
	systemPrompt := fmt.Sprintf(`You are a bounded remediation worker. Your task is to produce the COMPLETE replacement content for exactly ONE file.

TARGET FILE: %s

RULES:
- Return ONLY the complete file content, nothing else
- No markdown fences, no explanations
- Preserve existing structure and style where appropriate
- Apply ONLY the specific fix needed
- Do NOT add new imports or dependencies
- Do NOT change package declarations
- You MUST make at least one meaningful change to the file content
- Return the entire updated file with at least one necessary functional or textual change if a valid fix exists
- If you truly cannot determine any needed change, return the file content with a single comment line added at the top indicating "no change identified"

BEFORE the file content, on the FIRST LINE, provide a short summary of what you changed (the "intended delta"). Then a blank line, then the complete file content.`, targetFile)

	userPrompt := fmt.Sprintf(`Ticket: %s
Summary: Fix the issue described in the ticket

EXISTING CONTENT:
%s

Return a one-line intended delta summary, then a blank line, then the complete modified file content.`, jiraKey, truncate(existingContent, 6000))

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
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("L1 returned %d: %s", resp.StatusCode, string(body))
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
		return "", "", fmt.Errorf("empty L1 response")
	}

	fullResponse := strings.TrimSpace(llmResp.Choices[0].Message.Content)

	// Split into delta summary (first line) and file content (rest)
	var intendedDelta string
	var fileContent string
	if idx := strings.Index(fullResponse, "\n"); idx >= 0 {
		intendedDelta = strings.TrimSpace(fullResponse[:idx])
		fileContent = strings.TrimSpace(fullResponse[idx+1:])
	} else {
		intendedDelta = "no delta summary provided"
		fileContent = fullResponse
	}

	// Strip markdown fences if present
	fileContent = strings.TrimPrefix(fileContent, "```go")
	fileContent = strings.TrimPrefix(fileContent, "```yaml")
	fileContent = strings.TrimPrefix(fileContent, "```")
	fileContent = strings.TrimSuffix(fileContent, "```")
	fileContent = strings.TrimSpace(fileContent)

	return fileContent, intendedDelta, nil
}

// executeBoundedWithMLQ wraps bounded remediation with MLQ retry/escalation.
// When ZEN_MLQ_ENABLED is set, uses TaskExecutor.ExecuteWithRetry().
// Otherwise falls back to direct callL1ForBoundedFix.
func executeBoundedWithMLQ(ctx context.Context, cfg RemediationConfig, ticket *JiraTicket, targetFile, existingContent string) (newContent, intendedDelta string, mlqMeta map[string]interface{}, err error) {
	mlqMeta = make(map[string]interface{})

	// Check if MLQ is enabled
	if os.Getenv("ZEN_MLQ_ENABLED") == "" {
		// MLQ not enabled - use direct L1 call (existing behavior)
		log.Printf("[REMEDIATE] MLQ not enabled, using direct L1 call")
		newContent, intendedDelta, err = callL1ForBoundedFix(ctx, cfg, ticket.Key, targetFile, existingContent)
		mlqMeta["mlq_enabled"] = false
		return
	}

	// MLQ enabled - load config and create executor
	mlqConfigPath := os.Getenv("ZEN_MLQ_CONFIG")
	if mlqConfigPath == "" {
		mlqConfigPath = "/etc/zen-brain1/mlq-levels.yaml"
	}

	mlqInstance, err := mlq.NewMLQFromConfig(mlqConfigPath)
	if err != nil {
		log.Printf("[REMEDIATE] Failed to load MLQ config: %v, falling back to direct L1", err)
		newContent, intendedDelta, err = callL1ForBoundedFix(ctx, cfg, ticket.Key, targetFile, existingContent)
		mlqMeta["mlq_enabled"] = false
		mlqMeta["mlq_error"] = err.Error()
		return
	}

	// Build worker pools from MLQ config
	workerPools := make(map[int]*mlq.WorkerPool)
	for _, level := range mlqInstance.ListLevels() {
		l, ok := mlqInstance.GetLevel(level)
		if !ok || !l.Enabled {
			continue
		}
		// Single endpoint per level for now (can extend to multiple)
		endpoints := []string{l.Backend.APIEndpoint}
		workerPools[level] = mlq.NewWorkerPool(l, endpoints)
		log.Printf("[REMEDIATE] Created worker pool for level %d: %s", level, l.Backend.APIEndpoint)
	}

	// Create task executor
	executor := mlq.NewTaskExecutor(mlqInstance, workerPools)

	// Determine task class based on file type/issue
	taskClass := "bounded-execution"
	if strings.HasSuffix(targetFile, ".md") {
		taskClass = "documentation"
	} else if strings.Contains(ticket.Summary, "bug") || strings.Contains(ticket.Summary, "fix") {
		taskClass = "bugfix"
	}

	log.Printf("[REMEDIATE] MLQ executing with taskClass=%s", taskClass)

	// Track generated content across attempts
	var generatedContent, generatedDelta string

	// Execute with retry
	telemetry := executor.ExecuteWithRetry(ctx, ticket.Key, taskClass, ticket.Key,
		func(ctx context.Context, workerEndpoint string) (string, error) {
			log.Printf("[REMEDIATE] MLQ attempt on endpoint: %s", workerEndpoint)

			// Temporarily override L1 endpoint for this attempt
			originalEndpoint := cfg.L1Endpoint
			cfg.L1Endpoint = workerEndpoint

			content, delta, attemptErr := callL1ForBoundedFix(ctx, cfg, ticket.Key, targetFile, existingContent)

			cfg.L1Endpoint = originalEndpoint // Restore

			if attemptErr != nil {
				return "", attemptErr
			}

			// Store successful content
			generatedContent = content
			generatedDelta = delta

			// Return artifact path (the content itself for now)
			return "content-generated", nil
		})

	// Populate MLQ metadata
	mlqMeta["mlq_enabled"] = true
	mlqMeta["mlq_task_id"] = telemetry.TaskID
	mlqMeta["mlq_selected_level"] = telemetry.InitialLevel
	mlqMeta["mlq_final_level"] = telemetry.FinalLevel
	mlqMeta["mlq_attempt_count"] = len(telemetry.Attempts)
	mlqMeta["mlq_retry_count"] = telemetry.TotalRetries
	mlqMeta["mlq_escalated"] = telemetry.Escalated
	mlqMeta["mlq_fallback_used"] = telemetry.FallbackUsed
	mlqMeta["mlq_final_result"] = telemetry.FinalResult

	if len(telemetry.Attempts) > 0 {
		lastAttempt := telemetry.Attempts[len(telemetry.Attempts)-1]
		mlqMeta["mlq_worker_endpoint"] = lastAttempt.WorkerEndpoint
		if lastAttempt.Error != "" {
			mlqMeta["mlq_failure_class"] = classifyMLQFailure(lastAttempt.Error)
		}
	}

	// Check if MLQ succeeded
	if telemetry.FinalResult != "success" {
		err = fmt.Errorf("MLQ execution failed: %s", telemetry.FinalResult)
		mlqMeta["mlq_failure_class"] = telemetry.FinalResult
		return
	}

	// Use the stored content from the successful attempt
	newContent = generatedContent
	intendedDelta = generatedDelta

	log.Printf("[REMEDIATE] MLQ complete: level=%d attempts=%d escalated=%v",
		telemetry.FinalLevel, len(telemetry.Attempts), telemetry.Escalated)
	return
}

// classifyMLQFailure categorizes failure types for telemetry.
func classifyMLQFailure(errStr string) string {
	lower := strings.ToLower(errStr)
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "timeout"
	case strings.Contains(lower, "empty") || strings.Contains(lower, "no content"):
		return "empty_output"
	case strings.Contains(lower, "parse") || strings.Contains(lower, "json"):
		return "parse_failure"
	case strings.Contains(lower, "no effective change"):
		return "no_effective_change"
	case strings.Contains(lower, "auth") || strings.Contains(lower, "forbidden"):
		return "auth_failure"
	case strings.Contains(lower, "connection") || strings.Contains(lower, "unreachable"):
		return "infra_failure"
	default:
		return "unknown"
	}
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
