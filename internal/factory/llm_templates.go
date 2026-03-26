// Package factory provides LLM-powered templates that generate real code.
//
// These templates replace hardcoded shell scripts with LLM-based code generation,
// producing actual implementations instead of placeholder code.
package factory

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LLMTemplateType defines types of LLM-powered templates.
type LLMTemplateType string

const (
	LLMTemplateImplementation LLMTemplateType = "implementation"
	LLMTemplateBugFix         LLMTemplateType = "bugfix"
	LLMTemplateRefactor       LLMTemplateType = "refactor"
	LLMTemplateTest           LLMTemplateType = "test"
	LLMTemplateDocumentation  LLMTemplateType = "documentation"
	LLMTemplateMigration      LLMTemplateType = "migration"
)

// LLMTemplateConfig configures an LLM-powered template.
type LLMTemplateConfig struct {
	// Type of template
	Type LLMTemplateType

	// WorkType and WorkDomain for template matching
	WorkType   string
	WorkDomain string

	// Whether to validate generated code (compile/test)
	ValidateCode bool

	// Whether to create tests alongside implementation
	CreateTests bool

	// Whether to create documentation
	CreateDocs bool

	// Custom prompt additions
	CustomPrompt string

	// Timeout for LLM generation
	GenerationTimeout time.Duration
}

// LLMTemplateExecutor executes LLM-powered templates.
type LLMTemplateExecutor struct {
	generator *LLMGenerator
	config    *LLMTemplateConfig
}

// NewLLMTemplateExecutor creates a new LLM template executor.
func NewLLMTemplateExecutor(generator *LLMGenerator, config *LLMTemplateConfig) (*LLMTemplateExecutor, error) {
	if generator == nil {
		return nil, fmt.Errorf("LLM generator is required")
	}
	if config == nil {
		return nil, fmt.Errorf("template config is required")
	}

	return &LLMTemplateExecutor{
		generator: generator,
		config:    config,
	}, nil
}

// Execute runs the LLM-powered template.
func (e *LLMTemplateExecutor) Execute(ctx context.Context, spec *FactoryTaskSpec, workspacePath string) ([]string, error) {
	start := time.Now()
	var filesCreated []string

	// 1. Gather project context
	req, err := e.buildGenerationRequest(ctx, spec, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("build generation request: %w", err)
	}

	// 2. Generate implementation
	implResult, err := e.generator.GenerateImplementation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	log.Printf("[LLMTemplate] Generated %s implementation for %s (model=%s, tokens=%d)",
		e.config.Type, spec.WorkItemID, implResult.Model, implResult.TokensUsed)
	// H006: Debug evidence for generation result
	log.Printf("[LLMTemplate] H006: Generation result - code_length=%d chars, language=%s", len(implResult.Code), implResult.Language)

	// 2b. Reject empty/trivial output
	if strings.TrimSpace(implResult.Code) == "" {
		return nil, fmt.Errorf("LLM generated empty code for task %s", spec.WorkItemID)
	}

	// 3. Write generated code to target file
	targetPath := e.determineTargetPath(spec, workspacePath, implResult.Language)
	log.Printf("[LLMTemplate] H006: Default target path: %s", targetPath)
	// Override target path if task specified explicit target files
	if len(req.TargetFiles) > 0 {
		overridePath := filepath.Join(workspacePath, req.TargetFiles[0])
		targetPath = overridePath
		log.Printf("[LLMTemplate] H006: Override active - using explicit target path: %s (internal/office fallback disabled)", targetPath)
	}
	if err := e.writeFile(targetPath, implResult.Code); err != nil {
		return nil, fmt.Errorf("write implementation: %w", err)
	}
	filesCreated = append(filesCreated, targetPath)
	log.Printf("[LLMTemplate] Created: %s", targetPath)

	// 4. Optionally generate tests
	if e.config.CreateTests && e.config.Type != LLMTemplateTest {
		testReq := &GenerationRequest{
			WorkItemID:    spec.WorkItemID,
			Title:         spec.Title,
			Objective:     spec.Objective,
			WorkType:      "test",
			WorkDomain:    string(spec.WorkDomain),
			ProjectType:   req.ProjectType,
			PackageName:   req.PackageName,
			ExistingCode:  implResult.Code,
		}

		testResult, err := e.generator.GenerateImplementation(ctx, testReq)
		if err != nil {
			log.Printf("[LLMTemplate] Warning: test generation failed: %v", err)
		} else {
			testPath := e.determineTestPath(targetPath, implResult.Language)
			if err := e.writeFile(testPath, testResult.Code); err != nil {
				log.Printf("[LLMTemplate] Warning: write test failed: %v", err)
			} else {
				filesCreated = append(filesCreated, testPath)
				log.Printf("[LLMTemplate] Created test: %s", testPath)
			}
		}
	}

	// 5. Optionally generate documentation
	if e.config.CreateDocs && e.config.Type != LLMTemplateDocumentation {
		docReq := &GenerationRequest{
			WorkItemID:   spec.WorkItemID,
			Title:        spec.Title,
			Objective:    spec.Objective,
			WorkType:     "documentation",
			WorkDomain:   string(spec.WorkDomain),
			ExistingCode: implResult.Code,
		}

		docResult, err := e.generator.GenerateDocumentation(ctx, docReq)
		if err != nil {
			log.Printf("[LLMTemplate] Warning: documentation generation failed: %v", err)
		} else {
			docPath := e.determineDocPath(targetPath)
			if err := e.writeFile(docPath, docResult.Code); err != nil {
				log.Printf("[LLMTemplate] Warning: write doc failed: %v", err)
			} else {
				filesCreated = append(filesCreated, docPath)
				log.Printf("[LLMTemplate] Created doc: %s", docPath)
			}
		}
	}

	// 6. Validate code if requested — HARD GATE when toolchain is available
	if e.config.ValidateCode {
		// Check if build toolchain is available in container
		if _, toolErr := exec.LookPath("go"); toolErr != nil {
			log.Printf("[LLMTemplate] Skipping code validation: go toolchain not available in container (postflight will validate)")
		} else if err := e.validateCode(ctx, workspacePath, targetPath); err != nil {
			log.Printf("[LLMTemplate] HARD FAILURE: code validation failed: %v", err)
			return filesCreated, fmt.Errorf("code validation hard failure: %w", err)
		}
	}

	duration := time.Since(start)
	log.Printf("[LLMTemplate] Completed %s for %s in %v, files=%d",
		e.config.Type, spec.WorkItemID, duration, len(filesCreated))

	return filesCreated, nil
}

// buildGenerationRequest gathers context for LLM generation.
func (e *LLMTemplateExecutor) buildGenerationRequest(ctx context.Context, spec *FactoryTaskSpec, workspacePath string) (*GenerationRequest, error) {
	req := &GenerationRequest{
		WorkItemID:         spec.WorkItemID,
		Title:              spec.Title,
		Objective:          spec.Objective,
		Description:        spec.Description,
		AcceptanceCriteria: spec.AcceptanceCriteria,
		WorkType:           string(e.config.Type),
		WorkDomain:         string(spec.WorkDomain),
		ProjectType:        "go", // Default to Go, will detect
		PackageName:        "main",
		RelatedFiles:       make(map[string]string),
		Constraints:        []string{},
	}

	// Check if this is a rescue task with structured prompt
	isRescueTask := strings.Contains(spec.Objective, "ADAPT") ||
		strings.Contains(spec.Objective, "Rescue") ||
		strings.Contains(spec.Objective, "0.1") ||
		strings.Contains(spec.Objective, "zen-structured-prompt")

	if isRescueTask {
		req.StructuredPrompt = true
		req.JiraKey = spec.WorkItemID
		req.WorkTypeLabel = "rescue_implementation"
		req.TimeoutSec = 2700 // 45 min for rescue tasks

		// Parse objective for source/target files
		sourceFile := extractFileFromObjective(spec.Objective, "SOURCE:")
		targetFile := extractFileFromObjective(spec.Objective, "TARGET:")
		allowedPaths := extractListFromObjective(spec.Objective, "Allowed paths:")
		existingTypes := extractListFromObjective(spec.Objective, "Use these existing types:")

		if sourceFile != "" {
			req.ContextFiles = []string{sourceFile}
		}
		if targetFile != "" {
			req.TargetFiles = []string{targetFile}
		}
		if len(allowedPaths) > 0 {
			req.AllowedPaths = allowedPaths
		}
		if len(existingTypes) > 0 {
			req.ExistingTypes = existingTypes
		}

		// Default packages for rescue tasks
		req.ExistingPackages = []string{
			"github.com/kube-zen/zen-brain1/internal/llm",
			"github.com/kube-zen/zen-brain1/pkg/llm",
			"github.com/kube-zen/zen-brain1/internal/mlq",
		}

		// Forbidden paths
		req.ForbiddenPaths = []string{
			"cmd/",
			"deployments/",
			"charts/",
			"docs/",
		}
	}

	// Detect project type
	req.ProjectType = e.detectProjectType(workspacePath)

	// Detect module/package name
	if req.ProjectType == "go" {
		req.ModuleName = e.detectGoModule(workspacePath)
		req.PackageName = e.detectPackageName(workspacePath)
	}

	// H006: Debug logging for TargetFiles parsing
	if len(spec.TargetFiles) > 0 {
		log.Printf("[LLMTemplate] H006: Task %s has explicit TargetFiles=%v", spec.WorkItemID, spec.TargetFiles)
		// H002: Propagate spec.TargetFiles into req for normal (non-rescue) explicit-target tasks.
		// Rescue tasks set req.TargetFiles inside the isRescueTask block above.
		if !isRescueTask {
			req.TargetFiles = spec.TargetFiles
			log.Printf("[LLMTemplate] H006: Propagated %d TargetFiles from spec to req for normal task", len(req.TargetFiles))
		}
	} else {
		log.Printf("[LLMTemplate] H006: Task %s has NO explicit TargetFiles (will use fallback)", spec.WorkItemID)
	}

	// Read existing code if modifying a file
	// ZB-281 C030: When TargetFiles is explicitly set (structured prompt), use that path
	// instead of guessTargetPath which generates a wrong slug-based path.
	// ZB-281 W004: Also fall back to ZEN_SOURCE_REPO for isolated-dir workspaces.
	sourcePaths := []string{workspacePath}
	if repoPath := os.Getenv("ZEN_SOURCE_REPO"); repoPath != "" {
		sourcePaths = append([]string{repoPath}, sourcePaths...)
		log.Printf("[LLMTemplate] H006: ZEN_SOURCE_REPO detected, searching paths: %v", sourcePaths)
	}

	if len(req.TargetFiles) > 0 {
		loaded := false
		for _, srcPath := range sourcePaths {
			explicitTarget := filepath.Join(srcPath, req.TargetFiles[0])
			if content, err := os.ReadFile(explicitTarget); err == nil {
				req.ExistingCode = string(content)
				req.TargetPath = filepath.Join(workspacePath, req.TargetFiles[0])
				log.Printf("[LLMTemplate] H006: Loaded existing code from %s: %s (%d bytes) -> TargetPath=%s", srcPath, req.TargetFiles[0], len(content), req.TargetPath)
				loaded = true
				break
			}
		}
		if !loaded {
			log.Printf("[LLMTemplate] H006: WARNING - Could not load target file %s from any source path (tried: %v)", req.TargetFiles[0], sourcePaths)
		}
	}
	// Fallback to guessTargetPath if explicit target didn't load
	if req.ExistingCode == "" {
		targetPath := e.guessTargetPath(spec, workspacePath)
		if content, err := os.ReadFile(targetPath); err == nil {
			req.ExistingCode = string(content)
			req.TargetPath = targetPath
		}
	}

	// Read related files for context
	relatedFiles := e.findRelatedFiles(workspacePath, string(spec.WorkDomain))
	for _, path := range relatedFiles {
		if content, err := os.ReadFile(path); err == nil {
			relPath, _ := filepath.Rel(workspacePath, path)
			req.RelatedFiles[relPath] = string(content)
		}
	}
	log.Printf("[LLMTemplate] H006: Loaded %d related file(s) for context: %v", len(req.RelatedFiles), getSortedMapKeys(req.RelatedFiles))

	// ZB-281 C030: For structured rescue tasks, inject grounded repo context
	// that the model must use instead of inventing types.
	if req.StructuredPrompt {
		groundedContextFiles := []string{
			"pkg/llm/provider.go",
			"pkg/llm/types.go",
		}
		for _, relPath := range groundedContextFiles {
			injected := false
			for _, srcPath := range sourcePaths {
				fullPath := filepath.Join(srcPath, relPath)
				if content, err := os.ReadFile(fullPath); err == nil {
					req.RelatedFiles[relPath] = string(content)
					log.Printf("[LLMTemplate] Injected grounded context: %s (%d bytes) from %s", relPath, len(content), srcPath)
					injected = true
					break
				}
			}
			if !injected {
				log.Printf("[LLMTemplate] WARNING: Could not read grounded context file: %s (tried: %v)", relPath, sourcePaths)
			}
		}

		// ZB-281 C030/C031: Add explicit constraints forbidding type invention
		req.Constraints = append(req.Constraints,
			"CRITICAL: You MUST modify the existing target file in place. Do NOT synthesize greenfield code.",
			"CRITICAL: Use ONLY types, methods, interfaces, and symbols that appear in the provided existing code or related context files.",
			"FORBIDDEN: Do NOT invent SelectRequest, SelectSlot, SelectedSlots, GetProviderConfig, or any pkg/llm surface not present in the provided context.",
			"FORBIDDEN: Do NOT import github.com/stretchr/testify/assert or any testify/mock packages.",
			"FORBIDDEN: Do NOT create new files unless explicitly requested.",
			"PRESERVE the existing package name and all existing function/type signatures in the target file.",
			"Use only imports that already exist in the target file or are in the provided context files.",
		)
	}

	// Add custom prompt if provided
	if e.config.CustomPrompt != "" {
		req.Constraints = append(req.Constraints, e.config.CustomPrompt)
	}

	return req, nil
}

// detectProjectType detects the project type from workspace.
func (e *LLMTemplateExecutor) detectProjectType(workspacePath string) string {
	// Check for Go
	if _, err := os.Stat(filepath.Join(workspacePath, "go.mod")); err == nil {
		return "go"
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(workspacePath, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(workspacePath, "setup.py")); err == nil {
		return "python"
	}

	// Check for Node.js
	if _, err := os.Stat(filepath.Join(workspacePath, "package.json")); err == nil {
		return "node"
	}

	// Default
	return "go"
}

// detectGoModule extracts the Go module name.
func (e *LLMTemplateExecutor) detectGoModule(workspacePath string) string {
	goModPath := filepath.Join(workspacePath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "example.com/module"
	}

	// Parse module line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}

	return "example.com/module"
}

// detectPackageName determines the package name for the target.
func (e *LLMTemplateExecutor) detectPackageName(workspacePath string) string {
	// Check current directory for existing Go files
	entries, err := os.ReadDir(workspacePath)
	if err != nil {
		return "main"
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			// Read package name from file
			content, err := os.ReadFile(filepath.Join(workspacePath, entry.Name()))
			if err != nil {
				continue
			}

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "package ") {
					return strings.TrimPrefix(line, "package ")
				}
			}
		}
	}

	// Use directory name as package name
	return filepath.Base(workspacePath)
}

// guessTargetPath guesses the target file path based on work item.
func (e *LLMTemplateExecutor) guessTargetPath(spec *FactoryTaskSpec, workspacePath string) string {
	// Try to find existing file based on work item ID
	workItemSlug := strings.ToLower(spec.WorkItemID)
	workItemSlug = strings.ReplaceAll(workItemSlug, "-", "_")
	workItemSlug = strings.ReplaceAll(workItemSlug, " ", "_")

	// Check common locations
	locations := []string{
		filepath.Join(workspacePath, workItemSlug+".go"),
		filepath.Join(workspacePath, "internal", workItemSlug, workItemSlug+".go"),
		filepath.Join(workspacePath, "pkg", workItemSlug, workItemSlug+".go"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Default to new file in workspace
	return filepath.Join(workspacePath, workItemSlug+".go")
}

// findRelatedFiles finds related files for context.
// W022: Search ZEN_SOURCE_REPO for related files instead of empty workspace.
func (e *LLMTemplateExecutor) findRelatedFiles(workspacePath, workDomain string) []string {
	var files []string
	var searchPath string

	// W022: Check ZEN_SOURCE_REPO mount point first
	if repoPath := os.Getenv("ZEN_SOURCE_REPO"); repoPath != "" {
		searchPath = repoPath
		log.Printf("[LLMTemplate] Searching for related files in ZEN_SOURCE_REPO: %s", repoPath)
	} else {
		searchPath = workspacePath
	}

	// Look for interface files
	interfacesPath := filepath.Join(searchPath, "interface.go")
	if _, err := os.Stat(interfacesPath); err == nil {
		files = append(files, interfacesPath)
	}

	// Look for types files
	typesPath := filepath.Join(searchPath, "types.go")
	if _, err := os.Stat(typesPath); err == nil {
		files = append(files, typesPath)
	}

	// Look for existing implementation in similar domain
	if workDomain != "" {
		domainPath := filepath.Join(searchPath, "internal", workDomain)
		if entries, err := os.ReadDir(domainPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
					files = append(files, filepath.Join(domainPath, entry.Name()))
					if len(files) >= 3 {
						break // Limit context
					}
				}
			}
		}
	}

	return files
}

// determineTargetPath determines the output file path.
func (e *LLMTemplateExecutor) determineTargetPath(spec *FactoryTaskSpec, workspacePath, language string) string {
	ext := ".go"
	switch language {
	case "python":
		ext = ".py"
	case "javascript", "typescript":
		ext = ".ts"
	case "sql":
		ext = ".sql"
	}

	// Create slug from work item ID; fall back to task ID if empty.
	// An empty slug produces hidden files (e.g. ".go") which break go build.
	slugID := spec.WorkItemID
	if slugID == "" {
		slugID = spec.ID
	}
	slug := strings.ToLower(slugID)
	slug = strings.ReplaceAll(slug, "-", "_")
	slug = strings.ReplaceAll(slug, " ", "_")

	// Determine directory
	targetDir := workspacePath
	if spec.WorkDomain != "" {
		targetDir = filepath.Join(workspacePath, "internal", string(spec.WorkDomain))
		os.MkdirAll(targetDir, 0755)
	}

	return filepath.Join(targetDir, slug+ext)
}

// determineTestPath determines the test file path.
func (e *LLMTemplateExecutor) determineTestPath(sourcePath, language string) string {
	ext := filepath.Ext(sourcePath)
	switch language {
	case "go":
		return strings.TrimSuffix(sourcePath, ext) + "_test" + ext
	case "python":
		// tests/test_filename.py
		dir := filepath.Dir(sourcePath)
		base := filepath.Base(sourcePath)
		testDir := filepath.Join(filepath.Dir(dir), "tests")
		os.MkdirAll(testDir, 0755)
		return filepath.Join(testDir, "test_"+base)
	default:
		return strings.TrimSuffix(sourcePath, ext) + ".test" + ext
	}
}

// determineDocPath determines the documentation file path.
func (e *LLMTemplateExecutor) determineDocPath(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	
	// Create docs directory
	docsDir := filepath.Join(filepath.Dir(dir), "docs")
	os.MkdirAll(docsDir, 0755)
	
	return filepath.Join(docsDir, base+".md")
}

// writeFile writes content to a file with proper formatting.
func (e *LLMTemplateExecutor) writeFile(path, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// validateCode runs validation (compile/test) on generated code.
func (e *LLMTemplateExecutor) validateCode(ctx context.Context, workspacePath, targetPath string) error {
	ext := filepath.Ext(targetPath)
	
	switch ext {
	case ".go":
		return e.validateGo(ctx, workspacePath, targetPath)
	case ".py":
		return e.validatePython(ctx, workspacePath, targetPath)
	default:
		return nil // Skip validation for unknown types
	}
}

// validateGo validates Go code by running go build.
func (e *LLMTemplateExecutor) validateGo(ctx context.Context, workspacePath, targetPath string) error {
	// Run go build on the file
	cmd := fmt.Sprintf("cd %s && go build -o /dev/null %s 2>&1", workspacePath, targetPath)
	output, err := e.runCommand(ctx, cmd, 60*time.Second)
	if err != nil {
		return fmt.Errorf("go build failed: %w\nOutput: %s", err, output)
	}

	log.Printf("[LLMTemplate] Validated: go build passed for %s", targetPath)
	return nil
}

// validatePython validates Python code by running python -m py_compile.
func (e *LLMTemplateExecutor) validatePython(ctx context.Context, workspacePath, targetPath string) error {
	cmd := fmt.Sprintf("cd %s && python -m py_compile %s 2>&1", workspacePath, targetPath)
	output, err := e.runCommand(ctx, cmd, 30*time.Second)
	if err != nil {
		return fmt.Errorf("python compile failed: %w\nOutput: %s", err, output)
	}

	log.Printf("[LLMTemplate] Validated: python compile passed for %s", targetPath)
	return nil
}

// runCommand executes a shell command with timeout.
func (e *LLMTemplateExecutor) runCommand(ctx context.Context, cmd string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Parse command: "cd DIR && CMD ARGS..."
	parts := strings.SplitN(cmd, "&&", 2)
	if len(parts) == 2 {
		cdParts := strings.Fields(strings.TrimSpace(parts[0]))
		if len(cdParts) == 2 && cdParts[0] == "cd" {
			execCmd := exec.CommandContext(ctx, "sh", "-c", strings.TrimSpace(parts[1]))
			execCmd.Dir = cdParts[1]
			output, err := execCmd.CombinedOutput()
			return string(output), err
		}
	}

	execCmd := exec.CommandContext(ctx, "sh", "-c", cmd)
	output, err := execCmd.CombinedOutput()
	return string(output), err
}

// RegisterLLMTemplates registers LLM-powered templates in the registry.
func (r *WorkTypeTemplateRegistry) RegisterLLMTemplates(generator *LLMGenerator) {
	if generator == nil {
		return
	}

	// Register LLM-powered implementations
	// These will be selected when LLM mode is enabled

	// Note: LLM templates are executed via LLMTemplateExecutor, not shell scripts
	// The registry stores metadata for template selection

	r.registerLLMImplementationTemplate()
	r.registerLLMBugFixTemplate()
	r.registerLLMRefactorTemplate()
	r.registerLLMTestTemplate()
	r.registerLLMDocsTemplate()
	r.registerLLMMigrationTemplate()
}

// LLM template registrations (metadata only, execution via LLMTemplateExecutor)

func (r *WorkTypeTemplateRegistry) registerLLMImplementationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "implementation",
		WorkDomain:  "llm",
		Description: "LLM-powered implementation: generates real code based on work item context",
		Steps:       []ExecutionStepTemplate{}, // Steps executed by LLMTemplateExecutor
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerLLMBugFixTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "bugfix",
		WorkDomain:  "llm",
		Description: "LLM-powered bug fix: analyzes code and generates fix",
		Steps:       []ExecutionStepTemplate{},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerLLMRefactorTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "refactor",
		WorkDomain:  "llm",
		Description: "LLM-powered refactor: improves code quality while preserving behavior",
		Steps:       []ExecutionStepTemplate{},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerLLMTestTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "test",
		WorkDomain:  "llm",
		Description: "LLM-powered test generation: creates comprehensive tests from code",
		Steps:       []ExecutionStepTemplate{},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerLLMDocsTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "documentation",
		WorkDomain:  "llm",
		Description: "LLM-powered documentation: generates clear docs from code",
		Steps:       []ExecutionStepTemplate{},
	}
	r.registerTemplate(template)
}

func (r *WorkTypeTemplateRegistry) registerLLMMigrationTemplate() {
	template := &WorkTypeTemplate{
		WorkType:    "migration",
		WorkDomain:  "llm",
		Description: "LLM-powered migration: generates UP and DOWN SQL from description",
		Steps:       []ExecutionStepTemplate{},
	}
	r.registerTemplate(template)
}


// Helper functions for parsing rescue task objectives

func extractFileFromObjective(objective, marker string) string {
	lines := strings.Split(objective, "\n")
	for _, line := range lines {
		if strings.Contains(line, marker) {
			parts := strings.Split(line, marker)
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func extractListFromObjective(objective, marker string) []string {
	var result []string
	lines := strings.Split(objective, "\n")
	collecting := false
	for _, line := range lines {
		if strings.Contains(line, marker) {
			collecting = true
			continue
		}
		if collecting {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "-") {
				result = append(result, strings.TrimSpace(trimmed[1:]))
			} else if !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "\t") {
				// Stop collecting on next non-indented line
				collecting = false
			}
		}
	}
	return result
}

// getSortedMapKeys returns sorted keys from a map for stable logging.
// H006: Helper for debug logging to show which files were loaded.
func getSortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple sort for stable logging
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

