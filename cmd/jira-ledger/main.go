package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// jira-ledger: links useful-task batch runs to Jira as the primary work ledger.
//
// Reads a completed batch's telemetry/batch-index.json and final/ artifacts,
// extracts findings, creates Jira parent + child issues for traceability.
//
// ENV VARS:
//   JIRA_URL      — Jira base URL
//   JIRA_EMAIL    — Jira account email
//   JIRA_TOKEN    — Jira API token
//   JIRA_PROJECT  — Jira project key (default: ZB)
//   MAX_FINDINGS  — max child issues per run (default: 5)
//   DRY_RUN       — print what would be created without calling Jira (default: false)

const defaultProject = "ZB"

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	jiraURL := os.Getenv("JIRA_URL")
	jiraEmail := os.Getenv("JIRA_EMAIL")
	jiraToken := os.Getenv("JIRA_TOKEN")
	jiraProject := envOr("JIRA_PROJECT", defaultProject)
	maxFindings := envInt("MAX_FINDINGS", 5)
	dryRun := os.Getenv("DRY_RUN") != ""

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <run-dir>\n  e.g.: %s /var/lib/zen-brain1/runs/daily-sweep/20260326-174812\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}
	runDir := os.Args[1]

	if dryRun {
		log.Println("[LEDGER] DRY RUN MODE — no Jira calls will be made")
	}

	// Validate inputs
	if !dryRun && (jiraURL == "" || jiraEmail == "" || jiraToken == "") {
		log.Fatalf("[LEDGER] Jira credentials required. Set JIRA_URL, JIRA_EMAIL, JIRA_TOKEN (or DRY_RUN=1)")
	}

	// Load batch telemetry
	telemetryPath := filepath.Join(runDir, "telemetry", "batch-index.json")
	telemetry, err := loadTelemetry(telemetryPath)
	if err != nil {
		log.Fatalf("[LEDGER] Failed to load telemetry: %v", err)
	}

	// Load artifacts
	finalDir := filepath.Join(runDir, "final")
	artifacts, err := loadArtifacts(finalDir)
	if err != nil {
		log.Fatalf("[LEDGER] Failed to load artifacts: %v", err)
	}

	log.Printf("[LEDGER] Run: %s (%s), %d/%d tasks, %d artifacts",
		telemetry.BatchName, telemetry.BatchID,
		telemetry.Succeeded, telemetry.Total, len(artifacts))

	// Extract findings from artifacts
	findings := extractFindings(artifacts, maxFindings)
	log.Printf("[LEDGER] Extracted %d actionable findings", len(findings))

	// Build parent issue
	parentSummary := fmt.Sprintf("[zen-brain] %s — %s",
		telemetry.BatchName, time.Now().Format("2006-01-02"))
	parentBody := buildParentBody(telemetry, artifacts, findings)

	// Build child issues
	children := make([]ChildIssue, len(findings))
	for i, f := range findings {
		children[i] = ChildIssue{
			Summary: fmt.Sprintf("[%s] %s: %s", f.Type, f.Path, f.Description),
			Body:    buildChildBody(f),
			Priority: severityToPriority(f.Severity),
			Labels:  []string{"zen-brain", "finding", f.Type},
		}
	}

	if dryRun {
		log.Println("[LEDGER] === DRY RUN: Would create ===")
		log.Printf("[LEDGER] Parent: %s", parentSummary)
		log.Printf("[LEDGER]   Project: %s, Labels: zen-brain, discovery, %s", jiraProject, telemetry.BatchName)
		log.Printf("[LEDGER]   Body preview:\n%s", truncate(parentBody, 500))
		for _, c := range children {
			log.Printf("[LEDGER]   Child: %s (priority=%s)", c.Summary, c.Priority)
		}
		log.Println("[LEDGER] === DRY RUN END ===")
		return
	}

	// Create parent issue in Jira
	parentKey, err := createJiraIssue(jiraURL, jiraEmail, jiraToken, jiraProject, parentSummary, parentBody,
		[]string{"zen-brain", "discovery", telemetry.BatchName})
	if err != nil {
		log.Fatalf("[LEDGER] Failed to create parent issue: %v", err)
	}
	log.Printf("[LEDGER] ✅ Parent issue created: %s", parentKey)

	// Create child issues
	var childKeys []string
	for _, c := range children {
		key, err := createJiraIssue(jiraURL, jiraEmail, jiraToken, jiraProject, c.Summary, c.Body,
			c.Labels, c.Priority)
		if err != nil {
			log.Printf("[LEDGER] ⚠️  Child issue failed: %v", err)
			continue
		}
		childKeys = append(childKeys, key)
		log.Printf("[LEDGER] ✅ Child issue: %s — %s", key, c.Summary)
	}

	// Link children to parent via comment (simpler than sub-task conversion)
	for _, childKey := range childKeys {
		linkComment(jiraURL, jiraEmail, jiraToken, childKey,
			fmt.Sprintf("Parent run: %s\nRun ID: %s\nArtifacts: %s/final/",
				parentKey, telemetry.BatchID, runDir))
	}

	// Write jira-mapping.json
	mapping := JiraMapping{
		BatchID:     telemetry.BatchID,
		BatchName:   telemetry.BatchName,
		RunDir:      runDir,
		ParentKey:   parentKey,
		ChildKeys:   childKeys,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		TotalTasks:  telemetry.Total,
		Succeeded:   telemetry.Succeeded,
	}
	writeMapping(runDir, mapping)
	log.Printf("[LEDGER] Mapping written to %s/jira-mapping.json", runDir)
}

// --- Data types ---

type BatchTelemetry struct {
	BatchID   string `json:"batch_id"`
	BatchName string `json:"batch_name"`
	Total     int    `json:"total"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	WallMs    int64  `json:"wall_ms"`
	Lane      string `json:"lane"`
}

type Artifact struct {
	Name    string
	Path    string
	Content string
}

type Finding struct {
	Type        string // defect, dead-code, tech-debt, etc.
	Path        string // file or package reference
	Description string
	Severity    string // critical, high, medium, low
}

type ChildIssue struct {
	Summary  string
	Body     string
	Priority string
	Labels   []string
}

type JiraMapping struct {
	BatchID   string   `json:"batch_id"`
	BatchName string   `json:"batch_name"`
	RunDir    string   `json:"run_dir"`
	ParentKey string   `json:"parent_key"`
	ChildKeys []string `json:"child_keys"`
	CreatedAt string   `json:"created_at"`
	TotalTasks int    `json:"total_tasks"`
	Succeeded  int    `json:"succeeded"`
}

// --- Functions ---

func loadTelemetry(path string) (*BatchTelemetry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t BatchTelemetry
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func loadArtifacts(dir string) ([]Artifact, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var artifacts []Artifact
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		artifacts = append(artifacts, Artifact{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			Content: string(data),
		})
	}
	return artifacts, nil
}

func extractFindings(artifacts []Artifact, maxFindings int) []Finding {
	var findings []Finding
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}

	for _, a := range artifacts {
		findingType := artifactToType(a.Name)
		lines := strings.Split(a.Content, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || len(line) > 200 {
				continue
			}

			// Look for severity markers
			sev := "medium"
			if strings.Contains(strings.ToLower(line), "critical") {
				sev = "critical"
			} else if strings.Contains(strings.ToLower(line), "high") {
				sev = "high"
			} else if strings.Contains(strings.ToLower(line), "low") {
				sev = "low"
			}

			// Skip if no actionable content
			if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") ||
				strings.HasPrefix(line, "|") || strings.HasPrefix(line, "```") ||
				strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [ ]") ||
				len(line) < 20 {
				continue
			}

			findings = append(findings, Finding{
				Type:        findingType,
				Path:        a.Name,
				Description: truncate(line, 120),
				Severity:    sev,
			})

			if len(findings) >= maxFindings {
				return findings
			}
		}
	}

	// Sort by severity
	slicesSortBySeverity(findings, severityOrder)
	if len(findings) > maxFindings {
		findings = findings[:maxFindings]
	}
	return findings
}

func artifactToType(name string) string {
	name = strings.TrimSuffix(name, ".md")
	name = strings.ReplaceAll(name, "-", "_")
	// Map common artifact names
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
	case strings.Contains(name, "package") || strings.Contains(name, "hotspot"):
		return "package-hotspot"
	case strings.Contains(name, "roadmap"):
		return "roadmap"
	case strings.Contains(name, "executive") || strings.Contains(name, "summary"):
		return "executive-summary"
	default:
		return "finding"
	}
}

func severityToPriority(sev string) string {
	switch strings.ToLower(sev) {
	case "critical":
		return "Highest"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Medium"
	}
}

func buildParentBody(t *BatchTelemetry, artifacts []Artifact, findings []Finding) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("h2. zen-brain Scheduled Discovery Run\n\n"))
	b.WriteString(fmt.Sprintf("*Schedule:* %s\n", t.BatchName))
	b.WriteString(fmt.Sprintf("*Run ID:* %s\n", t.BatchID))
	b.WriteString(fmt.Sprintf("*Model Lane:* %s\n", t.Lane))
	b.WriteString(fmt.Sprintf("*Results:* %d/%d tasks succeeded (%d failed)\n", t.Succeeded, t.Total, t.Failed))
	b.WriteString(fmt.Sprintf("*Wall Time:* %v\n", time.Duration(t.WallMs)*time.Millisecond))
	b.WriteString(fmt.Sprintf("*Artifacts:* %d reports produced\n\n", len(artifacts)))

	b.WriteString("h3. Artifacts\n\n")
	for _, a := range artifacts {
		b.WriteString(fmt.Sprintf("* %s\n", a.Name))
	}

	if len(findings) > 0 {
		b.WriteString("\nh3. Top Findings\n\n")
		for i, f := range findings {
			b.WriteString(fmt.Sprintf("%d. *[%s]* %s — %s\n", i+1, f.Severity, f.Type, truncate(f.Description, 100)))
		}
	}

	return b.String()
}

func buildChildBody(f Finding) string {
	return fmt.Sprintf("h2. Finding: %s\n\n*Source:* %s\n*Type:* %s\n*Severity:* %s\n\n%s\n\nh3. Recommended Action\n\nReview and address this finding from the scheduled discovery run.",
		f.Description, f.Path, f.Type, f.Severity, f.Description)
}

func createJiraIssue(url, email, token, project, summary, body string, labels []string, priority ...string) (string, error) {
	pri := "Medium"
	if len(priority) > 0 {
		pri = priority[0]
	}

	// Convert ADF-like body to plain text for simplicity
	plainBody := strings.ReplaceAll(body, "h2. ", "## ")
	plainBody = strings.ReplaceAll(plainBody, "h3. ", "### ")
	plainBody = strings.ReplaceAll(plainBody, "* ", "- ")

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":   map[string]string{"key": project},
			"summary":   summary,
			"description": map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []map[string]interface{}{
					{"type": "paragraph", "content": []map[string]interface{}{
						{"type": "text", "text": plainBody},
					}},
				},
			},
			"issuetype": map[string]string{"name": "Task"},
			"priority":  map[string]string{"name": pri},
			"labels":    labels,
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url+"/rest/api/3/issue", bytes.NewReader(bodyBytes))
	req.SetBasicAuth(email, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("Jira API %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	json.Unmarshal(respBody, &result)
	return result.Key, nil
}

func linkComment(url, email, token, issueKey, comment string) {
	payload := map[string]string{"body": comment}
	bodyBytes, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/rest/api/3/issue/%s/comment", url, issueKey), bytes.NewReader(bodyBytes))
	req.SetBasicAuth(email, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[LEDGER] Warning: comment on %s failed: %v", issueKey, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		log.Printf("[LEDGER] Warning: comment on %s returned %d", issueKey, resp.StatusCode)
	}
}

func writeMapping(runDir string, m JiraMapping) {
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(runDir, "jira-mapping.json"), data, 0644)
}

// --- Helpers ---

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n := 0
		for _, c := range v {
			if c < '0' || c > '9' { return fallback }
			n = n*10 + int(c-'0')
		}
		return n
	}
	return fallback
}
func truncate(s string, max int) string {
	if len(s) <= max { return s }
	return s[:max] + "..."
}
func slicesSortBySeverity(f []Finding, order map[string]int) {
	for i := 0; i < len(f)-1; i++ {
		for j := i + 1; j < len(f); j++ {
			if order[f[i].Severity] > order[f[j].Severity] {
				f[i], f[j] = f[j], f[i]
			}
		}
	}
}
