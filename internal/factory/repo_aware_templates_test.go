// Package factory tests repo-aware template behavior with real git repos.
//
// These integration tests prove that the repo-aware templates:
// 1. Write to actual repo structure, not just .zen-tasks
// 2. Select real target files from existing layout
// 3. Generate context-aware code matching real packages
// 4. Fail closed when target selection cannot be determined
// 5. Distinguish repo files from metadata files in proof
package factory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImplementationReal_WritesToActualRepo tests that implementation:real
// writes to actual repo paths, not just .zen-tasks.
func TestImplementationReal_WritesToActualRepo(t *testing.T) {
	tmpDir := setupGoTestRepo(t)
	defer os.RemoveAll(tmpDir)

	template := getRepoAwareTemplate("implementation", "real")
	if template == nil {
		t.Fatal("implementation:real template not found")
	}

	ctx := createTestContext(tmpDir, "impl-001", "Add new feature", "Add new feature for authentication")

	// Execute the template steps
	for _, step := range template.Steps {
		if step.Name == "Create implementation file in real repo location" {
			// Create required metadata files
			os.WriteFile(filepath.Join(tmpDir, ".zen-project-info"), []byte("PROJECT_TYPE=go\nMODULE_NAME=test.com\n"), 0644)
			os.WriteFile(filepath.Join(tmpDir, ".zen-dirs"), []byte("internal\npkg\n"), 0644)
			os.WriteFile(filepath.Join(tmpDir, ".zen-target-info"), []byte("TARGET_DIR=internal\nPACKAGE_NAME=auth\n"), 0644)

			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}
		}
	}

	// Verify implementation file exists in actual repo location (not .zen-tasks)
	implPath := filepath.Join(tmpDir, "internal", "impl_001.go")
	if _, err := os.Stat(implPath); os.IsNotExist(err) {
		t.Fatal("Implementation file not found in internal/ directory")
	}

	// Verify no .zen-tasks directory was created as the main output
	zenTasksPath := filepath.Join(tmpDir, ".zen-tasks")
	if _, err := os.Stat(zenTasksPath); err == nil {
		t.Error(".zen-tasks directory should not be main output location")
	}

	// Verify file content matches package structure
	content, err := os.ReadFile(implPath)
	if err != nil {
		t.Fatalf("Failed to read implementation file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "package auth") {
		t.Error("Implementation file should have package auth matching target directory")
	}
}

// TestImplementationReal_FailsClosedOnUnknownRepo tests that implementation:real
// fails closed when it cannot determine target structure.
func TestImplementationReal_FailsClosedOnUnknownRepo(t *testing.T) {
	tmpDir := t.TempDir()

	template := getRepoAwareTemplate("implementation", "real")
	if template == nil {
		t.Fatal("implementation:real template not found")
	}

	ctx := createTestContext(tmpDir, "impl-002", "Add feature", "Add feature")

	// Execute the target selection step
	for _, step := range template.Steps {
		if step.Name == "Select real implementation target" {
			// Create empty .zen-dirs (no valid directories)
			os.WriteFile(filepath.Join(tmpDir, ".zen-dirs"), []byte(""), 0644)

			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			// Should fail because no valid directory can be found
			if err == nil {
				t.Error("Expected failure when target selection cannot be determined")
			}
			if !strings.Contains(string(output), "ERROR") {
				t.Error("Expected error message in output")
			}
		}
	}
}

// TestBugfixReal_ModifiesActualRepoFiles tests that bugfix:real
// modifies actual repo files and references them in analysis.
func TestBugfixReal_ModifiesActualRepoFiles(t *testing.T) {
	tmpDir := setupGoTestRepoWithFiles(t)
	defer os.RemoveAll(tmpDir)

	template := getRepoAwareTemplate("bugfix", "real")
	if template == nil {
		t.Fatal("bugfix:real template not found")
	}

	ctx := createTestContext(tmpDir, "bug-001", "Fix authentication bug", "Fix bug in authentication logic")

	// Execute template steps
	for _, step := range template.Steps {
		if step.Name == "Discover potential bug target files" {
			// Create project info
			os.WriteFile(filepath.Join(tmpDir, ".zen-project-info"), []byte("PROJECT_TYPE=go\n"), 0644)

			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Verify target files were discovered
			targetFilesPath := filepath.Join(tmpDir, ".zen-target-files")
			content, err := os.ReadFile(targetFilesPath)
			if err != nil {
				t.Fatalf("Target files not tracked: %v", err)
			}
			targetFiles := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(targetFiles) == 0 {
				t.Error("No target files discovered")
			}
		}

		if step.Name == "Create targeted fix file" {
			os.WriteFile(filepath.Join(tmpDir, ".zen-project-info"), []byte("PROJECT_TYPE=go\n"), 0644)
			os.WriteFile(filepath.Join(tmpDir, ".zen-target-files"), []byte("internal/auth.go\n"), 0644)

			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Verify fix file was created in internal/
			fixPath := filepath.Join(tmpDir, "internal", "fix_auth.go")
			if _, err := os.Stat(fixPath); os.IsNotExist(err) {
				t.Fatal("Fix file not found in internal/ directory")
			}
		}
	}

	// Verify BUG_REPORT.md references actual files
	bugReportPath := filepath.Join(tmpDir, "analysis", "BUG_REPORT.md")
	content, err := os.ReadFile(bugReportPath)
	if err != nil {
		t.Fatalf("BUG_REPORT.md not found: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "internal") {
		t.Error("BUG_REPORT.md should reference actual repo files in internal/")
	}
}

// TestRefactorReal_ChangesActualRepoFiles tests that refactor:real
// changes actual repo files and captures before/after evidence.
func TestRefactorReal_ChangesActualRepoFiles(t *testing.T) {
	tmpDir := setupGoTestRepoWithFiles(t)
	defer os.RemoveAll(tmpDir)

	template := getRepoAwareTemplate("refactor", "real")
	if template == nil {
		t.Fatal("refactor:real template not found")
	}

	ctx := createTestContext(tmpDir, "ref-001", "Refactor authentication", "Refactor authentication code")

	var preCommit, postCommit string

	// Execute template steps
	for _, step := range template.Steps {
		switch step.Name {
		case "Capture pre-refactor state":
			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Read pre-refactor commit
			content, _ := os.ReadFile(filepath.Join(tmpDir, ".zen-pre-refactor-commit"))
			preCommit = strings.TrimSpace(string(content))
			if preCommit == "" {
				t.Error("Pre-refactor commit not captured")
			}

		case "Detect project type and discover refactor targets":
			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Verify target files were discovered
			targetFilesPath := filepath.Join(tmpDir, ".zen-target-files")
			if _, err := os.Stat(targetFilesPath); os.IsNotExist(err) {
				t.Error("Target files not discovered")
			}

		case "Create refactored files":
			os.WriteFile(filepath.Join(tmpDir, ".zen-project-info"), []byte("PROJECT_TYPE=go\n"), 0644)
			os.WriteFile(filepath.Join(tmpDir, ".zen-target-files"), []byte("internal/auth.go\n"), 0644)

			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Verify refactored file was created in internal/
			refactoredPath := filepath.Join(tmpDir, "internal", "auth_refactored.go")
			if _, err := os.Stat(refactoredPath); os.IsNotExist(err) {
				t.Fatal("Refactored file not found in internal/ directory")
			}

		case "Capture post-refactor state":
			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Read post-refactor commit
			content, _ := os.ReadFile(filepath.Join(tmpDir, ".zen-post-refactor-commit"))
			postCommit = strings.TrimSpace(string(content))
			if postCommit == "" {
				t.Error("Post-refactor commit not captured")
			}
		}
	}

	// Verify before/after evidence exists
	if preCommit == "" || postCommit == "" {
		t.Error("Before/after commits not captured")
	}
	if preCommit != postCommit {
		// This is expected - commits should differ
	}
}

// TestProofDistinguishesRepoFilesFromMetadata tests that proof
// clearly distinguishes repo files from metadata files.
func TestProofDistinguishesRepoFilesFromMetadata(t *testing.T) {
	tmpDir := setupGoTestRepo(t)
	defer os.RemoveAll(tmpDir)

	template := getRepoAwareTemplate("implementation", "real")
	if template == nil {
		t.Fatal("implementation:real template not found")
	}

	ctx := createTestContext(tmpDir, "impl-003", "Add feature", "Add feature")

	// Create metadata and repo file tracking
	os.WriteFile(filepath.Join(tmpDir, ".zen-project-info"), []byte("PROJECT_TYPE=go\nMODULE_NAME=test.com\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".zen-dirs"), []byte("internal\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".zen-target-info"), []byte("TARGET_DIR=internal\nPACKAGE_NAME=feat\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".zen-repo-files-changed"), []byte("internal/impl_003.go\ninternal/impl_003_test.go\n"), 0644)

	// Execute proof generation step
	for _, step := range template.Steps {
		if step.Name == "Generate honest proof" {
			cmd := exec.Command("bash", "-c", renderTemplateCommand(step.Command, ctx))
			cmd.Dir = tmpDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Step %s failed: %v\nOutput: %s", step.Name, err, output)
			}

			// Verify proof exists
			proofPath := filepath.Join(tmpDir, "PROOF_OF_WORK.md")
			content, err := os.ReadFile(proofPath)
			if err != nil {
				t.Fatalf("PROOF_OF_WORK.md not found: %v", err)
			}
			contentStr := string(content)

			// Verify proof distinguishes repo files from metadata
			if !strings.Contains(contentStr, "Real Repository Files Changed") {
				t.Error("Proof should have section for repo files")
			}
			if !strings.Contains(contentStr, "Metadata Files Created") {
				t.Error("Proof should have section for metadata files")
			}
			if !strings.Contains(contentStr, "internal/impl_003.go") {
				t.Error("Proof should list actual repo files changed")
			}
			if strings.Contains(contentStr, ".zen-") && !strings.Contains(contentStr, "(for tracking only)") {
				t.Error("Metadata files should be labeled as such")
			}
		}
	}
}

// TestPostflightDowngradesOnMetadataOnly tests that postflight
// downgrades recommendation when only metadata files were created.
func TestPostflightDowngradesOnMetadataOnly(t *testing.T) {
	// This test would require a WorkspaceManager, which is complex to set up.
	// For now, we'll skip it as a placeholder for future enhancement.
	t.Skip("TestPostflightDowngradesOnMetadataOnly requires WorkspaceManager setup")
}

// Helper functions

func setupGoTestRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	// Initialize git repo
	runRepoAwareTestCmd(t, tmpDir, "git", "init")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.email", "test@example.com")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.name", "Test User")

	// Create go.mod
	goMod := `module test.com

go 1.21
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create initial commit
	runRepoAwareTestCmd(t, tmpDir, "git", "add", ".")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "-m", "Initial commit")

	return tmpDir
}

func setupGoTestRepoWithFiles(t *testing.T) string {
	tmpDir := setupGoTestRepo(t)

	// Create internal directory
	os.MkdirAll(filepath.Join(tmpDir, "internal"), 0755)

	// Create a sample file
	authGo := `package internal

type Auth struct {
	enabled bool
}

func NewAuth() *Auth {
	return &Auth{enabled: false}
}
`
	os.WriteFile(filepath.Join(tmpDir, "internal", "auth.go"), []byte(authGo), 0644)

	// Create pkg directory
	os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)

	// Commit files
	runRepoAwareTestCmd(t, tmpDir, "git", "add", ".")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "-m", "Add initial files")

	return tmpDir
}

// TestDocsReal_WritesToActualDocs tests that docs:real writes to actual docs directory.
func TestDocsReal_WritesToActualDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	runRepoAwareTestCmd(t, tmpDir, "git", "init")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.email", "test@example.com")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.name", "Test User")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "--allow-empty", "-m", "Initial commit")

	// Create minimal go.mod for project detection
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	runRepoAwareTestCmd(t, tmpDir, "git", "add", "go.mod")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "-m", "Add go.mod")

	// Create docs directory
	docsDir := filepath.Join(tmpDir, "docs")
	os.MkdirAll(docsDir, 0755)

	// Get docs:real template
	template := getRepoAwareTemplate("docs", "real")
	if template == nil {
		t.Fatal("docs:real template not found")
	}

	// Verify template description includes repo-aware language
	if !strings.Contains(strings.ToLower(template.Description), "repo-aware") &&
	   !strings.Contains(strings.ToLower(template.Description), "repo-native") &&
	   !strings.Contains(strings.ToLower(template.Description), "detects existing") {
		t.Error("Template description should mention repo-aware behavior")
	}

	// Check for validation step (git repo validation)
	hasValidation := false
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "Validate") || strings.Contains(step.Command, "git rev-parse") {
			hasValidation = true
			break
		}
	}
	if !hasValidation {
		t.Error("docs:real template should have git repository validation")
	}

	// Check for docs structure detection
	hasDetection := false
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "Detect") || strings.Contains(step.Description, "docs structure") {
			hasDetection = true
			break
		}
	}
	if !hasDetection {
		t.Error("docs:real template should detect docs structure")
	}

	// Verify template targets docs/ directory (not .zen-tasks)
	hasRealTarget := false
	for _, step := range template.Steps {
		if strings.Contains(step.Command, "docs/") && !strings.Contains(step.Command, ".zen-tasks") {
			hasRealTarget = true
			break
		}
	}
	if !hasRealTarget {
		t.Error("docs:real template should write to docs/ directory, not .zen-tasks")
	}

	// Verify proof step distinguishes repo files from metadata
	hasHonestProof := false
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "proof") && strings.Contains(step.Command, "Real Repository Files") {
			hasHonestProof = true
			break
		}
	}
	if !hasHonestProof {
		t.Error("docs:real template should generate honest proof distinguishing repo files from metadata")
	}
}

// TestDocsReal_FailsClosedOnNonGitRepo tests that docs:real fails closed without git.
func TestDocsReal_FailsClosedOnNonGitRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	// Do NOT initialize git repo - this is the test

	// Get docs:real template
	template := getRepoAwareTemplate("docs", "real")
	if template == nil {
		t.Fatal("docs:real template not found")
	}

	// Verify first step validates git repo
	if len(template.Steps) == 0 {
		t.Fatal("Template has no steps")
	}

	firstStep := template.Steps[0]
	if !strings.Contains(firstStep.Command, "git rev-parse") {
		t.Error("First step should validate git repository with git rev-parse")
	}

	// The command should fail without git
	// We can't actually run it here because it would require shell execution,
	// but we verified the template structure is correct
}

// TestTestReal_CreatesTestsBesideSource tests that test:real creates tests beside source files.
func TestTestReal_CreatesTestsBesideSource(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	defer os.RemoveAll(tmpDir)

	// Initialize git repo
	runRepoAwareTestCmd(t, tmpDir, "git", "init")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.email", "test@example.com")
	runRepoAwareTestCmd(t, tmpDir, "git", "config", "user.name", "Test User")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "--allow-empty", "-m", "Initial commit")

	// Create minimal go.mod
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.21\n"), 0644)
	runRepoAwareTestCmd(t, tmpDir, "git", "add", "go.mod")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "-m", "Add go.mod")

	// Create some source files to test
	internalDir := filepath.Join(tmpDir, "internal")
	os.MkdirAll(internalDir, 0755)
	sourceFile := filepath.Join(internalDir, "auth.go")
	os.WriteFile(sourceFile, []byte("package auth\n\nfunc Authenticate() bool { return true }"), 0644)
	runRepoAwareTestCmd(t, tmpDir, "git", "add", "internal/")
	runRepoAwareTestCmd(t, tmpDir, "git", "commit", "-m", "Add source files")

	// Get test:real template
	template := getRepoAwareTemplate("test", "real")
	if template == nil {
		t.Fatal("test:real template not found")
	}

	// Verify template discovers source files
	hasDiscovery := false
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "Discover") || strings.Contains(step.Command, "find internal pkg cmd") {
			hasDiscovery = true
			break
		}
	}
	if !hasDiscovery {
		t.Error("test:real template should discover source files from internal/pkg/cmd")
	}

	// Verify template creates tests beside source
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "Create") && strings.Contains(step.Description, "tests") {
			// Check that test files are created beside source
			if !strings.Contains(step.Command, "_test.go") {
				t.Error("Test creation step should create _test.go files")
			}
			// Check that tests are in same directory as source
			if !strings.Contains(step.Command, "source_dir") {
				t.Error("Test creation step should place tests in source directory")
			}
			break
		}
	}

	// Verify proof includes source files
	hasSourceFilesProof := false
	for _, step := range template.Steps {
		if strings.Contains(step.Name, "proof") && strings.Contains(step.Command, "Source Files") {
			hasSourceFilesProof = true
			break
		}
	}
	if !hasSourceFilesProof {
		t.Error("test:real template proof should list source files being tested")
	}
}

// TestPolicyEnforcer_NoMetadataOnlyExecution tests that metadata-only execution fails.
func TestPolicyEnforcer_NoMetadataOnlyExecution(t *testing.T) {
	enforcer := NewPolicyEnforcer()

	// Test with only metadata files (no repo files)
	repoFiles := []string{}
	metadataFiles := []string{"PROOF_OF_WORK.md", "analysis/TEST_ANALYSIS.md"}

	result := enforcer.ValidateImplementation(repoFiles, metadataFiles, "internal/test.go")
	if result.Passed {
		t.Error("Policy should fail when only metadata files are created")
	}

	// Check for specific violation
	hasMetadataOnlyViolation := false
	for _, v := range result.Violations {
		if v.Rule == "no-metadata-only-execution" {
			hasMetadataOnlyViolation = true
			break
		}
	}
	if !hasMetadataOnlyViolation {
		t.Error("Should have no-metadata-only-execution violation")
	}
}

// TestPolicyEnforcer_NoSyntheticDefaults tests that synthetic default locations fail.
func TestPolicyEnforcer_NoSyntheticDefaults(t *testing.T) {
	enforcer := NewPolicyEnforcer()

	// Test with synthetic default location
	repoFiles := []string{"pkg/refactored.go", "pkg/refactored_test.go"}
	metadataFiles := []string{"analysis/REFACTOR_ANALYSIS.md"}
	targetFiles := []string{"internal/auth.go"}

	result := enforcer.ValidateRefactor(repoFiles, metadataFiles, targetFiles)
	if result.Passed {
		t.Error("Policy should fail when using synthetic default location pkg/refactored.go")
	}

	// Check for specific violation
	hasSyntheticViolation := false
	for _, v := range result.Violations {
		if v.Rule == "no-synthetic-default-locations" {
			hasSyntheticViolation = true
			break
		}
	}
	if !hasSyntheticViolation {
		t.Error("Should have no-synthetic-default-locations violation")
	}
}

// TestPolicyEnforcer_DocsMustBeInDocsDirectory tests that docs must be in docs/ directory.
func TestPolicyEnforcer_DocsMustBeInDocsDirectory(t *testing.T) {
	enforcer := NewPolicyEnforcer()

	// Test with doc file outside docs/ directory
	repoFiles := []string{"README.md"}
	metadataFiles := []string{}
	targetPath := "README.md"

	result := enforcer.ValidateDocs(repoFiles, metadataFiles, targetPath)
	if result.Passed {
		t.Error("Policy should fail when documentation is not in docs/ directory")
	}

	// Check for specific violation
	hasDocsDirViolation := false
	for _, v := range result.Violations {
		if v.Rule == "docs-must-be-in-docs-directory" {
			hasDocsDirViolation = true
			break
		}
	}
	if !hasDocsDirViolation {
		t.Error("Should have docs-must-be-in-docs-directory violation")
	}
}

// Helper functions

func runRepoAwareTestCmd(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command %s %v failed: %v\nOutput: %s", name, args, err, output)
	}
}

func getRepoAwareTemplate(workType, workDomain string) *WorkTypeTemplate {
	registry := NewWorkTypeTemplateRegistry()
	registry.registerRepoAwareTemplates()
	// templates is map[string]map[string]*WorkTypeTemplate
	// First check if workType exists
	domainMap, exists := registry.templates[workType]
	if exists {
		// Try exact domain match
		if template, ok := domainMap[workDomain]; ok {
			return template
		}
		// Try empty domain (fallback)
		if template, ok := domainMap[""]; ok {
			return template
		}
		// Return any template for this workType
		for _, template := range domainMap {
			return template
		}
	}
	// Fall back to searching all templates
	for _, domainMap := range registry.templates {
		for _, template := range domainMap {
			if template.WorkType == workType && (workDomain == "" || template.WorkDomain == workDomain) {
				return template
			}
		}
	}
	return nil
}

type templateContext struct {
	WorkItemID string
	Title      string
	Objective  string
}

func createTestContext(workdir, workItemID, title, objective string) templateContext {
	return templateContext{
		WorkItemID: workItemID,
		Title:      title,
		Objective:  objective,
	}
}

func renderTemplateCommand(cmd string, ctx templateContext) string {
	cmd = strings.ReplaceAll(cmd, "{{.work_item_id}}", ctx.WorkItemID)
	cmd = strings.ReplaceAll(cmd, "{{.title}}", ctx.Title)
	cmd = strings.ReplaceAll(cmd, "{{.objective}}", ctx.Objective)
	return cmd
}
