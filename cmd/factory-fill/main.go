// Command factory-fill implements a backlog-aware dispatch loop that keeps
// the L1 factory filled whenever ready work exists in Jira.
//
// Operating policy (R035–R041):
//   - Adaptive concurrency: no static W values
//   - Work-conserving: idle workers while runnable work exists is a bug
//   - 2 model slots always reserved for scheduled tasks
//   - Resource-aware: throttle on CPU soft/hard cap with hysteresis
//   - Conservative fallback on telemetry failure
//   - Jira In Progress must reflect actual active work
package main

import (
	"bytes"
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

	"context"
	"github.com/kube-zen/zen-brain1/internal/concurrency"
	"github.com/kube-zen/zen-brain1/internal/readiness"
	"github.com/kube-zen/zen-brain1/internal/secrets"
)

// ─── Ticket Readiness Classification (PHASE 1) ───────────────────────

type TicketReadiness string

const (
	ReadyForExecution   TicketReadiness = "ready_for_execution"
	ReadyWithReview     TicketReadiness = "ready_with_review"
	BlockedMissingCtx   TicketReadiness = "blocked_missing_context"
	BlockedGovernance   TicketReadiness = "blocked_missing_governance"
	TooLargeForL1       TicketReadiness = "too_large_for_l1"
	ScanBatchArtifact   TicketReadiness = "scan_batch_artifact"
	DuplicateOrStale    TicketReadiness = "duplicate_or_stale"
	BlockedInsufficient TicketReadiness = "blocked_insufficient_spec"
)

type ClassifiedTicket struct {
	Key         string
	Summary     string
	Description string // Extracted from ADF
	Labels      []string
	Priority    string
	Status      string
	Readiness   TicketReadiness
}

type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"` // ADF object or string — we don't use it
		Labels      []string        `json:"labels"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
	} `json:"fields"`
}

type jiraConfig struct {
	url, email, token, project string
	enabled                    bool
}

func loadJiraConfig() jiraConfig {
	// Detect cluster mode
	clusterMode := os.Getenv("KUBERNETES_SERVICE_HOST") != ""

	var dirPath string
	if clusterMode {
		dirPath = "/zen-lock/secrets"
	}

	// Use canonical resolver
	material, err := secrets.ResolveJira(context.Background(), secrets.JiraResolveOptions{
		DirPath:          dirPath,
		FilePath:         "",           // No host file in cluster
		AllowEnvFallback: !clusterMode, // Allow env fallback only in local mode
		ClusterMode:      clusterMode,
	})

	if err != nil {
		log.Printf("[JIRA] ❌ FAILED to resolve credentials: %v", err)
		return jiraConfig{enabled: false}
	}

	if material.Source == "none" {
		log.Printf("[JIRA] ❌ No credentials found (cluster=%v)", clusterMode)
		return jiraConfig{enabled: false}
	}

	log.Printf("[JIRA] ✅ Credentials loaded from %s", material.Source)
	return jiraConfig{
		url:     material.BaseURL,
		email:   material.Email,
		token:   material.APIToken,
		project: material.ProjectKey,
		enabled: true,
	}
}

func classifyTicket(issue jiraIssue) ClassifiedTicket {
	// Extract plain text from ADF description
	description := extractTextFromADF(issue.Fields.Description)

	ct := ClassifiedTicket{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: description,
		Labels:      issue.Fields.Labels,
		Priority:    issue.Fields.Priority.Name,
		Status:      issue.Fields.Status.Name,
	}

	for _, l := range ct.Labels {
		switch l {
		case "scheduled-batch", "hourly-scan", "daily-sweep", "quad-hourly-summary":
			ct.Readiness = ScanBatchArtifact
			return ct
		case "ai:completed", "ai:blocked":
			ct.Readiness = DuplicateOrStale
			return ct
		case "needs-detail", "needs-triage":
			ct.Readiness = BlockedInsufficient
			return ct
		}
	}

	// G013/G015: Readiness gate — validate ticket quality before allowing execution
	rdyCheck := readinessValidator.Check(readiness.TicketInput{
		Key:         issue.Key,
		Title:       issue.Fields.Summary,
		Description: string(issue.Fields.Description), // raw JSON string (may be ADF) — readiness checks length only
		Labels:      issue.Fields.Labels,
	})
	if rdyCheck.Status == readiness.StatusNotReady {
		ct.Readiness = BlockedInsufficient
		return ct
	}

	isSecurity := false
	for _, l := range ct.Labels {
		if l == "security" || l == "security:high" || l == "remote-code-execution" {
			isSecurity = true
			break
		}
	}

	switch {
	case isSecurity:
		ct.Readiness = ReadyWithReview
	case ct.Priority == "High" || ct.Priority == "Highest":
		ct.Readiness = ReadyWithReview
	default:
		ct.Readiness = ReadyForExecution
	}
	return ct
}

func classifyTickets(issues []jiraIssue) []ClassifiedTicket {
	out := make([]ClassifiedTicket, len(issues))
	for i, issue := range issues {
		out[i] = classifyTicket(issue)
	}
	return out
}

func countReadiness(tickets []ClassifiedTicket, r TicketReadiness) int {
	n := 0
	for _, t := range tickets {
		if t.Readiness == r {
			n++
		}
	}
	return n
}

// ─── Jira API ────────────────────────────────────────────────────────

func jiraSearch(jcfg jiraConfig, jql string, maxResults int) ([]jiraIssue, int, error) {
	if !jcfg.enabled {
		return nil, 0, fmt.Errorf("jira not configured")
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     []string{"summary", "description", "labels", "status", "priority"},
	})
	req, err := http.NewRequest("POST", jcfg.url+"/rest/api/3/search/jql", strings.NewReader(string(payload)))
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var result struct {
		Total  int         `json:"total"`
		Issues []jiraIssue `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}
	return result.Issues, result.Total, nil
}

func jiraTransition(jcfg jiraConfig, key, targetName string) bool {
	if !jcfg.enabled {
		return false
	}
	// Get transitions
	req, _ := http.NewRequest("GET", jcfg.url+"/rest/api/3/issue/"+key+"/transitions", nil)
	req.SetBasicAuth(jcfg.email, jcfg.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	var tr struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
	}
	// Manual parse — the struct tag is wrong, use generic
	var trRaw struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	json.NewDecoder(resp.Body).Decode(&trRaw)
	resp.Body.Close()
	_ = tr // suppress unused

	var tid string
	for _, t := range trRaw.Transitions {
		if strings.EqualFold(t.Name, targetName) {
			tid = t.ID
			break
		}
	}
	if tid == "" {
		return false
	}

	body, _ := json.Marshal(map[string]interface{}{
		"transition": map[string]string{"id": tid},
	})
	req2, _ := http.NewRequest("POST", jcfg.url+"/rest/api/3/issue/"+key+"/transitions", strings.NewReader(string(body)))
	req2.SetBasicAuth(jcfg.email, jcfg.token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		return false
	}
	resp2.Body.Close()
	return resp2.StatusCode == 204
}

// ─── Factory State ───────────────────────────────────────────────────

type FactoryState struct {
	Active          int32 // currently in-flight
	Done            int32 // completed this run
	Failed          int32 // failed this run
	SafeConcurrency int
}

type factoryConfig struct {
	RepoRoot        string
	ArtifactRoot    string
	EvidenceRoot    string
	MetricsDir      string
	SafeConcurrency int // DEPRECATED: kept for backward compat, controller overrides
	L1Endpoint      string
	L1Model         string
	PollInterval    time.Duration
	TimeoutSec      int
	MaxDispatch     int // max tickets to fetch per cycle
	Jcfg            jiraConfig
	ConcurrencyCfg  concurrency.Config
}

// Package-level readiness validator (G013/G015).
// Initialized once, used by classifyTicket to enforce the executable contract.
var readinessValidator = readiness.NewValidator()

func loadConfig() factoryConfig {
	jcfg := loadJiraConfig()
	concCfg := concurrency.LoadConfigFromEnv()
	return factoryConfig{
		RepoRoot:        envOr("REPO_ROOT", "/home/neves/zen/zen-brain1"),
		ArtifactRoot:    envOr("ARTIFACT_ROOT", "/var/lib/zen-brain1/runs"),
		EvidenceRoot:    envOr("EVIDENCE_ROOT", "/var/lib/zen-brain1/evidence"),
		MetricsDir:      envOr("METRICS_DIR", "/var/lib/zen-brain1/metrics"),
		SafeConcurrency: envIntOr("SAFE_L1_CONCURRENCY", 5), // DEPRECATED: controller overrides
		L1Endpoint:      envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:         envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		PollInterval:    envDurationOr("POLL_INTERVAL", "30s"),
		TimeoutSec:      envIntOr("TIMEOUT_SEC", 120),
		MaxDispatch:     envIntOr("MAX_DISPATCH", 15),
		Jcfg:            jcfg,
		ConcurrencyCfg:  concCfg,
	}
}

// ─── Worker Terminal Result (PHASE A FIX) ────────────────────────────

// WorkerTerminalResult mirrors the struct from remediation-worker.
// This is the contract between worker subprocess and factory-fill dispatcher.
// WorkerTerminalResult is the authoritative terminal classification from the worker.
// PHASE 0 FIX: Added git evidence fields. Success requires git-backed execution.
type WorkerTerminalResult struct {
	JiraKey       string `json:"jira_key"`
	TerminalClass string `json:"terminal_class"`
	QualityScore  int    `json:"quality_score"`
	QualityPassed bool   `json:"quality_passed"`
	L1Status      string `json:"l1_status"`
	JiraState     string `json:"jira_state"`
	EvidencePath  string `json:"evidence_path"`
	BlockerReason string `json:"blocker_reason,omitempty"`
	GateLogPath   string `json:"gate_log_path,omitempty"`
	Timestamp     string `json:"timestamp"`

	// PHASE 0 FIX: Git evidence fields — required for success
	ExecutionMode    string   `json:"execution_mode"`          // "proposal_only" | "git_backed_execution"
	GitBranch        string   `json:"git_branch,omitempty"`    // local branch name
	RemoteBranch     string   `json:"remote_branch,omitempty"` // pushed branch (origin/zb/...)
	GitCommit        string   `json:"git_commit,omitempty"`    // SHA
	FilesChanged     []string `json:"files_changed,omitempty"`
	DiffStatPath     string   `json:"diff_stat_path,omitempty"`
	ValidationReport string   `json:"validation_report_path,omitempty"`
	ProofOfWorkPath  string   `json:"proof_of_work_path,omitempty"`
}

// HasGitEvidence returns true if this result represents real git-backed work.
func (r *WorkerTerminalResult) HasGitEvidence() bool {
	return r.ExecutionMode == "git_backed_execution" &&
		r.GitCommit != "" &&
		r.RemoteBranch != "" &&
		len(r.FilesChanged) > 0
}

// readWorkerTerminalResult reads the terminal classification file for a ticket.
func readWorkerTerminalResult(resultDir, jiraKey string) (*WorkerTerminalResult, error) {
	path := filepath.Join(resultDir, jiraKey+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read terminal result for %s: %w", jiraKey, err)
	}
	var result WorkerTerminalResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse terminal result for %s: %w", jiraKey, err)
	}
	return &result, nil
}

// resultDir returns the directory for terminal result files.
func resultDir() string {
	return envOr("RESULT_DIR", "/tmp/zen-brain1-worker-results")
}

// ─── Dispatch ────────────────────────────────────────────────────────

func dispatchTicket(cfg factoryConfig, ticket ClassifiedTicket) bool {
	jcfg := cfg.Jcfg
	log.Printf("[DISPATCH][%s] === ENTER dispatchTicket (summary len=%d, desc len=%d, readiness=%s) ===",
		ticket.Key, len(ticket.Summary), len(ticket.Description), ticket.Readiness)

	// PHASE 3 NORMALIZATION: Auto-enrich ticket with bounded execution packet
	// before dispatching to worker
	log.Printf("[DISPATCH][%s] PHASE: before normalizeTicket", ticket.Key)
	if err := normalizeTicket(jcfg, ticket); err != nil {
		log.Printf("[NORMALIZE][%s] ❌ FAILED: %v", ticket.Key, err)
		// Check if this is a "no valid files" error - skip dispatch
		if strings.Contains(err.Error(), "no valid target files") {
			log.Printf("[DISPATCH][%s] ❌ SKIPPING - ticket lacks valid target files, marking needs_human_context", ticket.Key)
			jiraTransition(jcfg, ticket.Key, "needs_human_context")
			writeMissingTargetFileResult(resultDir(), ticket.Key)
			return false
		}
		// For other errors, continue - worker will try to determine target file itself
	} else {
		log.Printf("[NORMALIZE][%s] ✅ SUCCESS: enriched with execution packet", ticket.Key)
	}
	log.Printf("[DISPATCH][%s] PHASE: after normalizeTicket", ticket.Key)

	log.Printf("[DISPATCH] %s: moving to In Progress (%s, readiness=%s)", ticket.Key, ticket.Summary[:min(len(ticket.Summary), 50)], ticket.Readiness)

	// Move to In Progress so Jira reflects reality
	if !jiraTransition(jcfg, ticket.Key, "In Progress") {
		log.Printf("[DISPATCH] %s: could not move to In Progress — skipping", ticket.Key)
		return false
	}

	// Clean any stale terminal result for this ticket
	resultDir := resultDir()
	os.MkdirAll(resultDir, 0755)
	staleResult := filepath.Join(resultDir, ticket.Key+".json")
	os.Remove(staleResult)

	// PHASE 0.5 FIX: Single canonical invocation path — no fallback
	// The only valid execution path is: zen-brain worker remediate --ticket-key <key>
	// If zen-brain is not available, fail closed immediately.
	canonicalBin := envOr("CANONICAL_BIN", "zen-brain")

	if _, err := exec.LookPath(canonicalBin); err != nil {
		// PHASE 0.5 FIX: No worker binary → FAIL CLOSED, no fallback
		log.Printf("[DISPATCH] %s: ❌ FAIL CLOSED — zen-brain not found on PATH", ticket.Key)
		jiraTransition(jcfg, ticket.Key, "PAUSED")
		writeMissingWorkerResult(resultDir, ticket.Key)
		return false
	}

	workerBin := canonicalBin
	workerArgs := []string{"worker", "remediate", "--ticket-key", ticket.Key}

	cmd := exec.Command(workerBin, workerArgs...)

	// PHASE 4 FALLBACK: Inject normalized packet if available
	normalizedPacket := os.Getenv("ZEN_NORMALIZED_PACKET_" + ticket.Key)

	log.Printf("[DISPATCH][%s] PHASE: before worker launch (normalized packet len=%d)", ticket.Key, len(normalizedPacket))
	if normalizedPacket != "" {
		log.Printf("[DISPATCH][%s] normalized packet present, first 200 chars: %s", ticket.Key, truncate(normalizedPacket, 200))
	} else {
		log.Printf("[DISPATCH][%s] ⚠️  NO normalized packet - worker will need to infer target file", ticket.Key)
	}

	cmd.Env = append(os.Environ(),
		"MODE=pilot",
		"PILOT_KEYS="+ticket.Key,
		"L1_ENDPOINT="+cfg.L1Endpoint,
		"L1_MODEL="+cfg.L1Model,
		"REPO_ROOT="+cfg.RepoRoot,
		"ARTIFACT_ROOT="+cfg.ArtifactRoot,
		"EVIDENCE_ROOT="+cfg.EvidenceRoot,
		"JIRA_URL="+jcfg.url,
		"JIRA_EMAIL="+jcfg.email,
		"JIRA_API_TOKEN="+jcfg.token,
		"JIRA_PROJECT_KEY="+jcfg.project,
		fmt.Sprintf("REMEDIATION_TIMEOUT=%d", cfg.TimeoutSec),
		"RESULT_DIR="+resultDir,
		"ZEN_DETERMINISTIC_CANARY="+os.Getenv("ZEN_DETERMINISTIC_CANARY"),
		// PHASE 4: Pass normalized packet to worker
		"ZEN_NORMALIZED_PACKET="+normalizedPacket,
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	log.Printf("[DISPATCH][%s] PHASE: after worker launch (exit code=%v, output len=%d)", ticket.Key, err, len(outputStr))

	if err != nil {
		log.Printf("[DISPATCH] %s: worker process error: %v\n%s", ticket.Key, err, lastNLines(outputStr, 3))
		// Worker process crashed — try to read terminal result anyway
		termResult, readErr := readWorkerTerminalResult(resultDir, ticket.Key)
		if readErr != nil {
			// No terminal result — move to RETRYING
			log.Printf("[DISPATCH] %s: no terminal result file, moving to RETRYING", ticket.Key)
			jiraTransition(jcfg, ticket.Key, "RETRYING")
			return false
		}
		// Terminal result exists — use it (worker may have written it before crashing)
		return handleTerminalResult(jcfg, ticket.Key, termResult)
	}

	// PHASE A FIX: Read explicit terminal classification instead of scraping stdout
	termResult, readErr := readWorkerTerminalResult(resultDir, ticket.Key)
	if readErr != nil {
		// PHASE 0 FIX: No terminal result file → FAIL CLOSED (no stdout heuristics)
		log.Printf("[DISPATCH] %s: ❌ FAIL CLOSED — no terminal result file after worker exit", ticket.Key)
		jiraTransition(jcfg, ticket.Key, "RETRYING")
		return false
	}

	return handleTerminalResult(jcfg, ticket.Key, termResult)
}

// handleTerminalResult processes the explicit terminal classification from the worker.
// PHASE A FIX: This is the authoritative state transition — no more stdout guessing.
func handleTerminalResult(jcfg jiraConfig, key string, result *WorkerTerminalResult) bool {
	log.Printf("[DISPATCH] %s: terminal class=%s quality=%d passed=%v jira=%s",
		key, result.TerminalClass, result.QualityScore, result.QualityPassed, result.JiraState)

	switch result.TerminalClass {
	case "done":
		// PHASE 0 FIX: done requires git evidence
		if !result.HasGitEvidence() {
			log.Printf("[DISPATCH] %s: ⚠️ 'done' without git evidence → treating as proposal_only", key)
			jiraTransition(jcfg, key, "PAUSED")
			return false
		}
		log.Printf("[DISPATCH] %s: ✅ done (git-backed: commit=%s branch=%s)", key, shortSHA(result.GitCommit), result.RemoteBranch)
		return true

	case "needs_review":
		// PHASE 0 FIX: needs_review requires git evidence (pushed branch awaiting human review)
		if !result.HasGitEvidence() {
			log.Printf("[DISPATCH] %s: ⚠️ 'needs_review' without git evidence → treating as proposal_only", key)
			jiraTransition(jcfg, key, "PAUSED")
			return false
		}
		log.Printf("[DISPATCH] %s: ✅ needs_review (git-backed: commit=%s branch=%s)", key, shortSHA(result.GitCommit), result.RemoteBranch)
		return true

	case "paused":
		// Quality gate rejected or blocked — already moved to PAUSED by worker
		log.Printf("[DISPATCH] %s: ⏸️ paused (quality gate: %d/25)", key, result.QualityScore)
		// Verify Jira state — if still In Progress, force to PAUSED
		if result.JiraState != "PAUSED" {
			log.Printf("[DISPATCH] %s: correcting state to PAUSED (was %s)", key, result.JiraState)
			jiraTransition(jcfg, key, "PAUSED")
		}
		return false

	case "blocked_invalid_payload":
		// Quality gate rejected with explicit invalid payload classification
		log.Printf("[DISPATCH] %s: 🚫 blocked invalid payload (quality: %d/25, reason: %s)",
			key, result.QualityScore, result.BlockerReason)
		if result.JiraState != "PAUSED" {
			jiraTransition(jcfg, key, "PAUSED")
		}
		return false

	case "retrying":
		log.Printf("[DISPATCH] %s: 🔄 retrying (L1 failed)", key)
		if result.JiraState != "RETRYING" {
			jiraTransition(jcfg, key, "RETRYING")
		}
		return false

	case "to_escalate":
		log.Printf("[DISPATCH] %s: ⬆️ escalated", key)
		if result.JiraState != "TO_ESCALATE" {
			jiraTransition(jcfg, key, "TO_ESCALATE")
		}
		return false

	case "failed":
		log.Printf("[DISPATCH] %s: ❌ failed: %s", key, result.BlockerReason)
		jiraTransition(jcfg, key, "RETRYING")
		return false

	default:
		log.Printf("[DISPATCH] %s: ⚠️ unknown terminal class %q — falling back to PAUSED", key, result.TerminalClass)
		jiraTransition(jcfg, key, "PAUSED")
		return false
	}
}

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// shortSHA returns first 7 chars of a git SHA for logging.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// writeMissingWorkerResult creates a terminal result file when no worker binary was found.
// This ensures the next dispatch cycle knows why the ticket was paused.
func writeMissingWorkerResult(resultDir, jiraKey string) {
	result := WorkerTerminalResult{
		JiraKey:       jiraKey,
		TerminalClass: "paused",
		QualityPassed: false,
		BlockerReason: "no worker binary found on PATH (neither zen-brain nor remediation-worker)",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(result)
	path := filepath.Join(resultDir, jiraKey+".json")
	os.WriteFile(path, data, 0644)
}

// writeMissingTargetFileResult creates a terminal result file when no valid target files were inferred.
// This ensures the next dispatch cycle knows why the ticket was marked needs_human_context.
func writeMissingTargetFileResult(resultDir, jiraKey string) {
	result := WorkerTerminalResult{
		JiraKey:       jiraKey,
		TerminalClass: "needs_human_context",
		QualityPassed: false,
		BlockerReason: "inferred component(s) from ticket but no matching file paths found in current repository",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(result)
	path := filepath.Join(resultDir, jiraKey+".json")
	os.WriteFile(path, data, 0644)
}

// ─── Fill Loop ───────────────────────────────────────────────────────

func runFillCycle(cfg factoryConfig, state *FactoryState, ctrl *concurrency.Controller, dash *concurrency.Dashboard) {
	active := int(atomic.LoadInt32(&state.Active))

	log.Printf("[FACTORY] Cycle start: active=%d", active)

	// Fetch backlog tickets — use a larger window than MaxDispatch to avoid
	// starvation where ready tickets are beyond the fetch limit.
	// MaxDispatch still caps actual dispatches below (after readiness filtering).
	const candidateFetchLimit = 50
	tickets, err := fetchBacklogTickets(cfg.Jcfg, candidateFetchLimit)
	if err != nil {
		log.Printf("[FACTORY] Failed to fetch backlog: %v", err)
		return
	}

	// Classify
	classified := classifyTickets(tickets)

	readyCount := 0
	for _, t := range classified {
		if t.Readiness == ReadyForExecution || t.Readiness == ReadyWithReview {
			readyCount++
		}
	}

	log.Printf("[FACTORY] Backlog: %d fetched, %d ready, %d blocked, %d scan-artifacts, %d stale, %d insufficient-spec",
		len(classified), readyCount,
		countReadiness(classified, BlockedMissingCtx)+countReadiness(classified, BlockedGovernance),
		countReadiness(classified, ScanBatchArtifact),
		countReadiness(classified, DuplicateOrStale),
		countReadiness(classified, BlockedInsufficient))

	if readyCount == 0 {
		log.Printf("[FACTORY] No ready tickets — idle")
		// Record metrics even when idle
		m := ctrl.Metrics()
		m.ReadyCount = 0
		dash.Record(m)
		return
	}

	// Dynamic concurrency calculation (R035–R041)
	desired, throttleReason := ctrl.DesiredConcurrency(readyCount, active)
	slotsAvailable := desired - active
	if slotsAvailable < 0 {
		slotsAvailable = 0
	}

	// Update metrics with actual backlog info
	m := ctrl.Metrics()
	m.BacklogCount = len(classified)
	m.ReadyCount = readyCount
	dash.Record(m)

	log.Printf("[FACTORY] Dynamic concurrency: desired=%d running=%d slots=%d cpu=%.1f%% throttle=%q",
		desired, active, slotsAvailable, m.CPUPercent, throttleReason)

	// Health signal: idle workers while runnable work exists = bug (R041 invariant 1)
	if slotsAvailable > 0 && active < desired {
		log.Printf("[FACTORY] ⚠️ IDLE WORKERS BUG: %d slots available with %d ready tickets — filling now",
			slotsAvailable, readyCount)
	}

	if slotsAvailable <= 0 {
		if throttleReason != "" {
			log.Printf("[FACTORY] Throttled: %s", throttleReason)
		} else {
			log.Printf("[FACTORY] Factory at desired capacity (%d/%d) — waiting", active, desired)
		}
		return
	}

	// Pick tickets to dispatch (up to slots available)
	toDispatch := min(slotsAvailable, readyCount)
	ready := filterReady(classified)

	// Sort: ReadyForExecution first, then by priority
	sortTicketsByPriority(ready)

	if len(ready) > toDispatch {
		ready = ready[:toDispatch]
	}

	log.Printf("[FACTORY] Dispatching %d tickets (reserved=%d for scheduled)", len(ready), m.ReservedSlots)

	var wg sync.WaitGroup
	for _, ticket := range ready {
		wg.Add(1)
		atomic.AddInt32(&state.Active, 1)
		go func(t ClassifiedTicket) {
			defer wg.Done()
			defer atomic.AddInt32(&state.Active, -1)

			if dispatchTicket(cfg, t) {
				atomic.AddInt32(&state.Done, 1)
			} else {
				atomic.AddInt32(&state.Failed, 1)
			}
		}(ticket)
	}
	wg.Wait()

	// PHASE A FIX: Post-dispatch state reconciliation
	reconcileDispatchedStates(cfg, ready)
}

// reconcileDispatchedStates checks every ticket dispatched this cycle.
// PHASE A FIX: Now also reads terminal result files as the authoritative source.
// Falls back to label-based heuristic if terminal result is missing.
func reconcileDispatchedStates(cfg factoryConfig, dispatched []ClassifiedTicket) {
	jcfg := cfg.Jcfg
	rDir := resultDir()
	fixed := 0

	for _, ticket := range dispatched {
		// PHASE A FIX: Check terminal result file first (authoritative)
		termResult, err := readWorkerTerminalResult(rDir, ticket.Key)
		if err == nil {
			// We have an explicit terminal classification
			// Verify Jira state matches
			issues, _, searchErr := jiraSearch(jcfg,
				fmt.Sprintf("project=%s AND key=%s", jcfg.project, ticket.Key), 1)
			if searchErr != nil || len(issues) == 0 {
				log.Printf("[RECONCILE] %s: could not fetch Jira state: %v", ticket.Key, searchErr)
				continue
			}

			actualStatus := issues[0].Fields.Status.Name
			_ = termResult.JiraState // used for logging in handleTerminalResult

			// If terminal result says done/paused/retrying but Jira still says In Progress, fix it
			if actualStatus == "In Progress" {
				switch termResult.TerminalClass {
				case "done", "needs_review":
					log.Printf("[RECONCILE] %s: terminal=%s but Jira=In Progress → Done", ticket.Key, termResult.TerminalClass)
					if jiraTransition(jcfg, ticket.Key, "Done") {
						fixed++
					}
				case "paused", "blocked_invalid_payload":
					log.Printf("[RECONCILE] %s: terminal=%s but Jira=In Progress → PAUSED", ticket.Key, termResult.TerminalClass)
					if jiraTransition(jcfg, ticket.Key, "PAUSED") {
						fixed++
					}
				case "retrying", "failed":
					log.Printf("[RECONCILE] %s: terminal=%s but Jira=In Progress → RETRYING", ticket.Key, termResult.TerminalClass)
					if jiraTransition(jcfg, ticket.Key, "RETRYING") {
						fixed++
					}
				case "to_escalate":
					log.Printf("[RECONCILE] %s: terminal=%s but Jira=In Progress → TO_ESCALATE", ticket.Key, termResult.TerminalClass)
					if jiraTransition(jcfg, ticket.Key, "TO_ESCALATE") {
						fixed++
					}
				default:
					log.Printf("[RECONCILE] %s: unknown terminal=%s, Jira=%s — no action", ticket.Key, termResult.TerminalClass, actualStatus)
				}
			} else {
				log.Printf("[RECONCILE] %s: ✅ terminal=%s Jira=%s (consistent)", ticket.Key, termResult.TerminalClass, actualStatus)
			}
			continue
		}

		// No terminal result — fall back to label-based heuristic
		issues, _, fetchErr := jiraSearch(jcfg,
			fmt.Sprintf("project=%s AND key=%s", jcfg.project, ticket.Key), 1)
		if fetchErr != nil || len(issues) == 0 {
			log.Printf("[RECONCILE] %s: could not fetch state: %v", ticket.Key, fetchErr)
			continue
		}

		issue := issues[0]
		status := issue.Fields.Status.Name
		labels := issue.Fields.Labels

		hasRemediated := false
		hasQualityBlocked := false
		hasQualityReady := false
		for _, l := range labels {
			if l == "ai:remediated" {
				hasRemediated = true
			}
			if l == "quality:blocked-invalid-payload" {
				hasQualityBlocked = true
			}
			if l == "quality:ready-for-execution" || l == "quality:ready-with-review" {
				hasQualityReady = true
			}
		}

		switch {
		case status == "Done":
			log.Printf("[RECONCILE] %s: ✅ already Done", ticket.Key)

		case status == "In Progress" && hasRemediated && hasQualityBlocked:
			log.Printf("[RECONCILE] %s: ⚠️ quality-gate rejected but stuck In Progress → PAUSED", ticket.Key)
			if jiraTransition(jcfg, ticket.Key, "PAUSED") {
				log.Printf("[RECONCILE] %s: moved to PAUSED", ticket.Key)
				fixed++
			}

		case status == "In Progress" && hasRemediated && hasQualityReady:
			log.Printf("[RECONCILE] %s: ⚠️ quality passed but stuck In Progress → Done", ticket.Key)
			if jiraTransition(jcfg, ticket.Key, "Done") {
				log.Printf("[RECONCILE] %s: moved to Done", ticket.Key)
				fixed++
			}

		case status == "In Progress" && !hasRemediated:
			log.Printf("[RECONCILE] %s: ⚠️ no remediation happened, still In Progress → RETRYING", ticket.Key)
			if jiraTransition(jcfg, ticket.Key, "RETRYING") {
				log.Printf("[RECONCILE] %s: moved to RETRYING", ticket.Key)
				fixed++
			}

		case status == "RETRYING":
			log.Printf("[RECONCILE] %s: already RETRYING — factory-fill will pick up next cycle", ticket.Key)

		case status == "PAUSED":
			log.Printf("[RECONCILE] %s: already PAUSED", ticket.Key)

		default:
			log.Printf("[RECONCILE] %s: state=%s labels=%v — no action needed", ticket.Key, status, labels)
		}
	}

	if fixed > 0 {
		log.Printf("[RECONCILE] Fixed %d/%d tickets with incorrect terminal states", fixed, len(dispatched))
	}
}

// normalizeTicket enriches a ticket with a canonical bounded execution packet
// PHASE 3: Automatic normalization before worker dispatch
func normalizeTicket(jcfg jiraConfig, ticket ClassifiedTicket) error {
	log.Printf("[NORMALIZE] %s: starting normalization (summary=%d bytes, description=%d bytes)",
		ticket.Key, len(ticket.Summary), len(ticket.Description))

	// PHASE 1 FIX: Use in-memory ticket data - NO redundant Jira fetch
	if ticket.Description == "" {
		log.Printf("[NORMALIZE] %s: ❌ FAILED - no description in ticket data", ticket.Key)
		return fmt.Errorf("no description in in-memory ticket data")
	}

	// Infer target files from in-memory ticket context
	targetFiles := inferTargetFiles(ticket.Summary, ticket.Description)

	if len(targetFiles) == 0 {
		log.Printf("[NORMALIZE] %s: ❌ FAILED - no valid target files exist in repository", ticket.Key)
		// Mark ticket as needing human context - do not dispatch
		log.Printf("[NORMALIZE] %s: marking as needs_human_context - inferred component but no matching files found", ticket.Key)
		return fmt.Errorf("no valid target files in repository for inferred component")
	}

	log.Printf("[NORMALIZE] %s: inferred %d validated target file(s): %v", ticket.Key, len(targetFiles), targetFiles)

	// Generate canonical execution packet
	packet := generateExecutionPacket(ticket, targetFiles)

	// PHASE 4 FALLBACK: Try Jira writeback, but don't fail if it doesn't work
	// For now, we'll inject the packet in-process for the worker
	// Store packet in environment variable for worker to consume
	os.Setenv("ZEN_NORMALIZED_PACKET_"+ticket.Key, packet)

	log.Printf("[NORMALIZE] %s: ✅ SUCCESS - execution packet created and injected in-process", ticket.Key)
	log.Printf("[NORMALIZE] %s: packet preview:\n%s", ticket.Key, truncate(packet, 300))

	return nil
}

// inferTargetFiles extracts candidate file paths from ticket text using repository-true inference
func inferTargetFiles(summary, description string) []string {
	var candidates []string
	text := strings.ToLower(summary + " " + description)

	log.Printf("[NORMALIZE-INFERENCE] searching for file paths in %d bytes of text", len(text))

	// Repository-true component mappings - verified against actual repo structure
	// These paths exist in the current checkout
	componentFiles := map[string][]string{
		"scheduler":    {"cmd/scheduler/main.go"},
		"factory-fill": {"cmd/factory-fill/main.go"},
		"foreman":      {"cmd/foreman/main.go"},
		"readiness":    {"internal/readiness/validator.go"},
		"docs":         {"docs/README.md"},
		"config":       {"config/policy/README.md"},
		"validator":    {"internal/readiness/validator.go"},
		"auth":         {"internal/apiserver/auth.go", "internal/office/jira/auth_check.go"},
		"api":          {"internal/apiserver/auth.go", "internal/apiserver/server.go"},
		"worker":       {"cmd/zen-brain/worker.go"},
		"analyzer":     {"internal/analyzer/analyzer.go"},
		"factory":      {"internal/factory/factory.go"},
		"agent":        {"internal/agent/binding.go"},
		"jira":         {"internal/office/jira/auth_check.go"},
	}

	// Check for component mentions in ticket text
	for component, paths := range componentFiles {
		if strings.Contains(text, component) {
			candidates = append(candidates, paths...)
			log.Printf("[NORMALIZE-INFERENCE] found component keyword '%s' -> %v", component, paths)
		}
	}

	// Check for explicit file path mentions in description
	pathPatterns := []string{"docs/", "internal/", "cmd/", "config/"}

	for _, pattern := range pathPatterns {
		idx := strings.Index(text, pattern)
		for idx >= 0 && len(candidates) < 10 { // Limit candidates
			rest := text[idx:]
			// Find end of path (stop at whitespace, punctuation, or quote)
			end := len(rest)
			for i, c := range rest {
				if c == ' ' || c == '\n' || c == '\t' || c == '"' || c == ',' || c == ')' || c == '}' {
					end = i
					break
				}
			}

			candidate := rest[:end]
			// Check if it looks like a valid file path
			if strings.Contains(candidate, ".") && !strings.Contains(candidate, " ") && len(candidate) > len(pattern)+3 {
				// Restore original case from description
				origIdx := strings.Index(strings.ToLower(description), candidate)
				if origIdx >= 0 {
					candidate = description[origIdx : origIdx+len(candidate)]
				}
				candidates = append(candidates, candidate)
				log.Printf("[NORMALIZE-INFERENCE] found explicit path pattern '%s' -> %s", pattern, candidate)
			}

			// Find next occurrence
			nextIdx := strings.Index(text[idx+len(pattern):], pattern)
			if nextIdx < 0 {
				break
			}
			idx = idx + len(pattern) + nextIdx
		}
	}

	// VALIDATION: Check each candidate against actual repository
	var validFiles []string
	var discarded []string

	for _, f := range candidates {
		// Skip directories (paths ending with /)
		if strings.HasSuffix(f, "/") {
			log.Printf("[NORMALIZE-INFERENCE] discard (directory): %s", f)
			discarded = append(discarded, fmt.Sprintf("directory: %s", f))
			continue
		}

		// Check if file exists in repository
		if fileExists(f) {
			validFiles = append(validFiles, f)
			log.Printf("[NORMALIZE-INFERENCE] ✓ validated: %s", f)
		} else {
			log.Printf("[NORMALIZE-INFERENCE] discard (not found): %s", f)
			discarded = append(discarded, fmt.Sprintf("not found: %s", f))
		}
	}

	// Deduplicate valid files
	seen := make(map[string]bool)
	var result []string
	for _, f := range validFiles {
		if !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}

	log.Printf("[NORMALIZE-INFERENCE] discarded %d candidates: %v", len(discarded), discarded)
	log.Printf("[NORMALIZE-INFERENCE] final validated result: %d unique file(s): %v", len(result), result)
	return result
}

// fileExists checks if a file exists in the repository
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// generateExecutionPacket creates the canonical bounded execution packet
func generateExecutionPacket(ticket ClassifiedTicket, targetFiles []string) string {
	var sb strings.Builder

	sb.WriteString("BOUNDED_EXECUTION:\n")
	sb.WriteString(fmt.Sprintf("  version: \"1.0\"\n"))
	sb.WriteString("  target_files:\n")

	for _, f := range targetFiles {
		sb.WriteString(fmt.Sprintf("    - path: %s\n", f))
		sb.WriteString(fmt.Sprintf("      confidence: 0.85\n"))
		sb.WriteString("      reason: \"Inferred from ticket context (summary + description)\"\n")
	}

	sb.WriteString(fmt.Sprintf("  scope:\n"))
	sb.WriteString(fmt.Sprintf("    blast_radius: low\n"))
	sb.WriteString(fmt.Sprintf("    execution_class: bounded_fix\n"))
	sb.WriteString(fmt.Sprintf("    bounded: true\n"))

	sb.WriteString(fmt.Sprintf("  inference_metadata:\n"))
	sb.WriteString(fmt.Sprintf("    generated_by: factory-fill-normalizer-v1\n"))
	sb.WriteString(fmt.Sprintf("    generated_at: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("    overall_confidence: 0.80\n"))
	sb.WriteString(fmt.Sprintf("    source: in-memory-ticket-data\n"))

	return sb.String()
}

// truncate safely truncates a string
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// jiraGetIssue fetches a single issue from Jira
func jiraGetIssue(jcfg jiraConfig, key string) (*jiraIssue, error) {
	req, _ := http.NewRequest("GET", jcfg.url+"/rest/api/2/issue/"+key+"?fields=summary,description", nil)
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var issue jiraIssue
	if err := json.NewDecoder(resp.Body).Decode(&issue); err != nil {
		return nil, err
	}

	return &issue, nil
}

// jiraUpdateDescription updates the description field of a Jira issue
func jiraUpdateDescription(jcfg jiraConfig, key, description string) error {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"description": description,
		},
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", jcfg.url+"/rest/api/2/issue/"+key, bytes.NewReader(data))
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	return nil
}

// extractTextFromADF extracts plain text from Jira ADF (Atlassian Document Format)
func extractTextFromADF(adf json.RawMessage) string {
	var parse func(interface{}) string
	parse = func(v interface{}) string {
		var sb strings.Builder

		switch val := v.(type) {
		case string:
			return val
		case []interface{}:
			for _, item := range val {
				sb.WriteString(parse(item))
			}
		case map[string]interface{}:
			if text, ok := val["text"].(string); ok {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
			if content, ok := val["content"]; ok {
				sb.WriteString(parse(content))
				sb.WriteString("\n")
			}
		}

		return sb.String()
	}

	var parsed interface{}
	if err := json.Unmarshal(adf, &parsed); err != nil {
		return string(adf)
	}

	return parse(parsed)
}

func fetchBacklogTickets(jcfg jiraConfig, max int) ([]jiraIssue, error) {
	// Fetch actionable backlog tickets (bug label, not scheduled-batch)
	issues, _, err := jiraSearch(jcfg,
		fmt.Sprintf("project=%s AND status=Backlog AND labels=bug ORDER BY priority DESC, created ASC", jcfg.project),
		max)
	if err != nil {
		return nil, err
	}

	// Also fetch retrying tickets
	retrying, _, err := jiraSearch(jcfg,
		fmt.Sprintf("project=%s AND status=RETRYING ORDER BY updated ASC", jcfg.project),
		10)
	if err == nil {
		issues = append(issues, retrying...)
	}

	return issues, nil
}

func filterReady(tickets []ClassifiedTicket) []ClassifiedTicket {
	var ready []ClassifiedTicket
	for _, t := range tickets {
		if t.Readiness == ReadyForExecution || t.Readiness == ReadyWithReview {
			ready = append(ready, t)
		}
	}
	return ready
}

func sortTicketsByPriority(tickets []ClassifiedTicket) {
	priorityVal := func(p string) int {
		switch strings.ToLower(p) {
		case "highest":
			return 0
		case "high":
			return 1
		case "medium":
			return 2
		default:
			return 3
		}
	}
	readinessVal := func(r TicketReadiness) int {
		if r == ReadyForExecution {
			return 0
		}
		return 1
	}
	for i := 0; i < len(tickets); i++ {
		for j := i + 1; j < len(tickets); j++ {
			ri, rj := readinessVal(tickets[i].Readiness), readinessVal(tickets[j].Readiness)
			if ri != rj {
				if ri > rj {
					tickets[i], tickets[j] = tickets[j], tickets[i]
				}
				continue
			}
			if priorityVal(tickets[i].Priority) > priorityVal(tickets[j].Priority) {
				tickets[i], tickets[j] = tickets[j], tickets[i]
			}
		}
	}
}

// ─── Dashboard (PHASE 6) ─────────────────────────────────────────────

type BoardSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	BacklogReady int       `json:"backlog_ready"`
	BacklogTotal int       `json:"backlog_total"`
	Retrying     int       `json:"retrying"`
	InProgress   int       `json:"in_progress"`
	SafeTarget   int       `json:"safe_target"`
	ActualActive int       `json:"actual_active"`
	DoneCount    int       `json:"done_count"`
	FailedCount  int       `json:"failed_count"`
	Underfill    bool      `json:"underfill"`
	Notes        string    `json:"notes,omitempty"`
}

func (s *FactoryState) GetDone() int   { return int(atomic.LoadInt32(&s.Done)) }
func (s *FactoryState) GetFailed() int { return int(atomic.LoadInt32(&s.Failed)) }

func writeDashboard(cfg factoryConfig, state *FactoryState, ctrl *concurrency.Controller) {
	// Get current board counts
	backlogIssues, _, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=Backlog AND labels=bug", cfg.Jcfg.project), 1)
	_ = backlogIssues

	retrying, _, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=RETRYING", cfg.Jcfg.project), 1)
	_ = retrying

	inProg, _, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=\"In Progress\"", cfg.Jcfg.project), 1)
	_ = inProg

	// Re-classify backlog to count ready
	allBacklog, totalBacklog, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=Backlog AND labels=bug", cfg.Jcfg.project), 50)
	readyCount := 0
	for _, issue := range allBacklog {
		ct := classifyTicket(issue)
		if ct.Readiness == ReadyForExecution || ct.Readiness == ReadyWithReview {
			readyCount++
		}
	}

	// Get actual counts from search total
	_, retryCount, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=RETRYING", cfg.Jcfg.project), 1)
	_, inProgCount, _ := jiraSearch(cfg.Jcfg,
		fmt.Sprintf("project=%s AND status=\"In Progress\"", cfg.Jcfg.project), 1)

	active := int(atomic.LoadInt32(&state.Active))
	cm := ctrl.Metrics()
	underfill := active < cm.DesiredGeneral && readyCount > 0

	snap := BoardSnapshot{
		Timestamp:    time.Now(),
		BacklogReady: readyCount,
		BacklogTotal: totalBacklog,
		Retrying:     retryCount,
		InProgress:   inProgCount,
		SafeTarget:   int(cm.DesiredGeneral), // now dynamic
		ActualActive: active,
		DoneCount:    state.GetDone(),
		FailedCount:  state.GetFailed(),
		Underfill:    underfill,
	}

	// Write JSON
	data, _ := json.MarshalIndent(snap, "", "  ")
	dashPath := filepath.Join(cfg.MetricsDir, "factory-dashboard.json")
	os.WriteFile(dashPath, data, 0644)

	// Append to JSONL history
	histPath := filepath.Join(cfg.MetricsDir, "factory-fill-history.jsonl")
	line, _ := json.Marshal(snap)
	if f, err := os.OpenFile(histPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		f.Write(line)
		f.Write([]byte("\n"))
		f.Close()
	}

	// Human-readable dashboard
	var sb strings.Builder
	sb.WriteString("# Factory Fill Dashboard\n\n")
	sb.WriteString(fmt.Sprintf("Updated: %s\n\n", snap.Timestamp.Format(time.RFC3339)))
	sb.WriteString("| Metric | Value |\n|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Backlog (ready) | %d |\n", snap.BacklogReady))
	sb.WriteString(fmt.Sprintf("| Backlog (total) | %d |\n", snap.BacklogTotal))
	sb.WriteString(fmt.Sprintf("| Retrying | %d |\n", snap.Retrying))
	sb.WriteString(fmt.Sprintf("| In Progress | %d |\n", snap.InProgress))
	sb.WriteString(fmt.Sprintf("| Safe Target | %d |\n", snap.SafeTarget))
	sb.WriteString(fmt.Sprintf("| Actual Active | %d |\n", snap.ActualActive))
	sb.WriteString(fmt.Sprintf("| Done this run | %d |\n", snap.DoneCount))
	sb.WriteString(fmt.Sprintf("| Failed this run | %d |\n", snap.FailedCount))
	if underfill {
		sb.WriteString(fmt.Sprintf("\n⚠️ **UNDERFILL**: %d slots unused with %d ready tickets\n",
			snap.SafeTarget-snap.ActualActive, snap.BacklogReady))
	} else {
		sb.WriteString("\n✅ Factory utilization OK\n")
	}
	sb.WriteString(fmt.Sprintf("\n## Concurrency Controller\n"))
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total slots | %d |\n", cm.TotalSlots))
	sb.WriteString(fmt.Sprintf("| Reserved (scheduled) | %d |\n", cm.ReservedSlots))
	sb.WriteString(fmt.Sprintf("| Usable (general) | %d |\n", cm.UsableSlots))
	sb.WriteString(fmt.Sprintf("| Desired general workers | %d |\n", cm.DesiredGeneral))
	sb.WriteString(fmt.Sprintf("| CPU pressure | %.1f%% |\n", cm.CPUPercent))
	if cm.IsThrottled {
		sb.WriteString(fmt.Sprintf("| Throttle reason | %s |\n", cm.ThrottleReason))
	} else {
		sb.WriteString("| Throttle reason | none |\n")
	}
	if cm.IsConservative {
		sb.WriteString("| Mode | CONSERVATIVE (telemetry degraded) |\n")
	} else {
		sb.WriteString("| Mode | normal |\n")
	}
	sb.WriteString("\n## Operating Policy\n")
	sb.WriteString("- Adaptive concurrency (no static W values)\n")
	sb.WriteString("- Work-conserving: idle workers with runnable work = BUG\n")
	sb.WriteString("- 2 model slots reserved for scheduled tasks\n")
	sb.WriteString("- Throttle on CPU soft cap (80%), hard stop at 90%\n")
	sb.WriteString("- Hysteresis prevents concurrency oscillation\n")
	sb.WriteString("- Conservative fallback on telemetry failure\n")
	sb.WriteString("- Jira In Progress reflects actual active work\n")
	sb.WriteString("- Success = done-rate + honest attribution\n")

	os.MkdirAll(filepath.Join(cfg.RepoRoot, "docs/05-OPERATIONS/evidence"), 0755)
	os.WriteFile(filepath.Join(cfg.RepoRoot, "docs/05-OPERATIONS/evidence/factory-fill-and-backlog-utilization.md"), []byte(sb.String()), 0644)

	log.Printf("[DASHBOARD] ready=%d retry=%d in_prog=%d target=%d active=%d underfill=%v",
		snap.BacklogReady, snap.Retrying, snap.InProgress, snap.SafeTarget, snap.ActualActive, underfill)
}

// ─── Main ────────────────────────────────────────────────────────────

// reconcileStuckInProgress checks all In Progress tickets and fixes terminal states.
// This runs at the start of every cycle, even when no ready backlog tickets exist.
// PHASE A FIX: Also checks terminal result files for authoritative classification.
func reconcileStuckInProgress(jcfg jiraConfig) {
	if !jcfg.enabled {
		return
	}

	issues, _, err := jiraSearch(jcfg,
		fmt.Sprintf("project=%s AND status=\"In Progress\" ORDER BY updated ASC", jcfg.project),
		50)
	if err != nil || len(issues) == 0 {
		return
	}

	rDir := resultDir()
	fixed := 0
	for _, issue := range issues {
		key := issue.Key
		labels := issue.Fields.Labels

		// PHASE A FIX: Check terminal result file first (authoritative)
		if termResult, err := readWorkerTerminalResult(rDir, key); err == nil {
			switch termResult.TerminalClass {
			case "done", "needs_review":
				log.Printf("[RECONCILE] %s: terminal=%s (file), stuck In Progress → Done", key, termResult.TerminalClass)
				if jiraTransition(jcfg, key, "Done") {
					fixed++
				}
				continue
			case "paused", "blocked_invalid_payload":
				log.Printf("[RECONCILE] %s: terminal=%s (file), stuck In Progress → PAUSED", key, termResult.TerminalClass)
				if jiraTransition(jcfg, key, "PAUSED") {
					fixed++
				}
				continue
			case "retrying", "failed":
				log.Printf("[RECONCILE] %s: terminal=%s (file), stuck In Progress → RETRYING", key, termResult.TerminalClass)
				if jiraTransition(jcfg, key, "RETRYING") {
					fixed++
				}
				continue
			case "to_escalate":
				log.Printf("[RECONCILE] %s: terminal=%s (file), stuck In Progress → TO_ESCALATE", key, termResult.TerminalClass)
				if jiraTransition(jcfg, key, "TO_ESCALATE") {
					fixed++
				}
				continue
			}
		}

		// Fall back to label-based heuristic
		hasRemediated := false
		hasQualityBlocked := false
		hasQualityReady := false
		for _, l := range labels {
			switch l {
			case "ai:remediated":
				hasRemediated = true
			case "quality:blocked-invalid-payload":
				hasQualityBlocked = true
			case "quality:ready-for-execution", "quality:ready-with-review":
				hasQualityReady = true
			}
		}

		switch {
		case hasRemediated && hasQualityBlocked:
			// Worker ran, quality gate rejected — move to PAUSED
			log.Printf("[RECONCILE] %s: remediated but quality-blocked, stuck In Progress → PAUSED", key)
			if jiraTransition(jcfg, key, "PAUSED") {
				fixed++
			}

		case hasRemediated && hasQualityReady:
			// Worker ran, quality passed, but didn't get to Done — try Done
			log.Printf("[RECONCILE] %s: remediated and quality-passed, stuck In Progress → Done", key)
			if jiraTransition(jcfg, key, "Done") {
				fixed++
			}

		case hasRemediated && !hasQualityBlocked && !hasQualityReady:
			// Remediated but no quality label at all — likely gate didn't fire. Move to Done.
			log.Printf("[RECONCILE] %s: remediated but no quality label, stuck In Progress → Done", key)
			if jiraTransition(jcfg, key, "Done") {
				fixed++
			}

		case !hasRemediated:
			// Dispatched but worker didn't process (env issue, L1 fail, etc.) — move to RETRYING
			log.Printf("[RECONCILE] %s: no remediation, stuck In Progress → RETRYING", key)
			if jiraTransition(jcfg, key, "RETRYING") {
				fixed++
			}
		}
	}

	if fixed > 0 {
		log.Printf("[RECONCILE] Fixed %d/%d stuck In Progress tickets", fixed, len(issues))
	}
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("[FACTORY-FILL] === Backlog-aware factory fill starting ===")

	cfg := loadConfig()
	if !cfg.Jcfg.enabled {
		log.Fatalf("[FACTORY-FILL] Jira not configured — ensure canonical source is available (ZenLock mount at /zen-lock/secrets for cluster, or env fallback for local dev)")
	}

	os.MkdirAll(cfg.MetricsDir, 0755)
	os.MkdirAll(cfg.ArtifactRoot, 0755)
	os.MkdirAll(cfg.EvidenceRoot, 0755)

	state := &FactoryState{SafeConcurrency: cfg.SafeConcurrency}

	// Initialize dynamic concurrency controller (R035–R041)
	ctrl := concurrency.NewController(cfg.ConcurrencyCfg, nil)
	dash := concurrency.NewDashboard(cfg.MetricsDir)

	log.Printf("[FACTORY-FILL] Config: total_slots=%d reserved=%d usable=%d poll=%v l1=%s",
		cfg.ConcurrencyCfg.TotalSlots, cfg.ConcurrencyCfg.ReservedSlots,
		cfg.ConcurrencyCfg.TotalSlots-cfg.ConcurrencyCfg.ReservedSlots,
		cfg.PollInterval, cfg.L1Endpoint)

	// PHASE 4: Emit startup capability summary (no secrets exposed)
	clusterMode := os.Getenv("KUBERNETES_SERVICE_HOST") != ""
	jiraOpts := secrets.JiraResolveOptions{
		DirPath:          "/zen-lock/secrets",
		FilePath:         filepath.Join(os.Getenv("HOME"), ".zen-brain", "secrets", "jira.yaml"),
		AllowEnvFallback: !clusterMode,
		ClusterMode:      clusterMode,
	}
	jiraMaterial, err := secrets.ResolveJira(context.Background(), jiraOpts)
	if err != nil {
		log.Printf("[CAPABILITY] Jira resolver error: %v", err)
	} else {
		log.Printf("[CAPABILITY] Jira Token Source: %s", jiraMaterial.Source)
		caps, capErr := secrets.CheckJiraCapabilities(context.Background(), jiraMaterial)
		if capErr != nil {
			log.Printf("[CAPABILITY] Jira capability check error: %v", capErr)
		} else {
			log.Printf("[CAPABILITY] %s", secrets.FormatJiraCapabilitySummary(jiraMaterial, caps))
		}
	}

	// State reconciliation: always run before fill cycle to fix any tickets stuck In Progress
	// from previous cycles where the worker exited clean but quality gate rejected.
	reconcileStuckInProgress(cfg.Jcfg)

	forceRun := os.Getenv("FORCE_RUN") != "" || os.Getenv("ONCE") != ""
	if forceRun {
		runFillCycle(cfg, state, ctrl, dash)
		writeDashboard(cfg, state, ctrl)
		log.Printf("[FACTORY-FILL] Force run complete: done=%d failed=%d", state.GetDone(), state.GetFailed())
		return
	}

	// Daemon mode
	log.Printf("[FACTORY-FILL] Daemon mode (poll=%v)", cfg.PollInterval)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Initial fill
	runFillCycle(cfg, state, ctrl, dash)
	writeDashboard(cfg, state, ctrl)

	for range ticker.C {
		runFillCycle(cfg, state, ctrl, dash)
		writeDashboard(cfg, state, ctrl)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────

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
		if n > 0 {
			return n
		}
	}
	return fallback
}

func envDurationOr(key, fallback string) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	d, _ := time.ParseDuration(fallback)
	return d
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
