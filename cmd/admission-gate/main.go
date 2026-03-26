package main

import (
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

// zen-brain fail-closed admission gate.
//
// Every task MUST pass through this gate before dispatch.
// The gate checks auth, packet structure, context budgets, and evidence contracts.
// Results are recorded in a durable failure ledger for empirical routing.
//
// ENV VARS:
//   JIRA_URL      — Jira base URL (empty = skip Jira preflight)
//   JIRA_EMAIL    — Jira email
//   JIRA_TOKEN    — Jira API token
//   JIRA_PROJECT  — Jira project key (default: ZB)
//   L1_ENDPOINT   — L1 health endpoint (default: http://localhost:56227/health)
//   LEDGER_DIR    — failure ledger directory (default: /var/lib/zen-brain1/ledger)
//   STRICT        — fail on any warning (default: true)

const (
	defaultL1Endpoint = "http://localhost:56227/health"
	defaultLedgerDir  = "/var/lib/zen-brain1/ledger"
)

// --- Packet structure (workorder) ---

type TaskPacket struct {
	TaskID     string `json:"task_id"`
	Intent     string `json:"intent"`
	Phase      string `json:"phase"`
	MaxSteps   int    `json:"max_steps"`
	MaxTools  int    `json:"max_tool_calls"`
	TargetFiles []string `json:"target_files"`
	Acceptance string `json:"acceptance_criteria"`
	Evidence   string `json:"evidence_needed"`
	Rollback   string `json:"rollback_plan"`
	TaskClass  string `json:"task_class"`
	Lane       string `json:"target_lane"`
}

// --- Preflight results ---

type PreflightResult struct {
	Name      string `json:"name"`
	Passed    bool   `json:"passed"`
	Blocked   bool   `json:"blocked"` // true = hard fail, false = warning
	Message   string `json:"message"`
	Duration  string `json:"duration"`
}

type AdmissionDecision struct {
	Allowed        bool             `json:"allowed"`
	Reason         string           `json:"reason"`
	CauseClass     string           `json:"cause_class"` // infra, auth, packet, tool, model, none
	Preflights     []PreflightResult `json:"preflights"`
	TaskID         string           `json:"task_id"`
	DecisionTime   string           `json:"decision_time"`
	Lane           string           `json:"lane"`
}

// --- Failure ledger ---

type LedgerEntry struct {
	TaskID          string    `json:"task_id"`
	TaskClass       string    `json:"task_class"`
	PacketType      string    `json:"packet_type"`
	LaneAttempted   string    `json:"lane_attempted"`
	AuthPreflight   string    `json:"auth_preflight"` // passed, failed, skipped
	ToolCount       int       `json:"tool_count"`
	ContextEstimate int       `json:"context_estimate"`
	FinalResult     string    `json:"final_result"` // admitted, blocked, failed
	CauseClass      string    `json:"cause_class"`
	Timestamp       time.Time `json:"timestamp"`
}

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	strict := os.Getenv("STRICT") != "false" // default true
	ledgerDir := envOr("LEDGER_DIR", defaultLedgerDir)

	os.MkdirAll(ledgerDir, 0755)

	mode := envOr("MODE", "gate")
	switch mode {
	case "preflight":
		// Just run preflight, print results, exit
		decision := runPreflights(strict, "")
		printDecision(decision)
		if !decision.Allowed {
			os.Exit(1)
		}
	case "gate":
		// Read packet from stdin, run full admission gate
		if len(os.Args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s gate <packet.json>\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "  echo '{...}' | %s gate -\n", os.Args[0])
			os.Exit(1)
		}
		var packet TaskPacket
		data, err := readInput(os.Args[1])
		if err != nil {
			log.Fatalf("[GATE] Failed to read input: %v", err)
		}
		if err := json.Unmarshal(data, &packet); err != nil {
			// No valid packet = rejected
			decision := AdmissionDecision{
				Allowed:    false,
				Reason:     fmt.Sprintf("Invalid packet JSON: %v", err),
				CauseClass: "packet",
				DecisionTime: time.Now().UTC().Format(time.RFC3339),
			}
			printDecision(decision)
			writeLedger(ledgerDir, LedgerEntry{
				FinalResult: "blocked",
				CauseClass:  "packet",
				Timestamp:   time.Now(),
			})
			os.Exit(1)
		}
		decision := runAdmission(packet, strict)
		printDecision(decision)
		writeLedger(ledgerDir, LedgerEntry{
			TaskID:        packet.TaskID,
			TaskClass:     packet.TaskClass,
			PacketType:    packet.Phase,
			LaneAttempted: packet.Lane,
			AuthPreflight: decision.preflightStatus("auth"),
			ToolCount:     packet.MaxTools,
			FinalResult:   map[bool]string{true: "admitted", false: "blocked"}[decision.Allowed],
			CauseClass:    decision.CauseClass,
			Timestamp:     time.Now(),
		})
		if !decision.Allowed {
			os.Exit(1)
		}
	case "ledger-status":
		printLedgerSummary(ledgerDir)
	default:
		fmt.Fprintf(os.Stderr, "Usage: %s {preflight|gate|ledger-status}\n", os.Args[0])
		os.Exit(1)
	}
}

func runAdmission(packet TaskPacket, strict bool) AdmissionDecision {
	var preflights []PreflightResult

	// 1. Auth preflight
	authResult := checkAuth()
	preflights = append(preflights, authResult)
	if authResult.Blocked {
		return AdmissionDecision{
			Allowed: false, Reason: authResult.Message,
			CauseClass: "auth", Preflights: preflights,
			TaskID: packet.TaskID, Lane: packet.Lane,
			DecisionTime: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 2. L1 health
	l1Result := checkL1()
	preflights = append(preflights, l1Result)
	if l1Result.Blocked {
		return AdmissionDecision{
			Allowed: false, Reason: l1Result.Message,
			CauseClass: "infra", Preflights: preflights,
			TaskID: packet.TaskID, Lane: packet.Lane,
			DecisionTime: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 3. Packet structure
	packetResult := validatePacket(packet, strict)
	preflights = append(preflights, packetResult)
	if packetResult.Blocked {
		return AdmissionDecision{
			Allowed: false, Reason: packetResult.Message,
			CauseClass: "packet", Preflights: preflights,
			TaskID: packet.TaskID, Lane: packet.Lane,
			DecisionTime: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 4. Context/tool budget
	budgetResult := checkBudget(packet, strict)
	preflights = append(preflights, budgetResult)
	if budgetResult.Blocked {
		return AdmissionDecision{
			Allowed: false, Reason: budgetResult.Message,
			CauseClass: "tool", Preflights: preflights,
			TaskID: packet.TaskID, Lane: packet.Lane,
			DecisionTime: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// 5. Evidence contract
	evidenceResult := checkEvidenceContract(packet, strict)
	preflights = append(preflights, evidenceResult)
	if evidenceResult.Blocked {
		return AdmissionDecision{
			Allowed: false, Reason: evidenceResult.Message,
			CauseClass: "packet", Preflights: preflights,
			TaskID: packet.TaskID, Lane: packet.Lane,
			DecisionTime: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// All passed
	warnings := ""
	for _, p := range preflights {
		if !p.Passed && !p.Blocked {
			warnings += p.Message + "; "
		}
	}
	return AdmissionDecision{
		Allowed:      true,
		Reason:       "All preflights passed" + warnSuffix(warnings),
		CauseClass:   "none",
		Preflights:   preflights,
		TaskID:       packet.TaskID,
		Lane:         packet.Lane,
		DecisionTime: time.Now().UTC().Format(time.RFC3339),
	}
}

func runPreflights(strict bool, taskID string) AdmissionDecision {
	return runAdmission(TaskPacket{TaskID: taskID}, strict)
}

// --- Preflight checks ---

func checkAuth() PreflightResult {
	start := time.Now()
	jiraURL := os.Getenv("JIRA_URL")
	jiraEmail := os.Getenv("JIRA_EMAIL")
	jiraToken := os.Getenv("JIRA_TOKEN")
	jiraProject := envOr("JIRA_PROJECT", "ZB")

	// If no Jira config, skip (not blocking)
	if jiraURL == "" && jiraEmail == "" && jiraToken == "" {
		return PreflightResult{
			Name: "jira-auth", Passed: true, Blocked: false,
			Message: "Jira not configured, skipping auth check",
			Duration: time.Since(start).String(),
		}
	}

	// If partial config, that's an error
	if jiraURL == "" || jiraEmail == "" || jiraToken == "" {
		return PreflightResult{
			Name: "jira-auth", Passed: false, Blocked: true,
			Message: "Jira partially configured (need JIRA_URL, JIRA_EMAIL, JIRA_TOKEN)",
			Duration: time.Since(start).String(),
		}
	}

	// Verify endpoint reachable + token authenticates
	req, _ := http.NewRequest("GET", strings.TrimSuffix(jiraURL, "/")+"/rest/api/3/myself", nil)
	req.SetBasicAuth(jiraEmail, jiraToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return PreflightResult{
			Name: "jira-auth", Passed: false, Blocked: true,
			Message: fmt.Sprintf("Jira unreachable: %v", err),
			Duration: time.Since(start).String(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return PreflightResult{
			Name: "jira-auth", Passed: false, Blocked: true,
			Message: "Jira authentication failed (401) — token expired or invalid",
			Duration: time.Since(start).String(),
		}
	}
	if resp.StatusCode != 200 {
		return PreflightResult{
			Name: "jira-auth", Passed: false, Blocked: true,
			Message: fmt.Sprintf("Jira returned %d — unexpected", resp.StatusCode),
			Duration: time.Since(start).String(),
		}
	}

	return PreflightResult{
		Name: "jira-auth", Passed: true, Blocked: false,
		Message: fmt.Sprintf("Jira authenticated (project: %s)", jiraProject),
		Duration: time.Since(start).String(),
	}
}

func checkL1() PreflightResult {
	start := time.Now()
	endpoint := envOr("L1_ENDPOINT", defaultL1Endpoint)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return PreflightResult{
			Name: "l1-health", Passed: false, Blocked: true,
			Message: fmt.Sprintf("L1 unreachable: %v", err),
			Duration: time.Since(start).String(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return PreflightResult{
			Name: "l1-health", Passed: false, Blocked: true,
			Message: fmt.Sprintf("L1 returned %d", resp.StatusCode),
			Duration: time.Since(start).String(),
		}
	}

	return PreflightResult{
		Name: "l1-health", Passed: true, Blocked: false,
		Message: "L1 healthy",
		Duration: time.Since(start).String(),
	}
}

func validatePacket(packet TaskPacket, strict bool) PreflightResult {
	start := time.Now()
	var errors []string

	if packet.Intent == "" {
		errors = append(errors, "intent required")
	}
	if packet.Phase == "" {
		errors = append(errors, "phase required")
	}
	if packet.MaxSteps <= 0 {
		errors = append(errors, "max_steps must be > 0")
	}
	if packet.MaxSteps > 50 {
		errors = append(errors, fmt.Sprintf("max_steps=%d exceeds limit (50)", packet.MaxSteps))
	}
	if packet.MaxTools < 0 {
		errors = append(errors, "max_tool_calls must be >= 0")
	}
	if packet.MaxTools > 20 {
		errors = append(errors, fmt.Sprintf("max_tool_calls=%d exceeds limit (20)", packet.MaxTools))
	}
	if packet.TargetLane() == "L1" && packet.MaxTools > 3 {
		errors = append(errors, fmt.Sprintf("L1 max_tool_calls=%d exceeds L1 limit (3)", packet.MaxTools))
	}
	if packet.Acceptance == "" {
		errors = append(errors, "acceptance_criteria required")
	}
	if packet.Evidence == "" {
		errors = append(errors, "evidence_needed required")
	}

	blocked := strict && len(errors) > 0
	message := "packet valid"
	if len(errors) > 0 {
		message = "packet errors: " + strings.Join(errors, "; ")
	}

	return PreflightResult{
		Name: "packet-structure", Passed: len(errors) == 0, Blocked: blocked,
		Message: message, Duration: time.Since(start).String(),
	}
}

func checkBudget(packet TaskPacket, strict bool) PreflightResult {
	start := time.Now()
	var warnings []string

	if packet.TargetLane() == "L1" && len(packet.TargetFiles) > 3 {
		warnings = append(warnings, fmt.Sprintf("L1 with %d target files (max 3 recommended)", len(packet.TargetFiles)))
	}

	blocked := strict && len(warnings) > 0
	message := "budget OK"
	if len(warnings) > 0 {
		message = strings.Join(warnings, "; ")
	}

	return PreflightResult{
		Name: "budget-check", Passed: len(warnings) == 0, Blocked: blocked,
		Message: message, Duration: time.Since(start).String(),
	}
}

func checkEvidenceContract(packet TaskPacket, strict bool) PreflightResult {
	start := time.Now()

	if packet.Evidence == "" && strict {
		return PreflightResult{
			Name: "evidence-contract", Passed: false, Blocked: true,
			Message: "evidence_needed is required in strict mode",
			Duration: time.Since(start).String(),
		}
	}
	if packet.Acceptance == "" && strict {
		return PreflightResult{
			Name: "evidence-contract", Passed: false, Blocked: true,
			Message: "acceptance_criteria is required in strict mode",
			Duration: time.Since(start).String(),
		}
	}

	return PreflightResult{
		Name: "evidence-contract", Passed: true, Blocked: false,
		Message: "evidence contract present",
		Duration: time.Since(start).String(),
	}
}

// --- Ledger ---

func writeLedger(dir string, entry LedgerEntry) {
	os.MkdirAll(dir, 0755)
	ts := entry.Timestamp.Format("20060102")
	path := filepath.Join(dir, fmt.Sprintf("ledger-%s.jsonl", ts))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[GATE] WARNING: ledger write failed: %v", err)
		return
	}
	defer f.Close()
	line, _ := json.Marshal(entry)
	f.Write(line)
	f.Write([]byte("\n"))
}

func printLedgerSummary(dir string) {
	// Read last 30 days of ledger
	now := time.Now()
	total, admitted, blocked := 0, 0, 0
	causes := map[string]int{}

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format("20060102")
		path := filepath.Join(dir, fmt.Sprintf("ledger-%s.jsonl", date))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var e LedgerEntry
			if json.Unmarshal([]byte(line), &e) != nil {
				continue
			}
			total++
			switch e.FinalResult {
			case "admitted": admitted++
			case "blocked": blocked++
			}
			causes[e.CauseClass]++
		}
	}

	fmt.Printf("=== Admission Ledger (last 30 days) ===\n")
	fmt.Printf("Total: %d | Admitted: %d | Blocked: %d\n", total, admitted, blocked)
	fmt.Printf("Cause classes:\n")
	for cause, count := range causes {
		fmt.Printf("  %s: %d\n", cause, count)
	}
}

// --- Helpers ---

func (p *TaskPacket) TargetLane() string {
	if p.Lane != "" {
		return p.Lane
	}
	return "L1"
}

func (d *AdmissionDecision) preflightStatus(name string) string {
	for _, p := range d.Preflights {
		if p.Name == name {
			if p.Passed {
				return "passed"
			}
			return "failed"
		}
	}
	return "skipped"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func printDecision(d AdmissionDecision) {
	icon := "✅"
if !d.Allowed {
	icon = "❌"
}
	log.Printf("[GATE] %s %s | cause=%s | lane=%s | %s", icon, d.Reason, d.CauseClass, d.Lane, d.DecisionTime)
	for _, p := range d.Preflights {
		status := "✅"
		if p.Blocked {
			status = "🚫"
		} else if !p.Passed {
			status = "⚠️"
		}
		log.Printf("[GATE]   %s %s: %s (%s)", status, p.Name, p.Message, p.Duration)
	}
}

func warnSuffix(w string) string {
	if w == "" {
		return ""
	}
	return " (warnings: " + strings.TrimSuffix(w, "; ") + ")"
}
