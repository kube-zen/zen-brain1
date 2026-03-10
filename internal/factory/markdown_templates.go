package factory

import (
	"fmt"
	"strings"
	"time"
)

// MarkdownProofOfWorkTemplate generates human-readable markdown proof-of-work.
type MarkdownProofOfWorkTemplate struct {
	JSONProof *ProofOfWorkSummary
}

// NewMarkdownProofOfWorkTemplate creates a new markdown template.
func NewMarkdownProofOfWorkTemplate(jsonProof *ProofOfWorkSummary) *MarkdownProofOfWorkTemplate {
	return &MarkdownProofOfWorkTemplate{
		JSONProof: jsonProof,
	}
}

// Generate creates a markdown proof-of-work document.
func (m *MarkdownProofOfWorkTemplate) Generate() string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Proof of Work\n\n")
	sb.WriteString(fmt.Sprintf("**Task ID:** `%s`\n", m.JSONProof.TaskID))
	sb.WriteString(fmt.Sprintf("**Session ID:** `%s`\n", m.JSONProof.SessionID))
	sb.WriteString(fmt.Sprintf("**Work Item ID:** `%s`\n", m.JSONProof.WorkItemID))
	sb.WriteString(fmt.Sprintf("**Source Key:** `%s`\n", m.JSONProof.SourceKey))
	sb.WriteString(fmt.Sprintf("**Title:** `%s`\n", m.JSONProof.Title))
	sb.WriteString(fmt.Sprintf("**Source System:** `%s`\n\n", m.getSourceSystem()))

	// Objective
	sb.WriteString("## Objective\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", m.JSONProof.Objective))

	// Result
	sb.WriteString("## Result\n\n")
	if m.JSONProof.Result == "completed" {
		sb.WriteString("✅ **Task completed successfully**\n\n")
	} else if m.JSONProof.Result == "failed" {
		sb.WriteString("❌ **Task failed**\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Status:** %s\n\n", m.JSONProof.Result))
	}

	// Duration
	if m.JSONProof.Duration > 0 {
		sb.WriteString(fmt.Sprintf("**Duration:** `%s`\n\n", m.JSONProof.Duration))
	}

	// Model and Agent
	sb.WriteString(fmt.Sprintf("**Model:** `%s`\n", m.JSONProof.ModelUsed))
	sb.WriteString(fmt.Sprintf("**Agent:** `%s`\n\n", m.JSONProof.AgentRole))

	// Execution Steps
	sb.WriteString("## Execution Steps\n\n")
	if len(m.JSONProof.CommandLog) == 0 {
		sb.WriteString("*No execution steps recorded*\n\n")
	} else {
		for i, command := range m.JSONProof.CommandLog {
			sb.WriteString(fmt.Sprintf("### Step %d\n\n", i+1))
			sb.WriteString(fmt.Sprintf("**Command:** `%s`\n", command))
		}
		sb.WriteString("\n")
	}

	// Files Changed
	sb.WriteString("## Files Changed\n\n")
	if len(m.JSONProof.FilesChanged) == 0 {
		sb.WriteString("*No files changed*\n\n")
	} else {
		for _, file := range m.JSONProof.FilesChanged {
			sb.WriteString(fmt.Sprintf("- `%s`\n", file))
		}
		sb.WriteString("\n")
	}

	// Git evidence (review:real lane)
	if m.JSONProof.GitStatusPath != "" || m.JSONProof.GitDiffStatPath != "" {
		sb.WriteString("## Git Evidence\n\n")
		if m.JSONProof.GitStatusPath != "" {
			sb.WriteString(fmt.Sprintf("- **Git status:** `%s`\n", m.JSONProof.GitStatusPath))
		}
		if m.JSONProof.GitDiffStatPath != "" {
			sb.WriteString(fmt.Sprintf("- **Git diff stat:** `%s`\n", m.JSONProof.GitDiffStatPath))
		}
		sb.WriteString("\n")
	}

	// Evidence and Artifacts
	sb.WriteString("## Evidence and Artifacts\n\n")
	if len(m.JSONProof.ArtifactPaths) == 0 {
		sb.WriteString("*No artifacts recorded*\n\n")
	} else {
		for _, artifactPath := range m.JSONProof.ArtifactPaths {
			sb.WriteString(fmt.Sprintf("- `%s`\n", artifactPath))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("*Generated: %s*\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("*Work Item: %s*\n", m.JSONProof.WorkItemID))

	return sb.String()
}

// getSourceSystem returns human-readable source system name.
func (m *MarkdownProofOfWorkTemplate) getSourceSystem() string {
	switch m.JSONProof.SourceSystem {
	case "jira":
		return "Jira"
	case "github":
		return "GitHub"
	default:
		return m.JSONProof.SourceSystem
	}
}

// formatStatus returns formatted status with emoji.
func (m *MarkdownProofOfWorkTemplate) formatStatus() string {
	switch m.JSONProof.Result {
	case "completed":
		return "✅ Completed"
	case "failed":
		return "❌ Failed"
	case "cancelled":
		return "⏹ Cancelled"
	case "in_progress":
		return "⏳ In Progress"
	default:
		return m.JSONProof.Result
	}
}

// truncateString limits string length and adds ellipsis if needed.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
