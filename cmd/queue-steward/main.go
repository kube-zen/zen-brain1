// Package main — queue-steward
//
// Queue Steward: L1 factory-floor supervisor for zen-brain1.
//
// Inspects Jira queue state, feeds it to L1 for structured recommendations,
// then executes safe actions (dispatch, requeue, pause, escalate).
//
// GLM-5 stays in supervisor/policy role — it does NOT do routine queue care.
// Underfilled factory with ready backlog is treated as a bug.
// Queue Steward manages flow, not strategy.
// Queue Steward does not bypass validation or approval rules.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/secrets"
)

// ─── Config ───

type stewardConfig struct {
	JiraURL      string
	JiraEmail    string
	JiraAPIToken string
	JiraProject  string
	L1Endpoint   string
	L1Model      string
	SafeTarget   int
	StaleMinutes int
	DiscoveryMax int
	ArtifactDir  string
	DryRun       bool
	Mode         string // "fast", "summary", "daily"
}

func loadConfig() stewardConfig {
	// ZB-CREDENTIAL-RAILS: Use canonical resolver for Jira credentials
	jiraCreds, err := secrets.ResolveJira(context.Background(), secrets.JiraResolveOptions{
		ClusterMode: false,
		DirPath:     "",
	})
	if err != nil {
		log.Printf("[WARN] Jira credential resolution failed: %v", err)
	}

	cfg := stewardConfig{
		JiraURL:      jiraCreds.BaseURL,
		JiraEmail:    jiraCreds.Email,
		JiraAPIToken: jiraCreds.APIToken,
		JiraProject:  jiraCreds.ProjectKey,
		L1Endpoint:   envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:      envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		SafeTarget:   envIntOr("SAFE_L1_CONCURRENCY", 5),
		StaleMinutes: envIntOr("STALE_MINUTES", 30),
		DiscoveryMax: envIntOr("DISCOVERY_MAX", 10),
		ArtifactDir:  envOr("ARTIFACT_DIR", "/var/lib/zen-brain1/evidence/queue-steward"),
		DryRun:       os.Getenv("DRY_RUN") != "",
		Mode:         envOr("STEWARD_MODE", "fast"),
	}

	// Apply defaults if resolver returned empty values
	if cfg.JiraURL == "" {
		cfg.JiraURL = "https://zen-mesh.atlassian.net"
	}
	if cfg.JiraEmail == "" {
		cfg.JiraEmail = "zen@zen-mesh.io"
	}
	if cfg.JiraProject == "" {
		cfg.JiraProject = "ZB"
	}

	return cfg
}

// ─── Jira State ───

type jiraTicket struct {
	Key     string   `json:"key"`
	Summary string   `json:"summary"`
	Status  string   `json:"status"`
	Labels  []string `json:"labels"`
	Updated string   `json:"updated"`
}

type queueSnapshot struct {
	Timestamp          string       `json:"timestamp"`
	SafeTarget         int          `json:"safe_target"`
	ReadyBacklog       []jiraTicket `json:"ready_backlog"`
	Retrying           []jiraTicket `json:"retrying"`
	SelectedForDev     []jiraTicket `json:"selected_for_dev"`
	InProgress         []jiraTicket `json:"in_progress"`
	Paused             []jiraTicket `json:"paused"`
	Blocked            []jiraTicket `json:"blocked"`
	ToEscalate         []jiraTicket `json:"to_escalate"`
	Done               int          `json:"done_count"`
	DiscoveryThrottled bool         `json:"discovery_throttled"`
	TargetInProgress   int          `json:"target_in_progress"`
	ActualInProgress   int          `json:"actual_in_progress"`
	FillRatio          float64      `json:"fill_ratio"`
}

func jiraSearch(cfg stewardConfig, jql string, fields []string) ([]jiraTicket, error) {
	body := map[string]interface{}{
		"jql":        jql,
		"maxResults": 100,
		"fields":     fields,
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/search/jql", bytes.NewReader(data))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
				Labels  []string `json:"labels"`
				Updated string   `json:"updated"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var tickets []jiraTicket
	for _, t := range result.Issues {
		tickets = append(tickets, jiraTicket{
			Key:     t.Key,
			Summary: t.Fields.Summary,
			Status:  t.Fields.Status.Name,
			Labels:  t.Fields.Labels,
			Updated: t.Fields.Updated,
		})
	}
	return tickets, nil
}

func jiraCount(cfg stewardConfig, jql string) int {
	body := map[string]interface{}{
		"jql":        jql,
		"maxResults": 0,
		"fields":     []string{},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/search/jql", bytes.NewReader(data))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Total
}

func jiraTransition(cfg stewardConfig, key, targetName string) bool {
	req, _ := http.NewRequest("GET", cfg.JiraURL+"/rest/api/3/issue/"+key+"/transitions", nil)
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var tr struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	json.NewDecoder(resp.Body).Decode(&tr)
	// Try again with different struct
	type transition struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var tr2 struct {
		Transitions []transition `json:"transitions"`
	}
	resp2, _ := http.NewRequest("GET", cfg.JiraURL+"/rest/api/3/issue/"+key+"/transitions", nil)
	resp2.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	resp2.Header.Set("Content-Type", "application/json")
	r2, err := http.DefaultClient.Do(resp2)
	if err != nil {
		// fallback to first parse
		for _, t := range tr.Transitions {
			if strings.EqualFold(t.Name, targetName) {
				return doTransition(cfg, key, t.ID)
			}
		}
		return false
	}
	defer r2.Body.Close()
	json.NewDecoder(r2.Body).Decode(&tr2)
	for _, t := range tr2.Transitions {
		if strings.EqualFold(t.Name, targetName) {
			return doTransition(cfg, key, t.ID)
		}
	}
	return false
}

func doTransition(cfg stewardConfig, key, id string) bool {
	body, _ := json.Marshal(map[string]interface{}{"transition": map[string]string{"id": id}})
	req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/issue/"+key+"/transitions", bytes.NewReader(body))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 204
}

// ─── Gather Queue Snapshot ───

func gatherSnapshot(cfg stewardConfig) queueSnapshot {
	proj := cfg.JiraProject
	fields := []string{"summary", "status", "labels", "updated"}

	readyBacklog, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status=Backlog AND labels=bug AND labels=ai:finding AND -labels="quality:blocked-invalid-payload" AND -labels="ai:blocked" ORDER BY updated ASC`, proj), fields)

	retrying, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status=RETRYING ORDER BY updated ASC`, proj), fields)

	selected, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status="Selected for Development" ORDER BY updated ASC`, proj), fields)

	inProgress, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status="In Progress" ORDER BY updated ASC`, proj), fields)

	paused, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status=PAUSED ORDER BY updated ASC`, proj), fields)

	blocked, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND labels="quality:blocked-invalid-payload" AND status=Backlog ORDER BY updated ASC`, proj), fields)

	toEscalate, _ := jiraSearch(cfg, fmt.Sprintf(
		`project="%s" AND status=TO_ESCALATE ORDER BY updated ASC`, proj), fields)

	doneCount := jiraCount(cfg, fmt.Sprintf(
		`project="%s" AND status=Done AND updated >= -24h`, proj))

	readyCount := len(readyBacklog)
	retryingCount := len(retrying)
	actualIP := len(inProgress)
	targetIP := min(cfg.SafeTarget, readyCount+retryingCount)

	var fillRatio float64
	if targetIP > 0 {
		fillRatio = float64(actualIP) / float64(targetIP)
	}

	discoveryThrottled := readyCount > cfg.DiscoveryMax

	return queueSnapshot{
		Timestamp:          time.Now().Format(time.RFC3339),
		SafeTarget:         cfg.SafeTarget,
		ReadyBacklog:       readyBacklog,
		Retrying:           retrying,
		SelectedForDev:     selected,
		InProgress:         inProgress,
		Paused:             paused,
		Blocked:            blocked,
		ToEscalate:         toEscalate,
		Done:               doneCount,
		DiscoveryThrottled: discoveryThrottled,
		TargetInProgress:   targetIP,
		ActualInProgress:   actualIP,
		FillRatio:          fillRatio,
	}
}

// ─── L1 Call ───

type stewardRecommendation struct {
	QueueHealthSummary      string  `json:"queue_health_summary"`
	TargetInProgress        int     `json:"target_in_progress"`
	ActualInProgress        int     `json:"actual_in_progress"`
	FillRatio               float64 `json:"fill_ratio"`
	DispatchRecommendations []struct {
		Key       string `json:"key"`
		Action    string `json:"action"`
		Readiness string `json:"readiness"`
	} `json:"dispatch_recommendations"`
	StaleTicketActions []struct {
		Key    string `json:"key"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	} `json:"stale_ticket_actions"`
	RetryActions []struct {
		Key    string `json:"key"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	} `json:"retry_actions"`
	ThrottleRecommendation struct {
		DiscoveryThrottled bool   `json:"discovery_throttled"`
		Reason             string `json:"reason"`
	} `json:"throttle_recommendation"`
	TicketsNeedingReview []string `json:"tickets_needing_review"`
	Notes                string   `json:"notes"`
}

func callL1(cfg stewardConfig, snapshot queueSnapshot) (*stewardRecommendation, error) {
	snapJSON, _ := json.MarshalIndent(snapshot, "", "  ")

	prompt := fmt.Sprintf(`You are the Queue Steward. Your job is to inspect the queue and produce structured recommendations.

INPUT (current queue state):
%s

RULES:
- You are a factory-floor queue supervisor. You manage flow, not strategy.
- You do NOT change policy, worker counts, or approval gates.
- You do NOT move tickets to Done unless validation already passed.
- Underfilled factory with ready backlog is a bug — recommend dispatch.
- Classify each ready ticket by readiness.
- Stale tickets (in Selected/In Progress/RETRYING > 30 min) need action.
- Discovery throttle: if ready_backlog > 10, recommend throttling discovery.
- Target in_progress = min(safe_concurrency, ready_backlog + retrying)
- Work split: 70%% remediation, 20%% roadmap, 10%% discovery

OUTPUT — respond with ONLY this JSON structure, no other text:
{
  "queue_health_summary": "one sentence",
  "target_in_progress": %d,
  "actual_in_progress": %d,
  "fill_ratio": %.2f,
  "dispatch_recommendations": [],
  "stale_ticket_actions": [],
  "retry_actions": [],
  "throttle_recommendation": {"discovery_throttled": false, "reason": ""},
  "tickets_needing_review": [],
  "notes": ""
}`, string(snapJSON), snapshot.TargetInProgress, snapshot.ActualInProgress, snapshot.FillRatio)

	// Call L1
	reqBody := map[string]interface{}{
		"model": cfg.L1Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are the Queue Steward for zen-brain1. Respond with only valid JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  2000,
	}
	data, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("POST", cfg.L1Endpoint+"/v1/chat/completions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("L1 call failed: %w", err)
	}
	defer resp.Body.Close()

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return nil, fmt.Errorf("L1 response parse error: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("L1 returned no choices")
	}

	content := llmResp.Choices[0].Message.Content
	// Extract JSON from response (might have markdown wrapping)
	content = extractJSON(content)

	var rec stewardRecommendation
	if err := json.Unmarshal([]byte(content), &rec); err != nil {
		return nil, fmt.Errorf("L1 output parse error: %w\nraw: %s", err, content)
	}
	return &rec, nil
}

func extractJSON(s string) string {
	// Try to find JSON object in the response
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Execute Actions ───

func executeActions(cfg stewardConfig, rec *stewardRecommendation, snapshot queueSnapshot) map[string]string {
	results := make(map[string]string)

	// Dispatch recommendations — move to In Progress via Selected for Development
	for _, d := range rec.DispatchRecommendations {
		if cfg.DryRun {
			results[d.Key] = fmt.Sprintf("DRY_RUN: would dispatch %s (readiness=%s)", d.Key, d.Readiness)
			continue
		}
		// Move Backlog → Selected for Development → In Progress
		if jiraTransition(cfg, d.Key, "Selected for Development") {
			if jiraTransition(cfg, d.Key, "In Progress") {
				results[d.Key] = fmt.Sprintf("dispatched (readiness=%s)", d.Readiness)
			} else {
				results[d.Key] = "moved to Selected for Development but failed In Progress transition"
			}
		} else {
			results[d.Key] = "failed to move to Selected for Development"
		}
	}

	// Stale ticket actions
	for _, a := range rec.StaleTicketActions {
		if cfg.DryRun {
			results[a.Key] = fmt.Sprintf("DRY_RUN: would %s (%s)", a.Action, a.Reason)
			continue
		}
		switch a.Action {
		case "requeue":
			jiraTransition(cfg, a.Key, "Backlog")
			results[a.Key] = fmt.Sprintf("requeued (%s)", a.Reason)
		case "pause":
			jiraTransition(cfg, a.Key, "PAUSED")
			results[a.Key] = fmt.Sprintf("paused (%s)", a.Reason)
		case "escalate":
			jiraTransition(cfg, a.Key, "TO_ESCALATE")
			results[a.Key] = fmt.Sprintf("escalated (%s)", a.Reason)
		default:
			results[a.Key] = fmt.Sprintf("unknown action: %s", a.Action)
		}
	}

	// Retry actions
	for _, a := range rec.RetryActions {
		if cfg.DryRun {
			results[a.Key] = fmt.Sprintf("DRY_RUN: would %s (%s)", a.Action, a.Reason)
			continue
		}
		switch a.Action {
		case "retry":
			jiraTransition(cfg, a.Key, "In Progress")
			results[a.Key] = fmt.Sprintf("retried (%s)", a.Reason)
		case "pause":
			jiraTransition(cfg, a.Key, "PAUSED")
			results[a.Key] = fmt.Sprintf("paused (%s)", a.Reason)
		case "escalate":
			jiraTransition(cfg, a.Key, "TO_ESCALATE")
			results[a.Key] = fmt.Sprintf("escalated (%s)", a.Reason)
		default:
			results[a.Key] = fmt.Sprintf("unknown retry action: %s", a.Action)
		}
	}

	return results
}

// ─── Artifacts ───

func writeArtifacts(cfg stewardConfig, snapshot queueSnapshot, rec *stewardRecommendation, actions map[string]string) error {
	os.MkdirAll(cfg.ArtifactDir, 0755)

	runID := time.Now().Format("20060102-150405")

	// queue-health.json
	health := map[string]interface{}{
		"run_id":              runID,
		"timestamp":           snapshot.Timestamp,
		"mode":                cfg.Mode,
		"safe_target":         snapshot.SafeTarget,
		"target_in_progress":  snapshot.TargetInProgress,
		"actual_in_progress":  snapshot.ActualInProgress,
		"fill_ratio":          snapshot.FillRatio,
		"ready_backlog":       len(snapshot.ReadyBacklog),
		"retrying":            len(snapshot.Retrying),
		"selected_for_dev":    len(snapshot.SelectedForDev),
		"in_progress":         len(snapshot.InProgress),
		"paused":              len(snapshot.Paused),
		"blocked":             len(snapshot.Blocked),
		"to_escalate":         len(snapshot.ToEscalate),
		"done_24h":            snapshot.Done,
		"discovery_throttled": snapshot.DiscoveryThrottled,
		"l1_summary":          rec.QueueHealthSummary,
		"dispatches":          len(rec.DispatchRecommendations),
		"stale_actions":       len(rec.StaleTicketActions),
		"retry_actions":       len(rec.RetryActions),
	}
	healthJSON, _ := json.MarshalIndent(health, "", "  ")
	os.WriteFile(filepath.Join(cfg.ArtifactDir, "queue-health.json"), healthJSON, 0644)

	// queue-actions.json
	actionsJSON, _ := json.MarshalIndent(map[string]interface{}{
		"run_id":          runID,
		"timestamp":       snapshot.Timestamp,
		"actions":         actions,
		"recommendations": rec,
	}, "", "  ")
	os.WriteFile(filepath.Join(cfg.ArtifactDir, "queue-actions.json"), actionsJSON, 0644)

	// queue-health.md
	var md strings.Builder
	md.WriteString("# Queue Steward Report\n\n")
	md.WriteString(fmt.Sprintf("**Run ID:** %s  \n", runID))
	md.WriteString(fmt.Sprintf("**Mode:** %s  \n", cfg.Mode))
	md.WriteString(fmt.Sprintf("**Timestamp:** %s  \n\n", snapshot.Timestamp))
	md.WriteString(fmt.Sprintf("**Summary:** %s\n\n", rec.QueueHealthSummary))

	md.WriteString("## Queue State\n\n")
	md.WriteString("| Metric | Value |\n|--------|-------|\n")
	md.WriteString(fmt.Sprintf("| Ready backlog | %d |\n", len(snapshot.ReadyBacklog)))
	md.WriteString(fmt.Sprintf("| Retry | %d |\n", len(snapshot.Retrying)))
	md.WriteString(fmt.Sprintf("| Selected for Dev | %d |\n", len(snapshot.SelectedForDev)))
	md.WriteString(fmt.Sprintf("| In Progress | %d |\n", len(snapshot.InProgress)))
	md.WriteString(fmt.Sprintf("| Paused | %d |\n", len(snapshot.Paused)))
	md.WriteString(fmt.Sprintf("| Blocked | %d |\n", len(snapshot.Blocked)))
	md.WriteString(fmt.Sprintf("| To Escalate | %d |\n", len(snapshot.ToEscalate)))
	md.WriteString(fmt.Sprintf("| Done (24h) | %d |\n", snapshot.Done))
	md.WriteString(fmt.Sprintf("| Safe target | %d |\n", snapshot.SafeTarget))
	md.WriteString(fmt.Sprintf("| Target in-progress | %d |\n", snapshot.TargetInProgress))
	md.WriteString(fmt.Sprintf("| Actual in-progress | %d |\n", snapshot.ActualInProgress))
	md.WriteString(fmt.Sprintf("| Fill ratio | %.0f%% |\n", snapshot.FillRatio*100))
	md.WriteString(fmt.Sprintf("| Discovery throttled | %v |\n", snapshot.DiscoveryThrottled))

	if len(actions) > 0 {
		md.WriteString("\n## Actions Taken\n\n")
		keys := make([]string, 0, len(actions))
		for k := range actions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			md.WriteString(fmt.Sprintf("- **%s**: %s\n", k, actions[k]))
		}
	}

	if rec.Notes != "" {
		md.WriteString(fmt.Sprintf("\n## Notes\n\n%s\n", rec.Notes))
	}

	os.WriteFile(filepath.Join(cfg.ArtifactDir, "queue-health.md"), []byte(md.String()), 0644)
	os.WriteFile(filepath.Join(cfg.ArtifactDir, fmt.Sprintf("queue-health-%s.md", runID)), []byte(md.String()), 0644)

	log.Printf("[STEWARD] Artifacts written to %s", cfg.ArtifactDir)
	return nil
}

// ─── Helpers ───

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ─── Main ───

func main() {
	cfg := loadConfig()

	if cfg.JiraAPIToken == "" {
		token, err := os.ReadFile(envOr("JIRA_TOKEN_FILE", os.Getenv("HOME")+"/zen/keys/zen-brain/secrets.d/jira.enc"))
		if err == nil {
			cfg.JiraAPIToken = strings.TrimSpace(string(token))
		}
	}

	log.Printf("[STEWARD] Queue Steward starting (mode=%s, target=%d, dry_run=%v)",
		cfg.Mode, cfg.SafeTarget, cfg.DryRun)

	// Phase 1: Gather snapshot
	log.Printf("[STEWARD] Gathering queue snapshot...")
	snapshot := gatherSnapshot(cfg)
	log.Printf("[STEWARD] Snapshot: ready=%d retrying=%d in_progress=%d paused=%d blocked=%d target_ip=%d actual_ip=%d fill=%.0f%%",
		len(snapshot.ReadyBacklog), len(snapshot.Retrying), len(snapshot.InProgress),
		len(snapshot.Paused), len(snapshot.Blocked),
		snapshot.TargetInProgress, snapshot.ActualInProgress, snapshot.FillRatio*100)

	// Phase 2: Call L1 for recommendations
	log.Printf("[STEWARD] Calling L1 for recommendations...")
	rec, err := callL1(cfg, snapshot)
	if err != nil {
		log.Printf("[STEWARD] ⚠️ L1 call failed: %v — using heuristic fallback", err)
		rec = heuristicRecommendation(cfg, snapshot)
	}
	log.Printf("[STEWARD] L1 summary: %s", rec.QueueHealthSummary)
	log.Printf("[STEWARD] L1 recommends: %d dispatches, %d stale actions, %d retry actions",
		len(rec.DispatchRecommendations), len(rec.StaleTicketActions), len(rec.RetryActions))

	// Phase 3: Execute safe actions
	log.Printf("[STEWARD] Executing actions...")
	actions := executeActions(cfg, rec, snapshot)
	for k, v := range actions {
		log.Printf("[STEWARD]   %s: %s", k, v)
	}

	// Phase 4: Write artifacts
	writeArtifacts(cfg, snapshot, rec, actions)

	// Summary
	log.Printf("[STEWARD] === Run complete ===")
	log.Printf("[STEWARD] Fill: %d/%d (%.0f%%), Discovery throttled: %v",
		snapshot.ActualInProgress, snapshot.TargetInProgress, snapshot.FillRatio*100, snapshot.DiscoveryThrottled)

	if snapshot.ActualInProgress < snapshot.TargetInProgress && len(snapshot.ReadyBacklog) > 0 {
		log.Printf("[STEWARD] ⚠️ UNDERFILL: %d ready tickets but only %d in-progress — factory not filled",
			len(snapshot.ReadyBacklog), snapshot.ActualInProgress)
	}
}

// heuristicRecommendation provides a deterministic fallback when L1 is unavailable.
func heuristicRecommendation(cfg stewardConfig, snap queueSnapshot) *stewardRecommendation {
	rec := &stewardRecommendation{
		QueueHealthSummary: fmt.Sprintf("L1 unavailable; heuristic: %d ready, %d in-progress, target=%d", len(snap.ReadyBacklog), len(snap.InProgress), snap.TargetInProgress),
		TargetInProgress:   snap.TargetInProgress,
		ActualInProgress:   snap.ActualInProgress,
		FillRatio:          snap.FillRatio,
		ThrottleRecommendation: struct {
			DiscoveryThrottled bool   `json:"discovery_throttled"`
			Reason             string `json:"reason"`
		}{
			DiscoveryThrottled: snap.DiscoveryThrottled,
			Reason:             fmt.Sprintf("ready backlog %d vs threshold %d", len(snap.ReadyBacklog), cfg.DiscoveryMax),
		},
	}

	// Recommend dispatch for underfill
	shortfall := snap.TargetInProgress - snap.ActualInProgress
	if shortfall > 0 {
		for i := 0; i < shortfall && i < len(snap.ReadyBacklog); i++ {
			rec.DispatchRecommendations = append(rec.DispatchRecommendations, struct {
				Key       string `json:"key"`
				Action    string `json:"action"`
				Readiness string `json:"readiness"`
			}{snap.ReadyBacklog[i].Key, "dispatch", "ready_for_execution"})
		}
	}

	// Stale detection: RETRYING tickets
	for _, t := range snap.Retrying {
		rec.RetryActions = append(rec.RetryActions, struct {
			Key    string `json:"key"`
			Action string `json:"action"`
			Reason string `json:"reason"`
		}{t.Key, "retry", "in RETRYING state"})
	}

	return rec
}

// Ensure fs is imported (used indirectly by writeArtifacts mkdir)
var _ fs.FileMode = 0
