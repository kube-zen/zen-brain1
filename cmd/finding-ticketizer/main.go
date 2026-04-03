package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/secrets"
)

// ─── Data Types ────────────────────────────────────────────────────────

// ParsedFinding represents one finding extracted from a discovery artifact.
type ParsedFinding struct {
	Index        int    `json:"index"`
	File         string `json:"file"`
	Category     string `json:"category"`
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	WhyItMatters string `json:"why_it_matters"`
	SourceReport string `json:"source_report"`
	SourceType   string `json:"source_type"` // defects, bug_hunting, stub_hunting, etc.
}

// FindingFingerprint is a dedup key computed from finding attributes.
type FindingFingerprint struct {
	Fingerprint  string `json:"fingerprint"`
	Type         string `json:"type"`      // category
	File         string `json:"file"`      // normalized file path
	Signature    string `json:"signature"` // short description hash
	FirstSeen    string `json:"first_seen"`
	LastSeen     string `json:"last_seen"`
	LinkedJira   string `json:"linked_jira"` // empty if no ticket yet
	Status       string `json:"status"`      // new, triaged, ticketed, remediated
	SourceReport string `json:"source_report"`
}

// TicketizationRequest is what L1 receives for one finding.
type TicketizationRequest struct {
	FindingID     string `json:"finding_id"`
	Type          string `json:"type"`
	PriorityHint  string `json:"priority_hint"`
	FilePath      string `json:"file_path"`
	Evidence      string `json:"evidence"`
	WhyItMatters  string `json:"why_it_matters"`
	SourceReport  string `json:"source_report"`
	ExistingMatch string `json:"existing_match"` // Jira key if dedup match found
}

// TicketizationOutput is what L1 produces.
type TicketizationOutput struct {
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Problem       string `json:"problem"`
	Evidence      string `json:"evidence"`
	Impact        string `json:"impact"`
	FixDirection  string `json:"fix_direction"`
	Labels        string `json:"labels"`
	Priority      string `json:"priority"`
	DedupDecision string `json:"dedup_decision"` // create_new, update_existing, ignore_noise
	ExistingKey   string `json:"existing_key"`
	FollowUpType  string `json:"follow_up_type"` // no_followup, bounded_fix_l1, manual_review
}

// Config for the ticketizer.
type Config struct {
	L1Endpoint    string
	L1Model       string
	JiraURL       string
	JiraEmail     string
	JiraToken     string
	JiraProject   string
	ArtifactRoot  string
	LedgerPath    string
	MaxFindings   int      // per source class
	SourceClasses []string // which discovery classes to ticketize
	TimeoutSec    int
	Mode          string // "findings" or "roadmap"
}

// ─── Finding Parser ───────────────────────────────────────────────────

// parseFindingsFromArtifact reads a markdown artifact and extracts table-row findings.
// Supports both | # | File | Category | Severity | Description | and
//
//	| # | File | Category | Risk | Why It Matters | formats.
func parseFindingsFromArtifact(path string, sourceType string) ([]ParsedFinding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var findings []ParsedFinding
	scanner := bufio.NewScanner(f)
	inTable := false
	headerSeen := false
	idx := 0

	tableRowRe := regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|`)
	// For 6-column tables (with Why It Matters)
	tableRow6Re := regexp.MustCompile(`^\|\s*(\d+)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|`)
	separatorRe := regexp.MustCompile(`^\|[-|]+\|$`)

	for scanner.Scan() {
		line := scanner.Text()

		// Detect table start
		if strings.HasPrefix(line, "|") && strings.Contains(line, "#") && !inTable {
			inTable = true
			headerSeen = false
			continue
		}

		if inTable && separatorRe.MatchString(line) {
			headerSeen = true
			continue
		}

		if inTable && !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}

		if !inTable || !headerSeen {
			continue
		}

		// Try 6-column first (File | Category | Severity | Why It Matters | extra)
		if m := tableRow6Re.FindStringSubmatch(line); m != nil {
			num, _ := strconv.Atoi(m[1])
			if num != idx+1 {
				continue
			}
			idx++
			findings = append(findings, ParsedFinding{
				Index:        idx,
				File:         cleanCell(m[2]),
				Category:     cleanCell(m[3]),
				Severity:     cleanCell(m[4]),
				Description:  cleanCell(m[5]),
				WhyItMatters: cleanCell(m[6]),
				SourceReport: filepath.Base(path),
				SourceType:   sourceType,
			})
			continue
		}

		// Try 5-column (File | Category | Severity | Description)
		if m := tableRowRe.FindStringSubmatch(line); m != nil {
			num, _ := strconv.Atoi(m[1])
			if num != idx+1 {
				continue
			}
			idx++
			findings = append(findings, ParsedFinding{
				Index:        idx,
				File:         cleanCell(m[2]),
				Category:     cleanCell(m[3]),
				Severity:     cleanCell(m[4]),
				Description:  cleanCell(m[5]),
				SourceReport: filepath.Base(path),
				SourceType:   sourceType,
			})
		}
	}

	return findings, scanner.Err()
}

func cleanCell(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "`")
	s = strings.TrimSuffix(s, "`")
	s = strings.TrimPrefix(s, "**")
	s = strings.TrimSuffix(s, "**")
	return s
}

// ─── Fingerprint / Dedup Ledger ───────────────────────────────────────

func computeFingerprint(f ParsedFinding) string {
	// Normalize: lowercase type, lowercase file, lowercase category
	sig := strings.ToLower(f.Category) + ":" + strings.ToLower(f.File) + ":" + strings.ToLower(f.Description[:min(len(f.Description), 80)])
	h := sha256.Sum256([]byte(sig))
	return hex.EncodeToString(h[:8])
}

func loadLedger(path string) ([]FindingFingerprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ledger []FindingFingerprint
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, err
	}
	return ledger, nil
}

func saveLedger(path string, ledger []FindingFingerprint) error {
	data, _ := json.MarshalIndent(ledger, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func findExistingMatch(ledger []FindingFingerprint, fp string) *FindingFingerprint {
	for i := range ledger {
		if ledger[i].Fingerprint == fp {
			return &ledger[i]
		}
	}
	return nil
}

func updateLedgerEntry(ledger *[]FindingFingerprint, fp string, sourceReport string, jiraKey string, status string) {
	now := time.Now().Format(time.RFC3339)
	for i := range *ledger {
		if (*ledger)[i].Fingerprint == fp {
			(*ledger)[i].LastSeen = now
			(*ledger)[i].SourceReport = sourceReport
			if jiraKey != "" {
				(*ledger)[i].LinkedJira = jiraKey
			}
			if status != "" {
				(*ledger)[i].Status = status
			}
			return
		}
	}
	// New entry
	*ledger = append(*ledger, FindingFingerprint{
		Fingerprint:  fp,
		Type:         "",
		File:         "",
		Signature:    fp[:8],
		FirstSeen:    now,
		LastSeen:     now,
		LinkedJira:   jiraKey,
		Status:       status,
		SourceReport: sourceReport,
	})
}

// ─── L1 Ticketization Call ───────────────────────────────────────────

func ticketizeViaL1(endpoint, model string, req TicketizationRequest, timeoutSec int) (*TicketizationOutput, error) {
	prompt := buildTicketizationPrompt(req)

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "/no_think You are a Jira ticket triage assistant. Given one codebase finding, produce a structured Jira issue draft. Output ONLY valid JSON. Do not add markdown fences or extra text."},
			{"role": "user", "content": prompt},
		},
		"max_tokens":           1024,
		"temperature":          0.3,
		"chat_template_kwargs": map[string]interface{}{"enable_thinking": false},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(timeoutSec)
	defer cancel()
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("L1 request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode L1 response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty L1 response")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out TicketizationOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("parse L1 JSON output: %w\nraw: %s", err, content)
	}
	return &out, nil
}

func buildTicketizationPrompt(req TicketizationRequest) string {
	var sb strings.Builder
	sb.WriteString("# Finding to Ticketize\n\n")
	sb.WriteString(fmt.Sprintf("Finding ID: %s\n", req.FindingID))
	sb.WriteString(fmt.Sprintf("Type: %s\n", req.Type))
	sb.WriteString(fmt.Sprintf("Priority Hint: %s\n", req.PriorityHint))
	sb.WriteString(fmt.Sprintf("File: %s\n", req.FilePath))
	sb.WriteString(fmt.Sprintf("Evidence: %s\n", req.Evidence))
	sb.WriteString(fmt.Sprintf("Why It Matters: %s\n", req.WhyItMatters))
	sb.WriteString(fmt.Sprintf("Source Report: %s\n", req.SourceReport))
	if req.ExistingMatch != "" {
		sb.WriteString(fmt.Sprintf("Existing Jira Match: %s (consider update_existing or ignore_noise)\n", req.ExistingMatch))
	}
	sb.WriteString("\n# Output Contract\n\n")
	sb.WriteString("Output ONLY valid JSON with these fields:\n")
	sb.WriteString("- title: string (concise Jira summary)\n")
	sb.WriteString("- summary: string (1-2 sentence description)\n")
	sb.WriteString("- problem: string (what is wrong)\n")
	sb.WriteString("- evidence: string (relevant code snippet or data)\n")
	sb.WriteString("- impact: string (why it matters)\n")
	sb.WriteString("- fix_direction: string (suggested fix approach)\n")
	sb.WriteString("- labels: string (comma-separated, e.g. \"ai:finding,bug\")\n")
	sb.WriteString("- priority: string (High, Medium, Low)\n")
	sb.WriteString("- dedup_decision: string (create_new, update_existing, ignore_noise)\n")
	sb.WriteString("- existing_key: string (empty or Jira key if matching existing)\n")
	sb.WriteString("- follow_up_type: string (no_followup, bounded_fix_l1, manual_review)\n")
	return sb.String()
}

func contextWithTimeout(sec int) (context.Context, context.CancelFunc) {
	if sec <= 0 {
		sec = 120
	}
	return context.WithTimeout(context.Background(), time.Duration(sec)*time.Second)
}

// ─── Jira Issue Creation ─────────────────────────────────────────────

type jiraConfig struct {
	url     string
	email   string
	token   string
	project string
	enabled bool
}

func jiraCreateFindingIssue(cfg jiraConfig, out *TicketizationOutput) string {
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

	desc := fmt.Sprintf("%s\n\nProblem: %s\nEvidence: %s\nImpact: %s\nFix Direction: %s",
		out.Summary, out.Problem, out.Evidence, out.Impact, out.FixDirection)

	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":     map[string]string{"key": cfg.project},
			"summary":     out.Title,
			"description": map[string]interface{}{"type": "doc", "version": 1, "content": []adfPara{{Type: "paragraph", Content: []adfContent{{Type: "text", Text: desc}}}}},
			"issuetype":   map[string]string{"name": "Task"},
			"priority":    map[string]string{"name": normalizePriority(out.Priority)},
			"labels":      strings.Split(out.Labels, ","),
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		cfg.url+"/rest/api/3/issue",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(cfg.email, cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[JIRA-TICKET] create failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[JIRA-TICKET] create returned %d: %s", resp.StatusCode, string(body))
		return ""
	}

	var result struct {
		Key string `json:"key"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Key
}

func jiraAddFindingComment(cfg jiraConfig, issueKey, comment string) bool {
	if !cfg.enabled {
		return false
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
	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		cfg.url+"/rest/api/3/issue/"+issueKey+"/comment",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(cfg.email, cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 201
}

func jiraSearchOpenFindings(cfg jiraConfig) []map[string]interface{} {
	if !cfg.enabled {
		return nil
	}

	jql := fmt.Sprintf("project = %s AND labels = ai:finding AND status != Done ORDER BY created DESC", cfg.project)
	payload := map[string]string{"jql": jql, "maxResults": "50"}
	bodyBytes, _ := json.Marshal(payload)
	ctx, cancel := contextWithTimeout(10)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		cfg.url+"/rest/api/3/search",
		strings.NewReader(string(bodyBytes)))
	req.SetBasicAuth(cfg.email, cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[JIRA-TICKET] search failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string   `json:"summary"`
				Labels  []string `json:"labels"`
			} `json:"fields"`
		} `json:"issues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	var issues []map[string]interface{}
	for _, iss := range result.Issues {
		issues = append(issues, map[string]interface{}{
			"key":     iss.Key,
			"summary": iss.Fields.Summary,
			"labels":  iss.Fields.Labels,
		})
	}
	return issues
}

// ─── Main Flow ────────────────────────────────────────────────────────

// RoadmapItem represents one actionable item extracted from ROADMAP_ITEMS.md or PROGRESS.md.
type RoadmapItem struct {
	Section    string `json:"section"`
	ItemID     string `json:"item_id"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	SourcePath string `json:"source_path"`
}

// parseRoadmapItems extracts actionable items from PROGRESS.md.
// It looks for lines matching "**X.Y Title**" or "| **X.Y Title** | Status |"
// and collects those that are not "Done".
func parseRoadmapItems(progressPath string) []RoadmapItem {
	f, err := os.Open(progressPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var items []RoadmapItem
	scanner := bufio.NewScanner(f)
	currentSection := ""

	// Multiple source format support
	blockRe := regexp.MustCompile(`^## Block \d+`)
	sectionRe := regexp.MustCompile(`^##+ `)
	// ROADMAP_ITEMS.md format: "- **ID**: Description"
	roadmapItemRe := regexp.MustCompile(`^\-\s+\*\*(\w[\w-]*)\*\*:\s*(.+)`)
	// PROGRESS.md table format: "| **X.Y Title** | Status |"
	tableItemRe := regexp.MustCompile(`^\|\s*\*\*(\d+\.\d+)\s+(.+?)\*\*\s*\|\s*(.+?)\s*\|`)
	// Inline bold format
	itemRe := regexp.MustCompile(`^\|?\s*\*\*(\d+\.\d+)\s+(.+?)\*\*\s*\|?\s*(Done|Added|Complete|Partial|In Progress|TODO|WIP|Implemented)?`)

	for scanner.Scan() {
		line := scanner.Text()

		if m := blockRe.FindStringSubmatch(line); m != nil {
			currentSection = m[0]
			continue
		}
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			currentSection = m[0]
			continue
		}

		// Try ROADMAP_ITEMS.md format: "- **IL-1**: Description"
		if m := roadmapItemRe.FindStringSubmatch(line); m != nil {
			items = append(items, RoadmapItem{
				Section:    currentSection,
				ItemID:     m[1],
				Status:     "open",
				Title:      strings.TrimSpace(m[2]),
				SourcePath: progressPath,
			})
			continue
		}

		// Try table row format
		if m := tableItemRe.FindStringSubmatch(line); m != nil {
			status := strings.TrimSpace(m[3])
			if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Complete") ||
				strings.EqualFold(status, "Added") || strings.EqualFold(status, "Implemented") {
				continue
			}
			items = append(items, RoadmapItem{
				Section:    currentSection,
				ItemID:     m[1],
				Status:     status,
				Title:      strings.TrimSpace(m[2]),
				SourcePath: progressPath,
			})
			continue
		}

		// Try inline bold format
		if m := itemRe.FindStringSubmatch(line); m != nil {
			status := strings.TrimSpace(m[3])
			if status == "" {
				status = "unknown"
			}
			if strings.EqualFold(status, "Done") || strings.EqualFold(status, "Complete") ||
				strings.EqualFold(status, "Added") || strings.EqualFold(status, "Implemented") {
				continue
			}
			items = append(items, RoadmapItem{
				Section:    currentSection,
				ItemID:     m[1],
				Status:     status,
				Title:      strings.TrimSpace(m[2]),
				SourcePath: progressPath,
			})
		}
	}

	return items
}

func computeRoadmapFingerprint(item RoadmapItem) string {
	sig := strings.ToLower(item.ItemID) + ":" + strings.ToLower(item.Title[:min(len(item.Title), 60)])
	h := sha256.Sum256([]byte(sig))
	return hex.EncodeToString(h[:8])
}

func runRoadmapTicketization(cfg Config, jcfg jiraConfig) {
	progressPath := envOr("ROADMAP_SOURCE", "ROADMAP_ITEMS.md")
	items := parseRoadmapItems(progressPath)

	if len(items) == 0 {
		log.Printf("[ROADMAP-TICKET] no actionable items found in %s", progressPath)
		return
	}

	log.Printf("[ROADMAP-TICKET] found %d actionable items in %s", len(items), progressPath)

	// Load dedup ledger
	ledger, _ := loadLedger(cfg.LedgerPath)
	if ledger == nil {
		ledger = []FindingFingerprint{}
	}

	// Filter and dedup
	var actionable []RoadmapItem
	seen := make(map[string]bool)

	for _, item := range items {
		fp := computeRoadmapFingerprint(item)
		if seen[fp] {
			continue
		}
		seen[fp] = true

		existing := findExistingMatch(ledger, "roadmap-"+fp)
		if existing != nil && existing.LinkedJira != "" {
			lastSeen, _ := time.Parse(time.RFC3339, existing.LastSeen)
			if time.Since(lastSeen) < 24*time.Hour {
				log.Printf("[ROADMAP-TICKET] skip %s: recently ticketed as %s", item.ItemID, existing.LinkedJira)
				continue
			}
		}

		actionable = append(actionable, item)
		if len(actionable) >= cfg.MaxFindings {
			break
		}
	}

	log.Printf("[ROADMAP-TICKET] actionable: %d (max %d)", len(actionable), cfg.MaxFindings)

	created, updated, skipped := 0, 0, 0

	for _, item := range actionable {
		fp := computeRoadmapFingerprint(item)
		fullFp := "roadmap-" + fp

		existing := findExistingMatch(ledger, fullFp)
		existingMatch := ""
		if existing != nil && existing.LinkedJira != "" {
			existingMatch = existing.LinkedJira
		}

		req := TicketizationRequest{
			FindingID:     item.ItemID,
			Type:          "roadmap_item",
			PriorityHint:  "Medium",
			FilePath:      item.SourcePath,
			Evidence:      fmt.Sprintf("Section: %s\nStatus: %s\nTitle: %s", item.Section, item.Status, item.Title),
			WhyItMatters:  fmt.Sprintf("Item %s from zen-brain1 roadmap. Status: %s. Source: %s", item.ItemID, item.Status, item.SourcePath),
			SourceReport:  filepath.Base(item.SourcePath),
			ExistingMatch: existingMatch,
		}

		out, err := ticketizeViaL1(cfg.L1Endpoint, cfg.L1Model, req, cfg.TimeoutSec)
		if err != nil {
			log.Printf("[ROADMAP-TICKET] %s: L1 failed: %v", item.ItemID, err)
			continue
		}

		log.Printf("[ROADMAP-TICKET] %s: dedup=%s priority=%s title=%q",
			item.ItemID, out.DedupDecision, out.Priority, out.Title)

		switch out.DedupDecision {
		case "ignore_noise":
			skipped++
			updateLedgerEntry(&ledger, fullFp, item.SourcePath, "", "ignored")

		case "update_existing":
			if existingMatch != "" {
				comment := fmt.Sprintf("[zen-brain1 roadmap] Re-seen: %s\nStatus: %s\n%s",
					item.Title, item.Status, out.Summary)
				if jiraAddFindingComment(jcfg, existingMatch, comment) {
					updated++
				}
				updateLedgerEntry(&ledger, fullFp, item.SourcePath, existingMatch, "ticketed")
			}

		default: // create_new
			if jcfg.enabled {
				key := jiraCreateFindingIssue(jcfg, out)
				if key != "" {
					created++
					log.Printf("[ROADMAP-TICKET] %s: created %s — %s", item.ItemID, key, out.Title)
					updateLedgerEntry(&ledger, fullFp, item.SourcePath, key, "ticketed")
				} else {
					log.Printf("[ROADMAP-TICKET] %s: Jira create failed", item.ItemID)
				}
			} else {
				log.Printf("[ROADMAP-TICKET] %s: would create — %s (dry-run)", item.ItemID, out.Title)
				created++
				updateLedgerEntry(&ledger, fullFp, item.SourcePath, "DRY-"+fullFp, "ticketed")
			}
		}
	}

	saveLedger(cfg.LedgerPath, ledger)
	log.Printf("[ROADMAP-TICKET] === done: %d created, %d updated, %d skipped ===", created, updated, skipped)
}

var (
	// Source class → artifact file mapping
	sourceArtifacts = map[string]string{
		"defects":      "defects.md",
		"bug_hunting":  "bug-hunting.md",
		"stub_hunting": "stub-hunting.md",
	}
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Parse simple flags: -run-dir <path> -schedule <name>
	runDirFlag := ""
	scheduleFlag := ""
	for i := 0; i < len(os.Args)-1; i++ {
		switch os.Args[i] {
		case "-run-dir":
			runDirFlag = os.Args[i+1]
		case "-schedule":
			scheduleFlag = os.Args[i+1]
		}
	}

	// Detect cluster mode
	clusterMode := os.Getenv("KUBERNETES_SERVICE_HOST") != ""

	var dirPath string
	if clusterMode {
		dirPath = "/zen-lock/secrets"
	}

	// Use canonical resolver
	material, err := secrets.ResolveJira(context.Background(), secrets.JiraResolveOptions{
		DirPath:          dirPath,
		FilePath:         "",
		AllowEnvFallback: !clusterMode,
		ClusterMode:      clusterMode,
	})

	jcfg := jiraConfig{
		url:     "",
		email:   "",
		token:   "",
		project: envOr("JIRA_PROJECT_KEY", "ZB"),
		enabled: false,
	}

	if err == nil && material.Source != "none" {
		jcfg.url = material.BaseURL
		jcfg.email = material.Email
		jcfg.token = material.APIToken
		jcfg.enabled = true
		log.Printf("[JIRA] ✅ Credentials loaded from %s", material.Source)
	} else {
		log.Printf("[JIRA] ❌ No credentials found (cluster=%v): %v", clusterMode, err)
	}

	cfg := Config{
		L1Endpoint:    envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:       envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		JiraURL:       jcfg.url,
		JiraEmail:     jcfg.email,
		JiraToken:     jcfg.token,
		JiraProject:   jcfg.project,
		ArtifactRoot:  envOr("ARTIFACT_ROOT", "/var/lib/zen-brain1/runs"),
		LedgerPath:    envOr("LEDGER_PATH", "/var/lib/zen-brain1/ticketizer/finding-ledger.json"),
		MaxFindings:   envIntOr("MAX_FINDINGS", 5),
		SourceClasses: strings.Split(envOr("SOURCE_CLASSES", "defects,stub_hunting"), ","),
		TimeoutSec:    envIntOr("TICKETIZER_TIMEOUT", 120),
		Mode:          envOr("MODE", "findings"), // "findings" or "roadmap"
	}

	// ── Roadmap mode ──
	if cfg.Mode == "roadmap" {
		runRoadmapTicketization(cfg, jcfg)
		return
	}

	if !jcfg.enabled {
		log.Printf("[TICKETIZER] Jira not configured — dry-run mode")
	}

	// Ensure ledger directory exists
	os.MkdirAll(filepath.Dir(cfg.LedgerPath), 0755)

	// Load dedup ledger
	ledger, _ := loadLedger(cfg.LedgerPath)
	if ledger == nil {
		ledger = []FindingFingerprint{}
	}

	// Collect findings from run dir(s)
	var runDirs []struct{ path, name string }

	if runDirFlag != "" {
		// Scheduler mode: one specific run dir
		runDirs = append(runDirs, struct{ path, name string }{runDirFlag, scheduleFlag})
	} else {
		// Standalone mode: latest run from each schedule
		for _, sched := range []string{"hourly-scan", "quad-hourly-summary", "daily-sweep"} {
			latest := latestRunDir(filepath.Join(cfg.ArtifactRoot, sched))
			if latest != "" {
				runDirs = append(runDirs, struct{ path, name string }{latest, sched})
			}
		}
	}

	// Parse findings from all run dirs
	var allFindings []ParsedFinding
	for _, rd := range runDirs {
		for _, sourceClass := range cfg.SourceClasses {
			artifactFile, ok := sourceArtifacts[sourceClass]
			if !ok {
				continue
			}
			artifactPath := filepath.Join(rd.path, "final", artifactFile)
			findings, err := parseFindingsFromArtifact(artifactPath, sourceClass)
			if err != nil {
				continue
			}
			if len(findings) > 0 {
				log.Printf("[TICKETIZER] %s: %d findings from %s", sourceClass, len(findings), artifactPath)
				allFindings = append(allFindings, findings...)
			}
		}
	}

	if len(allFindings) == 0 {
		log.Printf("[TICKETIZER] no findings to ticketize")
		return
	}

	log.Printf("[TICKETIZER] total: %d findings, max actionable: %d", len(allFindings), cfg.MaxFindings)

	// ── Dedup + actionability filter ──
	var actionable []ParsedFinding
	seen := make(map[string]bool)

	for _, f := range allFindings {
		fp := computeFingerprint(f)
		if seen[fp] {
			continue
		}
		seen[fp] = true

		existing := findExistingMatch(ledger, fp)
		if existing != nil && existing.LinkedJira != "" {
			lastSeen, _ := time.Parse(time.RFC3339, existing.LastSeen)
			if time.Since(lastSeen) < 24*time.Hour {
				log.Printf("[TICKETIZER] skip %s: recently triaged as %s", fp[:8], existing.LinkedJira)
				continue
			}
		}

		if isLowConfidence(f) {
			log.Printf("[TICKETIZER] skip %s: low confidence", fp[:8])
			continue
		}

		actionable = append(actionable, f)
		if len(actionable) >= cfg.MaxFindings {
			break
		}
	}

	log.Printf("[TICKETIZER] actionable: %d", len(actionable))

	// ── Ticketize each via L1 ──
	created, updated, skipped := 0, 0, 0

	for _, f := range actionable {
		fp := computeFingerprint(f)
		existing := findExistingMatch(ledger, fp)
		existingMatch := ""
		if existing != nil && existing.LinkedJira != "" {
			existingMatch = existing.LinkedJira
		}

		req := TicketizationRequest{
			FindingID:     fp[:8],
			Type:          f.Category,
			PriorityHint:  f.Severity,
			FilePath:      f.File,
			Evidence:      f.Description,
			WhyItMatters:  f.WhyItMatters,
			SourceReport:  f.SourceReport,
			ExistingMatch: existingMatch,
		}

		out, err := ticketizeViaL1(cfg.L1Endpoint, cfg.L1Model, req, cfg.TimeoutSec)
		if err != nil {
			log.Printf("[TICKETIZER] %s: L1 failed: %v", fp[:8], err)
			continue
		}

		log.Printf("[TICKETIZER] %s: dedup=%s follow_up=%s priority=%s title=%q",
			fp[:8], out.DedupDecision, out.FollowUpType, out.Priority, out.Title)

		switch out.DedupDecision {
		case "ignore_noise":
			skipped++
			updateLedgerEntry(&ledger, fp, f.SourceReport, "", "ignored")

		case "update_existing":
			if existingMatch != "" {
				comment := fmt.Sprintf("[zen-brain1] Re-seen in %s\n%s\nSeverity: %s",
					f.SourceReport, f.WhyItMatters, f.Severity)
				if jiraAddFindingComment(jcfg, existingMatch, comment) {
					updated++
					log.Printf("[TICKETIZER] %s: updated %s", fp[:8], existingMatch)
				}
				updateLedgerEntry(&ledger, fp, f.SourceReport, existingMatch, "ticketed")
			}

		default: // create_new or anything else
			if jcfg.enabled {
				key := jiraCreateFindingIssue(jcfg, out)
				if key != "" {
					created++
					log.Printf("[TICKETIZER] %s: created %s — %s", fp[:8], key, out.Title)
					updateLedgerEntry(&ledger, fp, f.SourceReport, key, "ticketed")
				} else {
					log.Printf("[TICKETIZER] %s: Jira create failed", fp[:8])
				}
			} else {
				log.Printf("[TICKETIZER] %s: would create — %s (dry-run)", fp[:8], out.Title)
				created++
				updateLedgerEntry(&ledger, fp, f.SourceReport, "DRY-"+fp[:8], "ticketed")
			}
		}
	}

	saveLedger(cfg.LedgerPath, ledger)
	log.Printf("[TICKETIZER] === done: %d created, %d updated, %d skipped ===", created, updated, skipped)
}

// ─── Helpers ──────────────────────────────────────────────────────────

func isLowConfidence(f ParsedFinding) bool {
	// Skip findings with no concrete file path
	if f.File == "" || f.File == "N/A" || f.File == "?" || strings.HasPrefix(f.File, "pkg/") && len(f.File) <= 5 {
		return true
	}
	// Skip findings with empty or generic description
	if f.Description == "" || f.Description == "N/A" || f.Description == "Unknown" {
		return true
	}
	// Skip findings whose "file" is just a package name with no actual path
	if !strings.Contains(f.File, ".") && !strings.Contains(f.File, "/") {
		return true
	}
	return false
}

func latestRunDir(base string) string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	var latest string
	for _, e := range entries {
		if e.IsDir() {
			latest = e.Name()
		}
	}
	if latest == "" {
		return ""
	}
	return filepath.Join(base, latest)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizePriority(p string) string {
	p = strings.Title(strings.ToLower(strings.TrimSpace(p)))
	switch p {
	case "High", "Medium", "Low", "Highest", "Lowest":
		return p
	default:
		return "Medium"
	}
}
