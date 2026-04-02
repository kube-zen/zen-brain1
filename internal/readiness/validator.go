// Package readiness implements task-quality validation for zen-brain.
//
// A Jira item may enter READY / executable queues only if it meets the
// minimum executable contract (G013):
//  1. Problem statement (what is broken, where, expected vs actual)
//  2. Scope (affected component/service/path)
//  3. Evidence (log, repro steps, stack trace, or user-visible symptom)
//  4. Acceptance criteria (concrete end state)
//  5. Constraints / risk notes (if relevant)
//
// Tickets that fail are routed to triage, not dispatched to workers.
// "In Progress" with no executable contract is a process bug.
package readiness

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// ReadinessStatus represents whether a ticket is ready for execution.
type ReadinessStatus string

const (
	StatusReady    ReadinessStatus = "ready"
	StatusNotReady ReadinessStatus = "not_ready"
)

// FailureReason identifies what's missing from a ticket.
type FailureReason string

const (
	MissingProblemStatement   FailureReason = "missing_problem_statement"
	MissingScope              FailureReason = "missing_scope"
	MissingEvidence           FailureReason = "missing_repro_or_evidence"
	MissingAcceptanceCriteria FailureReason = "missing_acceptance_criteria"
	TitleTooGeneric           FailureReason = "title_too_generic"
	DescriptionTooShort       FailureReason = "description_too_short"
)

// CheckResult holds the result of a ticket readiness check.
type CheckResult struct {
	Status  ReadinessStatus `json:"status"`
	Reasons []FailureReason `json:"reasons,omitempty"`
	Score   int             `json:"score"` // 0-5, how many criteria met
	Total   int             `json:"total"` // always 5
	Action  string          `json:"action,omitempty"`
	Comment string          `json:"comment,omitempty"`
}

// RecommendedAction for non-executable tickets.
const (
	ActionMoveToNeedsDetail    = "move_to: NEEDS-DETAIL"
	ActionRequestClarification = "comment: request_missing_information"
	ActionDoNotDispatch        = "do_not_dispatch: true"
)

// ClarificationTemplate is the user-facing message for non-executable tickets.
const ClarificationTemplate = `Cannot start execution yet. Please provide:
1. what is broken (problem statement)
2. where it occurs (component/service/path)
3. repro steps or evidence (log, stack trace, screenshot)
4. expected vs actual behavior
5. acceptance criteria for done`

// Generic title phrases that indicate insufficient specification (G018).
var genericTitlePhrases = []string{
	"bug in code",
	"fix it",
	"investigate",
	"please check",
	"doesn't work",
	"not working",
	"broken",
	"error",
	"issue",
	"something wrong",
	"check this",
	"look into",
	"see this",
	"urgent", // "urgent" alone without context
}

// Minimum description length (characters) to be considered non-empty.
const minDescriptionLength = 50

// Minimum title length to not be flagged as too generic.
const minTitleLength = 15

// Validator checks ticket readiness against the executable contract.
type Validator struct {
	mu sync.RWMutex

	// Metrics
	TotalChecked     int            `json:"total_checked"`
	PassedCount      int            `json:"passed_count"`
	RejectedCount    int            `json:"rejected_count"`
	RejectionReasons map[string]int `json:"rejection_reasons"`
	AutoNormalized   int            `json:"auto_normalized_count"`
	SentBackCount    int            `json:"sent_back_count"`

	// Configuration
	RequireScope         bool `json:"require_scope"`       // always true
	RequireConstraints   bool `json:"require_constraints"` // false — optional
	MinDescriptionLength int  `json:"min_description_length"`
	MinTitleLength       int  `json:"min_title_length"`
}

// NewValidator creates a readiness validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		RejectionReasons:     make(map[string]int),
		RequireScope:         true,
		RequireConstraints:   false, // constraints are encouraged but not blocking
		MinDescriptionLength: minDescriptionLength,
		MinTitleLength:       minTitleLength,
	}
}

// TicketInput is the input for readiness validation.
// Designed to be populated from Jira issue fields.
type TicketInput struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Labels      []string `json:"labels,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Component   string   `json:"component,omitempty"`
}

// Check evaluates a ticket against the executable contract.
// Returns pass/fail with explicit reasons and recommended action.
func (v *Validator) Check(ticket TicketInput) CheckResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.TotalChecked++
	reasons := make([]FailureReason, 0)
	score := 0

	// ── Criterion 1: Problem Statement ──
	// Title must not be too generic, description must not be empty
	titleLower := strings.ToLower(strings.TrimSpace(ticket.Title))
	// Extract plain text from ADF description if present
	descriptionText := extractTextFromADF(ticket.Description)
	descLower := strings.ToLower(strings.TrimSpace(descriptionText))

	titleGeneric := false
	if len(strings.TrimSpace(ticket.Title)) < v.MinTitleLength {
		titleGeneric = true
		reasons = append(reasons, TitleTooGeneric)
	} else {
		for _, phrase := range genericTitlePhrases {
			if titleLower == phrase || titleLower == phrase+"." || titleLower == phrase+"!" {
				titleGeneric = true
				reasons = append(reasons, TitleTooGeneric)
				break
			}
		}
	}

	if titleGeneric || len(descLower) < v.MinDescriptionLength {
		reasons = append(reasons, MissingProblemStatement)
	} else {
		score++
	}

	// ── Criterion 2: Scope ──
	hasScope := false
	if ticket.Component != "" {
		hasScope = true
	}
	// Check if scope is mentioned in description or title
	scopeIndicators := []string{
		"component:", "service:", "module:", "directory:",
		"path:", "endpoint:", "api/", "controller:", "handler:", "package:",
		"internal/", "cmd/", "pkg/", "src/",
		" service ", // "auth service" matches
	}
	// Also check for file references (e.g. "handler.go", "main.go")
	scopeFilePattern := false
	if !hasScope {
		for _, word := range []string{ticket.Title, ticket.Description} {
			if containsFileRef(word) {
				scopeFilePattern = true
				break
			}
		}
	}
	if !hasScope {
		for _, indicator := range scopeIndicators {
			if strings.Contains(descLower, indicator) {
				hasScope = true
				break
			}
		}
	}
	if !hasScope {
		hasScope = scopeFilePattern
	}
	if !hasScope {
		for _, l := range ticket.Labels {
			if strings.Contains(strings.ToLower(l), "component:") ||
				strings.Contains(strings.ToLower(l), "area:") {
				hasScope = true
				break
			}
		}
	}
	if hasScope {
		score++
	} else if v.RequireScope {
		reasons = append(reasons, MissingScope)
	}

	// ── Criterion 3: Evidence / Repro ──
	hasEvidence := false
	evidenceIndicators := []string{
		"error", "exception", "stack", "trace", "log",
		"repro", "reproduce", "steps to", "steps:",
		"how to reproduce", "when i", "occurs when",
		"screenshot", "observed", "actual", "symptom",
		"panic", "crash", "fail", "timeout", "refused",
		"500", "404", "403", "nil pointer", "null pointer",
		"segfault", "oom", "out of memory",
		"expected:", "actual:", "got:", "want:",
		"before:", "after:", "diff",
		"https://", "http://", // URLs as evidence references
	}
	for _, indicator := range evidenceIndicators {
		if strings.Contains(descLower, indicator) {
			hasEvidence = true
			break
		}
	}
	// Also match "evidence:" and "acceptance:" with colon
	if !hasEvidence {
		if strings.Contains(descLower, "evidence:") || strings.Contains(descLower, "evidence ") {
			hasEvidence = true
		}
	}
	// Also check for structured evidence (code blocks, logs)
	if strings.Contains(descLower, "```") || strings.Contains(descLower, "traceback") {
		hasEvidence = true
	}
	if hasEvidence {
		score++
	} else {
		reasons = append(reasons, MissingEvidence)
	}

	// ── Criterion 4: Acceptance Criteria ──
	hasAcceptance := false
	acceptanceIndicators := []string{
		"acceptance", "criteria", "should", "must",
		"expected behavior", "correct behavior", "desired",
		"when fixed", "after fix", "success means",
		"verify:", "validation", "test case",
		"definition of done", "done when", "done if",
		"outcome", "result should", "should be able to",
	}
	for _, indicator := range acceptanceIndicators {
		if strings.Contains(descLower, indicator) {
			hasAcceptance = true
			break
		}
	}
	// Also match "acceptance criteria:" with colon
	if !hasAcceptance {
		if strings.Contains(descLower, "acceptance criteria:") || strings.Contains(descLower, "acceptance criteria ") {
			hasAcceptance = true
		}
	}
	// Title mentioning "should" or "must" counts lightly
	if !hasAcceptance && strings.Contains(titleLower, "should") {
		hasAcceptance = true
	}
	if hasAcceptance {
		score++
	} else {
		reasons = append(reasons, MissingAcceptanceCriteria)
	}

	// ── Criterion 5: Constraints / Risk (optional, not blocking) ──
	hasConstraints := false
	constraintIndicators := []string{
		"urgency", "deadline", "rollback", "blast radius",
		"dependency", "blocked by", "depends on",
		"risk", "impact", "affects", "breaking change",
		"reversible", "irreversible", "backwards compatible",
		"migration", "deploy", "release",
	}
	for _, indicator := range constraintIndicators {
		if strings.Contains(descLower, indicator) {
			hasConstraints = true
			break
		}
	}
	if hasConstraints {
		score++
	}
	// NOTE: missing constraints is NOT a failure reason (G013: "if relevant")

	// ── Verdict ──
	result := CheckResult{
		Status: StatusReady,
		Score:  score,
		Total:  5,
	}

	if len(reasons) > 0 {
		result.Status = StatusNotReady
		result.Reasons = reasons
		result.Action = ActionDoNotDispatch + "\n" + ActionMoveToNeedsDetail
		result.Comment = generateClarification(reasons)

		v.RejectedCount++
		for _, r := range reasons {
			v.RejectionReasons[string(r)]++
		}
		log.Printf("[READINESS] ❌ %s REJECTED: %s (score=%d/5)",
			ticket.Key, strings.Join(reasonsStr(reasons), ", "), score)
	} else {
		v.PassedCount++
		log.Printf("[READINESS] ✅ %s READY (score=%d/5)", ticket.Key, score)
	}

	return result
}

// generateClarification produces a focused message based on what's missing.
func generateClarification(reasons []FailureReason) string {
	var missing []string
	for _, r := range reasons {
		switch r {
		case MissingProblemStatement:
			missing = append(missing, "problem statement (what is broken and expected vs actual behavior)")
		case MissingScope:
			missing = append(missing, "scope (which component, service, or path is affected)")
		case MissingEvidence:
			missing = append(missing, "evidence or repro steps (logs, stack trace, screenshot, or how to trigger)")
		case MissingAcceptanceCriteria:
			missing = append(missing, "acceptance criteria (how do we know it's fixed)")
		case TitleTooGeneric:
			missing = append(missing, "more specific title (current title is too vague to understand the issue)")
		case DescriptionTooShort:
			missing = append(missing, "description (current description is too short)")
		}
	}

	if len(missing) == 0 {
		return ClarificationTemplate
	}

	return fmt.Sprintf("Cannot start execution yet. This ticket is missing:\n%s\n\nPlease update the ticket and it will be automatically re-evaluated.",
		joinNumbered(missing))
}

func joinNumbered(items []string) string {
	var sb strings.Builder
	for i, item := range items {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, item)
	}
	return sb.String()
}

func reasonsStr(reasons []FailureReason) []string {
	out := make([]string, len(reasons))
	for i, r := range reasons {
		out[i] = string(r)
	}
	return out
}

// containsFileRef checks if text contains a file reference like "handler.go", "main.go", etc.
func containsFileRef(text string) bool {
	// Match patterns like "something.go", "something.py", "something.ts", etc.
	// Also matches paths like "internal/auth/handler.go"
	dotFilePatterns := []string{".go", ".py", ".ts", ".js", ".rs", ".java", ".yaml", ".yml", ".toml", ".json", ".md"}
	for _, ext := range dotFilePatterns {
		if strings.Contains(text, ext) {
			return true
		}
	}
	return false
}

// Metrics returns a snapshot of validator metrics for observability (G017).
func (v *Validator) Metrics() ValidatorMetrics {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return ValidatorMetrics{
		TotalChecked:     v.TotalChecked,
		PassedCount:      v.PassedCount,
		RejectedCount:    v.RejectedCount,
		RejectionReasons: copyMap(v.RejectionReasons),
		AutoNormalized:   v.AutoNormalized,
		SentBackCount:    v.SentBackCount,
	}
}

// ValidatorMetrics is the observable state of the readiness validator.
type ValidatorMetrics struct {
	TotalChecked     int            `json:"total_checked"`
	PassedCount      int            `json:"passed_count"`
	RejectedCount    int            `json:"rejected_count"`
	RejectionReasons map[string]int `json:"rejection_reasons,omitempty"`
	AutoNormalized   int            `json:"auto_normalized_count"`
	SentBackCount    int            `json:"sent_back_count"`
	Timestamp        string         `json:"timestamp"`
}

// RecordMetrics writes metrics to a JSON file for observability.
func (v *Validator) RecordMetrics(path string) error {
	m := v.Metrics()
	m.Timestamp = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeToFile(path, data)
}

func copyMap(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// extractTextFromADF converts Jira ADF (Atlassian Document Format) to plain text.
// ADF is a JSON array of document nodes like:
//
//	[{"type":"paragraph","content":[{"type":"text","text":"Actual text"}]}]
//
// This function recursively extracts all "text" fields from the ADF structure.
func extractTextFromADF(adf interface{}) string {
	var sb strings.Builder

	switch v := adf.(type) {
	case string:
		// If it's already a string (not ADF), return it as-is
		return v

	case []interface{}:
		// ADF is typically an array of nodes
		for _, item := range v {
			sb.WriteString(extractTextFromADF(item))
		}

	case map[string]interface{}:
		// ADF node: look for "type" and "content" fields
		if nodeType, ok := v["type"].(string); ok {
			switch nodeType {
			case "paragraph", "heading", "listItem", "codeBlock", "blockquote":
				// Extract content recursively
				if content, ok := v["content"]; ok {
					sb.WriteString(extractTextFromADF(content))
					sb.WriteString("\n")
				}
				// codeBlock has direct text field
				if nodeType == "codeBlock" {
					if text, ok := v["text"].(string); ok {
						sb.WriteString(text)
						sb.WriteString("\n")
					}
				}
			case "text":
				// Direct text node - extract the "text" field
				if text, ok := v["text"].(string); ok {
					sb.WriteString(text)
					sb.WriteString(" ")
				}
			case "inlineCard", "bulletList", "orderedList", "rule", "panel":
				// These nodes have content but just extract it
				if content, ok := v["content"]; ok {
					sb.WriteString(extractTextFromADF(content))
					sb.WriteString("\n")
				}
			case "hardBreak":
				sb.WriteString("\n\n")
			}
		}

	case nil:
		// Ignore null values
	}

	return sb.String()
}
