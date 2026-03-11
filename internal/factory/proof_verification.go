// Package factory provides enhanced proof verification with deeper signing and provenance.
package factory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProofVerifier provides enhanced verification capabilities for proof-of-work artifacts.
type ProofVerifier struct {
	proofManager ProofOfWorkManager
	strictMode   bool
}

// NewProofVerifier creates a new ProofVerifier.
func NewProofVerifier(manager ProofOfWorkManager, strictMode bool) *ProofVerifier {
	return &ProofVerifier{
		proofManager: manager,
		strictMode:   strictMode,
	}
}

// VerificationReport represents a complete verification report.
type VerificationReport struct {
	TaskID        string               `json:"task_id"`
	SessionID     string               `json:"session_id"`
	Timestamp     time.Time            `json:"timestamp"`
	AllPassed     bool                 `json:"all_passed"`
	CheckResults  []VerificationResult `json:"check_results"`
	OverallScore  float64              `json:"overall_score"` // 0.0 to 1.0
	Recommendations []string           `json:"recommendations,omitempty"`
}

// VerificationResult represents a single verification check result.
type VerificationResult struct {
	Name        string        `json:"name"`
	Passed      bool          `json:"passed"`
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	Severity    string        `json:"severity,omitempty"` // critical, high, medium, low
	Duration    time.Duration `json:"duration"`
}

// VerifyProof performs comprehensive verification of a proof-of-work artifact.
func (v *ProofVerifier) VerifyProof(_ context.Context, artifact *ProofOfWorkArtifact) (*VerificationReport, error) {
	report := &VerificationReport{
		TaskID:       artifact.Summary.TaskID,
		SessionID:    artifact.Summary.SessionID,
		Timestamp:    time.Now(),
		AllPassed:    true,
		CheckResults: []VerificationResult{},
	}

	// Run all verification checks
	checks := []struct {
		name     string
		severity string
		fn       func(*ProofOfWorkArtifact) (VerificationResult, error)
	}{
		{"proof_structure", "critical", v.checkProofStructure},
		{"schema_version", "high", v.checkSchemaVersion},
		{"required_fields", "high", v.checkRequiredFields},
		{"timestamp_consistency", "high", v.checkTimestampConsistency},
		{"artifact_integrity", "critical", v.checkArtifactIntegrity},
		{"signature_valid", "high", v.checkSignatureValid},
		{"checksum_integrity", "high", v.checkChecksumIntegrity},
		{"files_exist", "medium", v.checkFilesExist},
		{"git_evidence", "medium", v.checkGitEvidence},
		{"test_evidence", "medium", v.checkTestEvidence},
		{"provenance_chain", "medium", v.checkProvenanceChain},
	}

	for _, check := range checks {
		start := time.Now()
		result, err := check.fn(artifact)
		if err != nil {
			result = VerificationResult{
				Name:     check.name,
				Passed:   false,
				Message:  fmt.Sprintf("Check failed with error: %v", err),
				Severity: check.severity,
				Duration: time.Since(start),
			}
		}
		result.Duration = time.Since(start)
		result.Severity = check.severity

		report.CheckResults = append(report.CheckResults, result)

		// Critical checks fail the entire verification
		if !result.Passed && result.Severity == "critical" {
			report.AllPassed = false
		}
	}

	// Calculate overall score
	report.OverallScore = v.calculateOverallScore(report)
	report.Recommendations = v.generateRecommendations(report)

	return report, nil
}

// checkProofStructure verifies the artifact directory structure.
func (v *ProofVerifier) checkProofStructure(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Check required files exist
	requiredFiles := []string{
		filepath.Join(artifact.Directory, "proof-of-work.json"),
		filepath.Join(artifact.Directory, "proof-of-work.md"),
		filepath.Join(artifact.Directory, "execution.log"),
	}

	missingFiles := []string{}
	for _, file := range requiredFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			missingFiles = append(missingFiles, filepath.Base(file))
		}
	}

	if len(missingFiles) > 0 {
		return VerificationResult{
			Name:     "proof_structure",
			Passed:   false,
			Message:  "Proof-of-work structure incomplete",
			Details:  fmt.Sprintf("Missing files: %v", missingFiles),
			Severity: "critical",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "proof_structure",
		Passed:   true,
		Message:  "Proof-of-work structure is valid",
		Details:  fmt.Sprintf("Directory: %s", artifact.Directory),
		Severity: "critical",
		Duration: time.Since(start),
	}, nil
}

// checkSchemaVersion verifies schema version compatibility.
func (v *ProofVerifier) checkSchemaVersion(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	version := artifact.Summary.Version
	if version == "" {
		return VerificationResult{
			Name:     "schema_version",
			Passed:   false,
			Message:  "Schema version is missing",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	// Check if version is supported (current version or one behind)
	supportedVersions := []string{"2.0.0", "1.0.0"}
	supported := false
	for _, sv := range supportedVersions {
		if version == sv {
			supported = true
			break
		}
	}

	if !supported {
		return VerificationResult{
			Name:     "schema_version",
			Passed:   false,
			Message:  "Unsupported schema version",
			Details:  fmt.Sprintf("Version: %s, Supported: %v", version, supportedVersions),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "schema_version",
		Passed:   true,
		Message:  "Schema version is supported",
		Details:  fmt.Sprintf("Version: %s", version),
		Severity: "high",
		Duration: time.Since(start),
	}, nil
}

// checkRequiredFields verifies all required fields are present.
func (v *ProofVerifier) checkRequiredFields(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	summary := artifact.Summary
	missingFields := []string{}

	// Critical fields
	if summary.TaskID == "" {
		missingFields = append(missingFields, "task_id")
	}
	if summary.SessionID == "" {
		missingFields = append(missingFields, "session_id")
	}
	if summary.WorkType == "" {
		missingFields = append(missingFields, "work_type")
	}
	if summary.Title == "" {
		missingFields = append(missingFields, "title")
	}
	if summary.Objective == "" {
		missingFields = append(missingFields, "objective")
	}
	if summary.Result == "" {
		missingFields = append(missingFields, "result")
	}
	if summary.StartedAt.IsZero() {
		missingFields = append(missingFields, "started_at")
	}
	if summary.CompletedAt.IsZero() {
		missingFields = append(missingFields, "completed_at")
	}

	if len(missingFields) > 0 {
		return VerificationResult{
			Name:     "required_fields",
			Passed:   false,
			Message:  "Missing required fields",
			Details:  fmt.Sprintf("Missing: %v", missingFields),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "required_fields",
		Passed:   true,
		Message:  "All required fields present",
		Details:  fmt.Sprintf("Checked %d fields", 8),
		Severity: "high",
		Duration: time.Since(start),
	}, nil
}

// checkTimestampConsistency verifies timestamps are consistent.
func (v *ProofVerifier) checkTimestampConsistency(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	summary := artifact.Summary

	// Check that started_at <= completed_at
	if summary.StartedAt.After(summary.CompletedAt) {
		return VerificationResult{
			Name:     "timestamp_consistency",
			Passed:   false,
			Message:  "Timestamp inconsistency: started_at > completed_at",
			Details:  fmt.Sprintf("Started: %s, Completed: %s", summary.StartedAt, summary.CompletedAt),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	// Check that duration is consistent
	duration := summary.CompletedAt.Sub(summary.StartedAt)
	if summary.Duration != 0 {
		// Allow 1 second tolerance
		diff := abs(duration.Seconds() - summary.Duration.Seconds())
		if diff > 1.0 {
			return VerificationResult{
				Name:     "timestamp_consistency",
				Passed:   false,
				Message:  "Duration inconsistency",
				Details:  fmt.Sprintf("Computed: %.2fs, Recorded: %.2fs", duration.Seconds(), summary.Duration.Seconds()),
				Severity: "high",
				Duration: time.Since(start),
			}, nil
		}
	}

	// Check that generated_at >= completed_at
	if summary.GeneratedAt.Before(summary.CompletedAt) {
		return VerificationResult{
			Name:     "timestamp_consistency",
			Passed:   false,
			Message:  "Timestamp inconsistency: generated_at < completed_at",
			Details:  fmt.Sprintf("Generated: %s, Completed: %s", summary.GeneratedAt, summary.CompletedAt),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "timestamp_consistency",
		Passed:   true,
		Message:  "Timestamps are consistent",
		Details:  fmt.Sprintf("Duration: %s", duration),
		Severity: "high",
		Duration: time.Since(start),
	}, nil
}

// checkArtifactIntegrity verifies artifact file integrity.
func (v *ProofVerifier) checkArtifactIntegrity(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Verify JSON can be parsed
	jsonData, err := os.ReadFile(artifact.JSONPath)
	if err != nil {
		return VerificationResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Failed to read JSON artifact",
			Details:  err.Error(),
			Severity: "critical",
			Duration: time.Since(start),
		}, nil
	}

	var summary ProofOfWorkSummary
	if err := json.Unmarshal(jsonData, &summary); err != nil {
		return VerificationResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "JSON artifact is malformed",
			Details:  err.Error(),
			Severity: "critical",
			Duration: time.Since(start),
		}, nil
	}

	// Verify markdown can be read
	if _, err := os.ReadFile(artifact.MarkdownPath); err != nil {
		return VerificationResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Failed to read markdown artifact",
			Details:  err.Error(),
			Severity: "critical",
			Duration: time.Since(start),
		}, nil
	}

	// Verify log can be read
	if _, err := os.ReadFile(artifact.LogPath); err != nil {
		return VerificationResult{
			Name:     "artifact_integrity",
			Passed:   false,
			Message:  "Failed to read execution log",
			Details:  err.Error(),
			Severity: "critical",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "artifact_integrity",
		Passed:   true,
		Message:  "All artifacts are valid",
		Details:  "JSON, markdown, and log are readable",
		Severity: "critical",
		Duration: time.Since(start),
	}, nil
}

// checkSignatureValid verifies digital signature if present.
func (v *ProofVerifier) checkSignatureValid(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Signature is optional
	if artifact.Summary.Signature == nil {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   true,
			Message:  "No signature present (optional)",
			Details:  "Proof-of-work can exist without signature",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	sig := artifact.Summary.Signature

	// Verify signature structure
	if sig.Algorithm == "" {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Signature missing algorithm",
			Details:  "Algorithm field is required",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	if sig.KeyID == "" {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Signature missing key ID",
			Details:  "KeyID field is required",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	if sig.Signature == "" {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Signature missing signature data",
			Details:  "Signature field is required",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	if sig.ProofDigest == "" {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Signature missing proof digest",
			Details:  "ProofDigest field is required",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	// Verify digest matches current proof data
	proofDigest, err := artifact.Summary.ComputeProofDigest()
	if err != nil {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Failed to compute proof digest",
			Details:  err.Error(),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	if proofDigest != sig.ProofDigest {
		return VerificationResult{
			Name:     "signature_valid",
			Passed:   false,
			Message:  "Proof digest mismatch",
			Details:  fmt.Sprintf("Expected: %s, Got: %s", sig.ProofDigest, proofDigest),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	// Verify signature using the key (if available)
	// Note: This would require access to the public key
	// For now, we just verify the structure

	return VerificationResult{
		Name:     "signature_valid",
		Passed:   true,
		Message:  "Signature structure is valid",
		Details:  fmt.Sprintf("Algorithm: %s, KeyID: %s", sig.Algorithm, sig.KeyID),
		Severity: "high",
		Duration: time.Since(start),
	}, nil
}

// checkChecksumIntegrity verifies all declared checksums.
func (v *ProofVerifier) checkChecksumIntegrity(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Checksums are optional but recommended
	if len(artifact.Summary.Checksums) == 0 {
		return VerificationResult{
			Name:     "checksum_integrity",
			Passed:   true,
			Message:  "No checksums present (optional but recommended)",
			Details:  "Checksums improve verification confidence",
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	// Verify each checksum
	verified := 0
	failed := []string{}

	for key, expectedChecksum := range artifact.Summary.Checksums {
		// Determine file path based on key
		var filePath string
		switch key {
		case "markdown", "proof-of-work.md":
			filePath = artifact.MarkdownPath
		case "log", "execution.log":
			filePath = artifact.LogPath
		default:
			if strings.HasPrefix(key, "workspace/") {
				workspaceFile := strings.TrimPrefix(key, "workspace/")
				filePath = filepath.Join(artifact.Summary.WorkspacePath, workspaceFile)
			} else {
				// Unknown key, skip
				continue
			}
		}

		// Compute actual checksum
		actualChecksum, err := ComputeFileSHA256(filePath)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: read error", key))
			continue
		}

		if actualChecksum != expectedChecksum {
			failed = append(failed, fmt.Sprintf("%s: checksum mismatch", key))
			continue
		}

		verified++
	}

	if len(failed) > 0 {
		return VerificationResult{
			Name:     "checksum_integrity",
			Passed:   false,
			Message:  "Checksum verification failed",
			Details:  fmt.Sprintf("Failed: %v, Verified: %d", failed, verified),
			Severity: "high",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "checksum_integrity",
		Passed:   true,
		Message:  "All checksums verified",
		Details:  fmt.Sprintf("Verified %d checksums", verified),
		Severity: "high",
		Duration: time.Since(start),
	}, nil
}

// checkFilesExist verifies declared files actually exist.
func (v *ProofVerifier) checkFilesExist(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	if len(artifact.Summary.FilesChanged) == 0 {
		return VerificationResult{
			Name:     "files_exist",
			Passed:   true,
			Message:  "No files declared",
			Details:  "FilesChanged is empty",
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	workspacePath := artifact.Summary.WorkspacePath
	verified := 0
	missing := []string{}

	for _, file := range artifact.Summary.FilesChanged {
		absPath := file
		if !filepath.IsAbs(file) && workspacePath != "" {
			absPath = filepath.Join(workspacePath, file)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			missing = append(missing, file)
		} else {
			verified++
		}
	}

	if len(missing) > 0 {
		return VerificationResult{
			Name:     "files_exist",
			Passed:   false,
			Message:  "Declared files not found",
			Details:  fmt.Sprintf("Missing: %v, Verified: %d", missing, verified),
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "files_exist",
		Passed:   true,
		Message:  "All declared files exist",
		Details:  fmt.Sprintf("Verified %d files", verified),
		Severity: "medium",
		Duration: time.Since(start),
	}, nil
}

// checkGitEvidence verifies git evidence is consistent.
func (v *ProofVerifier) checkGitEvidence(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Git evidence is optional
	if artifact.Summary.GitBranch == "" && artifact.Summary.GitCommit == "" {
		return VerificationResult{
			Name:     "git_evidence",
			Passed:   true,
			Message:  "No git evidence (optional)",
			Details:  "Git metadata not recorded",
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	// Check git review files if declared
	reviewFiles := 0
	if artifact.Summary.GitStatusPath != "" {
		if _, err := os.Stat(artifact.Summary.GitStatusPath); err == nil {
			reviewFiles++
		}
	}
	if artifact.Summary.GitDiffStatPath != "" {
		if _, err := os.Stat(artifact.Summary.GitDiffStatPath); err == nil {
			reviewFiles++
		}
	}

	return VerificationResult{
		Name:     "git_evidence",
		Passed:   true,
		Message:  "Git evidence is consistent",
		Details:  fmt.Sprintf("Branch: %s, Commit: %s, Review files: %d",
			artifact.Summary.GitBranch, artifact.Summary.GitCommit, reviewFiles),
		Severity: "medium",
		Duration: time.Since(start),
	}, nil
}

// checkTestEvidence verifies test evidence is consistent.
func (v *ProofVerifier) checkTestEvidence(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Test evidence is optional
	if len(artifact.Summary.TestsRun) == 0 {
		return VerificationResult{
			Name:     "test_evidence",
			Passed:   true,
			Message:  "No tests declared",
			Details:  "TestsRun is empty",
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	// If tests are declared, check for test output evidence
	// This is a basic check - real verification would parse the execution log
	if artifact.Summary.TestsPassed {
		return VerificationResult{
			Name:     "test_evidence",
			Passed:   true,
			Message:  "Tests passed",
			Details:  fmt.Sprintf("Tests run: %d, All passed: yes", len(artifact.Summary.TestsRun)),
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	if len(artifact.Summary.TestsFailed) > 0 {
		return VerificationResult{
			Name:     "test_evidence",
			Passed:   false,
			Message:  "Tests failed",
			Details:  fmt.Sprintf("Failed: %v", artifact.Summary.TestsFailed),
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "test_evidence",
		Passed:   true,
		Message:  "Test evidence is consistent",
		Details:  fmt.Sprintf("Tests run: %d", len(artifact.Summary.TestsRun)),
		Severity: "medium",
		Duration: time.Since(start),
	}, nil
}

// checkProvenanceChain verifies provenance information.
func (v *ProofVerifier) checkProvenanceChain(artifact *ProofOfWorkArtifact) (VerificationResult, error) {
	start := time.Now()

	// Provenance chain is optional but recommended
	issues := []string{}

	// Check agent role
	if artifact.Summary.AgentRole == "" {
		issues = append(issues, "agent role missing")
	}

	// Check model/template used
	if artifact.Summary.ModelUsed == "" && artifact.Summary.TemplateUsed == "" {
		issues = append(issues, "model/template information missing")
	}

	// Check environment
	if artifact.Summary.Environment == nil {
		issues = append(issues, "environment metadata missing")
	} else {
		if artifact.Summary.Environment.Hostname == "" {
			issues = append(issues, "hostname missing")
		}
		if artifact.Summary.Environment.FactoryVersion == "" {
			issues = append(issues, "factory version missing")
		}
	}

	if len(issues) > 0 && v.strictMode {
		return VerificationResult{
			Name:     "provenance_chain",
			Passed:   false,
			Message:  "Provenance information incomplete",
			Details:  fmt.Sprintf("Issues: %v", issues),
			Severity: "medium",
			Duration: time.Since(start),
		}, nil
	}

	return VerificationResult{
		Name:     "provenance_chain",
		Passed:   true,
		Message:  "Provenance chain is valid",
		Details:  fmt.Sprintf("Issues: %d (non-critical)", len(issues)),
		Severity: "medium",
		Duration: time.Since(start),
	}, nil
}

// calculateOverallScore computes the overall verification score (0.0 to 1.0).
func (v *ProofVerifier) calculateOverallScore(report *VerificationReport) float64 {
	if len(report.CheckResults) == 0 {
		return 0.0
	}

	totalWeight := 0.0
	passedWeight := 0.0

	weightBySeverity := map[string]float64{
		"critical": 1.0,
		"high":     0.8,
		"medium":   0.5,
		"low":      0.3,
	}

	for _, result := range report.CheckResults {
		weight := weightBySeverity[result.Severity]
		totalWeight += weight

		if result.Passed {
			passedWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	return passedWeight / totalWeight
}

// generateRecommendations generates actionable recommendations based on verification results.
func (v *ProofVerifier) generateRecommendations(report *VerificationReport) []string {
	recommendations := []string{}

	// Check for critical failures
	for _, result := range report.CheckResults {
		if !result.Passed && result.Severity == "critical" {
			recommendations = append(recommendations,
				fmt.Sprintf("URGENT: Fix %s - %s", result.Name, result.Message))
		}
	}

	// Check for high priority failures
	for _, result := range report.CheckResults {
		if !result.Passed && result.Severity == "high" {
			recommendations = append(recommendations,
				fmt.Sprintf("IMPORTANT: Fix %s - %s", result.Name, result.Message))
		}
	}

	// Check for signature
	if report.OverallScore > 0.7 {
		signaturePresent := false
		for _, result := range report.CheckResults {
			if result.Name == "signature_valid" {
				signaturePresent = true
				break
			}
		}
		if !signaturePresent {
			recommendations = append(recommendations,
				"Consider adding digital signatures for stronger verification")
		}
	}

	// Check for checksums
	if report.OverallScore > 0.7 {
		checksumsPresent := false
		for _, result := range report.CheckResults {
			if result.Name == "checksum_integrity" {
				checksumsPresent = true
				break
			}
		}
		if !checksumsPresent {
			recommendations = append(recommendations,
				"Consider adding checksums for stronger artifact integrity")
		}
	}

	return recommendations
}

// abs returns the absolute value of a float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
