// Command factory-fill implements a backlog-aware dispatch loop that keeps
// the L1 factory filled whenever ready work exists in Jira.
//
// Operating policy:
//   - Underfilled factory with backlog present = BUG
//   - target_in_progress = min(safe_l1_concurrency, ready_backlog + retrying)
//   - If current in-progress < target: pull tickets, dispatch immediately
//   - Do not wait for the next scheduler cycle if ready tickets exist
//   - Jira In Progress must reflect actual active work
package main

import (
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

// ─── Ticket Readiness Classification (PHASE 1) ───────────────────────

type TicketReadiness string

const (
	ReadyForExecution TicketReadiness = "ready_for_execution"
	ReadyWithReview   TicketReadiness = "ready_with_review"
	BlockedMissingCtx TicketReadiness = "blocked_missing_context"
	BlockedGovernance TicketReadiness = "blocked_missing_governance"
	TooLargeForL1     TicketReadiness = "too_large_for_l1"
	ScanBatchArtifact TicketReadiness = "scan_batch_artifact"
	DuplicateOrStale  TicketReadiness = "duplicate_or_stale"
)

type ClassifiedTicket struct {
	Key       string
	Summary   string
	Labels    []string
	Priority  string
	Status    string
	Readiness TicketReadiness
}

type jiraIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary  string   `json:"summary"`
		Labels   []string `json:"labels"`
		Status   struct{ Name string `json:"name"` } `json:"status"`
		Priority struct{ Name string `json:"name"` } `json:"priority"`
	} `json:"fields"`
}

type jiraConfig struct {
	url, email, token, project string
	enabled                    bool
}

func loadJiraConfig() jiraConfig {
	url := os.Getenv("JIRA_URL")
	email := os.Getenv("JIRA_EMAIL")
	token := os.Getenv("JIRA_API_TOKEN")
	if token == "" {
		token = os.Getenv("JIRA_TOKEN")
	}
	project := envOr("JIRA_PROJECT_KEY", "ZB")
	return jiraConfig{url, email, token, project, url != "" && email != "" && token != ""}
}

func classifyTicket(issue jiraIssue) ClassifiedTicket {
	ct := ClassifiedTicket{
		Key:      issue.Key,
		Summary:  issue.Fields.Summary,
		Labels:   issue.Fields.Labels,
		Priority: issue.Fields.Priority.Name,
		Status:   issue.Fields.Status.Name,
	}

	for _, l := range ct.Labels {
		switch l {
		case "scheduled-batch", "hourly-scan", "daily-sweep", "quad-hourly-summary":
			ct.Readiness = ScanBatchArtifact
			return ct
		case "ai:completed", "ai:blocked":
			ct.Readiness = DuplicateOrStale
			return ct
		}
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
		"fields":     []string{"summary", "labels", "status", "priority"},
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
		Transitions []struct{ ID, Name string `json:"id,json:"` }
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
	Active    int32 // currently in-flight
	Done      int32 // completed this run
	Failed    int32 // failed this run
	SafeConcurrency int
}

type factoryConfig struct {
	RepoRoot         string
	ArtifactRoot     string
	EvidenceRoot     string
	MetricsDir       string
	SafeConcurrency  int
	L1Endpoint       string
	L1Model          string
	PollInterval     time.Duration
	TimeoutSec       int
	MaxDispatch      int // max tickets to fetch per cycle
	Jcfg             jiraConfig
}

func loadConfig() factoryConfig {
	jcfg := loadJiraConfig()
	return factoryConfig{
		RepoRoot:        envOr("REPO_ROOT", "/home/neves/zen/zen-brain1"),
		ArtifactRoot:    envOr("ARTIFACT_ROOT", "/var/lib/zen-brain1/runs"),
		EvidenceRoot:    envOr("EVIDENCE_ROOT", "/var/lib/zen-brain1/evidence"),
		MetricsDir:      envOr("METRICS_DIR", "/var/lib/zen-brain1/metrics"),
		SafeConcurrency: envIntOr("SAFE_L1_CONCURRENCY", 5),
		L1Endpoint:      envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:         envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		PollInterval:    envDurationOr("POLL_INTERVAL", "30s"),
		TimeoutSec:      envIntOr("TIMEOUT_SEC", 120),
		MaxDispatch:     envIntOr("MAX_DISPATCH", 15),
		Jcfg:            jcfg,
	}
}

// ─── Worker Terminal Result (PHASE A FIX) ────────────────────────────

// WorkerTerminalResult mirrors the struct from remediation-worker.
// This is the contract between worker subprocess and factory-fill dispatcher.
type WorkerTerminalResult struct {
	JiraKey         string `json:"jira_key"`
	TerminalClass   string `json:"terminal_class"`
	QualityScore    int    `json:"quality_score"`
	QualityPassed   bool   `json:"quality_passed"`
	L1Status        string `json:"l1_status"`
	JiraState       string `json:"jira_state"`
	EvidencePath    string `json:"evidence_path"`
	BlockerReason   string `json:"blocker_reason,omitempty"`
	GateLogPath     string `json:"gate_log_path,omitempty"`
	Timestamp       string `json:"timestamp"`
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

	// Run remediation-worker as subprocess for this single ticket
	workerBin := filepath.Join(filepath.Dir(os.Args[0]), "remediation-worker")
	if _, err := os.Stat(workerBin); err != nil {
		workerBin = filepath.Join(cfg.RepoRoot, "cmd/remediation-worker/remediation-worker")
	}

	cmd := exec.Command(workerBin)
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
	)

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

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
		// No terminal result file — fall back to stdout heuristics
		log.Printf("[DISPATCH] %s: no terminal result file, using stdout heuristics", ticket.Key)
		if strings.Contains(outputStr, "REJECTED") || strings.Contains(outputStr, "BLOCKED") {
			log.Printf("[DISPATCH] %s: quality gate rejected (stdout heuristic)", ticket.Key)
			jiraTransition(jcfg, ticket.Key, "PAUSED")
			return false
		}
		log.Printf("[DISPATCH] %s: ✅ completed (stdout heuristic)", ticket.Key)
		return true
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
		log.Printf("[DISPATCH] %s: ✅ done", key)
		return true

	case "needs_review":
		log.Printf("[DISPATCH] %s: ✅ done (needs review)", key)
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

// ─── Fill Loop ───────────────────────────────────────────────────────

func runFillCycle(cfg factoryConfig, state *FactoryState) {
	active := int(atomic.LoadInt32(&state.Active))
	slotsAvailable := cfg.SafeConcurrency - active

	log.Printf("[FACTORY] Cycle: active=%d target=%d slots=%d done=%d failed=%d",
		active, cfg.SafeConcurrency, slotsAvailable, state.GetDone(), state.GetFailed())

	if slotsAvailable <= 0 {
		log.Printf("[FACTORY] Factory full (%d/%d) — waiting", active, cfg.SafeConcurrency)
		return
	}

	// Fetch backlog tickets
	tickets, err := fetchBacklogTickets(cfg.Jcfg, cfg.MaxDispatch)
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

	log.Printf("[FACTORY] Backlog: %d fetched, %d ready, %d blocked, %d scan-artifacts, %d stale",
		len(classified), readyCount,
		countReadiness(classified, BlockedMissingCtx)+countReadiness(classified, BlockedGovernance),
		countReadiness(classified, ScanBatchArtifact),
		countReadiness(classified, DuplicateOrStale))

	if readyCount == 0 {
		log.Printf("[FACTORY] No ready tickets — idle")
		return
	}

	// Underfill detection: ready work exists but factory not full
	if active < cfg.SafeConcurrency && readyCount > slotsAvailable {
		log.Printf("[FACTORY] ⚠️ UNDERFILL: %d slots empty with %d ready tickets — filling", slotsAvailable, readyCount)
	}

	// Pick tickets to dispatch (up to slots available)
	toDispatch := min(slotsAvailable, readyCount)
	ready := filterReady(classified)

	// Sort: ReadyForExecution first, then by priority
	sortTicketsByPriority(ready)

	if len(ready) > toDispatch {
		ready = ready[:toDispatch]
	}

	log.Printf("[FACTORY] Dispatching %d tickets", len(ready))

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
	// The remediation-worker subprocess may exit 0 even when quality-gate rejects a ticket.
	// That leaves tickets stuck In Progress with no correct terminal state.
	// This step checks every dispatched ticket's actual Jira state and fixes it.
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
	Timestamp      time.Time `json:"timestamp"`
	BacklogReady   int       `json:"backlog_ready"`
	BacklogTotal   int       `json:"backlog_total"`
	Retrying       int       `json:"retrying"`
	InProgress     int       `json:"in_progress"`
	SafeTarget     int       `json:"safe_target"`
	ActualActive   int       `json:"actual_active"`
	DoneCount      int       `json:"done_count"`
	FailedCount    int       `json:"failed_count"`
	Underfill      bool      `json:"underfill"`
	Notes          string    `json:"notes,omitempty"`
}

func (s *FactoryState) GetDone() int   { return int(atomic.LoadInt32(&s.Done)) }
func (s *FactoryState) GetFailed() int { return int(atomic.LoadInt32(&s.Failed)) }

func writeDashboard(cfg factoryConfig, state *FactoryState) {
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
	underfill := active < cfg.SafeConcurrency && readyCount > 0

	snap := BoardSnapshot{
		Timestamp:    time.Now(),
		BacklogReady: readyCount,
		BacklogTotal: totalBacklog,
		Retrying:     retryCount,
		InProgress:   inProgCount,
		SafeTarget:   cfg.SafeConcurrency,
		ActualActive: active,
		DoneCount:         state.GetDone(),
		FailedCount:       state.GetFailed(),
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
	sb.WriteString("\n## Operating Policy\n")
	sb.WriteString("- Underfilled factory with backlog present = BUG\n")
	sb.WriteString("- target_in_progress = min(safe_target, ready_backlog + retrying)\n")
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
		log.Fatalf("[FACTORY-FILL] Jira not configured — set JIRA_URL, JIRA_EMAIL, JIRA_API_TOKEN")
	}

	os.MkdirAll(cfg.MetricsDir, 0755)
	os.MkdirAll(cfg.ArtifactRoot, 0755)
	os.MkdirAll(cfg.EvidenceRoot, 0755)

	state := &FactoryState{SafeConcurrency: cfg.SafeConcurrency}

	log.Printf("[FACTORY-FILL] Config: safe_target=%d, poll=%v, l1=%s",
		cfg.SafeConcurrency, cfg.PollInterval, cfg.L1Endpoint)

	// State reconciliation: always run before fill cycle to fix any tickets stuck In Progress
	// from previous cycles where the worker exited clean but quality gate rejected.
	reconcileStuckInProgress(cfg.Jcfg)

	forceRun := os.Getenv("FORCE_RUN") != "" || os.Getenv("ONCE") != ""
	if forceRun {
		runFillCycle(cfg, state)
		writeDashboard(cfg, state)
		log.Printf("[FACTORY-FILL] Force run complete: done=%d failed=%d", state.GetDone(), state.GetFailed())
		return
	}

	// Daemon mode
	log.Printf("[FACTORY-FILL] Daemon mode (poll=%v)", cfg.PollInterval)
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	// Initial fill
	runFillCycle(cfg, state)
	writeDashboard(cfg, state)

	for range ticker.C {
		runFillCycle(cfg, state)
		writeDashboard(cfg, state)
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
