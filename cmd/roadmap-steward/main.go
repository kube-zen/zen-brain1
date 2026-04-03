// Package main — roadmap-steward
//
// Roadmap Steward: L1 planning/backlog-shaping role for zen-brain1.
//
// Reads roadmap items, creates/updates Jira tickets, respects dedup/cooldown/backpressure.
// Does NOT execute work — only creates it.
// Does NOT loop on its own outputs.
// One item at a time. Bounded per run.
//
// GLM-5 supervises policy and tuning only; 0.8b L1 does the routine roadmap heavy lift.

package main

import (
	"bytes"
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
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/internal/secrets"
)

// ─── Config ───

type stewardConfig struct {
	JiraURL       string
	JiraEmail     string
	JiraAPIToken  string
	JiraProject   string
	L1Endpoint    string
	L1Model       string
	RoadmapSource string
	LedgerPath    string
	ArtifactDir   string
	MaxItems      int
	BacklogMax    int
	CooldownHours int
	DryRun        bool
	Mode          string
}

func loadConfig() stewardConfig {
	// ZB-CREDENTIAL-RAILS: Use canonical resolver for Jira credentials
	jiraCreds, err := secrets.ResolveJira(context.Background(), secrets.JiraResolveOptions{
		ClusterMode: false,
		DirPath:     "",
	})
	if err != nil {
		log.Printf("[WARN] Jira credential resolution failed: %v, using env fallback", err)
	}

	cfg := stewardConfig{
		JiraURL:       jiraCreds.BaseURL,
		JiraEmail:     jiraCreds.Email,
		JiraAPIToken:  jiraCreds.APIToken,
		JiraProject:   jiraCreds.ProjectKey,
		L1Endpoint:    envOr("L1_ENDPOINT", "http://localhost:56227"),
		L1Model:       envOr("L1_MODEL", "Qwen3.5-0.8B-Q4_K_M.gguf"),
		RoadmapSource: envOr("ROADMAP_SOURCE", "ROADMAP_ITEMS.md"),
		LedgerPath:    envOr("LEDGER_PATH", "/var/lib/zen-brain1/ticketizer/roadmap-ledger.json"),
		ArtifactDir:   envOr("ARTIFACT_DIR", "/var/lib/zen-brain1/evidence/roadmap-steward"),
		MaxItems:      envIntOr("MAX_ROADMAP_ITEMS", 3),
		BacklogMax:    envIntOr("BACKLOG_MAX", 10),
		CooldownHours: envIntOr("COOLDOWN_HOURS", 24),
		DryRun:        os.Getenv("DRY_RUN") != "",
		Mode:          envOr("STEWARD_MODE", "hourly"),
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

// ─── Data Types ───

type roadmapItem struct {
	ItemID  string
	Title   string
	Section string
	Source  string
}

type ledgerEntry struct {
	Fingerprint   string `json:"fingerprint"`
	ItemID        string `json:"item_id"`
	Title         string `json:"title"`
	Source        string `json:"source"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
	LinkedJira    string `json:"linked_jira"`
	LastAction    string `json:"last_action"`
	CooldownUntil string `json:"cooldown_until"`
	Status        string `json:"status"`
}

type ticketDraft struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Problem     string   `json:"problem"`
	Outcome     string   `json:"expected_outcome"`
	WhyNow      string   `json:"why_now"`
	Evidence    string   `json:"evidence"`
	Labels      []string `json:"labels"`
	Priority    string   `json:"priority"`
	Lane        string   `json:"suggested_lane"`
	DedupAction string   `json:"dedup_action"`
}

type decisionRecord struct {
	ItemID  string `json:"item_id"`
	Title   string `json:"title"`
	Action  string `json:"action"`
	JiraKey string `json:"jira_key,omitempty"`
	Reason  string `json:"reason"`
}

type stewardRunResult struct {
	RunID           string           `json:"run_id"`
	Timestamp       string           `json:"timestamp"`
	Mode            string           `json:"mode"`
	ItemsConsidered int              `json:"items_considered"`
	Backpressure    backpressureInfo `json:"backpressure"`
	Decisions       []decisionRecord `json:"decisions"`
	Created         int              `json:"created"`
	Updated         int              `json:"updated"`
	Skipped         int              `json:"skipped"`
}

type backpressureInfo struct {
	ReadyBacklog int  `json:"ready_backlog"`
	Throttled    bool `json:"throttled"`
	Threshold    int  `json:"threshold"`
}

// ─── Parse Roadmap Items ───

func parseRoadmapItems(path string) []roadmapItem {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[STEWARD] cannot read %s: %v", path, err)
		return nil
	}

	var items []roadmapItem
	var section string
	re := regexp.MustCompile(`^\-\s+\*\*(\w[\w-]*)\*\*:\s*(.+)`)

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
			section = trimmed
			continue
		}
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		items = append(items, roadmapItem{
			ItemID:  m[1],
			Title:   strings.TrimSpace(m[2]),
			Section: section,
			Source:  path,
		})
	}
	return items
}

// ─── Fingerprint ───

func computeFingerprint(item roadmapItem) string {
	sig := strings.ToLower(item.ItemID) + ":" + strings.ToLower(item.Title[:min(len(item.Title), 60)])
	h := sha256.Sum256([]byte(sig))
	return hex.EncodeToString(h[:8])
}

// ─── Ledger ───

func loadLedger(path string) []ledgerEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []ledgerEntry
	json.Unmarshal(data, &entries)
	return entries
}

func saveLedger(path string, entries []ledgerEntry) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.MarshalIndent(entries, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func findLedgerEntry(entries []ledgerEntry, fp string) *ledgerEntry {
	for i := range entries {
		if entries[i].Fingerprint == fp {
			return &entries[i]
		}
	}
	return nil
}

func updateOrCreateLedger(entries *[]ledgerEntry, item roadmapItem, fp string, jiraKey string, action string) {
	now := time.Now().Format(time.RFC3339)
	for i := range *entries {
		if (*entries)[i].Fingerprint == fp {
			(*entries)[i].LastSeen = now
			(*entries)[i].LinkedJira = jiraKey
			(*entries)[i].LastAction = action
			(*entries)[i].Status = action
			return
		}
	}
	*entries = append(*entries, ledgerEntry{
		Fingerprint: fp, ItemID: item.ItemID, Title: item.Title,
		Source: item.Source, FirstSeen: now, LastSeen: now,
		LinkedJira: jiraKey, LastAction: action, Status: action,
	})
}

// ─── Backpressure ───

func checkBackpressure(cfg stewardConfig) (int, bool) {
	jql := fmt.Sprintf(`project="%s" AND status=Backlog AND labels=bug AND labels=ai:finding AND -labels="quality:blocked-invalid-payload" AND -labels="ai:blocked"`, cfg.JiraProject)
	totalCount := countJiraResults(cfg, jql)
	throttled := totalCount >= cfg.BacklogMax
	return totalCount, throttled
}

func countJiraResults(cfg stewardConfig, jql string) int {
	totalCount := 0
	var token string
	for page := 0; page < 5; page++ {
		body := map[string]interface{}{"jql": jql, "maxResults": 100, "fields": []string{}}
		if token != "" {
			body["nextPageToken"] = token
		}
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/search/jql", bytes.NewReader(data))
		req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			break
		}
		var result struct {
			Issues        []interface{} `json:"issues"`
			IsLast        bool          `json:"isLast"`
			NextPageToken string        `json:"nextPageToken"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		totalCount += len(result.Issues)
		if result.IsLast || result.NextPageToken == "" {
			break
		}
		token = result.NextPageToken
	}
	return totalCount
}

// ─── Jira Operations ───

func jiraCreateTicket(cfg stewardConfig, title, summary string, labels []string) string {
	descContent := []interface{}{
		map[string]interface{}{"type": "paragraph", "content": []interface{}{
			map[string]interface{}{"type": "text", "text": summary},
		}},
	}
	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"project":     map[string]string{"key": cfg.JiraProject},
			"summary":     title,
			"description": map[string]interface{}{"type": "doc", "version": 1, "content": descContent},
			"issuetype":   map[string]string{"name": "Task"},
			"labels":      labels,
		},
	}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/issue", bytes.NewReader(data))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[STEWARD] Jira create failed: %v", err)
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Key string `json:"key"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Key
}

func jiraAddComment(cfg stewardConfig, key, comment string) bool {
	descContent := []interface{}{
		map[string]interface{}{"type": "paragraph", "content": []interface{}{
			map[string]interface{}{"type": "text", "text": comment},
		}},
	}
	body := map[string]interface{}{"body": map[string]interface{}{"type": "doc", "version": 1, "content": descContent}}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", cfg.JiraURL+"/rest/api/3/issue/"+key+"/comment", bytes.NewReader(data))
	req.SetBasicAuth(cfg.JiraEmail, cfg.JiraAPIToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 201
}

// ─── L1 Call ───

func draftTicketViaL1(cfg stewardConfig, item roadmapItem, existingJira string) (*ticketDraft, error) {
	existingNote := ""
	if existingJira != "" {
		existingNote = fmt.Sprintf("\n\nIMPORTANT: Existing Jira ticket: %s. Decide update or skip.", existingJira)
	}

	prompt := fmt.Sprintf(`Create a Jira ticket for this roadmap item:

Item ID: %s
Title: %s
Section: %s
Source: %s%s

RULES:
- Create a Jira ticket with title, summary, problem, expected_outcome
- Labels must include: ai:roadmap
- Priority: Medium unless P1 section
- Suggested lane: roadmap
- If existing ticket exists, set dedup_action to "update_existing", otherwise "create_new"

Respond with ONLY this JSON:
{"title":"...","summary":"...","problem":"...","expected_outcome":"...","why_now":"...","evidence":"...","labels":["ai:roadmap"],"priority":"Medium","suggested_lane":"roadmap","dedup_action":"create_new"}`,
		item.ItemID, item.Title, item.Section, item.Source, existingNote)

	reqBody := map[string]interface{}{
		"model": cfg.L1Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are the Roadmap Steward. Create Jira tickets. Respond with only valid JSON."},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  1500,
	}
	data, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("POST", cfg.L1Endpoint+"/v1/chat/completions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if len(llmResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices from L1")
	}

	content := llmResp.Choices[0].Message.Content
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON in L1 output")
	}

	var draft ticketDraft
	if err := json.Unmarshal([]byte(content[start:end+1]), &draft); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return &draft, nil
}

// ─── Artifacts ───

func writeArtifacts(cfg stewardConfig, result stewardRunResult) error {
	os.MkdirAll(cfg.ArtifactDir, 0755)
	runID := result.RunID

	artJSON, _ := json.MarshalIndent(result, "", "  ")
	os.WriteFile(filepath.Join(cfg.ArtifactDir, "roadmap-steward-latest.json"), artJSON, 0644)
	os.WriteFile(filepath.Join(cfg.ArtifactDir, fmt.Sprintf("roadmap-steward-%s.json", runID)), artJSON, 0644)

	var md strings.Builder
	md.WriteString("# Roadmap Steward Report\n\n")
	md.WriteString(fmt.Sprintf("**Run ID:** %s  \n", runID))
	md.WriteString(fmt.Sprintf("**Mode:** %s  \n", cfg.Mode))
	md.WriteString(fmt.Sprintf("**Timestamp:** %s  \n\n", result.Timestamp))
	md.WriteString(fmt.Sprintf("**Backpressure:** ready=%d, throttled=%v (threshold=%d)\n\n",
		result.Backpressure.ReadyBacklog, result.Backpressure.Throttled, result.Backpressure.Threshold))

	if len(result.Decisions) > 0 {
		md.WriteString("## Decisions\n\n| Item | Action | Jira Key | Reason |\n|------|--------|----------|--------|\n")
		for _, d := range result.Decisions {
			md.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", d.ItemID, d.Action, d.JiraKey, d.Reason))
		}
	}
	md.WriteString(fmt.Sprintf("\n**Summary:** %d created, %d updated, %d skipped\n",
		result.Created, result.Updated, result.Skipped))

	os.WriteFile(filepath.Join(cfg.ArtifactDir, "roadmap-steward-latest.md"), []byte(md.String()), 0644)
	os.WriteFile(filepath.Join(cfg.ArtifactDir, fmt.Sprintf("roadmap-steward-%s.md", runID)), []byte(md.String()), 0644)
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
		for _, c := range v {
			if c < '0' || c > '9' {
				return def
			}
			def = def*10 + int(c-'0')
		}
		return def
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

	log.Printf("[STEWARD] Roadmap Steward starting (mode=%s, max=%d, backlog_max=%d, cooldown=%dh, dry=%v)",
		cfg.Mode, cfg.MaxItems, cfg.BacklogMax, cfg.CooldownHours, cfg.DryRun)

	runID := time.Now().Format("20060102-150405")
	result := stewardRunResult{
		RunID:        runID,
		Timestamp:    time.Now().Format(time.RFC3339),
		Mode:         cfg.Mode,
		Backpressure: backpressureInfo{Threshold: cfg.BacklogMax},
	}

	// Phase 1: Backpressure
	readyBacklog, throttled := checkBackpressure(cfg)
	result.Backpressure.ReadyBacklog = readyBacklog
	result.Backpressure.Throttled = throttled
	log.Printf("[STEWARD] Backpressure: ready=%d, throttled=%v (threshold=%d)", readyBacklog, throttled, cfg.BacklogMax)

	if throttled {
		log.Printf("[STEWARD] ⛔ BACKPRESSURE: backlog %d >= %d — skipping", readyBacklog, cfg.BacklogMax)
		result.Decisions = append(result.Decisions, decisionRecord{
			ItemID: "-", Title: "backpressure", Action: "skip_backpressure",
			Reason: fmt.Sprintf("ready backlog %d >= threshold %d", readyBacklog, cfg.BacklogMax),
		})
		writeArtifacts(cfg, result)
		return
	}

	// Phase 2: Parse roadmap
	items := parseRoadmapItems(cfg.RoadmapSource)
	result.ItemsConsidered = len(items)
	log.Printf("[STEWARD] Parsed %d roadmap items from %s", len(items), cfg.RoadmapSource)

	if len(items) == 0 {
		log.Printf("[STEWARD] No roadmap items — nothing to do")
		writeArtifacts(cfg, result)
		return
	}

	// Phase 3: Load ledger
	ledger := loadLedger(cfg.LedgerPath)

	// Phase 4: Process one item at a time
	processed := 0
	for _, item := range items {
		if processed >= cfg.MaxItems {
			log.Printf("[STEWARD] Max items (%d) reached", cfg.MaxItems)
			break
		}

		fp := computeFingerprint(item)
		entry := findLedgerEntry(ledger, fp)

		// Check cooldown
		if entry != nil && entry.LinkedJira != "" {
			lastSeen, _ := time.Parse(time.RFC3339, entry.LastSeen)
			if time.Since(lastSeen) < time.Duration(cfg.CooldownHours)*time.Hour {
				log.Printf("[STEWARD] %s: skip — cooldown (linked: %s)", item.ItemID, entry.LinkedJira)
				result.Skipped++
				result.Decisions = append(result.Decisions, decisionRecord{
					ItemID: item.ItemID, Title: item.Title, Action: "skip_duplicate",
					JiraKey: entry.LinkedJira, Reason: fmt.Sprintf("cooldown: %dh remaining", cfg.CooldownHours),
				})
				continue
			}
		}

		// Draft via L1
		existingJira := ""
		if entry != nil {
			existingJira = entry.LinkedJira
		}

		draft, err := draftTicketViaL1(cfg, item, existingJira)
		if err != nil {
			log.Printf("[STEWARD] %s: L1 failed: %v — skipping", item.ItemID, err)
			result.Skipped++
			result.Decisions = append(result.Decisions, decisionRecord{
				ItemID: item.ItemID, Title: item.Title, Action: "skip_duplicate",
				Reason: fmt.Sprintf("L1 failed: %v", err),
			})
			continue
		}
		log.Printf("[STEWARD] %s: L1 drafted: action=%s title=%q", item.ItemID, draft.DedupAction, draft.Title)

		// Determine effective action
		effectiveAction := draft.DedupAction
		if effectiveAction == "update_existing" && existingJira == "" {
			effectiveAction = "create_new"
		}

		switch effectiveAction {
		case "update_existing":
			if existingJira != "" {
				comment := fmt.Sprintf("[zen-brain1 roadmap] Re-seen: %s\nSource: %s\n%s",
					item.Title, item.Source, draft.Summary)
				if !cfg.DryRun {
					jiraAddComment(cfg, existingJira, comment)
				} else {
					log.Printf("[STEWARD] %s: DRY_RUN would update %s", item.ItemID, existingJira)
				}
				result.Updated++
				result.Decisions = append(result.Decisions, decisionRecord{
					ItemID: item.ItemID, Title: draft.Title, Action: "update_existing",
					JiraKey: existingJira, Reason: "re-seen",
				})
				updateOrCreateLedger(&ledger, item, fp, existingJira, "updated")
			}

		case "skip_duplicate":
			result.Skipped++
			result.Decisions = append(result.Decisions, decisionRecord{
				ItemID: item.ItemID, Title: draft.Title, Action: "skip_duplicate",
				Reason: "L1 says duplicate",
			})

		default: // create_new
			labels := draft.Labels
			if len(labels) == 0 {
				labels = []string{"ai:roadmap"}
			}
			summary := draft.Summary
			if summary == "" {
				summary = fmt.Sprintf("Title: %s\n\nProblem: %s\n\nExpected: %s",
					draft.Title, draft.Problem, draft.Outcome)
			}
			if cfg.DryRun {
				key := fmt.Sprintf("DRY-%s", fp[:6])
				log.Printf("[STEWARD] %s: DRY_RUN would create %s", item.ItemID, key)
				result.Created++
				result.Decisions = append(result.Decisions, decisionRecord{
					ItemID: item.ItemID, Title: draft.Title, Action: "create_new",
					JiraKey: key, Reason: "dry run",
				})
				updateOrCreateLedger(&ledger, item, fp, key, "created_dry")
			} else {
				key := jiraCreateTicket(cfg, draft.Title, summary, labels)
				if key != "" {
					result.Created++
					log.Printf("[STEWARD] %s: created %s — %s", item.ItemID, key, draft.Title)
					result.Decisions = append(result.Decisions, decisionRecord{
						ItemID: item.ItemID, Title: draft.Title, Action: "create_new",
						JiraKey: key, Reason: "created",
					})
					updateOrCreateLedger(&ledger, item, fp, key, "created")
				} else {
					result.Skipped++
					result.Decisions = append(result.Decisions, decisionRecord{
						ItemID: item.ItemID, Title: draft.Title, Action: "skip_duplicate",
						Reason: "Jira create failed",
					})
				}
			}
		}
		processed++
	}

	// Save ledger
	saveLedger(cfg.LedgerPath, ledger)

	// Write artifacts
	writeArtifacts(cfg, result)

	log.Printf("[STEWARD] === done: %d created, %d updated, %d skipped ===",
		result.Created, result.Updated, result.Skipped)
}

var _ = io.EOF
