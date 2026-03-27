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
	"strings"
	"sync"
	"time"
)

// zen-brain1 internal recurring scheduler.
// Owns useful-task cadence. Systemd supervises the process; this decides what runs when.
//
// Loads schedule definitions from config/schedules/, determines due work,
// submits batches through the proven useful-batch runtime, records status.
//
// ENV VARS:
//   SCHEDULE_DIR   — schedule definitions (default: config/schedules/)
//   STATE_DIR      — scheduler state / last-run tracking (default: /var/lib/zen-brain1/scheduler)
//   ARTIFACT_ROOT  — artifact output root (default: /var/lib/zen-brain1/runs)
//   BATCH_BIN      — path to useful-batch binary (default: cmd/useful-batch/useful-batch)
//   POLL_INTERVAL  — how often to check for due work (default: 60s)
//   FORCE_RUN      — run all schedules once immediately, then exit (default: false)
//   ONCE           — alias for FORCE_RUN

const (
	defaultPollInterval = 60 * time.Second
	defaultScheduleDir  = "config/schedules"
	defaultStateDir     = "/var/lib/zen-brain1/scheduler"
	defaultArtifactRoot = "/var/lib/zen-brain1/runs"
	defaultBatchBin     = "cmd/useful-batch/useful-batch"
)

// Schedule represents a recurring workload definition.
type Schedule struct {
	Name        string   `yaml:"name" json:"name"`
	Tasks       []string `yaml:"tasks" json:"tasks"`
	Cadence     string   `yaml:"cadence" json:"cadence"`     // "hourly", "quad-hourly", "daily"
	Description string   `yaml:"description" json:"description"`
}

// ScheduleState tracks when each schedule last ran.
type ScheduleState struct {
	LastRun   time.Time `json:"last_run"`
	LastStatus string   `json:"last_status"` // "success", "partial", "failed"
	LastDir   string    `json:"last_dir"`
	NextDue   time.Time `json:"next_due"`
	RunCount  int       `json:"run_count"`
}

// SchedulerStatus is the overall status for operator queries.
type SchedulerStatus struct {
	Active      bool              `json:"active"`
	Schedules   []ScheduleEntry   `json:"schedules"`
	StateDir    string            `json:"state_dir"`
	ArtifactRoot string           `json:"artifact_root"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ScheduleEntry struct {
	Name        string `json:"name"`
	Cadence     string `json:"cadence"`
	Tasks       []string `json:"tasks"`
	LastRun     string `json:"last_run,omitempty"`
	NextDue     string `json:"next_due"`
	LastStatus  string `json:"last_status,omitempty"`
	RunCount    int    `json:"run_count"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	scheduleDir := envOr("SCHEDULE_DIR", defaultScheduleDir)
	stateDir := envOr("STATE_DIR", defaultStateDir)
	artifactRoot := envOr("ARTIFACT_ROOT", defaultArtifactRoot)
	batchBin := envOr("BATCH_BIN", defaultBatchBin)
	pollInterval := envDuration("POLL_INTERVAL", defaultPollInterval)
	forceRun := os.Getenv("FORCE_RUN") != "" || os.Getenv("ONCE") != ""

	os.MkdirAll(stateDir, 0755)
	os.MkdirAll(artifactRoot, 0755)

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
		runAllSchedules(schedules, stateDir, artifactRoot, batchBin)
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
				runSchedule(s, stateDir, artifactRoot, batchBin)
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
		if err := yamlUnmarshal(data, &s); err != nil {
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

func runSchedule(s Schedule, stateDir, artifactRoot, batchBin string) {
	log.Printf("[SCHED] 🚀 Running schedule: %s (%d tasks, cadence=%s)", s.Name, len(s.Tasks), s.Cadence)

	tasks := ""
	for i, t := range s.Tasks {
		if i > 0 {
			tasks += ","
		}
		tasks += t
	}

	cmd := exec.Command(batchBin)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BATCH_NAME=%s", s.Name),
		fmt.Sprintf("OUTPUT_ROOT=%s", artifactRoot),
		fmt.Sprintf("TASKS=%s", tasks),
		fmt.Sprintf("TIMEOUT=300"),
		fmt.Sprintf("WORKERS=5"),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[SCHED] ❌ %s FAILED: %v\n%s", s.Name, err, string(output))
		saveState(stateDir, s.Name, ScheduleState{
			LastRun:    time.Now(),
			LastStatus: "failed",
			RunCount:   loadState(stateDir, s.Name).RunCount + 1,
		})
		return
	}

	// Parse last line for run dir
	runDir := parseRunDir(string(output))
	status := "success"
	for _, line := range splitLines(string(output)) {
		if contains(line, "FAIL") {
			status = "partial"
			break
		}
	}

	log.Printf("[SCHED] ✅ %s completed: %s (dir=%s)", s.Name, status, runDir)
	saveState(stateDir, s.Name, ScheduleState{
		LastRun:    time.Now(),
		LastStatus: status,
		LastDir:    runDir,
		RunCount:   loadState(stateDir, s.Name).RunCount + 1,
	})

	// Sync batch results to Jira ledger (non-blocking — failures don't affect batch status)
	if runDir != "" && (status == "success" || status == "partial") {
		jiraCfg := loadJiraLedgerConfig()
		syncBatchToJira(jiraCfg, runDir, s.Name)
	}
}

func runAllSchedules(schedules []Schedule, stateDir, artifactRoot, batchBin string) {
	var wg sync.WaitGroup
	for _, s := range schedules {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSchedule(s, stateDir, artifactRoot, batchBin)
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
		Summary     string   `json:"summary"`
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
		Type: "paragraph",
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
func syncBatchToJira(jiraCfg jiraLedgerConfig, runDir, batchName string) {
	if !jiraCfg.enabled {
		log.Printf("[JIRA] Ledger disabled: JIRA_URL/JIRA_EMAIL/JIRA_API_TOKEN not set")
		return
	}

	// Read batch index
	idxPath := filepath.Join(runDir, "telemetry", "batch-index.json")
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		log.Printf("[JIRA] Cannot read batch index: %v", err)
		return
	}

	var batchIndex struct {
		BatchID   string `json:"batch_id"`
		Total     int    `json:"total"`
		Succeeded int    `json:"succeeded"`
		Failed    int    `json:"failed"`
		Results   []struct {
			TaskID       string `json:"task_id"`
			Title        string `json:"title"`
			Success      bool   `json:"success"`
			ArtifactPath string `json:"artifact_path"`
		} `json:"results"`
	}
	if err := json.Unmarshal(idxData, &batchIndex); err != nil {
		log.Printf("[JIRA] Cannot parse batch index: %v", err)
		return
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
		parentDesc += fmt.Sprintf("\n%s %s — %s", status, r.TaskID, r.Title)
	}

	parentKey := jiraCreateIssue(jiraCfg, parentSummary, parentDesc, parentLabels, "Medium")
	if parentKey == "" {
		log.Printf("[JIRA] Failed to create parent issue for batch %s", batchIndex.BatchID)
		return
	}
	log.Printf("[JIRA] Created parent issue: %s for batch %s", parentKey, batchIndex.BatchID)

	// Create child issues for each succeeded task
	childKeys := make(map[string]string)
	for _, r := range batchIndex.Results {
		if !r.Success {
			continue
		}

		// Read first few lines of artifact for issue body
		var artifactSnippet string
		if data, err := os.ReadFile(r.ArtifactPath); err == nil {
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

		taskLabels := []string{"zen-brain", "finding", batchName}
		taskSummary := fmt.Sprintf("[%s] %s: %s", strings.ToUpper(batchName), r.TaskID, r.Title)
		taskDesc := fmt.Sprintf("Generated by: %s batch %s\nTask class: %s\nArtifact: %s\n\n%s",
			batchName, batchIndex.BatchID, r.TaskID, r.ArtifactPath, artifactSnippet)

		childKey := jiraCreateIssue(jiraCfg, taskSummary, taskDesc, taskLabels, "Low")
		if childKey != "" {
			childKeys[r.TaskID] = childKey
			log.Printf("[JIRA] Created child issue: %s — %s", childKey, taskSummary)
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

func splitLines(s string) []string  { return splitString(s, '\n') }
func contains(s, sub string) bool   { return indexOf(s, sub) >= 0 }
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
	for i < j && s[i] == ' ' { i++ }
	for j > i && s[j-1] == ' ' { j-- }
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
