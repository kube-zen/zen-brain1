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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// zen-brain1 internal recurring scheduler.
// Owns useful-task cadence. Systemd supervises the process; this decides what runs when.
//
// Loads schedule definitions from config/schedules/, determines due work,
// submits batches through the canonical zen-brain runtime, records status.
//
// CANONICAL EXECUTION MODEL:
//   The scheduler uses `zen-brain` as the canonical executable.
//   Schedule behavior is selected by mode + task class, not by binary paths.
//
//   mode=batch (default): runs `zen-brain worker batch` with TASKS env
//   mode=dispatch:        runs `zen-brain dispatch <subcommand>` with args
//
// ENV VARS:
//   CANONICAL_BIN  — path to zen-brain binary (default: zen-brain)
//   SCHEDULE_DIR   — schedule definitions (default: config/schedules/)
//   STATE_DIR      — scheduler state / last-run tracking (default: /var/lib/zen-brain1/scheduler)
//   ARTIFACT_ROOT  — artifact output root (default: /var/lib/zen-brain1/runs)
//   BATCH_BIN      — DEPRECATED: legacy override, prefer CANONICAL_BIN
//   POLL_INTERVAL  — how often to check for due work (default: 60s)
//   FORCE_RUN      — run all schedules once immediately, then exit (default: false)
//   ONCE           — alias for FORCE_RUN

const (
	defaultPollInterval = 60 * time.Second
	defaultScheduleDir  = "config/schedules"
	defaultStateDir     = "/var/lib/zen-brain1/scheduler"
	defaultArtifactRoot = "/var/lib/zen-brain1/runs"
	defaultCanonicalBin = "zen-brain"
)

// Schedule represents a recurring workload definition.
type Schedule struct {
	Name        string   `yaml:"name" json:"name"`
	Tasks       []string `yaml:"tasks" json:"tasks"`
	Cadence     string   `yaml:"cadence" json:"cadence"` // "hourly", "quad-hourly", "daily"
	Description string   `yaml:"description" json:"description"`
	// Workers overrides the default concurrency for this schedule.
	// 0 or unset = use default (5). Set explicitly per schedule for evidence-based tuning.
	Workers int `yaml:"workers" json:"workers"`
	// Mode selects the execution mode:
	//   "batch" (default) — run through batch worker (TASKS env)
	//   "dispatch" — run as dispatch subcommand (e.g., queue, roadmap)
	//   "steward" — alias for dispatch with queue-steward semantics
	Mode string `yaml:"mode" json:"mode,omitempty"`
	// DispatchSubcommand overrides the dispatch subcommand (for mode=dispatch).
	// Defaults to deriving from the first task name (queue_steward → queue, roadmap_steward → roadmap).
	DispatchSubcommand string `yaml:"dispatch_subcommand,omitempty" json:"dispatch_subcommand,omitempty"`
	// StewardMode passes the steward mode flag (e.g., "fast", "summary", "hourly").
	StewardMode string `yaml:"steward_mode,omitempty" json:"steward_mode,omitempty"`
}

// ScheduleState tracks when each schedule last ran.
type ScheduleState struct {
	LastRun    time.Time `json:"last_run"`
	LastStatus string    `json:"last_status"` // "success", "partial", "failed"
	LastDir    string    `json:"last_dir"`
	NextDue    time.Time `json:"next_due"`
	RunCount   int       `json:"run_count"`
}

// SchedulerStatus is the overall status for operator queries.
type SchedulerStatus struct {
	Active       bool            `json:"active"`
	Schedules    []ScheduleEntry `json:"schedules"`
	StateDir     string          `json:"state_dir"`
	ArtifactRoot string          `json:"artifact_root"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type ScheduleEntry struct {
	Name       string   `json:"name"`
	Cadence    string   `json:"cadence"`
	Tasks      []string `json:"tasks"`
	LastRun    string   `json:"last_run,omitempty"`
	NextDue    string   `json:"next_due"`
	LastStatus string   `json:"last_status,omitempty"`
	RunCount   int      `json:"run_count"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	scheduleDir := envOr("SCHEDULE_DIR", defaultScheduleDir)
	stateDir := envOr("STATE_DIR", defaultStateDir)
	artifactRoot := envOr("ARTIFACT_ROOT", defaultArtifactRoot)
	canonicalBin := envOr("CANONICAL_BIN", defaultCanonicalBin)
	pollInterval := envDuration("POLL_INTERVAL", defaultPollInterval)
	forceRun := os.Getenv("FORCE_RUN") != "" || os.Getenv("ONCE") != ""

	os.MkdirAll(stateDir, 0755)
	os.MkdirAll(artifactRoot, 0755)

	// Ensure rolling metrics directory exists at startup
	metricsDir := envOr("METRICS_DIR", "/var/lib/zen-brain1/metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		log.Printf("[METRICS] WARNING: cannot create metrics dir %s: %v — rolling metrics disabled", metricsDir, err)
	} else {
		log.Printf("[METRICS] Rolling metrics dir: %s", metricsDir)
	}

	schedules, err := loadSchedules(scheduleDir)
	if err != nil {
		log.Fatalf("[SCHED] Failed to load schedules from %s: %v", scheduleDir, err)
	}
	if len(schedules) == 0 {
		log.Fatalf("[SCHED] No schedules found in %s", scheduleDir)
	}
	// Filter out schedules with no tasks
	valid := make([]Schedule, 0, len(schedules))
	for _, s := range schedules {
		if len(s.Tasks) == 0 {
			log.Printf("[SCHED] WARNING: %s has no tasks, skipping", s.Name)
			continue
		}
		valid = append(valid, s)
	}
	schedules = valid
	if len(schedules) == 0 {
		log.Fatalf("[SCHED] No valid schedules (all had empty task lists)")
	}
	log.Printf("[SCHED] Loaded %d schedules from %s", len(schedules), scheduleDir)
	for _, s := range schedules {
		log.Printf("[SCHED]   %s: %s (%d tasks, cadence=%s)", s.Name, s.Description, len(s.Tasks), s.Cadence)
	}

	if forceRun {
		log.Printf("[SCHED] FORCE_RUN mode: executing all schedules once")
		runAllSchedules(schedules, stateDir, artifactRoot, canonicalBin)
		writeStatus(schedules, stateDir, artifactRoot)
		return
	}

	// Daemon mode
	log.Printf("[SCHED] Entering daemon mode (poll=%v, state=%s, artifacts=%s)", pollInterval, stateDir, artifactRoot)
	statusPath := filepath.Join(stateDir, "scheduler-status.json")
	os.WriteFile(statusPath, []byte(`{"active":true}`), 0644)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		for _, s := range schedules {
			if isDue(s, stateDir) {
				runSchedule(s, stateDir, artifactRoot, canonicalBin)
				writeStatus(schedules, stateDir, artifactRoot)
			}
		}
	}
}

func loadSchedules(dir string) ([]Schedule, error) {
	var schedules []Schedule
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" && filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			log.Printf("[SCHED] WARNING: cannot read %s: %v", e.Name(), err)
			continue
		}
		var s Schedule
		if err := yaml.Unmarshal(data, &s); err != nil {
			log.Printf("[SCHED] WARNING: cannot parse %s: %v", e.Name(), err)
			continue
		}
		if s.Name == "" {
			s.Name = e.Name()[:len(e.Name())-len(filepath.Ext(e.Name()))]
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func cadenceDuration(cadence string) time.Duration {
	switch cadence {
	case "hourly":
		return 1 * time.Hour
	case "quad-hourly":
		return 4 * time.Hour
	case "daily":
		return 24 * time.Hour
	default:
		return 1 * time.Hour
	}
}

func isDue(s Schedule, stateDir string) bool {
	state := loadState(stateDir, s.Name)
	if state.LastRun.IsZero() {
		log.Printf("[SCHED] %s: never run, due now", s.Name)
		return true
	}
	elapsed := time.Since(state.LastRun)
	due := cadenceDuration(s.Cadence)
	if elapsed >= due {
		log.Printf("[SCHED] %s: last run %v ago (cadence=%v), due now", s.Name, elapsed.Round(time.Minute), due)
		return true
	}
	return false
}

func runSchedule(s Schedule, stateDir, artifactRoot, canonicalBin string) {
	log.Printf("[SCHED] 🚀 Running schedule: %s (%d tasks, cadence=%s)", s.Name, len(s.Tasks), s.Cadence)

	start := time.Now()

	mode := s.Mode
	if mode == "" {
		mode = "batch" // default
	}

	var cmd *exec.Cmd
	switch mode {
	case "dispatch", "steward":
		cmd = buildDispatchCommand(canonicalBin, s, artifactRoot)
	default: // "batch"
		cmd = buildBatchCommand(canonicalBin, s, artifactRoot)
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	now := time.Now()
	runDir := parseRunDir(outputStr)

	if err != nil {
		log.Printf("[SCHED] ❌ %s FAILED: %v", s.Name, err)
		saveState(stateDir, s.Name, ScheduleState{
			LastRun:    now,
			LastStatus: "failed",
			LastDir:    runDir,
			NextDue:    now.Add(cadenceDuration(s.Cadence)),
			RunCount:   loadState(stateDir, s.Name).RunCount + 1,
		})
		// Still write metrics on failure — no batch should complete without metrics
		if runDir != "" {
			writeRunMetrics(runDir, s.Name, "failed", outputStr, start, "", 0)
			writeRunSummary(runDir, s.Name, "failed", outputStr, start, "", 0)
			updateRollingMetrics(runDir, s.Name, "failed", outputStr, start, "", 0)
		}
		return
	}
	status := "success"
	for _, line := range splitLines(outputStr) {
		if contains(line, "FAIL") {
			status = "partial"
			break
		}
	}

	log.Printf("[SCHED] ✅ %s completed: %s (dir=%s)", s.Name, status, runDir)
	saveState(stateDir, s.Name, ScheduleState{
		LastRun:    now,
		LastStatus: status,
		LastDir:    runDir,
		NextDue:    now.Add(cadenceDuration(s.Cadence)),
		RunCount:   loadState(stateDir, s.Name).RunCount + 1,
	})

	// Sync batch results to Jira ledger (non-blocking — failures don't affect batch status)
	jiraParentKey := ""
	jiraChildCount := 0
	if runDir != "" && (status == "success" || status == "partial") {
		jiraCfg := loadJiraLedgerConfig()
		jiraParentKey, jiraChildCount = syncBatchToJira(jiraCfg, runDir, s.Name)

		// Phase 38 + Phase D: Run finding ticketizer for discovery classes
		// Only for schedules that contain ticketizable findings (defects, bug_hunting, stub_hunting)
		// Phase D: Discovery throttle — skip ticket creation when backlog drain is the priority
		ticketizableSchedules := map[string]bool{
			"hourly-scan": true,
			"daily-sweep": true,
		}
		if ticketizableSchedules[s.Name] && jiraCfg.enabled {
			backlogReady, _ := countBacklogTickets(jiraCfg)
			// Discovery throttle: if backlog has > 10 ready tickets, skip discovery
			// Policy: 70% remediation, 20% roadmap, 10% discovery
			// Do not create more work faster than the factory can close it
			if backlogReady > 10 {
				log.Printf("[SCHED] %s: DISCOVERY THROTTLED — backlog has %d ready tickets (> 10 threshold). Skipping ticketizer.",
					s.Name, backlogReady)
			} else {
				log.Printf("[SCHED] %s: discovery allowed — backlog has %d ready tickets (<= 10 threshold). Running ticketizer.",
					s.Name, backlogReady)
				runFindingTicketizer(runDir, s.Name, jiraCfg)
			}
		}
	}

	// Write canonical run metrics and human-readable summary
	if runDir != "" {
		writeRunMetrics(runDir, s.Name, status, outputStr, start, jiraParentKey, jiraChildCount)
		writeRunSummary(runDir, s.Name, status, outputStr, start, jiraParentKey, jiraChildCount)
		updateRollingMetrics(runDir, s.Name, status, outputStr, start, jiraParentKey, jiraChildCount)
	}
}

// buildBatchCommand constructs the canonical batch invocation:
//
//	zen-brain worker batch
//
// with TASKS, BATCH_NAME, OUTPUT_ROOT, WORKERS, TIMEOUT env vars.
func buildBatchCommand(canonicalBin string, s Schedule, artifactRoot string) *exec.Cmd {
	tasks := ""
	for i, t := range s.Tasks {
		if i > 0 {
			tasks += ","
		}
		tasks += t
	}

	defaultWorkers := 5
	if envW := os.Getenv("WORKERS_OVERRIDE"); envW != "" {
		if n, err := strconv.Atoi(envW); err == nil && n > 0 {
			defaultWorkers = n
		}
	}
	effectiveWorkers := defaultWorkers
	if s.Workers > 0 {
		effectiveWorkers = s.Workers
	}
	log.Printf("[SCHED] %s: effective WORKERS=%d (schedule=%d, default=%d)",
		s.Name, effectiveWorkers, s.Workers, defaultWorkers)

	cmd := exec.Command(canonicalBin, "worker", "batch")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BATCH_NAME=%s", s.Name),
		fmt.Sprintf("OUTPUT_ROOT=%s", artifactRoot),
		fmt.Sprintf("TASKS=%s", tasks),
		fmt.Sprintf("TIMEOUT=300"),
		fmt.Sprintf("WORKERS=%d", effectiveWorkers),
	)
	return cmd
}

// buildDispatchCommand constructs the canonical dispatch invocation:
//
//	zen-brain dispatch <subcommand> [args]
//
// The subcommand is derived from the task name or DispatchSubcommand field.
func buildDispatchCommand(canonicalBin string, s Schedule, artifactRoot string) *exec.Cmd {
	subcommand := s.DispatchSubcommand
	if subcommand == "" {
		// Derive from first task: queue_steward → queue, roadmap_steward → roadmap
		if len(s.Tasks) > 0 {
			taskName := s.Tasks[0]
			switch {
			case strings.HasPrefix(taskName, "queue"):
				subcommand = "queue"
			case strings.HasPrefix(taskName, "roadmap"):
				subcommand = "roadmap"
			default:
				subcommand = taskName
			}
		}
	}

	args := []string{"dispatch", subcommand}
	args = append(args, "--once")

	log.Printf("[SCHED] %s: dispatch mode → %s %v", s.Name, subcommand, args[2:])

	cmd := exec.Command(canonicalBin, args...)
	envExtras := []string{
		fmt.Sprintf("ARTIFACT_ROOT=%s", artifactRoot),
	}
	if s.StewardMode != "" {
		envExtras = append(envExtras, fmt.Sprintf("STEWARD_MODE=%s", s.StewardMode))
	}
	cmd.Env = append(os.Environ(), envExtras...)
	return cmd
}

func runAllSchedules(schedules []Schedule, stateDir, artifactRoot, canonicalBin string) {
	var wg sync.WaitGroup
	for _, s := range schedules {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSchedule(s, stateDir, artifactRoot, canonicalBin)
		}()
	}
	wg.Wait()
}

func writeStatus(schedules []Schedule, stateDir, artifactRoot string) {
	status := SchedulerStatus{
		Active:       true,
		StateDir:     stateDir,
		ArtifactRoot: artifactRoot,
		UpdatedAt:    time.Now(),
	}
	for _, s := range schedules {
		st := loadState(stateDir, s.Name)
		entry := ScheduleEntry{
			Name:       s.Name,
			Cadence:    s.Cadence,
			Tasks:      s.Tasks,
			LastStatus: st.LastStatus,
			RunCount:   st.RunCount,
		}
		if !st.LastRun.IsZero() {
			entry.LastRun = st.LastRun.Format(time.RFC3339)
		}
		nextDue := st.LastRun.Add(cadenceDuration(s.Cadence))
		entry.NextDue = nextDue.Format(time.RFC3339)
		status.Schedules = append(status.Schedules, entry)
	}
	data, _ := json.MarshalIndent(status, "", "  ")
	os.WriteFile(filepath.Join(stateDir, "scheduler-status.json"), data, 0644)
}

func statePath(stateDir, name string) string {
	return filepath.Join(stateDir, fmt.Sprintf("%s.json", name))
}

func loadState(stateDir, name string) ScheduleState {
	data, err := os.ReadFile(statePath(stateDir, name))
	if err != nil {
		return ScheduleState{}
	}
	var st ScheduleState
	json.Unmarshal(data, &st)
	return st
}

func saveState(stateDir, name string, st ScheduleState) {
	os.MkdirAll(stateDir, 0755)
	data, _ := json.MarshalIndent(st, "", "  ")
	os.WriteFile(statePath(stateDir, name), data, 0644)
}

func parseRunDir(output string) string {
	// useful-batch outputs: "[BATCH] === batch-name COMPLETE: N/N OK, wall=... ===\n[BATCH] Run dir: /path"
	// Look for "Run dir: /" — the path always starts with /
	for _, line := range splitLines(output) {
		idx := indexOf(line, "Run dir: /")
		if idx >= 0 {
			path := line[idx+9:] // skip "Run dir: "
			return trimSpace(path)
		}
	}
	return ""
}

// --- Jira Ledger Integration ---
// After each successful batch run, creates a Jira parent issue and child issues
// for actionable findings. Uses direct HTTP to /rest/api/3/issue — no external deps.
// If Jira auth fails, logs warning and continues (fail-open for local artifacts).

// jiraLedgerConfig holds Jira connection parameters from env vars.
type jiraLedgerConfig struct {
	baseURL    string
	email      string
	apiToken   string
	projectKey string
	enabled    bool
}

// countBacklogTickets returns (ready backlog count, total backlog count) for discovery throttle.
// Ready = bug + ai:finding + status=Backlog (actionable remediation work).
// Phase D: Discovery throttle — do not create more work faster than the factory can close it.
func countBacklogTickets(jiraCfg jiraLedgerConfig) (int, int) {
	if !jiraCfg.enabled {
		return 0, 0
	}
	// Count total backlog bug tickets
	body := map[string]interface{}{
		"jql":        fmt.Sprintf(`project="%s" AND status=Backlog AND labels=bug AND labels=ai:finding`, jiraCfg.projectKey),
		"maxResults": 0, // just need total
		"fields":     []string{},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", jiraCfg.baseURL+"/rest/api/3/search/jql", bytes.NewReader(data))
	req.SetBasicAuth(jiraCfg.email, jiraCfg.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		resp.Body.Close()
		return 0, 0
	}
	var result struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	// All backlog bug+ai:finding tickets are considered "ready" for throttle purposes
	return result.Total, result.Total
}

func loadJiraLedgerConfig() jiraLedgerConfig {
	baseURL := os.Getenv("JIRA_URL")
	email := os.Getenv("JIRA_EMAIL")
	apiToken := os.Getenv("JIRA_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_TOKEN")
	}
	projectKey := os.Getenv("JIRA_PROJECT_KEY")
	if projectKey == "" {
		projectKey = "ZB"
	}
	enabled := baseURL != "" && email != "" && apiToken != ""
	return jiraLedgerConfig{baseURL: baseURL, email: email, apiToken: apiToken, projectKey: projectKey, enabled: enabled}
}

// jiraCreateIssue creates a single Jira issue and returns the issue key.
// Returns empty string on any failure (never blocks the batch).
func jiraCreateIssue(cfg jiraLedgerConfig, summary, description string, labels []string, priority string) string {
	if !cfg.enabled {
		return ""
	}

	type adfContent struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type adfPara struct {
		Type    string       `json:"type"`
		Content []adfContent `json:"content"`
	}
	type issueFields struct {
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Summary     string `json:"summary"`
		Description struct {
			Type    string    `json:"type"`
			Version int       `json:"version"`
			Content []adfPara `json:"content"`
		} `json:"description"`
		IssueType struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority struct {
			Name string `json:"name"`
		} `json:"priority"`
		Labels []string `json:"labels,omitempty"`
	}

	type adfParagraph struct {
		Type    string       `json:"type"`
		Content []adfContent `json:"content"`
	}

	payload := struct {
		Fields issueFields `json:"fields"`
	}{}
	payload.Fields.Project.Key = cfg.projectKey
	payload.Fields.Summary = summary
	payload.Fields.Description.Type = "doc"
	payload.Fields.Description.Version = 1
	payload.Fields.Description.Content = []adfPara{{
		Type:    "paragraph",
		Content: []adfContent{{Type: "text", Text: description}},
	}}
	payload.Fields.IssueType.Name = "Task"
	payload.Fields.Priority.Name = priority
	if len(labels) > 0 {
		payload.Fields.Labels = labels
	}

	bodyBytes, _ := json.Marshal(payload)

	ctx, cancel := contextWithTimeout5s()
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		cfg.baseURL+"/rest/api/3/issue",
		bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("[JIRA] create issue request error: %v", err)
		return ""
	}
	req.SetBasicAuth(cfg.email, cfg.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[JIRA] create issue http error: %v", err)
		return ""
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		log.Printf("[JIRA] create issue failed (status %d): %s", resp.StatusCode, truncate(respBody, 200))
		return ""
	}

	var result struct {
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || result.Key == "" {
		log.Printf("[JIRA] create issue response parse error: %v", err)
		return ""
	}

	return result.Key
}

// syncBatchToJira creates Jira parent+child issues for a completed batch.
// Reads batch-index.json and artifact files, creates issues, writes Jira keys back.
// Returns (parentKey, childCount).
func syncBatchToJira(jiraCfg jiraLedgerConfig, runDir, batchName string) (string, int) {
	if !jiraCfg.enabled {
		log.Printf("[JIRA] Ledger disabled: JIRA_URL/JIRA_EMAIL/JIRA_API_TOKEN not set")
		return "", 0
	}

	// Read batch index
	idxPath := filepath.Join(runDir, "telemetry", "batch-index.json")
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		log.Printf("[JIRA] Cannot read batch index: %v", err)
		return "", 0
	}

	var batchIndex struct {
		BatchID   string `json:"batch_id"`
		Total     int    `json:"total"`
		Succeeded int    `json:"succeeded"`
		Failed    int    `json:"failed"`
		Results   []struct {
			TaskID           string      `json:"work_item_id"`
			TaskClass        string      `json:"task_class"`
			Title            string      `json:"title"`
			Success          bool        `json:"success"`
			State            string      `json:"state"`
			ValidationStatus string      `json:"validation_status"`
			ValidationIssues interface{} `json:"validation_issues"`
			Error            string      `json:"error"`
			ArtifactPath     string      `json:"artifact_path"`
			DurationMs       float64     `json:"duration_ms"`
		} `json:"results"`
	}
	if err := json.Unmarshal(idxData, &batchIndex); err != nil {
		log.Printf("[JIRA] Cannot parse batch index: %v", err)
		return "", 0
	}

	// Create parent issue for the batch
	parentLabels := []string{"zen-brain", "scheduled-batch", batchName}
	parentSummary := fmt.Sprintf("[%s] %s — %s", strings.ToUpper(batchName), batchIndex.BatchID,
		time.Now().Format("2006-01-02"))
	parentDesc := fmt.Sprintf("Scheduled batch run: %s\nBatch ID: %s\nResults: %d/%d succeeded, %d failed\nRun dir: %s\n\nTask breakdown:",
		batchName, batchIndex.BatchID, batchIndex.Succeeded, batchIndex.Total, batchIndex.Failed, runDir)
	for _, r := range batchIndex.Results {
		status := "✅"
		if !r.Success {
			status = "❌"
		}
		stateInfo := ""
		if r.ValidationStatus != "" {
			stateInfo = fmt.Sprintf(" [%s]", r.ValidationStatus)
		}
		parentDesc += fmt.Sprintf("\n%s %s: %s%s", status, r.TaskClass, r.Title, stateInfo)
	}

	parentKey := jiraCreateIssue(jiraCfg, parentSummary, parentDesc, parentLabels, "Medium")
	if parentKey == "" {
		log.Printf("[JIRA] Failed to create parent issue for batch %s", batchIndex.BatchID)
		return "", 0
	}
	log.Printf("[JIRA] Created parent issue: %s for batch %s", parentKey, batchIndex.BatchID)

	// Create child issues for each task (including failed — Jira should reflect reality)
	childKeys := make(map[string]string)
	for _, r := range batchIndex.Results {
		taskID := r.TaskID
		title := r.Title
		validationStatus := r.ValidationStatus
		success := r.Success
		artifactPath := r.ArtifactPath

		// Read first few lines of artifact for issue body
		var artifactSnippet string
		if data, err := os.ReadFile(artifactPath); err == nil {
			lines := strings.Split(string(data), "\n")
			var contentLines []string
			for _, l := range lines {
				trimmed := strings.TrimSpace(l)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				contentLines = append(contentLines, trimmed)
				if len(strings.Join(contentLines, " ")) > 400 {
					break
				}
			}
			artifactSnippet = strings.Join(contentLines, " ")
			if len(artifactSnippet) > 500 {
				artifactSnippet = artifactSnippet[:497] + "..."
			}
		}

		// Determine labels based on outcome
		taskLabels := []string{"zen-brain", batchName}
		var stateLabel string
		if success {
			taskLabels = append(taskLabels, "finding")
			stateLabel = "ai:completed"
		} else {
			taskLabels = append(taskLabels, "ai:blocked")
			stateLabel = "ai:blocked"
		}
		taskLabels = append(taskLabels, stateLabel)

		// Determine priority based on validation
		priority := "Low"
		if validationStatus == "context-fail" || validationStatus == "artifact-fail" {
			priority = "Low" // blocked findings are low priority
		} else if validationStatus == "success-needs-review" {
			priority = "Medium"
		}

		taskSummary := fmt.Sprintf("[%s] %s: %s", strings.ToUpper(batchName), taskID, title)
		taskDesc := fmt.Sprintf("Generated by: %s batch %s\nTask class: %s\nArtifact: %s\nValidation: %s\n\n%s",
			batchName, batchIndex.BatchID, taskID, artifactPath, validationStatus, artifactSnippet)

		childKey := jiraCreateIssue(jiraCfg, taskSummary, taskDesc, taskLabels, priority)
		if childKey != "" {
			childKeys[taskID] = childKey
			log.Printf("[JIRA] Created child issue: %s — %s (state=%s)", childKey, taskSummary, stateLabel)
		}
	}

	// Write Jira keys into run metadata
	jiraMeta := struct {
		ParentKey string            `json:"parent_jira_key"`
		ChildKeys map[string]string `json:"child_jira_keys"`
		Timestamp string            `json:"timestamp"`
	}{
		ParentKey: parentKey,
		ChildKeys: childKeys,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	metaBytes, _ := json.MarshalIndent(jiraMeta, "", "  ")
	metaPath := filepath.Join(runDir, "telemetry", "jira-ledger.json")
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		log.Printf("[JIRA] Failed to write ledger metadata: %v", err)
	} else {
		log.Printf("[JIRA] Wrote ledger metadata: %s (parent=%s, children=%d)", metaPath, parentKey, len(childKeys))
	}

	// Update batch index with Jira parent key (merge into existing JSON)
	var idxMap map[string]interface{}
	if json.Unmarshal(idxData, &idxMap) == nil {
		idxMap["jira_parent_key"] = parentKey
		idxMap["jira_child_keys"] = childKeys
		if merged, err := json.MarshalIndent(idxMap, "", "  "); err == nil {
			os.WriteFile(idxPath, merged, 0644)
		}
	}
	return parentKey, len(childKeys)
}

// --- Jira Per-Task State Updates (PHASE 35) ---

// jiraAddComment posts a comment to an existing Jira issue.
func jiraAddComment(cfg jiraLedgerConfig, issueKey, comment string) bool {
	if !cfg.enabled || issueKey == "" {
		return false
	}
	payload := struct {
		Body struct {
			Type    string `json:"type"`
			Version int    `json:"version"`
			Content []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"content"`
		} `json:"body"`
	}{}
	payload.Body.Type = "doc"
	payload.Body.Version = 1
	payload.Body.Content = []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}{{
		Type: "paragraph",
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: comment}},
	}}
	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout5s()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST",
		cfg.baseURL+"/rest/api/3/issue/"+issueKey+"/comment",
		bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.SetBasicAuth(cfg.email, cfg.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 201 {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	resp.Body.Close()
	return true
}

// jiraAddLabels adds labels to an existing Jira issue.
func jiraAddLabels(cfg jiraLedgerConfig, issueKey string, labels []string) bool {
	if !cfg.enabled || issueKey == "" || len(labels) == 0 {
		return false
	}
	payload := struct {
		Fields struct {
			Labels []string `json:"labels"`
		} `json:"fields"`
	}{}
	payload.Fields.Labels = labels
	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout5s()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "PUT",
		cfg.baseURL+"/rest/api/3/issue/"+issueKey,
		bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	req.SetBasicAuth(cfg.email, cfg.apiToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 204 {
		if resp != nil {
			resp.Body.Close()
		}
		return false
	}
	resp.Body.Close()
	return true
}

// updateChildIssueState updates a Jira child issue with runtime outcome.
// Maps validation_status to appropriate labels and comments.
func updateChildIssueState(cfg jiraLedgerConfig, childKey string, result map[string]interface{}) {
	if childKey == "" {
		return
	}
	taskClass, _ := result["task_class"].(string)
	validationStatus, _ := result["validation_status"].(string)
	state, _ := result["state"].(string)
	durationMs, _ := result["duration_ms"].(float64)
	issues, _ := result["validation_issues"].([]interface{})

	// Map state to labels and comment
	var labels []string
	var commentLines []string
	commentLines = append(commentLines, fmt.Sprintf("h2. Task Runtime Outcome\n"))
	commentLines = append(commentLines, fmt.Sprintf("*Task Class:* %s", taskClass))
	commentLines = append(commentLines, fmt.Sprintf("*State:* %s", state))
	commentLines = append(commentLines, fmt.Sprintf("*Validation:* %s", validationStatus))
	commentLines = append(commentLines, fmt.Sprintf("*Duration:* %.0fms", durationMs))

	switch state {
	case "done":
		labels = []string{"ai:completed"}
		commentLines = append(commentLines, "\n✅ Task completed successfully with grounded output.")
	case "needs_review":
		labels = []string{"ai:needs-review"}
		commentLines = append(commentLines, "\n⚠️ Task output needs review — findings may not be fully grounded.")
		if len(issues) > 0 {
			commentLines = append(commentLines, "\n*Issues:*")
			for _, iss := range issues {
				if s, ok := iss.(string); ok {
					commentLines = append(commentLines, fmt.Sprintf("# %s", s))
				}
			}
		}
	case "validation_fail":
		labels = []string{"ai:blocked"}
		commentLines = append(commentLines, "\n❌ Task failed validation — output not usable.")
		if len(issues) > 0 {
			commentLines = append(commentLines, "\n*Validation failures:*")
			for _, iss := range issues {
				if s, ok := iss.(string); ok {
					commentLines = append(commentLines, fmt.Sprintf("# %s", s))
				}
			}
		}
	case "failed":
		labels = []string{"ai:blocked"}
		commentLines = append(commentLines, fmt.Sprintf("\n❌ Task failed at dispatch level."))
		if errMsg, ok := result["error"].(string); ok {
			commentLines = append(commentLines, fmt.Sprintf("*Error:* %s", errMsg))
		}
	default:
		commentLines = append(commentLines, fmt.Sprintf("\n❓ Unknown state: %s", state))
	}

	comment := strings.Join(commentLines, "\n")

	if ok := jiraAddComment(cfg, childKey, comment); ok {
		log.Printf("[JIRA] Updated child %s with state=%s validation=%s", childKey, state, validationStatus)
	} else {
		log.Printf("[JIRA] Failed to comment on child %s", childKey)
	}
	if ok := jiraAddLabels(cfg, childKey, labels); ok {
		log.Printf("[JIRA] Labeled child %s with %v", childKey, labels)
	}
}

// --- Run Metrics and Summary ---

// parseBatchOutput extracts task results from useful-batch output.
func parseBatchOutput(output string) (succeeded, failed, total int, results []map[string]interface{}) {
	total = 0
	for _, line := range splitLines(output) {
		if contains(line, "✅") && contains(line, "→") {
			succeeded++
			total++
			r := map[string]interface{}{"status": "success"}
			// Extract task name
			parts := splitString(trimSpace(line), ' ')
			if len(parts) >= 2 {
				r["task_id"] = parts[1]
			}
			if len(parts) >= 3 {
				r["artifact"] = parts[len(parts)-1]
			}
			results = append(results, r)
		} else if contains(line, "❌") {
			failed++
			total++
			r := map[string]interface{}{"status": "failed"}
			parts := splitString(trimSpace(line), ' ')
			if len(parts) >= 2 {
				r["task_id"] = parts[1]
			}
			results = append(results, r)
		}
	}
	return
}

// writeRunMetrics writes the canonical machine-readable metrics file for a run.
func writeRunMetrics(runDir, scheduleName, status string, output string, start time.Time, jiraParentKey string, jiraChildCount int) {
	succeeded, failed, total, _ := parseBatchOutput(output)
	wallSec := time.Since(start).Seconds()

	metrics := map[string]interface{}{
		"run_id":                             filepath.Base(runDir),
		"schedule_name":                      scheduleName,
		"started_at":                         start.UTC().Format(time.RFC3339),
		"completed_at":                       time.Now().UTC().Format(time.RFC3339),
		"wall_time_seconds":                  int(wallSec),
		"task_count_total":                   total,
		"task_count_l1_success":              succeeded,
		"task_count_l1_fail":                 failed,
		"task_count_l1_success_needs_review": 0,
		"task_count_l1_fail_l2_success":      0,
		"task_count_l1_fail_l2_fail":         0,
		"task_count_infra_fail":              0,
		"task_count_blocked_jira_auth":       0,
		"escalation_count":                   0,
		"artifact_count":                     succeeded,
		"jira_parent_issue_key":              jiraParentKey,
		"jira_child_issue_count":             jiraChildCount,
		"model_lane_summary":                 "L1 (qwen3.5:0.8b Q4_K_M)",
		"status":                             status,
		"artifact_root":                      runDir,
		"telemetry_root":                     filepath.Join(runDir, "telemetry"),
	}

	data, _ := json.MarshalIndent(metrics, "", "  ")
	path := filepath.Join(runDir, "telemetry", "run-metrics.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[METRICS] Failed to write run-metrics.json: %v", err)
	} else {
		log.Printf("[METRICS] Wrote %s (%d tasks, %d OK, %d fail, jira=%s+%d)",
			path, total, succeeded, failed, jiraParentKey, jiraChildCount)
	}
}

// writeRunSummary writes a human-readable markdown summary for a run.
func writeRunSummary(runDir, scheduleName, status string, output string, start time.Time, jiraParentKey string, jiraChildCount int) {
	succeeded, failed, total, results := parseBatchOutput(output)
	wallSec := time.Since(start).Seconds()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Run Summary: %s\n\n", scheduleName))
	sb.WriteString(fmt.Sprintf("## Metadata\n"))
	sb.WriteString(fmt.Sprintf("- **Run ID:** %s\n", filepath.Base(runDir)))
	sb.WriteString(fmt.Sprintf("- **Schedule:** %s\n", scheduleName))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", status))
	sb.WriteString(fmt.Sprintf("- **Started:** %s\n", start.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Completed:** %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Wall Time:** %.1fs\n\n", wallSec))

	sb.WriteString("## Task Outcomes\n\n")
	sb.WriteString(fmt.Sprintf("| Outcome | Count |\n|---------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| ✅ L1 Success | %d |\n", succeeded))
	sb.WriteString(fmt.Sprintf("| ❌ Failed | %d |\n", failed))
	sb.WriteString(fmt.Sprintf("| **Total** | **%d** |\n\n", total))

	if len(results) > 0 {
		sb.WriteString("### Task Breakdown\n\n")
		for _, r := range results {
			icon := "✅"
			if r["status"] == "failed" {
				icon = "❌"
			}
			sb.WriteString(fmt.Sprintf("- %s `%s`", icon, r["task_id"]))
			if art, ok := r["artifact"].(string); ok && art != "" {
				sb.WriteString(fmt.Sprintf(" → `%s`", art))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Jira Outcomes\n\n")
	if jiraParentKey != "" {
		sb.WriteString(fmt.Sprintf("- **Parent Issue:** %s\n", jiraParentKey))
		sb.WriteString(fmt.Sprintf("- **Child Issues:** %d\n", jiraChildCount))
	} else {
		sb.WriteString("- No Jira issues created (ledger disabled or auth failure)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Artifact Paths\n\n")
	finalDir := filepath.Join(runDir, "final")
	if entries, err := os.ReadDir(finalDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				info, _ := e.Info()
				sb.WriteString(fmt.Sprintf("- `%s` — %d bytes\n", e.Name(), info.Size()))
			}
		}
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("## Telemetry\n\n"))
	sb.WriteString(fmt.Sprintf("- `telemetry/batch-index.json` — batch-level results\n"))
	sb.WriteString(fmt.Sprintf("- `telemetry/run-metrics.json` — canonical metrics\n"))
	if jiraParentKey != "" {
		sb.WriteString(fmt.Sprintf("- `telemetry/jira-ledger.json` — Jira keys\n"))
	}
	sb.WriteString("\n")

	sb.WriteString("## Escalations\n\n")
	sb.WriteString("None.\n\n")

	sb.WriteString("## Blockers / Anomalies\n\n")
	if status == "failed" {
		sb.WriteString("- All tasks failed — check L1 endpoint health\n")
	} else if status == "partial" {
		sb.WriteString("- Some tasks failed — review individual task logs\n")
	} else if jiraParentKey == "" {
		sb.WriteString("- Jira ledger did not fire — check JIRA_TOKEN in scheduler env\n")
	} else {
		sb.WriteString("None.\n")
	}

	path := filepath.Join(runDir, "final", "run-summary.md")
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		log.Printf("[METRICS] Failed to write run-summary.md: %v", err)
	} else {
		log.Printf("[METRICS] Wrote %s", path)
	}
}

// updateRollingMetrics appends to history and updates latest summary.
func updateRollingMetrics(runDir, scheduleName, status string, output string, start time.Time, jiraParentKey string, jiraChildCount int) {
	succeeded, failed, total, _ := parseBatchOutput(output)
	wallSec := time.Since(start).Seconds()

	metricsDir := envOr("METRICS_DIR", "/var/lib/zen-brain1/metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		log.Printf("[METRICS] WARNING: cannot write rolling metrics, dir %s not writable: %v", metricsDir, err)
		return
	}

	// Append to history (JSONL — one object per line)
	historyPath := filepath.Join(metricsDir, "history.jsonl")
	entry := map[string]interface{}{
		"run_id":                 filepath.Base(runDir),
		"schedule_name":          scheduleName,
		"status":                 status,
		"started_at":             start.UTC().Format(time.RFC3339),
		"wall_time_seconds":      int(wallSec),
		"task_count_total":       total,
		"task_count_l1_success":  succeeded,
		"task_count_l1_fail":     failed,
		"jira_parent_issue_key":  jiraParentKey,
		"jira_child_issue_count": jiraChildCount,
		"artifact_root":          runDir,
		"timestamp":              time.Now().UTC().Format(time.RFC3339),
	}
	line, _ := json.Marshal(entry)
	f, err := os.OpenFile(historyPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[METRICS] Failed to open history: %v", err)
	} else {
		f.Write(line)
		f.Write([]byte("\n"))
		f.Close()
	}

	// Write latest summary
	latest := map[string]interface{}{
		"last_run_id":            filepath.Base(runDir),
		"last_schedule_name":     scheduleName,
		"last_status":            status,
		"last_wall_time_seconds": int(wallSec),
		"last_task_count_total":  total,
		"last_l1_success_count":  succeeded,
		"last_l1_fail_count":     failed,
		"last_escalation_count":  0,
		"last_jira_parent_key":   jiraParentKey,
		"last_jira_child_count":  jiraChildCount,
		"last_artifact_root":     runDir,
		"updated_at":             time.Now().UTC().Format(time.RFC3339),
	}
	latestData, _ := json.MarshalIndent(latest, "", "  ")
	os.WriteFile(filepath.Join(metricsDir, "latest-summary.json"), latestData, 0644)
	log.Printf("[METRICS] Updated rolling metrics: %s (%d history entries)", metricsDir, countLines(historyPath))
}

// countLines counts lines in a file.
func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

// --- Minimal stdlib helpers (no external deps) ---

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func yamlUnmarshal(data []byte, v interface{}) error {
	// Minimal YAML parser for flat structures. Supports:
	//   key: value
	//   key: [item1, item2]
	// Skips comments (#) and empty lines.
	m := make(map[string]interface{})
	for _, line := range splitLines(string(data)) {
		trimmed := trimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		parts := splitString(trimmed, ':')
		if len(parts) < 2 {
			continue
		}
		key := trimSpace(parts[0])
		val := trimSpace(parts[1])
		if val == "" && len(parts) > 2 {
			val = trimSpace(joinParts(parts[1:], ":"))
		}
		m[key] = val
	}

	// Map to struct
	if sm, ok := v.(*Schedule); ok {
		if n, ok := m["name"].(string); ok {
			sm.Name = n
		}
		if c, ok := m["cadence"].(string); ok {
			sm.Cadence = c
		}
		if d, ok := m["description"].(string); ok {
			sm.Description = d
		}
		if t, ok := m["tasks"].(string); ok {
			sm.Tasks = parseList(t)
		}
		return nil
	}
	return fmt.Errorf("unsupported type")
}

func parseList(s string) []string {
	s = trimSpace(s)
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		s = s[1 : len(s)-1]
	}
	var out []string
	for _, part := range splitString(s, ',') {
		if t := trimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func splitLines(s string) []string { return splitString(s, '\n') }
func contains(s, sub string) bool  { return indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func joinParts(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
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

func contextWithTimeout5s() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 15*time.Second)
}

func truncate(b []byte, maxLen int) string {
	s := string(b)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// runFindingTicketizer invokes the finding-ticketizer via the canonical
// zen-brain binary to convert discovery findings into Jira tickets.
// Runs as a subprocess — failures are logged but never block the scheduler.
func runFindingTicketizer(runDir, scheduleName string, jiraCfg jiraLedgerConfig) {
	// Canonical: use zen-brain worker ticketize (finds implementation binary on PATH)
	canonicalBin := envOr("CANONICAL_BIN", defaultCanonicalBin)

	log.Printf("[TICKETIZER] running for %s (run=%s)", scheduleName, filepath.Base(runDir))

	cmd := exec.Command(canonicalBin, "worker", "ticketize",
		"-run-dir", runDir,
		"-schedule", scheduleName,
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("JIRA_URL=%s", jiraCfg.baseURL),
		fmt.Sprintf("JIRA_EMAIL=%s", jiraCfg.email),
		fmt.Sprintf("JIRA_API_TOKEN=%s", jiraCfg.apiToken),
		fmt.Sprintf("JIRA_PROJECT_KEY=%s", jiraCfg.projectKey),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[TICKETIZER] failed for %s: %v", scheduleName, err)
	}
	// Log ticketizer output (last 5 lines for brevity)
	lines := strings.Split(string(output), "\n")
	start := len(lines) - 5
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:] {
		log.Printf("[TICKETIZER] %s", l)
	}
}
