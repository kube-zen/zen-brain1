// Package factory provides policy enforcement for factory execution.
//
// Policy enforcement ensures that templates cannot fall back to synthetic
// defaults when repo conditions are invalid. Fail-closed behavior is enforced.
package factory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PolicyViolation represents a policy violation.
type PolicyViolation struct {
	Rule       string
	Reason     string
	Fatal      bool
	Suggestion string
}

// PolicyResult contains policy validation results.
type PolicyResult struct {
	Passed      bool
	Violations  []PolicyViolation
	Suggestions []string
}

// PolicyEnforcer validates execution against policy rules.
type PolicyEnforcer struct {
	allowSyntheticDefaults bool
	allowMetadataOnly      bool
	minRepoFilesRequired   int
}

// NewPolicyEnforcer creates a new policy enforcer with fail-closed defaults.
func NewPolicyEnforcer() *PolicyEnforcer {
	return &PolicyEnforcer{
		allowSyntheticDefaults: false, // Fail-closed: no synthetic defaults
		allowMetadataOnly:      false, // Fail-closed: metadata-only is failure
		minRepoFilesRequired:   1,    // Must change at least 1 repo file
	}
}

// ValidateImplementation validates an implementation task.
func (p *PolicyEnforcer) ValidateImplementation(repoFiles []string, metadataFiles []string, targetPath string) *PolicyResult {
	result := &PolicyResult{
		Passed:     true,
		Violations: []PolicyViolation{},
	}

	// Rule: Must have a target path
	if targetPath == "" {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "implementation-must-have-target",
			Reason: "No target path specified for implementation",
			Fatal:  true,
			Suggestion: "Implementation must select a real target path from the repository structure. " +
				"Use internal/, pkg/, cmd/ or other existing directories.",
		})
	}

	// Rule: Target path must be within repository
	if targetPath != "" {
		if strings.Contains(targetPath, ".zen-tasks") {
			result.Passed = false
			result.Violations = append(result.Violations, PolicyViolation{
				Rule:       "implementation-must-be-in-repo",
				Reason:     fmt.Sprintf("Target path %s is in .zen-tasks (synthetic location)", targetPath),
				Fatal:      true,
				Suggestion: "Implementation must write to actual repository paths, not .zen-tasks.",
			})
		}

		// Check if target directory exists or is creatable
		targetDir := filepath.Dir(targetPath)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			// Check if parent of target dir exists
			parentDir := filepath.Dir(targetDir)
			if _, err := os.Stat(parentDir); os.IsNotExist(err) {
				result.Passed = false
				result.Violations = append(result.Violations, PolicyViolation{
					Rule:   "implementation-must-be-in-existing-structure",
					Reason: fmt.Sprintf("Target path %s is not within existing repo structure", targetPath),
					Fatal:  true,
					Suggestion: "Implementation must target directories that exist (internal/, pkg/, cmd/) " +
						"or be created directly under them.",
				})
			}
		}
	}

	// Rule: Must create at least 1 repo file
	if len(repoFiles) < p.minRepoFilesRequired {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "implementation-must-create-repo-files",
			Reason: fmt.Sprintf("Implementation created %d repo files, minimum required: %d", len(repoFiles), p.minRepoFilesRequired),
			Fatal:  true,
			Suggestion: "Implementation must create at least 1 file in the actual repository structure. " +
				"Check if target selection and file creation steps succeeded.",
		})
	}

	// Rule: Metadata-only execution is a failure
	if len(repoFiles) == 0 && len(metadataFiles) > 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "no-metadata-only-execution",
			Reason: "Execution created only metadata files, no repository files were changed",
			Fatal:  true,
			Suggestion: "Implementation must write to actual repository files. " +
				"Metadata-only execution is considered a failure.",
		})
	}

	return result
}

// ValidateBugfix validates a bugfix task.
func (p *PolicyEnforcer) ValidateBugfix(repoFiles []string, metadataFiles []string, targetFiles []string) *PolicyResult {
	result := &PolicyResult{
		Passed:     true,
		Violations: []PolicyViolation{},
	}

	// Rule: Must have identified target files
	if len(targetFiles) == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "bugfix-must-have-target-files",
			Reason: "No target files were identified for bugfix",
			Fatal:  true,
			Suggestion: "Bugfix must discover and identify actual files to fix from the repository. " +
				"Ensure the discovery step searches internal/, pkg/, cmd/ directories.",
		})
	}

	// Rule: Must create at least 1 fix file
	fixFiles := 0
	for _, file := range repoFiles {
		if strings.Contains(file, "fix_") || strings.Contains(file, "patch_") {
			fixFiles++
		}
	}
	if fixFiles == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "bugfix-must-create-fix-files",
			Reason: "Bugfix did not create any fix files (no 'fix_' or 'patch_' files found)",
			Fatal:  true,
			Suggestion: "Bugfix must create fix files targeting the discovered bugs. " +
				"Fix files should be named like 'fix_auth.go' beside the target file.",
		})
	}

	// Rule: Metadata-only execution is a failure
	if len(repoFiles) == 0 && len(metadataFiles) > 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "no-metadata-only-execution",
			Reason: "Bugfix created only metadata files, no repository files were changed",
			Fatal:  true,
			Suggestion: "Bugfix must write actual fix files to the repository. " +
				"Metadata-only execution is considered a failure.",
		})
	}

	return result
}

// ValidateRefactor validates a refactoring task.
func (p *PolicyEnforcer) ValidateRefactor(repoFiles []string, metadataFiles []string, targetFiles []string) *PolicyResult {
	result := &PolicyResult{
		Passed:     true,
		Violations: []PolicyViolation{},
	}

	// Rule: Must have identified target files
	if len(targetFiles) == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "refactor-must-have-target-files",
			Reason: "No target files were identified for refactoring",
			Fatal:  true,
			Suggestion: "Refactoring must discover and identify actual files to refactor from the repository. " +
				"Ensure the discovery step searches internal/, pkg/ directories.",
		})
	}

	// Rule: Must create refactored versions
	refactoredFiles := 0
	for _, file := range repoFiles {
		if strings.Contains(file, "_refactored.") || strings.Contains(file, "_refactored_") {
			refactoredFiles++
		}
	}
	if refactoredFiles == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "refactor-must-create-refactored-files",
			Reason: "Refactoring did not create any refactored files (no '*_refactored.*' files found)",
			Fatal:  true,
			Suggestion: "Refactoring must create refactored versions beside the original files. " +
				"Refactored files should be named like 'auth_refactored.go' beside 'auth.go'.",
		})
	}

	// Rule: No synthetic default locations
	for _, file := range repoFiles {
		if strings.Contains(file, "pkg/refactored.go") {
			result.Passed = false
			result.Violations = append(result.Violations, PolicyViolation{
				Rule:   "no-synthetic-default-locations",
				Reason: fmt.Sprintf("Refactoring created synthetic default file: %s", file),
				Fatal:  true,
				Suggestion: "Refactoring must create refactored files beside their originals, " +
					"not in synthetic default locations like pkg/refactored.go.",
			})
			break
		}
	}

	// Rule: Metadata-only execution is a failure
	if len(repoFiles) == 0 && len(metadataFiles) > 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "no-metadata-only-execution",
			Reason: "Refactoring created only metadata files, no repository files were changed",
			Fatal:  true,
			Suggestion: "Refactoring must write actual refactored files to the repository. " +
				"Metadata-only execution is considered a failure.",
		})
	}

	return result
}

// ValidateDocs validates a documentation task.
func (p *PolicyEnforcer) ValidateDocs(repoFiles []string, metadataFiles []string, targetPath string) *PolicyResult {
	result := &PolicyResult{
		Passed:     true,
		Violations: []PolicyViolation{},
	}

	// Rule: Must have a target path in docs/
	if targetPath == "" {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "docs-must-have-target",
			Reason: "No target path specified for documentation",
			Fatal:  true,
			Suggestion: "Documentation must target docs/, docs/api/, docs/guides/ or other documentation directories.",
		})
	}

	// Rule: Target path must be in docs/ directory
	if targetPath != "" && !strings.HasPrefix(targetPath, "docs/") {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "docs-must-be-in-docs-directory",
			Reason: fmt.Sprintf("Documentation target %s is not in docs/ directory", targetPath),
			Fatal:  true,
			Suggestion: "Documentation must be written to docs/ directory or subdirectories (docs/api/, docs/guides/).",
		})
	}

	// Rule: Must create at least 1 doc file
	docFiles := 0
	for _, file := range repoFiles {
		if strings.HasSuffix(file, ".md") {
			docFiles++
		}
	}
	if docFiles == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "docs-must-create-doc-files",
			Reason: "Documentation did not create any markdown files",
			Fatal:  true,
			Suggestion: "Documentation must create markdown files (.md) in the docs/ directory.",
		})
	}

	return result
}

// ValidateTest validates a testing task.
func (p *PolicyEnforcer) ValidateTest(repoFiles []string, metadataFiles []string, sourceFiles []string) *PolicyResult {
	result := &PolicyResult{
		Passed:     true,
		Violations: []PolicyViolation{},
	}

	// Rule: Must have identified source files to test
	if len(sourceFiles) == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "test-must-have-source-files",
			Reason: "No source files were identified for testing",
			Fatal:  true,
			Suggestion: "Testing must discover and identify actual source files to test from the repository. " +
				"Ensure the discovery step searches internal/, pkg/ directories.",
		})
	}

	// Rule: Must create test files
	testFiles := 0
	for _, file := range repoFiles {
		if strings.Contains(file, "_test.go") || strings.Contains(file, "_test.") {
			testFiles++
		}
	}
	if testFiles == 0 {
		result.Passed = false
		result.Violations = append(result.Violations, PolicyViolation{
			Rule:   "test-must-create-test-files",
			Reason: "Testing did not create any test files (no '*_test.go' or '*_test.*' files found)",
			Fatal:  true,
			Suggestion: "Testing must create test files beside the source files being tested.",
		})
	}

	// Rule: Test files must be beside source files
	for _, repoFile := range repoFiles {
		if strings.HasSuffix(repoFile, "_test.go") {
			// Check if corresponding source file exists in same directory
			testDir := filepath.Dir(repoFile)
			testBase := strings.TrimSuffix(filepath.Base(repoFile), "_test.go") + ".go"
			_ = filepath.Join(testDir, testBase) // Construct sourcePath for reference
			found := false
			for _, sourceFile := range sourceFiles {
				if strings.HasSuffix(sourceFile, testBase) && filepath.Dir(sourceFile) == testDir {
					found = true
					break
				}
			}
			if !found {
				// This is a warning, not fatal
				result.Violations = append(result.Violations, PolicyViolation{
					Rule:   "test-files-should-beside-source",
					Reason: fmt.Sprintf("Test file %s may not be beside its corresponding source file", repoFile),
					Fatal:  false,
					Suggestion: "Test files should be created beside the source files they test.",
				})
			}
		}
	}

	return result
}

// FormatPolicyResult formats a policy result for human-readable output.
func FormatPolicyResult(result *PolicyResult) string {
	if result.Passed {
		return "✅ Policy Validation: PASSED"
	}

	var sb strings.Builder
	sb.WriteString("❌ Policy Validation: FAILED\n\n")

	sb.WriteString("## Violations\n")
	for _, v := range result.Violations {
		if v.Fatal {
			sb.WriteString(fmt.Sprintf("- **[FATAL]** %s: %s\n", v.Rule, v.Reason))
		} else {
			sb.WriteString(fmt.Sprintf("- **[WARNING]** %s: %s\n", v.Rule, v.Reason))
		}
		sb.WriteString(fmt.Sprintf("  💡 Suggestion: %s\n\n", v.Suggestion))
	}

	if len(result.Suggestions) > 0 {
		sb.WriteString("\n## Additional Suggestions\n")
		for _, s := range result.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	return sb.String()
}
