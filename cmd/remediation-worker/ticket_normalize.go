package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ─── Ticket Normalization ─────────────────────────────────────────────
// Normalizes raw L1 output into a strict internal structure before Jira write.
// This ensures Jira receives quality-gated, execution-ready ticket payloads.

// NormalizedTicketPayload is the strict structure that Jira receives.
type NormalizedTicketPayload struct {
	JiraKey         string `json:"jira_key"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	Problem         string `json:"problem"`
	Evidence        string `json:"evidence"`
	ExpectedOutcome string `json:"expected_outcome"`
	FixDirection    string `json:"fix_direction"`
	TargetFiles     string `json:"target_files"`
	Validation      string `json:"validation"`
	// Governance
	HumanApprovalLevel int    `json:"human_approval_level"`
	RelatedProject     string `json:"related_project"`
	QueueLevel         int    `json:"queue_level"`
	SREDUncertainty    string `json:"sred_uncertainty,omitempty"`
	IRAPWorkPackage    string `json:"irap_work_package,omitempty"`
	EvidencePackLink   string `json:"evidence_pack_link,omitempty"`
	// Routing
	RoutingRecommendation string `json:"routing_recommendation"` // bounded_fix_l1, bounded_synthesis_l2, manual_review
	// Dedup
	DedupStatus string `json:"dedup_status"` // create_new, update_existing, ignore_noise
}

// TicketQualityScore rates ticket readiness 0-5.
type TicketQualityScore struct {
	Clarity              int `json:"clarity"`               // 0-5: is the problem clear?
	EvidenceQuality      int `json:"evidence_quality"`      // 0-5: is evidence specific?
	Boundedness          int `json:"boundedness"`           // 0-5: is the scope bounded?
	ValidationClarity    int `json:"validation_clarity"`    // 0-5: can success be verified?
	GovernanceCompletion int `json:"governance_completion"` // 0-5: are governance fields filled?
	Total                int `json:"total"`                 // sum 0-25
}

// TicketReadinessStatus is the final readiness classification.
type TicketReadinessStatus string

const (
	ReadyForExecution       TicketReadinessStatus = "ready_for_execution"
	ReadyWithReview         TicketReadinessStatus = "ready_with_review"
	NeedsReview             TicketReadinessStatus = "needs_review"
	BlockedMissingContext   TicketReadinessStatus = "blocked_missing_context"
	BlockedMissingGovernance TicketReadinessStatus = "blocked_missing_governance"
	InvalidTicketPayload    TicketReadinessStatus = "invalid_ticket_payload"
)

// TicketQualityReport combines readiness + score + issues.
type TicketQualityReport struct {
	JiraKey   string                `json:"jira_key"`
	Readiness TicketReadinessStatus `json:"readiness"`
	Score     TicketQualityScore    `json:"score"`
	Issues    []string              `json:"issues,omitempty"`
}

// ─── Normalization Functions ──────────────────────────────────────────

// stripMarkdownFences removes ``` markers from L1 output.
func stripMarkdownFences(content string) string {
	lines := strings.Split(content, "\n")
	var clean []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			continue
		}
		clean = append(clean, line)
	}
	return strings.Join(clean, "\n")
}

// stripFilePrefix removes "config/path.yaml" prefixes that 0.8b sometimes prepends.
func stripFilePrefix(content string) string {
	// Match lines that look like file paths at the start of content
	re := regexp.MustCompile(`^(?:\./)?(?:config|internal|docs|scripts|cmd)/[\w./-]+\.(?:yaml|yml|go|json|md|sh|toml)\n`)
	content = re.ReplaceAllString(content, "")
	// Strip leading --- document markers
	content = strings.TrimPrefix(content, "---")
	content = strings.TrimPrefix(content, "\n")
	return strings.TrimSpace(content)
}

// normalizeL1Output applies all normalization steps to raw L1 content.
func normalizeL1Output(raw string) string {
	content := strings.TrimSpace(raw)
	content = stripMarkdownFences(content)
	content = stripFilePrefix(content)
	// Remove trailing ``` if any survived
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	return content
}

// ─── Normalized Payload Builder ───────────────────────────────────────

// buildNormalizedPayload creates a quality-gated ticket payload from L1 output + packet context.
func buildNormalizedPayload(ticket RemediationTicket, result *RemediationOutput, packet RemediationPacket) NormalizedTicketPayload {
	payload := NormalizedTicketPayload{
		JiraKey:               ticket.Key,
		Title:                 ticket.Summary,
		Summary:               truncate(ticket.Description, 500),
		Problem:               extractProblem(ticket.Description),
		Evidence:              extractEvidence(packet.EvidencePaths, ticket.Description),
		ExpectedOutcome:       truncate(result.Explanation, 300),
		FixDirection:          truncate(result.EditDescription, 300),
		TargetFiles:           result.FileToEdit,
		Validation:            result.ValidationResult,
		HumanApprovalLevel:    ticket.ApprovalLevel,
		RelatedProject:        ticket.RelatedProject,
		QueueLevel:            ticket.QueueLevel,
		SREDUncertainty:       ticket.SREDCode,
		IRAPWorkPackage:       ticket.IRAPWorkPkg,
		EvidencePackLink:      ticket.EvidenceLink,
		RoutingRecommendation: determineRouting(result),
		DedupStatus:           "update_existing",
	}

	// Fill governance defaults if missing
	if payload.RelatedProject == "" {
		payload.RelatedProject = "zen-brain1"
	}
	if payload.QueueLevel == 0 {
		payload.QueueLevel = 1
	}
	if payload.HumanApprovalLevel == 0 {
		payload.HumanApprovalLevel = 2
	}
	if payload.RoutingRecommendation == "" {
		payload.RoutingRecommendation = "manual_review"
	}

	return payload
}

// ─── Quality Gate ─────────────────────────────────────────────────────

// qualityGate checks if a normalized payload meets minimum quality standards.
func qualityGate(payload NormalizedTicketPayload) TicketQualityReport {
	report := TicketQualityReport{
		JiraKey: payload.JiraKey,
	}

	// Score each dimension 0-5
	report.Score.Clarity = scoreDimension(payload.Problem != "", payload.Summary != "", len(payload.Problem) > 20)
	report.Score.EvidenceQuality = scoreDimension(payload.Evidence != "", strings.Contains(payload.Evidence, "/"), len(payload.Evidence) > 10)
	report.Score.Boundedness = scoreDimension(payload.TargetFiles != "", payload.ExpectedOutcome != "", payload.FixDirection != "")
	report.Score.ValidationClarity = scoreDimension(payload.Validation != "", len(payload.Validation) > 10)
	report.Score.GovernanceCompletion = scoreDimension(
		payload.RelatedProject != "",
		payload.RoutingRecommendation != "",
		payload.HumanApprovalLevel > 0,
		payload.SREDUncertainty != "" || payload.IRAPWorkPackage != "",
	)

	report.Score.Total = report.Score.Clarity + report.Score.EvidenceQuality +
		report.Score.Boundedness + report.Score.ValidationClarity +
		report.Score.GovernanceCompletion

	// Required fields check
	missing := []string{}
	if payload.Title == "" {
		missing = append(missing, "title")
	}
	if payload.Problem == "" {
		missing = append(missing, "problem")
	}
	if payload.Evidence == "" {
		missing = append(missing, "evidence")
	}
	if payload.ExpectedOutcome == "" {
		missing = append(missing, "expected_outcome")
	}
	if payload.Validation == "" {
		missing = append(missing, "validation")
	}
	if payload.RoutingRecommendation == "" {
		missing = append(missing, "routing_recommendation")
	}

	report.Issues = missing

	// Classify readiness
	switch {
	case len(missing) == 0 && report.Score.Total >= 20:
		report.Readiness = ReadyForExecution
	case len(missing) == 0 && report.Score.Total >= 15:
		report.Readiness = ReadyWithReview
	case len(missing) <= 2 && report.Score.Total >= 10:
		report.Readiness = NeedsReview
	case len(missing) > 2:
		report.Readiness = BlockedMissingContext
	default:
		report.Readiness = BlockedMissingGovernance
	}

	return report
}

// ─── Jira Body Builder ────────────────────────────────────────────────

// buildJiraCommentBody produces a human/AI-readable comment from a quality-gated payload.
func buildJiraCommentBody(payload NormalizedTicketPayload, report TicketQualityReport, epPath string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("[zen-brain1 remediation pilot — Phase 39]\n\n"))
	sb.WriteString(fmt.Sprintf("## Remediation Result: %s\n\n", payload.JiraKey))
	sb.WriteString(fmt.Sprintf("**Readiness:** %s (score: %d/25)\n", report.Readiness, report.Score.Total))
	sb.WriteString(fmt.Sprintf("**Routing:** %s\n", payload.RoutingRecommendation))
	sb.WriteString(fmt.Sprintf("**Dedup:** %s\n\n", payload.DedupStatus))

	sb.WriteString("### Problem\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", payload.Problem))

	sb.WriteString("### What Was Done\n")
	sb.WriteString(fmt.Sprintf("- Type: %s\n", payload.FixDirection))
	sb.WriteString(fmt.Sprintf("- Target: %s\n", payload.TargetFiles))
	sb.WriteString(fmt.Sprintf("- Explanation: %s\n\n", payload.ExpectedOutcome))

	sb.WriteString("### Evidence\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", payload.Evidence))

	sb.WriteString("### Validation\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", payload.Validation))

	sb.WriteString("### Governance\n")
	sb.WriteString(fmt.Sprintf("- Related Project: %s\n", payload.RelatedProject))
	sb.WriteString(fmt.Sprintf("- Human Approval Level: %d\n", payload.HumanApprovalLevel))
	sb.WriteString(fmt.Sprintf("- Queue Level: %d\n", payload.QueueLevel))
	if payload.SREDUncertainty != "" {
		sb.WriteString(fmt.Sprintf("- SR&ED Uncertainty: %s\n", payload.SREDUncertainty))
	}
	if payload.IRAPWorkPackage != "" {
		sb.WriteString(fmt.Sprintf("- IRAP Work Package: %s\n", payload.IRAPWorkPackage))
	}
	if epPath != "" {
		sb.WriteString(fmt.Sprintf("- Evidence Pack: %s\n", epPath))
	}

	if len(report.Issues) > 0 {
		sb.WriteString(fmt.Sprintf("\n### Quality Issues\n"))
		for _, issue := range report.Issues {
			sb.WriteString(fmt.Sprintf("- Missing: %s\n", issue))
		}
	}

	return sb.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func scoreDimension(conditions ...bool) int {
	score := 0
	for _, c := range conditions {
		if c {
			score++
		}
	}
	// Cap at 5
	if score > 5 {
		score = 5
	}
	return score
}

func extractProblem(desc string) string {
	// Extract from "Problem:" section if present
	re := regexp.MustCompile(`(?i)Problem:\s*(.+?)(?:\n\n|\nEvidence|\nImpact|\nFix|$)`)
	if m := re.FindStringSubmatch(desc); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Fallback: first 300 chars
	return truncate(desc, 300)
}

func extractEvidence(evidencePaths, desc string) string {
	parts := []string{}
	if evidencePaths != "" {
		parts = append(parts, "Source artifacts: "+evidencePaths)
	}
	// Extract from "Evidence:" section in description
	re := regexp.MustCompile(`(?i)Evidence:\s*(.+?)(?:\n\n|\nImpact|\nFix|$)`)
	if m := re.FindStringSubmatch(desc); len(m) > 1 {
		parts = append(parts, strings.TrimSpace(m[1]))
	}
	return strings.Join(parts, " | ")
}

func determineRouting(result *RemediationOutput) string {
	switch result.FinalStatus {
	case "success":
		return "bounded_fix_l1"
	case "needs_review":
		return "bounded_fix_l1"
	case "to_escalate":
		return "bounded_synthesis_l2"
	case "blocked":
		return "manual_review"
	default:
		return "manual_review"
	}
}

// Ensure compile-time interface check
var _ = json.Marshal
