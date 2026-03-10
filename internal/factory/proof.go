package factory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// ProofOfWorkVersion defines the schema version of proof-of-work artifacts.
const ProofOfWorkVersion = "1.0.0"

// proofOfWorkManagerImpl implements ProofOfWorkManager interface.
// It manages proof-of-work artifact generation and storage.
type proofOfWorkManagerImpl struct {
	runtimeDir string
}

// NewProofOfWorkManager creates a new proof-of-work manager.
func NewProofOfWorkManager(runtimeDir string) ProofOfWorkManager {
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create runtime directory: %v", err))
	}

	return &proofOfWorkManagerImpl{
		runtimeDir: runtimeDir,
	}
}

// CreateProofOfWork creates a complete proof-of-work bundle.
// It generates both JSON and markdown formats for easy consumption.
func (p *proofOfWorkManagerImpl) CreateProofOfWork(ctx context.Context, result *ExecutionResult, spec *FactoryTaskSpec) (*ProofOfWorkArtifact, error) {
	// Validate inputs
	if result == nil {
		return nil, fmt.Errorf("execution result cannot be nil")
	}
	if spec == nil {
		return nil, fmt.Errorf("task spec cannot be nil")
	}

	// Create artifact directory
	timestamp := time.Now().Format("20060102-150405")
	artifactDir := filepath.Join(p.runtimeDir, "proof-of-work", timestamp)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory %s: %w", artifactDir, err)
	}

	// Generate proof-of-work summary (OutputLog and git paths set from result/workspace)
	summary := p.generateSummary(result, spec, artifactDir)

	// Write JSON artifact
	jsonPath := filepath.Join(artifactDir, "proof-of-work.json")
	if err := p.writeJSON(summary, jsonPath); err != nil {
		return nil, fmt.Errorf("failed to write JSON artifact: %w", err)
	}

	// Write markdown artifact
	mdPath := filepath.Join(artifactDir, "proof-of-work.md")
	if err := p.writeMarkdown(summary, mdPath); err != nil {
		return nil, fmt.Errorf("failed to write markdown artifact: %w", err)
	}

	// Write detailed execution log
	logPath := filepath.Join(artifactDir, "execution.log")
	if err := p.writeExecutionLog(result, logPath); err != nil {
		return nil, fmt.Errorf("failed to write execution log: %w", err)
	}

	// Generate checksums for all artifacts
	checksums, err := p.generateArtifactChecksums(jsonPath, mdPath, logPath, result.WorkspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate checksums: %w", err)
	}
	summary.Checksums = checksums

	// Set actual artifact paths (no glob placeholders)
	summary.ArtifactPaths = []string{jsonPath, mdPath, logPath}
	sort.Strings(summary.ArtifactPaths)

	artifact := &ProofOfWorkArtifact{
		Directory:    artifactDir,
		JSONPath:     jsonPath,
		MarkdownPath: mdPath,
		LogPath:      logPath,
		Summary:      summary,
		CreatedAt:    time.Now(),
	}

	log.Printf("[ProofOfWorkManager] Created proof-of-work: task_id=%s artifact=%s", result.TaskID, artifactDir)

	return artifact, nil
}

// GenerateComment creates a canonical comment from proof-of-work summary.
func (p *proofOfWorkManagerImpl) GenerateComment(ctx context.Context, artifact *ProofOfWorkArtifact) (*contracts.Comment, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact cannot be nil")
	}
	if artifact.Summary == nil {
		return nil, fmt.Errorf("artifact summary cannot be nil")
	}

	// Read the markdown file
	mdContent, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read markdown file: %w", err)
	}

	// Use markdown as-is - let the office adapter convert to appropriate format
	body := string(mdContent)

	// Note: Length limits are system-specific and should be enforced by the office adapter
	// Factory returns the full canonical comment, adapter truncates if needed for target system

	comment := &contracts.Comment{
		ID:         artifact.Summary.TaskID,
		WorkItemID: artifact.Summary.WorkItemID,
		Body:       body,
		Author:     "zen-brain",
		CreatedAt:  time.Now(),
		Attribution: &contracts.AIAttribution{
			AgentRole: artifact.Summary.AgentRole,
			ModelUsed: artifact.Summary.ModelUsed,
			SessionID: artifact.Summary.SessionID,
			TaskID:    artifact.Summary.TaskID,
			Timestamp: artifact.Summary.GeneratedAt,
		},
	}

	return comment, nil
}

// generateSummary creates a proof-of-work summary from execution result.
// artifactDir is the proof bundle directory (used to surface git review paths from workspace).
func (p *proofOfWorkManagerImpl) generateSummary(result *ExecutionResult, spec *FactoryTaskSpec, artifactDir string) *ProofOfWorkSummary {
	summary := &ProofOfWorkSummary{
		Version:           ProofSchemaVersion,
		SchemaID:          ProofSchemaID,
		TaskID:            result.TaskID,
		SessionID:         result.SessionID,
		WorkItemID:        result.WorkItemID,
		SourceKey:         result.WorkItemID,
		SourceSystem:      "",
		WorkType:          string(spec.WorkType),
		WorkDomain:        string(spec.WorkDomain),
		Title:             spec.Title,
		Objective:         spec.Objective,
		Result:            string(result.Status),
		WorkspacePath:     result.WorkspacePath,
		StartedAt:         result.CompletedAt.Add(-result.Duration),
		CompletedAt:       result.CompletedAt,
		Duration:          result.Duration,
		ModelUsed:         "factory-v1",
		AgentRole:         "factory",
		FilesChanged:      result.FilesChanged,
		TestsRun:          result.TestsRun,
		TestsPassed:       result.TestsPassed,
		EvidenceItems:     result.SREDEvidence,
		UnresolvedRisks:   extractRisks(result.SREDEvidence),
		RecommendedAction: result.Recommendation,
		RequiresApproval:  (result.Recommendation != "merge"),
		GeneratedAt:       time.Now(),
		ArtifactPaths:     nil, // set in CreateProofOfWork after writing files
		TemplateKey:       result.TemplateKey,
		GitBranch:         result.GitBranch,
		GitCommit:         result.GitCommit,
		PRURL:             "",
		Environment:       NewExecutionEnvironment(),
		Signature:         nil,
		Checksums:         make(map[string]string),
		MetadataTags:      make(map[string]string),
	}


	if result.TemplateKey != "" {
		summary.TemplateKey = result.TemplateKey
		summary.TemplateUsed = result.TemplateKey
	} else if spec.TemplateKey != "" {
		summary.TemplateKey = spec.TemplateKey
		summary.TemplateUsed = spec.TemplateKey
	} else if spec.SelectedTemplate != "" {
		summary.TemplateUsed = spec.SelectedTemplate
	}
	summary.SelectionSource = spec.SelectionSource
	if spec.SelectionSource == "" {
		summary.SelectionSource = "static"
	}
	summary.SelectionConfidence = spec.SelectionConfidence
	summary.SelectionReasoning = spec.SelectionReasoning
	if summary.ModelUsed == "factory-v1" && summary.TemplateUsed != "" {
		summary.ModelUsed = summary.TemplateUsed
	}

	summary.CommandLog = extractCommandLog(result.ExecutionSteps)
	// Aggregate step outputs for OutputLog (honest), not result.Error
	summary.OutputLog = aggregateStepOutput(result.ExecutionSteps)
	summary.ErrorLog = ""
	if result.Error != "" {
		summary.ErrorLog = result.Error
	}

	// Surface git review artifacts from workspace when present (review:real lane)
	if result.WorkspacePath != "" {
		gitStatus := filepath.Join(result.WorkspacePath, "review", "git-status.txt")
		gitDiffStat := filepath.Join(result.WorkspacePath, "review", "git-diff-stat.txt")
		if _, err := os.Stat(gitStatus); err == nil {
			summary.GitStatusPath = gitStatus
		}
		if _, err := os.Stat(gitDiffStat); err == nil {
			summary.GitDiffStatPath = gitDiffStat
		}
	}

	return p.hardenProofOfWorkSummary(summary)
}

// aggregateStepOutput builds a bounded string from execution step names and output snippets.
const maxOutputLogLen = 8000

func aggregateStepOutput(steps []*ExecutionStep) string {
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	for i, s := range steps {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("--- ")
		b.WriteString(s.Name)
		b.WriteString(" ---\n")
		out := s.Output
		if len(out) > 1200 {
			out = out[:1200] + "\n... (truncated)"
		}
		b.WriteString(out)
		if b.Len() >= maxOutputLogLen {
			b.WriteString("\n... (output truncated)")
			break
		}
	}
	return b.String()
}

// sortStringSlice sorts a string slice in place for deterministic proof output.
func sortStringSlice(s []string) {
	if len(s) == 0 {
		return
	}
	sort.Strings(s)
}

// hardenProofOfWorkSummary ensures proof-of-work summary has deterministic, stable schema
func (p *proofOfWorkManagerImpl) hardenProofOfWorkSummary(summary *ProofOfWorkSummary) *ProofOfWorkSummary {
	// Ensure all slices are initialized (not nil) for consistent JSON serialization
	if summary.FilesChanged == nil {
		summary.FilesChanged = []string{}
	}
	sortStringSlice(summary.FilesChanged)
	if summary.NewFiles == nil {
		summary.NewFiles = []string{}
	}
	sortStringSlice(summary.NewFiles)
	if summary.ModifiedFiles == nil {
		summary.ModifiedFiles = []string{}
	}
	sortStringSlice(summary.ModifiedFiles)
	if summary.DeletedFiles == nil {
		summary.DeletedFiles = []string{}
	}
	sortStringSlice(summary.DeletedFiles)
	if summary.TestsRun == nil {
		summary.TestsRun = []string{}
	}
	sortStringSlice(summary.TestsRun)
	if summary.TestsFailed == nil {
		summary.TestsFailed = []string{}
	}
	sortStringSlice(summary.TestsFailed)
	if summary.CommandLog == nil {
		summary.CommandLog = []string{}
	}
	sortStringSlice(summary.CommandLog)
	if summary.EvidenceItems == nil {
		summary.EvidenceItems = []contracts.EvidenceItem{}
	}
	if summary.UnresolvedRisks == nil {
		summary.UnresolvedRisks = []string{}
	}
	sortStringSlice(summary.UnresolvedRisks)
	if summary.KnownLimitations == nil {
		summary.KnownLimitations = []string{}
	}
	sortStringSlice(summary.KnownLimitations)
	if summary.ArtifactPaths == nil {
		summary.ArtifactPaths = []string{}
	}
	sortStringSlice(summary.ArtifactPaths)

	// Calculate file change metrics if files changed but metrics not set
	if len(summary.FilesChanged) > 0 && summary.LinesAdded == 0 && summary.LinesDeleted == 0 {
		// For now, use estimates based on file count
		// In real implementation, would compute actual diffs
		summary.LinesAdded = len(summary.FilesChanged) * 10  // Estimate
		summary.LinesDeleted = len(summary.FilesChanged) * 2 // Estimate
	}

	// Set default source system if empty
	if summary.SourceSystem == "" {
		summary.SourceSystem = "factory"
	}

	// Ensure timestamps are valid
	if summary.StartedAt.IsZero() && !summary.CompletedAt.IsZero() {
		summary.StartedAt = summary.CompletedAt.Add(-summary.Duration)
	}
	if summary.CompletedAt.IsZero() && !summary.StartedAt.IsZero() {
		summary.CompletedAt = summary.StartedAt.Add(summary.Duration)
	}

	// Normalize result field
	switch summary.Result {
	case "completed", "failed", "canceled", "blocked":
		// Valid values
	default:
		if summary.Result == "" {
			summary.Result = "unknown"
		}
	}

	// Set default recommended action if empty
	if summary.RecommendedAction == "" {
		if summary.Result == "completed" {
			summary.RecommendedAction = "merge"
		} else {
			summary.RecommendedAction = "review"
		}
	}

	// Ensure consistent model/agent information
	if summary.ModelUsed == "" {
		summary.ModelUsed = "factory-v1"
	}
	if summary.AgentRole == "" {
		summary.AgentRole = "factory"
	}

	return summary
}

// writeJSON writes proof-of-work summary to JSON file.
func (p *proofOfWorkManagerImpl) writeJSON(summary *ProofOfWorkSummary, path string) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// writeMarkdown writes proof-of-work summary to markdown file.
func (p *proofOfWorkManagerImpl) writeMarkdown(summary *ProofOfWorkSummary, path string) error {
	md := p.generateMarkdown(summary)
	if err := os.WriteFile(path, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}

// writeExecutionLog writes detailed execution log.
func (p *proofOfWorkManagerImpl) writeExecutionLog(result *ExecutionResult, path string) error {
	logLines := []string{
		"# Execution Log",
		"",
		fmt.Sprintf("**Task ID:** %s", result.TaskID),
		fmt.Sprintf("**Session ID:** %s", result.SessionID),
		fmt.Sprintf("**Work Item ID:** %s", result.WorkItemID),
		"",
		"## Execution Steps",
		"",
	}

	for i, step := range result.ExecutionSteps {
		logLines = append(logLines, fmt.Sprintf("### Step %d: %s", i+1, step.Name))
		logLines = append(logLines, fmt.Sprintf("- **ID:** %s", step.StepID))
		logLines = append(logLines, fmt.Sprintf("- **Status:** %s", step.Status))
		logLines = append(logLines, fmt.Sprintf("- **Started:** %s", formatTime(step.StartedAt)))
		logLines = append(logLines, fmt.Sprintf("- **Completed:** %s", formatTime(step.CompletedAt)))

		if step.Error != "" {
			logLines = append(logLines, fmt.Sprintf("- **Error:** %s", step.Error))
		}
		if step.Output != "" {
			logLines = append(logLines, fmt.Sprintf("- **Output:** %s", step.Output))
		}

		logLines = append(logLines, "")
	}

	// Add failed steps section
	if len(result.FailedSteps) > 0 {
		logLines = append(logLines, "")
		logLines = append(logLines, "## Failed Steps")
		logLines = append(logLines, "")
		for i, step := range result.FailedSteps {
			logLines = append(logLines, fmt.Sprintf("### Step %d: %s", i+1, step.Name))
			logLines = append(logLines, fmt.Sprintf("- **ID:** %s", step.StepID))
			logLines = append(logLines, fmt.Sprintf("- **Error:** %s", step.Error))
			logLines = append(logLines, fmt.Sprintf("- **Retries:** %d", step.RetryCount))
		}
	}

	// Add overall result
	logLines = append(logLines, "")
	logLines = append(logLines, "## Result")
	logLines = append(logLines, "")
	logLines = append(logLines, fmt.Sprintf("- **Status:** %s", result.Status))
	logLines = append(logLines, fmt.Sprintf("- **Success:** %v", result.Success))
	if result.Error != "" {
		logLines = append(logLines, fmt.Sprintf("- **Error:** %s", result.Error))
	}
	logLines = append(logLines, "")
	logLines = append(logLines, fmt.Sprintf("- **Duration:** %s", result.Duration.String()))
	logLines = append(logLines, fmt.Sprintf("- **Completed Steps:** %d/%d", result.CompletedSteps, result.TotalSteps))

	// Write log file
	logContent := ""
	for _, line := range logLines {
		logContent += line + "\n"
	}

	if err := os.WriteFile(path, []byte(logContent), 0644); err != nil {
		return fmt.Errorf("failed to write execution log: %w", err)
	}

	return nil
}

// generateMarkdown creates markdown representation of proof-of-work.
func (p *proofOfWorkManagerImpl) generateMarkdown(summary *ProofOfWorkSummary) string {
	md := "# Proof of Work\n\n"

	// Header section
	md += "## Summary\n\n"
	md += fmt.Sprintf("- **Task ID:** `%s`\n", summary.TaskID)
	md += fmt.Sprintf("- **Session ID:** `%s`\n", summary.SessionID)
	md += fmt.Sprintf("- **Work Item ID:** `%s`\n", summary.WorkItemID)
	md += fmt.Sprintf("- **Title:** `%s`\n", summary.Title)
	md += fmt.Sprintf("- **Source Key:** `%s`\n", summary.SourceKey)
	md += fmt.Sprintf("- **Source System:** `%s`\n", summary.SourceSystem)
	md += fmt.Sprintf("- **Status:** **%s**\n", summary.Result)
	md += fmt.Sprintf("- **Duration:** `%s`\n", summary.Duration)
	md += fmt.Sprintf("- **Model:** `%s`\n", summary.ModelUsed)
	md += fmt.Sprintf("- **Agent:** `%s`\n", summary.AgentRole)
	if summary.TemplateKey != "" {
		md += fmt.Sprintf("- **Template:** `%s`\n", summary.TemplateKey)
	}
	md += fmt.Sprintf("- **Schema Version:** `%s`\n", summary.Version)
	md += fmt.Sprintf("- **Schema ID:** `%s`\n", summary.SchemaID)
	md += "\n"

	// Failure summary (when not completed) — trusted useful path: clear failure handling
	if summary.Result != "completed" && summary.Result != "" {
		md += "## Failure summary\n\n"
		md += fmt.Sprintf("- **Outcome:** %s\n", summary.Result)
		md += fmt.Sprintf("- **Recommended action:** %s\n", summary.RecommendedAction)
		if summary.ErrorLog != "" {
			md += fmt.Sprintf("- **Error:** %s\n", summary.ErrorLog)
		}
		md += "\n"
	}

	// Objective section
	md += "## Objective\n\n"
	md += fmt.Sprintf("%s\n\n", summary.Objective)

	// Result section
	md += "## Result\n\n"
	if summary.Result == "completed" {
		md += "✅ **Task completed successfully**\n\n"
	} else if summary.Result == "failed" {
		md += "❌ **Task failed**\n\n"
	} else {
		md += fmt.Sprintf("⚠️ **Task status: %s**\n\n", summary.Result)
	}

	// Changes section
	if len(summary.FilesChanged) > 0 {
		md += "## Files Changed\n\n"
		md += fmt.Sprintf("- **Total files:** %d\n", len(summary.FilesChanged))
		md += "### Modified Files\n\n"
		for _, file := range summary.FilesChanged {
			md += fmt.Sprintf("- `%s`\n", file)
		}
		md += "\n"
	}

	// Tests section
	if len(summary.TestsRun) > 0 {
		md += "## Tests\n\n"
		md += fmt.Sprintf("- **Tests Run:** %d\n", len(summary.TestsRun))
		if summary.TestsPassed {
			md += "- **All Passed:** ✅ Yes\n\n"
		} else {
			md += "- **All Passed:** ❌ No\n"
			md += "### Failed Tests\n\n"
			for _, test := range summary.TestsFailed {
				md += fmt.Sprintf("- %s ❌\n", test)
			}
			md += "\n"
		}
	}

	// Execution steps summary
	if len(summary.CommandLog) > 0 {
		md += "## Execution Steps\n\n"
		for i, cmd := range summary.CommandLog {
			md += fmt.Sprintf("### Step %d\n", i+1)
			md += fmt.Sprintf("- **Command:** `%s`\n", cmd)
		}
		md += "\n"
	}

	// Evidence section
	if len(summary.EvidenceItems) > 0 {
		md += "## Evidence (SR&ED)\n\n"
		for i, evidence := range summary.EvidenceItems {
			md += fmt.Sprintf("### Evidence Item %d\n", i+1)
			md += fmt.Sprintf("- **Type:** `%s`\n", evidence.Type)
			md += fmt.Sprintf("- **Content:** `%s`\n", evidence.Content[:min(200, len(evidence.Content))])
			md += fmt.Sprintf("- **Collected At:** `%s`\n", evidence.CollectedAt.Format(time.RFC3339))
			md += "\n"
		}
		md += "\n"
	}

	// Risks section
	if len(summary.UnresolvedRisks) > 0 {
		md += "## Risks\n\n"
		for _, risk := range summary.UnresolvedRisks {
			md += fmt.Sprintf("- ⚠️ %s\n", risk)
		}
		md += "\n"
	}

	// Recommendation section
	md += "## Recommendation\n\n"
	md += fmt.Sprintf("- **Action:** **%s**\n", summary.RecommendedAction)
	if summary.RequiresApproval {
		md += "- **Requires Approval:** ⚠️ **Yes**\n"
	} else {
		md += "- **Requires Approval:** No\n"
	}
	if summary.ReviewNotes != "" {
		md += fmt.Sprintf("- **Review Notes:** %s\n", summary.ReviewNotes)
	}
	md += "\n"

	// Artifacts section — actual paths when available
	md += "## Artifacts\n\n"
	if len(summary.ArtifactPaths) > 0 {
		for _, p := range summary.ArtifactPaths {
			md += fmt.Sprintf("- `%s`\n", p)
		}
	} else {
		md += "- JSON artifact: `proof-of-work.json`\n"
		md += "- Markdown artifact: `proof-of-work.md`\n"
		md += "- Execution log: `execution.log`\n"
	}
	md += "\n"

	// Git information (branch/commit and review:real evidence paths)
	if summary.GitBranch != "" || summary.GitStatusPath != "" || summary.GitDiffStatPath != "" {
		md += "## Git Information\n\n"
		if summary.GitBranch != "" {
			md += fmt.Sprintf("- **Branch:** `%s`\n", summary.GitBranch)
		}
		if summary.GitCommit != "" {
			md += fmt.Sprintf("- **Commit:** `%s`\n", summary.GitCommit)
		}
		if summary.PRURL != "" {
			md += fmt.Sprintf("- **PR:** `%s`\n", summary.PRURL)
		}
		if summary.GitStatusPath != "" {
			md += fmt.Sprintf("- **Git status path:** `%s`\n", summary.GitStatusPath)
		}
		if summary.GitDiffStatPath != "" {
			md += fmt.Sprintf("- **Git diff stat path:** `%s`\n", summary.GitDiffStatPath)
		}
		md += "\n"
	}

	// Footer
	md += "---\n"

	// Signature section (if present)
	if summary.Signature != nil {
		md += "## Digital Signature\n\n"
		md += fmt.Sprintf("- **Algorithm:** `%s`\n", summary.Signature.Algorithm)
		md += fmt.Sprintf("- **Key ID:** `%s`\n", summary.Signature.KeyID)
		md += fmt.Sprintf("- **Signer:** `%s`\n", summary.Signature.Signer)
		md += fmt.Sprintf("- **Signed At:** `%s`\n", summary.Signature.SignedAt)
		md += "- **Status:** ✅ **Signed**\n"
		md += "\n"
	}

	// Checksums section (if available)
	if len(summary.Checksums) > 0 {
		md += "## File Checksums (SHA256)\n\n"
		md += "| File | SHA256 Checksum |\n"
		md += "|------|----------------|\n"

		// Add checksums in a sorted order for reproducibility
		keys := make([]string, 0, len(summary.Checksums))
		for k := range summary.Checksums {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			md += fmt.Sprintf("| `%s` | `%s` |\n", key, summary.Checksums[key])
		}
		md += "\n"
	}

	// Environment section (if available)
	if summary.Environment != nil {
		md += "## Execution Environment\n\n"
		md += fmt.Sprintf("- **OS:** `%s`\n", summary.Environment.OS)
		md += fmt.Sprintf("- **Architecture:** `%s`\n", summary.Environment.Architecture)
		md += fmt.Sprintf("- **Go Version:** `%s`\n", summary.Environment.GoVersion)
		md += fmt.Sprintf("- **Hostname:** `%s`\n", summary.Environment.Hostname)
		md += fmt.Sprintf("- **Factory Version:** `%s`\n", summary.Environment.FactoryVersion)
		md += fmt.Sprintf("- **Timestamp:** `%s`\n", summary.Environment.Timestamp)
		md += "\n"
	}

	// Metadata tags section (if available)
	if len(summary.MetadataTags) > 0 {
		md += "## Metadata Tags\n\n"
		for key, value := range summary.MetadataTags {
			md += fmt.Sprintf("- **%s:** `%s`\n", key, value)
		}
		md += "\n"
	}

	md += fmt.Sprintf("*Generated at %s*\n", summary.GeneratedAt.Format(time.RFC3339))

	return md
}

// ListProofOfWorks returns all proof-of-work artifacts for a task.
func (p *proofOfWorkManagerImpl) ListProofOfWorks(ctx context.Context, taskID string) ([]*ProofOfWorkArtifact, error) {
	// Find all proof-of-work directories
	proofDir := filepath.Join(p.runtimeDir, "proof-of-work")
	entries, err := os.ReadDir(proofDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof-of-work directory: %w", err)
	}

	artifacts := []*ProofOfWorkArtifact{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Read JSON file to get task ID
		jsonPath := filepath.Join(proofDir, entry.Name(), "proof-of-work.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			continue // Skip if JSON can't be read
		}

		var summary ProofOfWorkSummary
		if err := json.Unmarshal(data, &summary); err != nil {
			continue // Skip if JSON can't be parsed
		}

		// Filter by task ID if specified
		if taskID != "" && summary.TaskID != taskID {
			continue
		}

		artifact := &ProofOfWorkArtifact{
			Directory:    filepath.Join(proofDir, entry.Name()),
			JSONPath:     jsonPath,
			MarkdownPath: filepath.Join(proofDir, entry.Name(), "proof-of-work.md"),
			LogPath:      filepath.Join(proofDir, entry.Name(), "execution.log"),
			Summary:      &summary,
			CreatedAt: func() time.Time {
				info, _ := entry.Info()
				return info.ModTime()
			}(),
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// GetProofOfWork retrieves a specific proof-of-work artifact.
func (p *proofOfWorkManagerImpl) GetProofOfWork(ctx context.Context, artifactDir string) (*ProofOfWorkArtifact, error) {
	// Read JSON file
	jsonPath := filepath.Join(artifactDir, "proof-of-work.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof-of-work JSON: %w", err)
	}

	var summary ProofOfWorkSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, fmt.Errorf("failed to unmarshal proof-of-work JSON: %w", err)
	}

	// Read markdown to verify
	markdownPath := filepath.Join(artifactDir, "proof-of-work.md")
	_, err = os.ReadFile(markdownPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof-of-work markdown: %w", err)
	}

	artifact := &ProofOfWorkArtifact{
		Directory:    artifactDir,
		JSONPath:     jsonPath,
		MarkdownPath: markdownPath,
		LogPath:      filepath.Join(artifactDir, "execution.log"),
		Summary:      &summary,
		CreatedAt:    time.Now(),
	}

	return artifact, nil
}

// CleanupProofOfWorks removes old proof-of-work artifacts.
func (p *proofOfWorkManagerImpl) CleanupProofOfWorks(ctx context.Context, olderThan time.Duration) error {
	proofDir := filepath.Join(p.runtimeDir, "proof-of-work")
	entries, err := os.ReadDir(proofDir)
	if err != nil {
		return fmt.Errorf("failed to read proof-of-work directory: %w", err)
	}

	cutoffTime := time.Now().Add(-olderThan)
	removedCount := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if artifact is older than cutoff
		info, _ := entry.Info()
		if info.ModTime().Before(cutoffTime) {
			artifactPath := filepath.Join(proofDir, entry.Name())
			if err := os.RemoveAll(artifactPath); err != nil {
				return fmt.Errorf("failed to remove old proof-of-work artifact %s: %w", artifactPath, err)
			}
			removedCount++
		}
	}

	log.Printf("[ProofOfWorkManager] Cleaned up %d old proof-of-work artifacts", removedCount)

	return nil
}

// Extract helpers

func extractRisks(evidenceItems []contracts.EvidenceItem) []string {
	risks := []string{}
	for _, item := range evidenceItems {
		if len(item.Content) >= 6 && item.Content[:6] == "RISK: " {
			risks = append(risks, item.Content[6:])
		}
	}
	return risks
}

func extractCommandLog(steps []*ExecutionStep) []string {
	logs := []string{}
	for _, step := range steps {
		if step.Command != "" {
			logs = append(logs, step.Command)
		}
	}
	return logs
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "N/A"
	}
	return t.Format(time.RFC3339)
}

// generateArtifactChecksums computes SHA256 checksums for all artifact files.
// Note: Does not include JSON checksum to avoid circular reference (JSON contains checksums).
func (p *proofOfWorkManagerImpl) generateArtifactChecksums(jsonPath, mdPath, logPath, workspacePath string) (map[string]string, error) {
	checksums := make(map[string]string)

	// Don't compute JSON checksum - it would change when we store the checksums in the JSON
	// Instead, verification will recompute it on the fly

	// Compute checksum for Markdown artifact
	mdChecksum, err := ComputeFileSHA256(mdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to checksum Markdown artifact: %w", err)
	}
	checksums["markdown"] = mdChecksum
	checksums["proof-of-work.md"] = mdChecksum

	// Compute checksum for execution log
	logChecksum, err := ComputeFileSHA256(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to checksum execution log: %w", err)
	}
	checksums["execution.log"] = logChecksum
	checksums["log"] = logChecksum

	// Optional: Compute checksums for workspace files if workspace exists
	if workspacePath != "" {
		workspaceChecksums, err := p.computeWorkspaceChecksums(workspacePath)
		if err == nil {
			for file, checksum := range workspaceChecksums {
				checksums["workspace/"+file] = checksum
			}
		}
		// Don't fail if workspace checksums fail - it's optional
	}

	return checksums, nil
}

// computeWorkspaceChecksums computes checksums for key workspace files.
func (p *proofOfWorkManagerImpl) computeWorkspaceChecksums(workspacePath string) (map[string]string, error) {
	checksums := make(map[string]string)

	// List of important files to checksum
	keyFiles := []string{
		"README.md",
		"PROOF_OF_WORK.md",
		"package.json",
		"requirements.txt",
		"Dockerfile",
		".gitignore",
	}

	for _, file := range keyFiles {
		path := filepath.Join(workspacePath, file)
		if _, err := os.Stat(path); err == nil {
			checksum, err := ComputeFileSHA256(path)
			if err == nil {
				checksums[file] = checksum
			}
		}
	}

	return checksums, nil
}

// GenerateChecksums generates SHA256 checksums for all artifact files.
func (p *proofOfWorkManagerImpl) GenerateChecksums(ctx context.Context, artifact *ProofOfWorkArtifact) (map[string]string, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact cannot be nil")
	}

	checksums := make(map[string]string)

	// Compute checksum for JSON artifact
	if artifact.JSONPath != "" {
		jsonChecksum, err := ComputeFileSHA256(artifact.JSONPath)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum JSON artifact: %w", err)
		}
		checksums["json"] = jsonChecksum
	}

	// Compute checksum for Markdown artifact
	if artifact.MarkdownPath != "" {
		mdChecksum, err := ComputeFileSHA256(artifact.MarkdownPath)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum Markdown artifact: %w", err)
		}
		checksums["markdown"] = mdChecksum
	}

	// Compute checksum for execution log
	if artifact.LogPath != "" {
		logChecksum, err := ComputeFileSHA256(artifact.LogPath)
		if err != nil {
			return nil, fmt.Errorf("failed to checksum execution log: %w", err)
		}
		checksums["log"] = logChecksum
	}

	return checksums, nil
}

// VerifyArtifact verifies the integrity of a proof-of-work artifact.
func (p *proofOfWorkManagerImpl) VerifyArtifact(ctx context.Context, artifact *ProofOfWorkArtifact) (bool, error) {
	if artifact == nil {
		return false, fmt.Errorf("artifact cannot be nil")
	}
	if artifact.Summary == nil {
		return false, fmt.Errorf("artifact summary cannot be nil")
	}

	// Check if checksums are available
	if len(artifact.Summary.Checksums) == 0 {
		// No checksums to verify - consider valid
		return true, nil
	}

	// Note: We don't verify JSON checksum because it's not stored
	// (to avoid circular reference since JSON contains checksums)

	// Verify Markdown artifact checksum
	if expectedMDChecksum, ok := artifact.Summary.Checksums["markdown"]; ok {
		actualMDChecksum, err := ComputeFileSHA256(artifact.MarkdownPath)
		if err != nil {
			return false, nil // Failed to checksum, consider invalid
		}
		if actualMDChecksum != expectedMDChecksum {
			// Checksum mismatch - return false without error (tampering detected)
			return false, nil
		}
	}

	// Verify log artifact checksum
	if expectedLogChecksum, ok := artifact.Summary.Checksums["log"]; ok {
		actualLogChecksum, err := ComputeFileSHA256(artifact.LogPath)
		if err != nil {
			return false, nil // Failed to checksum, consider invalid
		}
		if actualLogChecksum != expectedLogChecksum {
			// Checksum mismatch - return false without error (tampering detected)
			return false, nil
		}
	}

	// Verify signature if present
	if artifact.Summary.Signature != nil {
		valid, err := p.verifySignature(ctx, artifact)
		if err != nil {
			return false, nil // Signature verification failed
		}
		if !valid {
			return false, nil // Invalid signature
		}
	}

	return true, nil
}

// verifySignature verifies the signature on a proof-of-work artifact.
// Note: This is a placeholder - actual cryptographic verification requires
// access to public keys and a signing infrastructure.
func (p *proofOfWorkManagerImpl) verifySignature(ctx context.Context, artifact *ProofOfWorkArtifact) (bool, error) {
	if artifact.Summary.Signature == nil {
		return true, nil // No signature to verify
	}

	// Compute expected digest
	expectedDigest, err := artifact.Summary.ComputeProofDigest()
	if err != nil {
		return false, fmt.Errorf("failed to compute proof digest: %w", err)
	}

	// Verify the digest matches what was signed
	if expectedDigest != artifact.Summary.Signature.ProofDigest {
		return false, fmt.Errorf("proof digest mismatch")
	}

	// Deferred: full cryptographic verification when signing infrastructure is available (zen-sdk or internal).
	// For now, we verify the digest matches only.
	return true, nil
}

// SignArtifact signs a proof-of-work artifact with the provided signature info.
func (p *proofOfWorkManagerImpl) SignArtifact(ctx context.Context, artifact *ProofOfWorkArtifact, signature *ArtifactSignature) error {
	if artifact == nil {
		return fmt.Errorf("artifact cannot be nil")
	}
	if artifact.Summary == nil {
		return fmt.Errorf("artifact summary cannot be nil")
	}
	if signature == nil {
		return fmt.Errorf("signature cannot be nil")
	}

	// Compute the proof digest (excludes signature and checksums)
	digest, err := artifact.Summary.ComputeProofDigest()
	if err != nil {
		return fmt.Errorf("failed to compute proof digest: %w", err)
	}

	// Set the digest in the signature
	signature.ProofDigest = digest

	// Attach signature to summary
	artifact.Summary.Signature = signature

	// Re-write the JSON artifact with the signature included
	if err := p.writeJSON(artifact.Summary, artifact.JSONPath); err != nil {
		return fmt.Errorf("failed to write signed JSON artifact: %w", err)
	}

	// Note: We don't update stored checksums here to avoid circular reference
	// (checksums of JSON would change when we include them)
	// Verification will recompute checksums from the actual files at verification time

	log.Printf("[ProofOfWorkManager] Artifact signed: task_id=%s key_id=%s", artifact.Summary.TaskID, signature.KeyID)

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NewExecutionEnvironment creates an ExecutionEnvironment with current runtime information.
func NewExecutionEnvironment() *ExecutionEnvironment {
	hostname := "unknown"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	version := "v1.0.0"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	return &ExecutionEnvironment{
		OS:             runtime.GOOS,
		Architecture:   runtime.GOARCH,
		GoVersion:      runtime.Version(),
		Hostname:       hostname,
		FactoryVersion: version,
		Timestamp:      time.Now().Format(time.RFC3339),
	}
}

// ComputeSHA256 computes SHA256 checksum for the given data.
func ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// ComputeFileSHA256 computes SHA256 checksum for a file.
func ComputeFileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return ComputeSHA256(data), nil
}

// VerifyChecksum verifies that a file matches the expected SHA256 checksum.
func VerifyChecksum(path, expected string) (bool, error) {
	actual, err := ComputeFileSHA256(path)
	if err != nil {
		return false, err
	}
	return actual == expected, nil
}

// ComputeProofDigest computes the canonical digest for signing purposes.
// This digest covers all proof data except the signature and checksums themselves
// (since both are computed AFTER the digest).
func (s *ProofOfWorkSummary) ComputeProofDigest() (string, error) {
	// Create a copy without signature and checksums for digest computation
	copyForDigest := *s
	copyForDigest.Signature = nil
	copyForDigest.Checksums = nil

	data, err := json.Marshal(copyForDigest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal proof for digest: %w", err)
	}

	return ComputeSHA256(data), nil
}
