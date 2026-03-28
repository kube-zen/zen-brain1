package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/metrics"
)

// ─── Configuration ────────────────────────────────────────────────────

type remediationConfig struct {
	L1Endpoint    string
	L1Model       string
	JiraURL       string
	JiraEmail     string
	JiraToken     string
	JiraProject   string
	ArtifactRoot  string
	EvidenceRoot  string
	RepoRoot      string
	MaxTickets    int    // max tickets to process per run
	TimeoutSec    int
	Lane          string // "l1" or "l2"
	SREDCode      string // default SR&ED uncertainty code if applicable
	IRAPWorkPkg   string // default IRAP work package if applicable
}

type jiraConfig struct {
	url    string
	email  string
	token  string
	project string
	enabled bool
}

// ─── Terminal Classification (PHASE A FIX) ──────────────────────────
//
// Every remediation run produces an explicit terminal classification file.
// The factory-fill dispatcher reads this file instead of scraping stdout.
// This ensures quality-gate-rejected tickets NEVER stay In Progress.

// TerminalClass is the explicit outcome of a single ticket remediation.
type TerminalClass string

const (
	TerminalDone                TerminalClass = "done"
	TerminalNeedsReview         TerminalClass = "needs_review"
	TerminalPaused              TerminalClass = "paused"
	TerminalRetrying            TerminalClass = "retrying"
	TerminalToEscalate          TerminalClass = "to_escalate"
	TerminalBlockedInvalidPayl  TerminalClass = "blocked_invalid_payload"
	TerminalFailed              TerminalClass = "failed" // L1 call failed entirely
)

// WorkerTerminalResult is written to RESULT_DIR/{JIRA_KEY}.json after each ticket.
type WorkerTerminalResult struct {
	JiraKey         string        `json:"jira_key"`
	TerminalClass   TerminalClass `json:"terminal_class"`
	QualityScore    int           `json:"quality_score"`
	QualityPassed   bool          `json:"quality_passed"`
	L1Status        string        `json:"l1_status"`          // success, needs_review, blocked, to_escalate
	JiraState       string        `json:"jira_state"`         // final Jira status after transition
	EvidencePath    string        `json:"evidence_path"`
	BlockerReason   string        `json:"blocker_reason,omitempty"`
	Issues          []string      `json:"issues,omitempty"`
	GateLogPath     string        `json:"gate_log_path,omitempty"`
	Timestamp       string        `json:"timestamp"`
}

// writeTerminalResult writes the terminal classification to a JSON file.
func writeTerminalResult(resultDir string, res WorkerTerminalResult) {
	if resultDir == "" {
		return
	}
	os.MkdirAll(resultDir, 0755)
	res.Timestamp = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Printf("[TERMINAL-RESULT] marshal error for %s: %v", res.JiraKey, err)
		return
	}
	path := filepath.Join(resultDir, res.JiraKey+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[TERMINAL-RESULT] write error for %s: %v", res.JiraKey, err)
		return
	}
	log.Printf("[TERMINAL-RESULT] %s: wrote %s → class=%s quality=%d passed=%v jira=%s",
		res.JiraKey, path, res.TerminalClass, res.QualityScore, res.QualityPassed, res.JiraState)
}

// terminalResultDir returns the directory for terminal result files.
func terminalResultDir() string {
	return envOr("RESULT_DIR", "/tmp/zen-brain1-worker-results")
}

// ─── Data Types ────────────────────────────────────────────────────────

// RemediationTicket represents a Jira ticket selected for remediation.
type RemediationTicket struct {
	Key            string   `json:"key"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	Priority       string   `json:"priority"`
	Labels         []string `json:"labels"`
	Status         string   `json:"status"`
	ApprovalLevel  int      `json:"approval_level"`
	RelatedProject string   `json:"related_project"`
	EvidenceLink   string   `json:"evidence_link"`
	QueueLevel     int      `json:"queue_level"`
	SREDCode       string   `json:"sred_code"`
	IRAPWorkPkg    string   `json:"irap_work_pkg"`
	FollowUpType   string   `json:"follow_up_type"` // bounded_fix_l1, bounded_synthesis_l2, manual_review
	SourceReport   string   `json:"source_report"`
}

// RemediationPacket is what L1 receives for one bounded remediation task.
type RemediationPacket struct {
	JiraKey        string `json:"jira_key"`
	ProblemSummary string `json:"problem_summary"`
	TargetFiles    string `json:"target_files"`
	EvidencePaths  string `json:"evidence_paths"`
	SuccessCriteria string `json:"success_criteria"`
	ValidationCmds string `json:"validation_cmds"`
	Constraints    string `json:"constraints"`
}

// RemediationOutput is what L1 produces.
type RemediationOutput struct {
	RemediationType string `json:"remediation_type"` // code_edit, config_change, doc_update, cannot_fix
	FileToEdit      string `json:"file_to_edit"`
	ChangeType      string `json:"change_type,omitempty"` // create, modify, delete
	EditDescription string `json:"edit_description"`
	NewContent      string `json:"new_content,omitempty"`   // kept for compat, but 0.8b no longer fills this
	ConfigChanges   string `json:"config_changes,omitempty"`
	Fields          map[string]interface{} `json:"fields,omitempty"` // structured key-value changes
	Explanation     string `json:"explanation"`
	FinalStatus     string `json:"final_status"`     // success, needs_review, blocked, to_escalate
	BlockerReason   string `json:"blocker_reason,omitempty"`
	ValidationResult string `json:"validation_result"`
	ComplianceNote  string `json:"compliance_note,omitempty"`
}

// EvidencePack represents the skeleton for compliance evidence.
type EvidencePack struct {
	JiraKey        string    `json:"jira_key"`
	RelatedProject string    `json:"related_project"`
	ApprovalLevel  int       `json:"approval_level"`
	SREDCode       string    `json:"sred_code"`
	IRAPWorkPkg    string    `json:"irap_work_pkg"`
	SourceReport   string    `json:"source_report"`
	CreatedAt      time.Time `json:"created_at"`
	RunID          string    `json:"run_id"`
	RemediationResult string `json:"remediation_result"`
	EvidencePath   string    `json:"evidence_path"`
}

// ─── Helpers ──────────────────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, _ := strconv.Atoi(v)
		return n
	}
	return fallback
}

func loadJiraConfig() jiraConfig {
	jcfg := jiraConfig{
		url:     envOr("JIRA_URL", ""),
		email:   envOr("JIRA_EMAIL", ""),
		token:   envOr("JIRA_API_TOKEN", ""),
		project: envOr("JIRA_PROJECT_KEY", "ZB"),
	}
	// Try ZenLock-mounted secrets
	for _, path := range []string{
		"/zen-lock/secrets/JIRA_URL",
		"/zen-lock/secrets/JIRA_EMAIL",
		"/zen-lock/secrets/JIRA_API_TOKEN",
		"/zen-lock/secrets/JIRA_PROJECT_KEY",
	} {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			val := strings.TrimSpace(string(data))
			switch filepath.Base(path) {
			case "JIRA_URL":
				jcfg.url = val
			case "JIRA_EMAIL":
				jcfg.email = val
			case "JIRA_API_TOKEN":
				jcfg.token = val
			case "JIRA_PROJECT_KEY":
				jcfg.project = val
			}
		}
	}
	// Fallback to env from sudo cat
	if jcfg.url == "" {
		if data, err := exec.Command("sudo", "cat", "/etc/zen-brain1/jira.env").Output(); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "export ") {
					line = strings.TrimPrefix(line, "export ")
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.Trim(parts[1], "\"")
					switch key {
					case "JIRA_URL":
						jcfg.url = val
					case "JIRA_EMAIL":
						jcfg.email = val
					case "JIRA_API_TOKEN":
						jcfg.token = val
					case "JIRA_PROJECT_KEY":
						jcfg.project = val
					}
				}
			}
		}
	}
	jcfg.enabled = jcfg.url != "" && jcfg.email != "" && jcfg.token != "" && jcfg.project != ""
	return jcfg
}

func contextWithTimeout(sec int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
}

func normalizePriority(p string) string {
	switch strings.ToLower(p) {
	case "highest", "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	case "lowest":
		return "Lowest"
	default:
		return "Medium"
	}
}

// ─── Phase 2: Remediation Queue ───────────────────────────────────────

// fetchOpenTickets queries Jira for ai:finding tickets suitable for remediation.
func fetchOpenTickets(jcfg jiraConfig, maxApprovalLevel int) ([]RemediationTicket, error) {
	if !jcfg.enabled {
		return nil, fmt.Errorf("Jira not configured")
	}

	// JQL: open tickets with ai:finding label that haven't been remediated yet
	jql := fmt.Sprintf(`project = "%s" AND labels = ai:finding AND status NOT IN (Done, Closed) AND status NOT IN ("In Progress") ORDER BY priority DESC, created ASC`,
		jcfg.project)

	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": 20,
		"fields":     []string{"summary", "description", "priority", "labels", "status", "comment"},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(15)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		jcfg.url+"/rest/api/3/search/jql",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jira search returned %d: %s", resp.StatusCode, string(body))
	}

	var result jiraSearchResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Jira response: %w", err)
	}

	var tickets []RemediationTicket
	for _, issue := range result.Issues {
		// Extract plain text from description — not available in summary search
		desc := issue.Fields.Summary

		// Determine follow_up_type from description (ticketizer writes it)
		followUpType := "manual_review" // default
		for _, label := range issue.Fields.Labels {
			if strings.HasPrefix(label, "follow_up:") {
				followUpType = strings.TrimPrefix(label, "follow_up:")
			}
		}

		tickets = append(tickets, RemediationTicket{
			Key:           issue.Key,
			Summary:       issue.Fields.Summary,
			Description:   desc,
			Priority:      issue.Fields.Priority.Name,
			Labels:        issue.Fields.Labels,
			Status:        issue.Fields.Status.Name,
			ApprovalLevel: 2, // default: review-required for AI tickets
			FollowUpType:  followUpType,
		})
	}

	return tickets, nil
}

// selectTickets filters and ranks tickets for remediation.
func selectTickets(tickets []RemediationTicket, maxTickets int, allowedApprovalLevel int) []RemediationTicket {
	var selected []RemediationTicket

	for _, t := range tickets {
		// Skip tickets above allowed approval level
		if t.ApprovalLevel > allowedApprovalLevel {
			log.Printf("[QUEUE] skip %s: approval level %d exceeds max %d", t.Key, t.ApprovalLevel, allowedApprovalLevel)
			continue
		}

		// Prefer bounded_fix_l1 tickets
		if t.FollowUpType == "bounded_fix_l1" || t.FollowUpType == "bounded_synthesis_l2" {
			selected = append(selected, t)
			log.Printf("[QUEUE] select %s: type=%s priority=%s", t.Key, t.FollowUpType, t.Priority)
		}

		if len(selected) >= maxTickets {
			break
		}
	}

	return selected
}

// ─── Phase 1: Remediation Template ────────────────────────────────────

// buildRemediationPacket creates a structured L1 packet from a Jira ticket.
// Phase 39C: Try loading pre-built packet JSON first for rich context.
func buildRemediationPacket(ticket RemediationTicket, repoRoot string) RemediationPacket {
	// Try loading pre-built packet JSON (from config/task-templates/remediation/{KEY}-packet.json)
	packetPath := filepath.Join(repoRoot, "config", "task-templates", "remediation", ticket.Key+"-packet.json")
	if data, err := os.ReadFile(packetPath); err == nil {
		var prebuilt struct {
			TargetFiles     string `json:"target_files"`
			EvidencePaths   string `json:"evidence_paths"`
			SuccessCriteria string `json:"success_criteria"`
			ValidationCmds  string `json:"validation_cmds"`
			OutputContract  string `json:"output_contract"`
		}
		if json.Unmarshal(data, &prebuilt) == nil {
			log.Printf("[PACKET] %s: loaded pre-built packet from %s", ticket.Key, packetPath)
			return RemediationPacket{
				JiraKey:         ticket.Key,
				ProblemSummary:  ticket.Summary + "\n\n" + ticket.Description,
				TargetFiles:     prebuilt.TargetFiles,
				EvidencePaths:   prebuilt.EvidencePaths,
				SuccessCriteria: prebuilt.SuccessCriteria,
				ValidationCmds:  prebuilt.ValidationCmds,
				Constraints:     "Edit only the specific target files identified. If you cannot determine what to edit, return final_status: blocked. Output contract: " + prebuilt.OutputContract,
			}
		}
	}

	// Fallback: extract target files from ticket description
	targetFiles := "no specific target files identified"
	evidencePaths := ""

	// Try to extract file references from the description
	fileRefRe := regexp.MustCompile(`[\w./-]+\.(go|yaml|yml|json|md|sh|toml|cfg|conf)`)
	matches := fileRefRe.FindAllString(ticket.Description, 10)
	if len(matches) > 0 {
		var realFiles []string
		for _, m := range matches {
			fullPath := filepath.Join(repoRoot, m)
			if _, err := os.Stat(fullPath); err == nil {
				realFiles = append(realFiles, m)
			}
		}
		if len(realFiles) > 0 {
			targetFiles = strings.Join(realFiles, ", ")
		}
	}

	// Find latest evidence artifacts
	if artifactRoot := os.Getenv("ARTIFACT_ROOT"); artifactRoot != "" {
		var evidences []string
		entries, _ := os.ReadDir(artifactRoot)
		for _, e := range entries {
			if e.IsDir() {
				finalDir := filepath.Join(artifactRoot, e.Name(), "final")
				if files, err := filepath.Glob(filepath.Join(finalDir, "*.md")); err == nil && len(files) > 0 {
					evidences = append(evidences, files[0])
				}
			}
		}
		if len(evidences) > 0 {
			evidencePaths = strings.Join(evidences[:3], ", ") // cap at 3
		}
	}

	return RemediationPacket{
		JiraKey:         ticket.Key,
		ProblemSummary:  ticket.Summary + "\n\n" + ticket.Description,
		TargetFiles:     targetFiles,
		EvidencePaths:   evidencePaths,
		SuccessCriteria: "Issue should be resolved or clearly documented as blocked/to_escalate",
		ValidationCmds:  "go build ./... 2>&1",
		Constraints: "You must produce ONLY a bounded remediation. No architecture changes. No repo-wide refactors. Edit only the specific target files identified. If you cannot determine what to edit, return final_status: blocked.",
	}
}

// ─── Phase 5: L1 Remediation Execution ────────────────────────────────

// executeRemediationViaL1 sends a bounded remediation task to L1.
func executeRemediationViaL1(endpoint, model string, packet RemediationPacket, timeoutSec int) (*RemediationOutput, error) {
	systemPrompt := `You are a remediation worker for zen-brain1. Produce a bounded change description for the target files.
Return ONLY valid JSON: {"remediation_type":"code_edit|config_change|doc_update|cannot_fix","file_to_edit":"path","change_type":"create|modify|delete","edit_description":"what to change and why","fields":{"key":"value"},"explanation":"why","final_status":"success|needs_review|blocked|to_escalate","blocker_reason":null,"validation_result":null}
No new_content field. No markdown. No prose. Just the JSON object.`

	userPrompt := fmt.Sprintf(`Ticket: %s
Target: %s
Evidence: %s
Criteria: %s
Validate: %s
Constraints: %s
Return JSON only.`, packet.JiraKey, packet.TargetFiles, packet.EvidencePaths, truncate(packet.SuccessCriteria, 200), packet.ValidationCmds, truncate(packet.Constraints, 200))

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":      0.3,
		"max_tokens":       4096,
		"chat_template_kwargs": map[string]interface{}{"enable_thinking": false},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(timeoutSec)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		endpoint+"/v1/chat/completions",
		strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("L1 request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("L1 returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response — extract JSON from the response content
	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	json.Unmarshal(body, &llmResp)
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("empty L1 response")
	}

	content := llmResp.Choices[0].Message.Content

	// Strip markdown fences if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Extract JSON object from content (0.8b sometimes wraps in prose)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	var output RemediationOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		// 0.8b often produces newlines inside string values — repair and retry
		repaired := strings.ReplaceAll(content, "\n", " ")
		repaired = strings.ReplaceAll(repaired, "\t", " ")
		repaired = strings.ReplaceAll(repaired, "\r", "")
		repaired = regexp.MustCompile(`,\s*}`).ReplaceAllString(repaired, "}")
		repaired = regexp.MustCompile(`,\s*]`).ReplaceAllString(repaired, "]")
		if err2 := json.Unmarshal([]byte(repaired), &output); err2 != nil {
			return nil, fmt.Errorf("parse L1 JSON output: strict=%w repaired=%w (content: %s)", err, err2, content[:min(len(content), 200)])
		}
		log.Printf("[L1-REPAIR] %s: JSON repaired for parse (0.8b formatting)", packet.JiraKey)
	}

	// Normalize final_status
	switch strings.ToLower(output.FinalStatus) {
	case "success", "done", "completed":
		output.FinalStatus = "success"
	case "needs_review", "review", "uncertain":
		output.FinalStatus = "needs_review"
	case "blocked", "blocked_missing_info":
		output.FinalStatus = "blocked"
	case "to_escalate", "escalate", "escalation_required":
		output.FinalStatus = "to_escalate"
	default:
		output.FinalStatus = "needs_review"
	}

	return &output, nil
}

// executeRemediationViaL1WithTelemetry wraps executeRemediationViaL1 with per-task metrics collection.
// Returns the L1 output, timing info, and emits a telemetry record.
func executeRemediationViaL1WithTelemetry(endpoint, model string, packet RemediationPacket, timeoutSec int, runID, scheduleName string) (*RemediationOutput, error) {
	start := time.Now()
	promptChars := len(packet.TargetFiles) + len(packet.EvidencePaths) + len(packet.SuccessCriteria) + len(packet.ValidationCmds) + len(packet.Constraints)

	result, execErr := executeRemediationViaL1(endpoint, model, packet, timeoutSec)

	wallMs := time.Since(start).Milliseconds()

	// Build telemetry record
	builder := metrics.NewTaskRecord(runID, packet.JiraKey).
		JiraKey(packet.JiraKey).
		Schedule(scheduleName).
		Model(model).
		Lane("l1-local").
		Provider("llama-cpp").
		PromptSize(promptChars).
		Timing(start, wallMs).
		Attempt(1).
		TaskClass("remediation")

	if execErr != nil {
		// L1 call failed entirely (timeout, network, etc.)
		builder.CompletionClass(metrics.ClassTimeout).
			ProducedBy(metrics.ProducedByL1Failed).
			OutputSize(0)
	} else {
		outputChars := len(result.Explanation) + len(result.EditDescription)
		if result.Fields != nil {
			if b, err := json.Marshal(result.Fields); err == nil {
				outputChars += len(b)
			}
		}
		builder.OutputSize(outputChars).
			RemediationType(result.RemediationType).
			FinalStatus(result.FinalStatus)

		// Classify completion
		builder.CompletionClass(metrics.ClassifyCompletion(wallMs, outputChars, false, 0, 15))
		builder.ProducedBy(metrics.ClassifyProducedBy(outputChars, true, 0, 15, ""))
	}

	recordTelemetry(builder.Build())
	return result, execErr
}

// ─── Phase 3: Jira Outcome Update ─────────────────────────────────────

func updateJiraOutcome(jcfg jiraConfig, key string, result *RemediationOutput, epLink string) bool {
	if !jcfg.enabled {
		return false
	}

	// Build comment with remediation details
	comment := fmt.Sprintf("[zen-brain1 remediation] L1 completed remediation attempt.\n\n"+
		"Type: %s\n"+
		"File: %s\n"+
		"Description: %s\n"+
		"Explanation: %s\n"+
		"Status: %s\n"+
		"Validation: %s\n"+
		"Evidence Pack: %s",
		result.RemediationType, result.FileToEdit, result.EditDescription,
		result.Explanation, result.FinalStatus, result.ValidationResult, epLink)

	if result.BlockerReason != "" {
		comment += fmt.Sprintf("\nBlocker: %s", result.BlockerReason)
	}
	if result.ComplianceNote != "" {
		comment += fmt.Sprintf("\nCompliance: %s", result.ComplianceNote)
	}

	// Determine label based on outcome
	var labelsToAdd []string
	switch result.FinalStatus {
	case "success":
		labelsToAdd = []string{"ai:remediated"}
	case "needs_review":
		labelsToAdd = []string{"ai:needs-review"}
	case "blocked":
		labelsToAdd = []string{"ai:blocked"}
	case "to_escalate":
		labelsToAdd = []string{"ai:escalated"}
	}

	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{"type": "paragraph", "content": []map[string]string{{"type": "text", "text": comment}}},
			},
		},
	}

	// Add labels if Jira supports it (via update)
	if len(labelsToAdd) > 0 {
		payload["update"] = map[string]interface{}{
			"labels": []map[string]interface{}{
				{"add": labelsToAdd},
			},
		}
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		jcfg.url+"/rest/api/3/issue/"+key+"/comment",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[JIRA-UPDATE] comment failed for %s: %v", key, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[JIRA-UPDATE] comment returned %d for %s: %s", resp.StatusCode, key, string(body))
		return false
	}

	// Also add labels via PUT to issue
	if len(labelsToAdd) > 0 {
		addLabelsToIssue(jcfg, key, labelsToAdd)
	}

	return true
}

func addLabelsToIssue(jcfg jiraConfig, key string, labels []string) {
	// First get current labels
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		jcfg.url+"/rest/api/3/issue/"+key+"?fields=labels",
		nil)
	req.SetBasicAuth(jcfg.email, jcfg.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var issue struct {
		Fields struct {
			Labels []string `json:"labels"`
		} `json:"fields"`
	}
	json.NewDecoder(resp.Body).Decode(&issue)

	// Merge labels
	existing := make(map[string]bool)
	for _, l := range issue.Fields.Labels {
		existing[l] = true
	}
	var merged []string
	for _, l := range append(issue.Fields.Labels, labels...) {
		if !existing[l] || true { // include all
			merged = append(merged, l)
		}
	}

	updatePayload := map[string]interface{}{
		"fields": map[string]interface{}{
			"labels": merged,
		},
	}
	bodyBytes, _ := json.Marshal(updatePayload)
	ctx2, cancel2 := contextWithTimeout(10)
	defer cancel2()

	req2, _ := http.NewRequestWithContext(ctx2, "PUT",
		jcfg.url+"/rest/api/3/issue/"+key,
		strings.NewReader(string(bodyBytes)))
	req2.SetBasicAuth(jcfg.email, jcfg.token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		log.Printf("[JIRA-UPDATE] label update failed for %s: %v", key, err)
		return
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 204 {
		body, _ := io.ReadAll(resp2.Body)
		log.Printf("[JIRA-UPDATE] label update returned %d for %s: %s", resp2.StatusCode, key, string(body))
	}
}

// ─── Phase 4: Evidence Pack Skeleton ──────────────────────────────────

func createEvidencePack(cfg remediationConfig, ticket RemediationTicket, result *RemediationOutput) string {
	if cfg.EvidenceRoot == "" {
		return ""
	}

	// Create month-based directory
	now := time.Now()
	monthDir := fmt.Sprintf("evidence-%s", now.Format("2006-01"))
	packDir := filepath.Join(cfg.EvidenceRoot, monthDir, fmt.Sprintf("rem-%s", ticket.Key))
	manifestDir := filepath.Join(packDir, "manifest")
	reportsDir := filepath.Join(packDir, "reports")
	rejectionDir := filepath.Join(packDir, "rejection-log")

	for _, dir := range []string{packDir, manifestDir, reportsDir, rejectionDir} {
		os.MkdirAll(dir, 0755)
	}

	// Run ID for traceability
	runID := fmt.Sprintf("rem-%s-%s", ticket.Key, now.Format("20060102-150405"))

	// Create manifest
	manifest := EvidencePack{
		JiraKey:          ticket.Key,
		RelatedProject:   ticket.RelatedProject,
		ApprovalLevel:    ticket.ApprovalLevel,
		SREDCode:         ticket.SREDCode,
		IRAPWorkPkg:      ticket.IRAPWorkPkg,
		SourceReport:     ticket.SourceReport,
		CreatedAt:        now,
		RunID:            runID,
		RemediationResult: fmt.Sprintf("%s: %s", result.FinalStatus, result.EditDescription),
		EvidencePath:     packDir,
	}

	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(manifestDir, "manifest.json"), manifestJSON, 0644)

	// Create index
	index := fmt.Sprintf(`# Evidence Pack: %s

## Ticket
- **Key:** %s
- **Summary:** %s
- **Priority:** %s
- **Approval Level:** %d
- **Labels:** %s

## Remediation
- **Run ID:** %s
- **Type:** %s
- **File:** %s
- **Status:** %s
- **Explanation:** %s
- **Validation:** %s

## Compliance
- **SR&ED Code:** %s
- **IRAP Work Package:** %s
- **Related Project:** %s
- **Compliance Note:** %s

## Artifacts
- manifest/manifest.json
- reports/remediation-result.md
- rejection-log/ (if blocked/failed)
`,
		ticket.Key, ticket.Key, ticket.Summary, ticket.Priority,
		ticket.ApprovalLevel, strings.Join(ticket.Labels, ", "),
		runID, result.RemediationType, result.FileToEdit,
		result.FinalStatus, result.Explanation, result.ValidationResult,
		ticket.SREDCode, ticket.IRAPWorkPkg, ticket.RelatedProject,
		result.ComplianceNote,
	)
	os.WriteFile(filepath.Join(packDir, "index.md"), []byte(index), 0644)

	// Create remediation result report
	report := fmt.Sprintf(`# Remediation Result: %s

**Date:** %s
**Run ID:** %s
**Status:** %s

## Problem
%s

## What Was Done
- **Type:** %s
- **File:** %s
- **Description:** %s
- **Explanation:** %s

## Validation
%s

## Outcome
%s
`,
		ticket.Key, now.Format(time.RFC3339), runID, result.FinalStatus,
		ticket.Description, result.RemediationType, result.FileToEdit,
		result.EditDescription, result.Explanation,
		result.ValidationResult,
		result.Explanation,
	)
	os.WriteFile(filepath.Join(reportsDir, "remediation-result.md"), []byte(report), 0644)

	// If blocked or failed, create rejection note
	if result.FinalStatus == "blocked" || result.FinalStatus == "to_escalate" {
		rejection := fmt.Sprintf(`# Rejection/Blocker Note: %s

**Date:** %s
**Status:** %s

## Blocker Reason
%s

## What Was Attempted
%s

## Recommended Next Step
- If blocked: resolve the blocker and re-queue
- If escalated: human review needed
`,
			ticket.Key, now.Format(time.RFC3339), result.FinalStatus,
			result.BlockerReason, result.Explanation,
		)
		os.WriteFile(filepath.Join(rejectionDir, "blocker-note.md"), []byte(rejection), 0644)
	}

	return packDir
}

// JQL search result types
type jiraIssueSummary struct {
	Key    string `json:"key"`
	Fields struct {
		Summary  string          `json:"summary"`
		Priority jiraNameField   `json:"priority"`
		Labels   []string        `json:"labels"`
		Status   jiraNameField   `json:"status"`
		Created  string          `json:"created"`
	} `json:"fields"`
}

type jiraNameField struct {
	Name string `json:"name"`
}

type jiraSearchResult struct {
	Issues []jiraIssueSummary `json:"issues"`
}

// jiraIssueDetail has description for remediation packets
type jiraIssueDetail struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string                `json:"summary"`
		Description jiraDescriptionField  `json:"description"`
		Priority    jiraNameField         `json:"priority"`
		Labels      []string              `json:"labels"`
		Status      jiraNameField         `json:"status"`
	} `json:"fields"`
}

type jiraDescriptionField struct {
	Content []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"content"`
}

// ─── Phase 7: Compliance Status Report ────────────────────────────────

func generateComplianceReport(jcfg jiraConfig, evidenceRoot string, reportPath string) error {
	if !jcfg.enabled {
		return fmt.Errorf("Jira not configured")
	}

	// Fetch all ai:finding tickets with compliance-relevant data
	jql := fmt.Sprintf(`project = "%s" AND labels = ai:finding ORDER BY created DESC`, jcfg.project)
	payload := map[string]interface{}{
		"jql":        jql,
		"maxResults": 50,
		"fields":     []string{"summary", "priority", "labels", "status", "created"},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(15)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		jcfg.url+"/rest/api/3/search/jql",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Jira search %d: %s", resp.StatusCode, string(body))
	}

	var result jiraSearchResult
	json.NewDecoder(resp.Body).Decode(&result)

	// Check evidence packs
	var evidencePacks []string
	if evidenceRoot != "" {
		entries, _ := filepath.Glob(filepath.Join(evidenceRoot, "evidence-*", "rem-*", "index.md"))
		for _, e := range entries {
			evidencePacks = append(evidencePacks, e)
		}
	}

	// Build report
	var sb strings.Builder
	sb.WriteString("# Compliance / Grants Status\n\n")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total ai:finding tickets:** %d\n", len(result.Issues)))
	sb.WriteString(fmt.Sprintf("**Evidence packs:** %d\n\n", len(evidencePacks)))

	sb.WriteString("## Tickets\n\n")
	sb.WriteString("| Key | Priority | Status | Labels | SR&ED | IRAP |\n")
	sb.WriteString("|-----|----------|--------|--------|-------|------|\n")

	for _, issue := range result.Issues {
		sred := "—"
		irap := "—"
		for _, l := range issue.Fields.Labels {
			if strings.HasPrefix(l, "sred:") {
				sred = strings.TrimPrefix(l, "sred:")
			}
			if strings.HasPrefix(l, "irap:") {
				irap = strings.TrimPrefix(l, "irap:")
			}
		}
		labels := strings.Join(issue.Fields.Labels, ", ")
		if len(labels) > 40 {
			labels = labels[:40] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			issue.Key, issue.Fields.Priority.Name, issue.Fields.Status.Name,
			labels, sred, irap))
	}

	sb.WriteString("\n## Evidence Packs\n\n")
	if len(evidencePacks) == 0 {
		sb.WriteString("No evidence packs found.\n")
	} else {
		for _, ep := range evidencePacks {
			sb.WriteString(fmt.Sprintf("- %s\n", ep))
		}
	}

	sb.WriteString("\n## Missing Compliance Metadata\n\n")
	missing := 0
	for _, issue := range result.Issues {
		hasSRED := false
		hasIRAP := false
		hasEP := false
		for _, l := range issue.Fields.Labels {
			if strings.HasPrefix(l, "sred:") { hasSRED = true }
			if strings.HasPrefix(l, "irap:") { hasIRAP = true }
		}
		for _, ep := range evidencePacks {
			if strings.Contains(ep, issue.Key) { hasEP = true }
		}
		if !hasSRED || !hasIRAP || !hasEP {
			sb.WriteString(fmt.Sprintf("- %s: SR&ED=%v IRAP=%v EvidencePack=%v\n",
				issue.Key, hasSRED, hasIRAP, hasEP))
			missing++
		}
	}
	if missing == 0 {
		sb.WriteString("All tickets have compliance metadata.\n")
	}

	return os.WriteFile(reportPath, []byte(sb.String()), 0644)
}

// ─── Metrics Collector (global) ───────────────────────────────────────

var (
	metricsCollector *metrics.Collector
	metricsDir       = envOr("METRICS_DIR", metrics.DefaultMetricsDir)
)

func initMetrics() {
	var err error
	metricsCollector, err = metrics.NewCollector(metricsDir)
	if err != nil {
		log.Printf("[METRICS] Warning: failed to init metrics collector: %v (continuing without metrics)", err)
	}
}

func recordTelemetry(rec metrics.TaskTelemetryRecord) {
	if metricsCollector == nil {
		return
	}
	if err := metricsCollector.Record(rec); err != nil {
		log.Printf("[METRICS] Warning: failed to record telemetry: %v", err)
	}
}

func flushMetrics() {
	if metricsCollector != nil {
		metricsCollector.Close()
		// Compute and save summary from all records
		records, err := metrics.LoadRecordsFromDir(metricsDir)
		if err == nil && len(records) > 0 {
			_ = metrics.ComputeAndSave(metricsDir, records, "latest_run")
		}
	}
}

// ─── Main ─────────────────────────────────────────────────────────────

func main() {
	initMetrics()
	defer flushMetrics()

	jcfg := loadJiraConfig()

	mode := envOr("MODE", "remediate") // remediate, report, pilot

	if !jcfg.enabled {
		log.Printf("[REMEDIATION] Jira not configured — dry-run mode")
	}

	cfg := remediationConfig{
		L1Endpoint:    envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:       envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		JiraURL:       jcfg.url,
		JiraEmail:     jcfg.email,
		JiraToken:     jcfg.token,
		JiraProject:   jcfg.project,
		ArtifactRoot:  envOr("ARTIFACT_ROOT", "/var/lib/zen-brain1/runs"),
		EvidenceRoot:  envOr("EVIDENCE_ROOT", "/var/lib/zen-brain1/evidence"),
		RepoRoot:      envOr("REPO_ROOT", "/home/neves/zen/zen-brain1"),
		MaxTickets:    envIntOr("MAX_TICKETS", 3),
		TimeoutSec:    envIntOr("REMEDIATION_TIMEOUT", 120),
		SREDCode:      envOr("DEFAULT_SRED_CODE", ""),
		IRAPWorkPkg:   envOr("DEFAULT_IRAP_WORK_PKG", ""),
	}

	switch mode {
	case "report":
		log.Printf("[REMEDIATION] generating compliance/grants status report")
		reportPath := envOr("REPORT_PATH", "docs/05-OPERATIONS/evidence/compliance-grants-status.md")
		os.MkdirAll(filepath.Dir(reportPath), 0755)
		if err := generateComplianceReport(jcfg, cfg.EvidenceRoot, reportPath); err != nil {
			log.Fatalf("[REMEDIATION] report failed: %v", err)
		}
		log.Printf("[REMEDIATION] report written to %s", reportPath)
		return

	case "pilot":
		// Phase 5: Run specific tickets
		pilotKeys := strings.Split(envOr("PILOT_KEYS", "ZB-614,ZB-616,ZB-618"), ",")
		runPilot(cfg, jcfg, pilotKeys)
		return

	case "drain-backlog":
		// Backlog drain: close informational batch reports
		drainBatchReports(jcfg, cfg)
		return

	default:
		// Normal remediation cycle
		runRemediationCycle(cfg, jcfg)
	}
}

func runPilot(cfg remediationConfig, jcfg jiraConfig, pilotKeys []string) {
	log.Printf("[REMEDIATION-PILOT] processing %d pilot tickets: %v", len(pilotKeys), pilotKeys)

	os.MkdirAll(cfg.EvidenceRoot, 0755)
	os.MkdirAll(filepath.Join(cfg.EvidenceRoot, "..", "quality-gate-logs"), 0755)

	resultDir := terminalResultDir()
	os.MkdirAll(resultDir, 0755)

	// Track gate results across the pilot
	var gateResults []TicketQualityReport

	for _, key := range pilotKeys {
		key = strings.TrimSpace(key)
		log.Printf("[REMEDIATION-PILOT] %s: fetching ticket...", key)

		// Fetch the ticket details
		ticket, err := fetchSingleTicket(jcfg, key)
		if err != nil {
			log.Printf("[REMEDIATION-PILOT] %s: fetch failed: %v", key, err)
			// Write failed terminal result
			writeTerminalResult(resultDir, WorkerTerminalResult{
				JiraKey:       key,
				TerminalClass: TerminalFailed,
				QualityPassed: false,
				BlockerReason: fmt.Sprintf("fetch failed: %v", err),
			})
			continue
		}

		log.Printf("[REMEDIATION-PILOT] %s: %s (priority=%s, follow_up=%s)",
			key, ticket.Summary, ticket.Priority, ticket.FollowUpType)

		// Build remediation packet
		packet := buildRemediationPacket(*ticket, cfg.RepoRoot)

		// Execute via L1 (with telemetry)
		log.Printf("[REMEDIATION-PILOT] %s: dispatching to L1...", key)
		result, l1err := executeRemediationViaL1WithTelemetry(cfg.L1Endpoint, cfg.L1Model, packet, cfg.TimeoutSec, "pilot-"+time.Now().Format("20060102-150405"), "pilot")
		if l1err != nil {
			log.Printf("[REMEDIATION-PILOT] %s: L1 failed: %v", key, l1err)
			// Move to RETRYING — L1 call itself failed
			transitionJiraStatus(jcfg, key, "in_progress")
			transitionJiraStatus(jcfg, key, "retrying")
			writeTerminalResult(resultDir, WorkerTerminalResult{
				JiraKey:       key,
				TerminalClass: TerminalRetrying,
				QualityPassed: false,
				BlockerReason: fmt.Sprintf("L1 call failed: %v", l1err),
				JiraState:     "RETRYING",
			})
			continue
		}

		log.Printf("[REMEDIATION-PILOT] %s: L1 result: type=%s status=%s file=%s",
			key, result.RemediationType, result.FinalStatus, result.FileToEdit)

		// ── MANDATORY: Normalize L1 output ──
		normalizedPayload := buildNormalizedPayload(*ticket, result, packet)
		log.Printf("[NORMALIZE] %s: payload normalized (title=%q routing=%s)",
			key, normalizedPayload.Title, normalizedPayload.RoutingRecommendation)

		// ── MANDATORY: Quality gate ──
		gateReport := qualityGate(normalizedPayload)
		gateResults = append(gateResults, gateReport)

		log.Printf("[QUALITY-GATE] %s: score=%d/25 readiness=%s issues=%v",
			key, gateReport.Score.Total, gateReport.Readiness, gateReport.Issues)

		// Enforce hard gate: score < 15 => REJECTED
		if gateReport.Score.Total < 15 {
			log.Printf("[QUALITY-GATE] %s: REJECTED (score %d < 15). Writing blocked evidence only.", key, gateReport.Score.Total)
			epPath := createEvidencePack(cfg, *ticket, result)
			gateLogPath := writeGateLog(cfg, key, gateReport, "rejected")

			// PHASE A FIX: Move to In Progress first (if not already), then immediately to PAUSED
			transitionJiraStatus(jcfg, key, "in_progress")

			if jcfg.enabled {
				blockedResult := &RemediationOutput{
					RemediationType: result.RemediationType,
					FileToEdit:      result.FileToEdit,
					EditDescription: fmt.Sprintf("BLOCKED BY QUALITY GATE (score %d/25)", gateReport.Score.Total),
					Explanation:     result.Explanation,
					FinalStatus:     "blocked",
					BlockerReason:   fmt.Sprintf("Quality gate score %d/25 < 15. Missing: %s", gateReport.Score.Total, strings.Join(gateReport.Issues, ", ")),
				}
				updateJiraOutcome(jcfg, key, blockedResult, epPath)
				addLabelsToIssue(jcfg, key, []string{"quality:blocked-invalid-payload"})

				// PHASE A FIX: Explicit Jira comment for quality-gate rejection
				rejectComment := fmt.Sprintf("[zen-brain1 quality gate] REJECTED\n\n"+
					"Score: %d/25 (threshold: 15)\n"+
					"Gate result: %s\n"+
					"Reason(s): %s\n"+
					"Artifact/evidence: %s\n"+
					"Next action: Review evidence pack. If fixable, update ticket and re-queue.\n"+
					"Current Jira state: In Progress → PAUSED",
					gateReport.Score.Total, gateReport.Readiness,
					strings.Join(gateReport.Issues, ", "), epPath)
				postJiraComment(jcfg, key, rejectComment)
			}

			// PHASE A FIX: Immediately move to PAUSED — do NOT stay In Progress
			transitionJiraStatus(jcfg, key, "paused")

			// Write terminal result for factory-fill to consume
			writeTerminalResult(resultDir, WorkerTerminalResult{
				JiraKey:       key,
				TerminalClass: TerminalBlockedInvalidPayl,
				QualityScore:  gateReport.Score.Total,
				QualityPassed: false,
				L1Status:      result.FinalStatus,
				JiraState:     "PAUSED",
				EvidencePath:  epPath,
				BlockerReason: fmt.Sprintf("Quality gate score %d/25 < 15. Missing: %s", gateReport.Score.Total, strings.Join(gateReport.Issues, ", ")),
				Issues:        gateReport.Issues,
				GateLogPath:   gateLogPath,
			})
			continue
		}

		log.Printf("[QUALITY-GATE] %s: PASSED (score %d >= 15). Proceeding with %s.", key, gateReport.Score.Total, gateReport.Readiness)

		// Create evidence pack with gated result
		epPath := createEvidencePack(cfg, *ticket, result)
		gateLogPath := writeGateLog(cfg, key, gateReport, "passed")
		log.Printf("[REMEDIATION-PILOT] %s: evidence pack at %s", key, epPath)

		// Determine Jira labels based on readiness
		var qualityLabels []string
		switch gateReport.Readiness {
		case ReadyForExecution:
			qualityLabels = []string{"quality:ready-for-execution"}
		case ReadyWithReview:
			qualityLabels = []string{"quality:ready-with-review"}
		default:
			qualityLabels = []string{"quality:needs-review"}
		}

		// Determine terminal class from L1 final status
		tClass := classifyTerminalState(result.FinalStatus)

		// Update Jira with quality-gated normalized payload
		jiraFinalState := "Done"
		if jcfg.enabled {
			// Step 1: Move to In Progress
			transitionJiraStatus(jcfg, key, "in_progress")

			commentBody := buildJiraCommentBody(normalizedPayload, gateReport, epPath)
			if postJiraComment(jcfg, key, commentBody) {
				log.Printf("[REMEDIATION-PILOT] %s: Jira updated with quality-gated outcome", key)
			} else {
				log.Printf("[REMEDIATION-PILOT] %s: Jira comment failed", key)
			}
			allLabels := append([]string{"ai:remediated"}, qualityLabels...)
			addLabelsToIssue(jcfg, key, allLabels)

			// Step 2: Move to terminal state based on final status
			transitionToTerminal(jcfg, key, result.FinalStatus)
			jiraFinalState = jiraStatusName(result.FinalStatus)
		}

		// Write terminal result for factory-fill to consume
		writeTerminalResult(resultDir, WorkerTerminalResult{
			JiraKey:       key,
			TerminalClass: tClass,
			QualityScore:  gateReport.Score.Total,
			QualityPassed: true,
			L1Status:      result.FinalStatus,
			JiraState:     jiraFinalState,
			EvidencePath:  epPath,
			GateLogPath:   gateLogPath,
		})
	}

	// Write pilot summary with gate results
	writePilotSummary(cfg, pilotKeys, gateResults)
	log.Printf("[REMEDIATION-PILOT] === pilot complete ===")
}

// classifyTerminalState maps L1 FinalStatus to a TerminalClass.
func classifyTerminalState(finalStatus string) TerminalClass {
	switch strings.ToLower(finalStatus) {
	case "success":
		return TerminalDone
	case "needs_review":
		return TerminalNeedsReview
	case "blocked":
		return TerminalPaused
	case "retrying":
		return TerminalRetrying
	case "to_escalate":
		return TerminalToEscalate
	default:
		return TerminalNeedsReview
	}
}

func runRemediationCycle(cfg remediationConfig, jcfg jiraConfig) {
	log.Printf("[REMEDIATION] starting remediation cycle")

	resultDir := terminalResultDir()
	os.MkdirAll(resultDir, 0755)

	// Phase 2: Fetch and select tickets
	tickets, err := fetchOpenTickets(jcfg, 2) // max approval level 2
	if err != nil {
		log.Printf("[REMEDIATION] fetch failed: %v", err)
		return
	}

	log.Printf("[REMEDIATION] found %d open ai:finding tickets", len(tickets))

	selected := selectTickets(tickets, cfg.MaxTickets, 2)
	if len(selected) == 0 {
		log.Printf("[REMEDIATION] no tickets selected for remediation")
		return
	}

	log.Printf("[REMEDIATION] selected %d tickets for remediation", len(selected))

	os.MkdirAll(cfg.EvidenceRoot, 0755)
	os.MkdirAll(cfg.EvidenceRoot+"/../quality-gate-logs", 0755)

	passed, rejected, failed := 0, 0, 0

	for i := range selected {
		ticket := selected[i]
		log.Printf("[REMEDIATION] %s: processing (priority=%s)", ticket.Key, ticket.Priority)

		packet := buildRemediationPacket(ticket, cfg.RepoRoot)
		result, l1err := executeRemediationViaL1WithTelemetry(cfg.L1Endpoint, cfg.L1Model, packet, cfg.TimeoutSec, "cycle-"+time.Now().Format("20060102-150405"), "remediation-cycle")
		if l1err != nil {
			log.Printf("[REMEDIATION] %s: L1 failed: %v", ticket.Key, l1err)
			failed++
			// PHASE A FIX: L1 failed — move to RETRYING, don't leave in limbo
			transitionJiraStatus(jcfg, ticket.Key, "in_progress")
			transitionJiraStatus(jcfg, ticket.Key, "retrying")
			writeTerminalResult(resultDir, WorkerTerminalResult{
				JiraKey:       ticket.Key,
				TerminalClass: TerminalRetrying,
				QualityPassed: false,
				BlockerReason: fmt.Sprintf("L1 call failed: %v", l1err),
				JiraState:     "RETRYING",
			})
			continue
		}

		log.Printf("[REMEDIATION] %s: result=%s type=%s", ticket.Key, result.FinalStatus, result.RemediationType)

		// ── MANDATORY: Normalize + Quality gate (same as pilot) ──
		normalizedPayload := buildNormalizedPayload(ticket, result, packet)
		log.Printf("[NORMALIZE] %s: payload normalized (routing=%s)", ticket.Key, normalizedPayload.RoutingRecommendation)

		gateReport := qualityGate(normalizedPayload)
		log.Printf("[QUALITY-GATE] %s: score=%d/25 readiness=%s issues=%v",
			ticket.Key, gateReport.Score.Total, gateReport.Readiness, gateReport.Issues)

		epPath := createEvidencePack(cfg, ticket, result)
		gateLogPath := writeGateLog(cfg, ticket.Key, gateReport, "cycle")

		if gateReport.Score.Total < 15 {
			log.Printf("[QUALITY-GATE] %s: REJECTED (score %d < 15). Blocked.", ticket.Key, gateReport.Score.Total)
			rejected++

			// PHASE A FIX: Move to In Progress, then immediately to PAUSED
			transitionJiraStatus(jcfg, ticket.Key, "in_progress")

			if jcfg.enabled {
				blockedResult := &RemediationOutput{
					RemediationType: result.RemediationType,
					FileToEdit:      result.FileToEdit,
					EditDescription: fmt.Sprintf("BLOCKED BY QUALITY GATE (score %d/25): %s", gateReport.Score.Total, strings.Join(gateReport.Issues, ", ")),
					Explanation:     result.Explanation,
					FinalStatus:     "blocked",
					BlockerReason:   fmt.Sprintf("Quality gate score %d/25 below threshold 15", gateReport.Score.Total),
				}
				updateJiraOutcome(jcfg, ticket.Key, blockedResult, epPath)
				addLabelsToIssue(jcfg, ticket.Key, []string{"quality:blocked-invalid-payload"})

				// Explicit Jira comment for quality-gate rejection
				rejectComment := fmt.Sprintf("[zen-brain1 quality gate] REJECTED\n\n"+
					"Score: %d/25 (threshold: 15)\n"+
					"Gate result: %s\n"+
					"Reason(s): %s\n"+
					"Artifact/evidence: %s\n"+
					"Next action: Review evidence pack. If fixable, update ticket and re-queue.\n"+
					"Current Jira state: In Progress → PAUSED",
					gateReport.Score.Total, gateReport.Readiness,
					strings.Join(gateReport.Issues, ", "), epPath)
				postJiraComment(jcfg, ticket.Key, rejectComment)
			}

			// PHASE A FIX: Immediately move to PAUSED
			transitionJiraStatus(jcfg, ticket.Key, "paused")

			writeTerminalResult(resultDir, WorkerTerminalResult{
				JiraKey:       ticket.Key,
				TerminalClass: TerminalBlockedInvalidPayl,
				QualityScore:  gateReport.Score.Total,
				QualityPassed: false,
				L1Status:      result.FinalStatus,
				JiraState:     "PAUSED",
				EvidencePath:  epPath,
				BlockerReason: fmt.Sprintf("Quality gate score %d/25 < 15. Missing: %s", gateReport.Score.Total, strings.Join(gateReport.Issues, ", ")),
				Issues:        gateReport.Issues,
				GateLogPath:   gateLogPath,
			})
			continue
		}

		log.Printf("[QUALITY-GATE] %s: PASSED (%s, score %d).", ticket.Key, gateReport.Readiness, gateReport.Score.Total)
		passed++

		// Determine terminal class
		tClass := classifyTerminalState(result.FinalStatus)
		jiraFinalState := "Done"

		if jcfg.enabled {
			// Move to In Progress
			transitionJiraStatus(jcfg, ticket.Key, "in_progress")

			commentBody := buildJiraCommentBody(normalizedPayload, gateReport, epPath)
			if postJiraComment(jcfg, ticket.Key, commentBody) {
				log.Printf("[REMEDIATION] %s: Jira updated with gated outcome", ticket.Key)
			} else {
				failed++
			}
			var qualityLabels []string
			switch gateReport.Readiness {
			case ReadyForExecution:
				qualityLabels = []string{"quality:ready-for-execution"}
			case ReadyWithReview:
				qualityLabels = []string{"quality:ready-with-review"}
			default:
				qualityLabels = []string{"quality:needs-review"}
			}
			addLabelsToIssue(jcfg, ticket.Key, append([]string{"ai:remediated"}, qualityLabels...))

			// Move to terminal state
			transitionToTerminal(jcfg, ticket.Key, result.FinalStatus)
			jiraFinalState = jiraStatusName(result.FinalStatus)
		}

		writeTerminalResult(resultDir, WorkerTerminalResult{
			JiraKey:       ticket.Key,
			TerminalClass: tClass,
			QualityScore:  gateReport.Score.Total,
			QualityPassed: true,
			L1Status:      result.FinalStatus,
			JiraState:     jiraFinalState,
			EvidencePath:  epPath,
			GateLogPath:   gateLogPath,
		})
	}

	log.Printf("[REMEDIATION] === cycle complete: %d passed, %d rejected, %d failed ===", passed, rejected, failed)
}

// ─── Quality Gate Helpers ─────────────────────────────────────────────

// postJiraComment posts a plain-text comment to a Jira issue using Atlassian Document Format.
func postJiraComment(jcfg jiraConfig, key string, bodyText string) bool {
	if !jcfg.enabled {
		return false
	}

	// Build ADF comment payload
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"type":    "doc",
			"version": 1,
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]string{
						{"type": "text", "text": bodyText},
					},
				},
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(15)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		jcfg.url+"/rest/api/3/issue/"+key+"/comment",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(jcfg.email, jcfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[JIRA-COMMENT] failed for %s: %v", key, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[JIRA-COMMENT] returned %d for %s: %s", resp.StatusCode, key, string(body))
		return false
	}
	return true
}

// writeGateLog writes a quality gate decision to the gate log directory.
// ─── Jira State Machine ──────────────────────────────────────────────
// Transitions Jira issue status using the project's workflow.
// Transition IDs are global (same from any state) for the ZB project:
//   Backlog=11, Selected for Development=21, In Progress=31,
//   Done=41, PAUSED=51, RETRYING=61, TO_ESCALATE=71

// jiraStatusName maps internal final-status to Jira transition name.
func jiraStatusName(finalStatus string) string {
	switch strings.ToLower(finalStatus) {
	case "success", "done":
		return "Done"
	case "needs_review":
		return "Done" // moves to Done with review label
	case "paused":
		return "PAUSED"
	case "retrying":
		return "RETRYING"
	case "to_escalate":
		return "TO_ESCALATE"
	case "selected":
		return "Selected for Development"
	case "in_progress":
		return "In Progress"
	default:
		return ""
	}
}

// transitionJiraStatus moves a Jira issue to the specified status.
func transitionJiraStatus(jcfg jiraConfig, key string, targetStatus string) bool {
	if !jcfg.enabled {
		log.Printf("[TRANSITION] %s: skipped (Jira not configured)", key)
		return false
	}

	targetName := jiraStatusName(targetStatus)
	if targetName == "" {
		log.Printf("[TRANSITION] %s: unknown target status %q", key, targetStatus)
		return false
	}

	// Get available transitions
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		jcfg.url+"/rest/api/3/issue/"+key+"/transitions", nil)
	req.SetBasicAuth(jcfg.email, jcfg.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[TRANSITION] %s: get transitions failed: %v", key, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[TRANSITION] %s: get transitions returned %d: %s", key, resp.StatusCode, string(body))
		return false
	}

	var transResult struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&transResult); err != nil {
		log.Printf("[TRANSITION] %s: decode transitions failed: %v", key, err)
		return false
	}

	// Find matching transition
	var transitionID string
	for _, t := range transResult.Transitions {
		if strings.EqualFold(t.Name, targetName) {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		log.Printf("[TRANSITION] %s: no transition found for %q among %v", key, targetName,
			func() []string {
				var names []string
				for _, t := range transResult.Transitions {
					names = append(names, t.Name)
				}
				return names
			}())
		return false
	}

	// Execute transition
	payload := map[string]interface{}{
		"transition": map[string]interface{}{
			"id": transitionID,
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	ctx2, cancel2 := contextWithTimeout(10)
	defer cancel2()

	req2, _ := http.NewRequestWithContext(ctx2, "POST",
		jcfg.url+"/rest/api/3/issue/"+key+"/transitions",
		strings.NewReader(string(bodyBytes)))
	req2.SetBasicAuth(jcfg.email, jcfg.token)
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		log.Printf("[TRANSITION] %s: execute failed: %v", key, err)
		return false
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 204 && resp2.StatusCode != 200 {
		body, _ := io.ReadAll(resp2.Body)
		log.Printf("[TRANSITION] %s: execute returned %d: %s", key, resp2.StatusCode, string(body))
		return false
	}

	log.Printf("[TRANSITION] %s: moved to %s (transition ID %s)", key, targetName, transitionID)
	return true
}

// transitionToTerminal moves a ticket to its correct terminal state based on
// the remediation outcome. Called after all Jira updates (comment, labels) are done.
func transitionToTerminal(jcfg jiraConfig, key string, finalStatus string) bool {
	switch strings.ToLower(finalStatus) {
	case "success":
		return transitionJiraStatus(jcfg, key, "done")
	case "needs_review":
		// Needs review stays in-progress-like state — move to Done with review label
		return transitionJiraStatus(jcfg, key, "done")
	case "blocked":
		return transitionJiraStatus(jcfg, key, "paused")
	case "to_escalate":
		return transitionJiraStatus(jcfg, key, "to_escalate")
	default:
		log.Printf("[TRANSITION] %s: no terminal mapping for %q", key, finalStatus)
		return false
	}
}

// closeBatchReport moves an informational batch report ticket directly to Done.
func closeBatchReport(jcfg jiraConfig, key string) bool {
	return transitionJiraStatus(jcfg, key, "done")
}

// writeGateLog writes a quality gate decision to the gate log directory.
// Returns the path to the gate log file.
func writeGateLog(cfg remediationConfig, key string, report TicketQualityReport, decision string) string {
	logDir := filepath.Join(filepath.Dir(cfg.EvidenceRoot), "quality-gate-logs")
	os.MkdirAll(logDir, 0755)

	entry := map[string]interface{}{
		"jira_key":  key,
		"score":     report.Score,
		"readiness": string(report.Readiness),
		"decision":  decision,
		"issues":    report.Issues,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	path := filepath.Join(logDir, fmt.Sprintf("%s-%s.json", key, decision))
	os.WriteFile(path, data, 0644)
	log.Printf("[QUALITY-GATE-LOG] %s: written to %s", key, path)
	return path
}

// writePilotSummary writes a summary of all gate decisions in the pilot run.
func writePilotSummary(cfg remediationConfig, pilotKeys []string, gateResults []TicketQualityReport) {
	summaryDir := filepath.Join(cfg.EvidenceRoot, "pilot-summaries")
	os.MkdirAll(summaryDir, 0755)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Quality Gate Pilot Summary\n"))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Tickets:** %d\n\n", len(gateResults)))

	sb.WriteString("| Key | Score | Readiness | Decision | Issues |\n")
	sb.WriteString("|-----|-------|-----------|----------|--------|\n")

	for _, gr := range gateResults {
		decision := "rejected"
		if gr.Score.Total >= 15 {
			decision = "passed"
		}
		issues := strings.Join(gr.Issues, ", ")
		if issues == "" {
			issues = "none"
		}
		sb.WriteString(fmt.Sprintf("| %s | %d/25 | %s | %s | %s |\n",
			gr.JiraKey, gr.Score.Total, gr.Readiness, decision, issues))
	}

	sb.WriteString("\n## Score Breakdown\n\n")
	for _, gr := range gateResults {
		sb.WriteString(fmt.Sprintf("### %s (%s, %d/25)\n", gr.JiraKey, gr.Readiness, gr.Score.Total))
		sb.WriteString(fmt.Sprintf("- Clarity: %d/5\n", gr.Score.Clarity))
		sb.WriteString(fmt.Sprintf("- Evidence Quality: %d/5\n", gr.Score.EvidenceQuality))
		sb.WriteString(fmt.Sprintf("- Boundedness: %d/5\n", gr.Score.Boundedness))
		sb.WriteString(fmt.Sprintf("- Validation Clarity: %d/5\n", gr.Score.ValidationClarity))
		sb.WriteString(fmt.Sprintf("- Governance Completion: %d/5\n\n", gr.Score.GovernanceCompletion))
	}

	path := filepath.Join(summaryDir, fmt.Sprintf("pilot-%s.md", time.Now().Format("20060102-150405")))
	os.WriteFile(path, []byte(sb.String()), 0644)
	log.Printf("[PILOT-SUMMARY] written to %s", path)
}

// fetchSingleTicket gets details for one specific Jira ticket.
func fetchSingleTicket(jcfg jiraConfig, key string) (*RemediationTicket, error) {
	if !jcfg.enabled {
		return nil, fmt.Errorf("Jira not configured")
	}

	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		jcfg.url+"/rest/api/3/issue/"+key+"?fields=summary,description,priority,labels,status,comment",
		nil)
	req.SetBasicAuth(jcfg.email, jcfg.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jira returned %d: %s", resp.StatusCode, string(body))
	}

	var issue jiraIssueDetail

	json.NewDecoder(resp.Body).Decode(&issue)

	desc := ""
	for _, content := range issue.Fields.Description.Content {
		for _, text := range content.Content {
			desc += text.Text + " "
		}
	}

	return &RemediationTicket{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: strings.TrimSpace(desc),
		Priority:    issue.Fields.Priority.Name,
		Labels:      issue.Fields.Labels,
		Status:      issue.Fields.Status.Name,
	}, nil
}

// ─── Backlog Drain ──────────────────────────────────────────────────

// drainBatchReports closes informational batch report tickets that are
// stuck in Backlog. These are telemetry artifacts labeled ai:completed
// that were never transitioned to Done.
func drainBatchReports(jcfg jiraConfig, cfg remediationConfig) {
	if !jcfg.enabled {
		log.Printf("[DRAIN] Jira not configured, cannot drain")
		return
	}

	batchLabels := []string{"daily-sweep", "hourly-scan", "quad-hourly-summary"}
	maxPerLabel := envIntOr("DRAIN_MAX_PER_LABEL", 200)
	dryRun := os.Getenv("DRY_RUN") != ""

	if dryRun {
		log.Printf("[DRAIN] DRY RUN — no transitions will execute")
	}

	totalClosed := 0
	totalFailed := 0

	for _, label := range batchLabels {
		// Search for Backlog tickets with this label
		jql := fmt.Sprintf(`project = "%s" AND status = "Backlog" AND labels = "%s" ORDER BY key ASC`, jcfg.project, label)

		// Paginated search
		nextToken := ""
		page := 0
		closed := 0

		for closed < maxPerLabel {
			page++
			body := map[string]interface{}{
				"jql":        jql,
				"maxResults": 100,
				"fields":     []string{"summary", "labels"},
			}
			if nextToken != "" {
				body["nextPageToken"] = nextToken
			}

			bodyBytes, _ := json.Marshal(body)
			ctx, cancel := contextWithTimeout(30)
			req, _ := http.NewRequestWithContext(ctx, "POST",
				jcfg.url+"/rest/api/3/search/jql",
				strings.NewReader(string(bodyBytes)))
			req.SetBasicAuth(jcfg.email, jcfg.token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			cancel()
			if err != nil {
				log.Printf("[DRAIN] search failed for %s: %v", label, err)
				break
			}

			var searchResult struct {
				Issues []struct {
					Key    string `json:"key"`
					Fields struct {
						Summary string   `json:"summary"`
						Labels  []string `json:"labels"`
					} `json:"fields"`
				} `json:"issues"`
				IsLast        bool   `json:"isLast"`
				NextPageToken string `json:"nextPageToken"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
				resp.Body.Close()
				log.Printf("[DRAIN] decode failed for %s: %v", label, err)
				break
			}
			resp.Body.Close()

			if len(searchResult.Issues) == 0 {
				break
			}

			for _, issue := range searchResult.Issues {
				if closed >= maxPerLabel {
					break
				}
				if dryRun {
					log.Printf("[DRAIN] DRY RUN: would close %s: %s", issue.Key, issue.Fields.Summary[:min(60, len(issue.Fields.Summary))])
					closed++
					continue
				}

				if closeBatchReport(jcfg, issue.Key) {
					closed++
					totalClosed++
					log.Printf("[DRAIN] closed %s (%s): %d/%d for %s",
						issue.Key, label, closed, maxPerLabel, label)
				} else {
					totalFailed++
					log.Printf("[DRAIN] failed to close %s", issue.Key)
				}

				// Rate limit: 1 transition per second
				time.Sleep(1 * time.Second)
			}

			if searchResult.IsLast {
				break
			}
			nextToken = searchResult.NextPageToken
			if nextToken == "" {
				break
			}
		}

		log.Printf("[DRAIN] %s: closed %d tickets", label, closed)
	}

	log.Printf("[DRAIN] === complete: %d closed, %d failed ===", totalClosed, totalFailed)

	// Write drain report
	reportDir := filepath.Join(cfg.EvidenceRoot, "drain-reports")
	os.MkdirAll(reportDir, 0755)

	report := fmt.Sprintf("# Backlog Drain Report\n\n**Date:** %s\n**Mode:** %s\n\n"+
		"- Total closed: %d\n- Total failed: %d\n- Labels processed: %v\n",
		time.Now().Format(time.RFC3339),
		func() string {
			if dryRun {
				return "dry-run"
			}
			return "live"
		}(),
		totalClosed, totalFailed, batchLabels)

	reportPath := filepath.Join(reportDir, fmt.Sprintf("drain-%s.md", time.Now().Format("20060102-150405")))
	os.WriteFile(reportPath, []byte(report), 0644)
	log.Printf("[DRAIN] report written to %s", reportPath)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
