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
	EditDescription string `json:"edit_description"`
	NewContent      string `json:"new_content,omitempty"`
	ConfigChanges   string `json:"config_changes,omitempty"`
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
		jcfg.url+"/rest/api/3/search",
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
func buildRemediationPacket(ticket RemediationTicket, repoRoot string) RemediationPacket {
	// Extract target files from ticket description if available
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
	systemPrompt := `You are a remediation worker. You receive a bounded task from a Jira ticket and must attempt a specific fix.

RULES:
- Produce ONLY a bounded edit to the target files identified
- Do NOT invent new architecture or change unrelated files
- Do NOT do a repo-wide refactor
- If the problem is unclear or the target files cannot be identified, return final_status: blocked
- Explain what you changed and why

OUTPUT: You must produce ONLY valid JSON with exactly these fields:
{
  "remediation_type": "code_edit|config_change|doc_update|cannot_fix",
  "file_to_edit": "path/to/file relative to repo root",
  "edit_description": "what you changed and why",
  "new_content": "the complete new content of the file (if code_edit), or null",
  "config_changes": "description of config changes, or null",
  "explanation": "what was wrong and what you fixed",
  "final_status": "success|needs_review|blocked|to_escalate",
  "blocker_reason": "why blocked, if applicable, or null",
  "validation_result": "what validation would pass, or null",
  "compliance_note": "any SR&ED/IRAP-relevant notes, or null"
}

Do NOT include markdown, commentary, or anything outside the JSON object.`

	userPrompt := fmt.Sprintf(`## Remediation Task for %s

### Problem
%s

### Target Files
%s

### Evidence Paths
%s

### Success Criteria
%s

### Validation Commands
%s

### Constraints
%s

Produce your remediation output as JSON only.`, packet.JiraKey, packet.ProblemSummary, packet.TargetFiles, packet.EvidencePaths, packet.SuccessCriteria, packet.ValidationCmds, packet.Constraints)

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":      0.3,
		"max_tokens":       2048,
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

	var output RemediationOutput
	if err := json.Unmarshal([]byte(content), &output); err != nil {
		return nil, fmt.Errorf("parse L1 JSON output: %w (content: %s)", err, content[:min(len(content), 200)])
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
		jcfg.url+"/rest/api/3/search",
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

// ─── Main ─────────────────────────────────────────────────────────────

func main() {
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

	default:
		// Normal remediation cycle
		runRemediationCycle(cfg, jcfg)
	}
}

func runPilot(cfg remediationConfig, jcfg jiraConfig, pilotKeys []string) {
	log.Printf("[REMEDIATION-PILOT] processing %d pilot tickets: %v", len(pilotKeys), pilotKeys)

	os.MkdirAll(cfg.EvidenceRoot, 0755)

	for _, key := range pilotKeys {
		key = strings.TrimSpace(key)
		log.Printf("[REMEDIATION-PILOT] %s: fetching ticket...", key)

		// Fetch the ticket details
		ticket, err := fetchSingleTicket(jcfg, key)
		if err != nil {
			log.Printf("[REMEDIATION-PILOT] %s: fetch failed: %v", key, err)
			continue
		}

		log.Printf("[REMEDIATION-PILOT] %s: %s (priority=%s, follow_up=%s)",
			key, ticket.Summary, ticket.Priority, ticket.FollowUpType)

		// Build remediation packet
		packet := buildRemediationPacket(*ticket, cfg.RepoRoot)

		// Execute via L1
		log.Printf("[REMEDIATION-PILOT] %s: dispatching to L1...", key)
		result, err := executeRemediationViaL1(cfg.L1Endpoint, cfg.L1Model, packet, cfg.TimeoutSec)
		if err != nil {
			log.Printf("[REMEDIATION-PILOT] %s: L1 failed: %v", key, err)
			continue
		}

		log.Printf("[REMEDIATION-PILOT] %s: L1 result: type=%s status=%s file=%s",
			key, result.RemediationType, result.FinalStatus, result.FileToEdit)

		// Create evidence pack
		epPath := createEvidencePack(cfg, *ticket, result)
		log.Printf("[REMEDIATION-PILOT] %s: evidence pack at %s", key, epPath)

		// Update Jira
		if jcfg.enabled {
			if updateJiraOutcome(jcfg, key, result, epPath) {
				log.Printf("[REMEDIATION-PILOT] %s: Jira updated with remediation outcome", key)
			} else {
				log.Printf("[REMEDIATION-PILOT] %s: Jira update failed", key)
			}
		}
	}

	log.Printf("[REMEDIATION-PILOT] === pilot complete ===")
}

func runRemediationCycle(cfg remediationConfig, jcfg jiraConfig) {
	log.Printf("[REMEDIATION] starting remediation cycle")

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

	created, updated, failed := 0, 0, 0

	for i := range selected {
		ticket := selected[i]
		log.Printf("[REMEDIATION] %s: processing (priority=%s)", ticket.Key, ticket.Priority)

		packet := buildRemediationPacket(ticket, cfg.RepoRoot)
		result, err := executeRemediationViaL1(cfg.L1Endpoint, cfg.L1Model, packet, cfg.TimeoutSec)
		if err != nil {
			log.Printf("[REMEDIATION] %s: L1 failed: %v", ticket.Key, err)
			failed++
			continue
		}

		log.Printf("[REMEDIATION] %s: result=%s type=%s", ticket.Key, result.FinalStatus, result.RemediationType)

		epPath := createEvidencePack(cfg, ticket, result)

		if jcfg.enabled {
			if updateJiraOutcome(jcfg, ticket.Key, result, epPath) {
				updated++
			} else {
				failed++
			}
		} else {
			created++
		}
	}

	log.Printf("[REMEDIATION] === cycle complete: %d updated, %d failed ===", updated, failed)
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
