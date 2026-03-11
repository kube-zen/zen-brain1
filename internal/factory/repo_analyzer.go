// Package factory provides repo analysis utilities for repo-native execution.
package factory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepoInfo describes the detected repository structure.
type RepoInfo struct {
	// IsGitRepo indicates if we're in a git repository
	IsGitRepo bool
	// ProjectType indicates the detected project type (go, python, node, unknown)
	ProjectType string
	// ModuleName is the Go module name (if Go project)
	ModuleName string
	// PackagePaths are existing package directories discovered
	PackagePaths []string
	// StandardDirs are standard project directories found (cmd, internal, pkg, src, etc.)
	StandardDirs []string
	// EntrypointPaths are detected entrypoint locations (cmd/*, main.go, etc.)
	EntrypointPaths []string
}

// DetectRepoStructure analyzes the repository structure and returns RepoInfo.
func DetectRepoStructure(workdir string) (*RepoInfo, error) {
	info := &RepoInfo{}

	// Check if git repo
	if _, err := os.Stat(filepath.Join(workdir, ".git")); err == nil {
		info.IsGitRepo = true
	}

	// Detect project type and module info
	if _, err := os.Stat(filepath.Join(workdir, "go.mod")); err == nil {
		info.ProjectType = "go"
		info.ModuleName = parseGoModuleName(workdir)
		info.PackagePaths = discoverGoPackages(workdir)
		info.StandardDirs = discoverStandardGoDirs(workdir)
		info.EntrypointPaths = discoverGoEntrypoints(workdir)
	} else if _, err := os.Stat(filepath.Join(workdir, "package.json")); err == nil {
		info.ProjectType = "node"
		info.PackagePaths = discoverNodePackages(workdir)
		info.StandardDirs = discoverNodeDirs(workdir)
	} else if _, err := os.Stat(filepath.Join(workdir, "pyproject.toml")); err == nil {
		info.ProjectType = "python"
		info.PackagePaths = discoverPythonPackages(workdir)
		info.StandardDirs = discoverPythonDirs(workdir)
	}

	if info.ProjectType == "" {
		info.ProjectType = "unknown"
	}

	return info, nil
}

// SelectImplementationTarget selects an appropriate target path for new implementation code.
// It prefers existing directories and standard project layouts.
// Returns empty string if no suitable target can be found.
func SelectImplementationTarget(workdir, workItemID string, info *RepoInfo) (string, string, error) {
	if info == nil {
		var err error
		info, err = DetectRepoStructure(workdir)
		if err != nil {
			return "", "", fmt.Errorf("failed to detect repo structure: %w", err)
		}
	}

	var targetDir string
	var packageName string

	switch info.ProjectType {
	case "go":
		// For Go, prefer internal/<package> or create new internal package
		if len(info.PackagePaths) > 0 {
			// Use an existing internal package if available
			for _, pkgPath := range info.PackagePaths {
				if strings.HasPrefix(pkgPath, "internal/") {
					targetDir = filepath.Dir(pkgPath)
					packageName = filepath.Base(targetDir)
					break
				}
			}
		}
		// Fallback: create new internal package named after work item
		if targetDir == "" {
			// Sanitize work item ID for use as package name
			packageName = sanitizePackageName(workItemID)
			if packageName == "" {
				packageName = "impl"
			}
			targetDir = filepath.Join("internal", packageName)
		}

	case "node":
		// For Node, prefer src/ or lib/ directories
		if stringSliceContains(info.StandardDirs, "src") {
			targetDir = "src"
		} else if stringSliceContains(info.StandardDirs, "lib") {
			targetDir = "lib"
		} else {
			// Fallback to src/
			targetDir = "src"
		}
		packageName = sanitizePackageName(workItemID)

	case "python":
		// For Python, prefer src/ or project name directory
		if len(info.PackagePaths) > 0 {
			targetDir = filepath.Dir(info.PackagePaths[0])
			packageName = filepath.Base(targetDir)
		} else if stringSliceContains(info.StandardDirs, "src") {
			targetDir = "src"
		} else {
			targetDir = "src"
		}
		if packageName == "" {
			packageName = sanitizePackageName(workItemID)
		}

	default:
		return "", "", fmt.Errorf("unsupported project type: %s", info.ProjectType)
	}

	if targetDir == "" {
		return "", "", fmt.Errorf("could not determine target directory")
	}

	targetPath := filepath.Join(targetDir, getImplementationFilename(info.ProjectType, workItemID))

	return targetPath, packageName, nil
}

// SelectBugfixTarget analyzes the objective to discover potential bug target files.
// It returns a list of candidate file paths that likely need fixing.
func SelectBugfixTarget(workdir, objective, title string, info *RepoInfo) ([]string, error) {
	if info == nil {
		var err error
		info, err = DetectRepoStructure(workdir)
		if err != nil {
			return nil, fmt.Errorf("failed to detect repo structure: %w", err)
		}
	}

	var candidates []string

	// Search for files containing keywords from objective/title
	keywords := extractKeywords(objective + " " + title)

	switch info.ProjectType {
	case "go":
		// Search for .go files in internal/, pkg/, cmd/
		searchPaths := []string{"internal", "pkg", "cmd"}
		for _, searchPath := range searchPaths {
			files, err := findGoFiles(workdir, searchPath)
			if err != nil {
				continue
			}
			for _, file := range files {
				if fileMatchesKeywords(filepath.Join(workdir, file), keywords) {
					candidates = append(candidates, file)
				}
			}
		}

	case "node":
		// Search for .ts, .js files in src/, lib/
		searchPaths := []string{"src", "lib"}
		for _, searchPath := range searchPaths {
			files, err := findNodeFiles(workdir, searchPath)
			if err != nil {
				continue
			}
			for _, file := range files {
				if fileMatchesKeywords(filepath.Join(workdir, file), keywords) {
					candidates = append(candidates, file)
				}
			}
		}

	case "python":
		// Search for .py files
		files, err := findPythonFiles(workdir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if fileMatchesKeywords(filepath.Join(workdir, file), keywords) {
				candidates = append(candidates, file)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no potential bug target files found for objective: %s", objective)
	}

	return candidates, nil
}

// SelectRefactorTarget discovers files that are candidates for refactoring.
// It returns files in the most complex packages or those with code smells.
func SelectRefactorTarget(workdir string, info *RepoInfo) ([]string, error) {
	if info == nil {
		var err error
		info, err = DetectRepoStructure(workdir)
		if err != nil {
			return nil, fmt.Errorf("failed to detect repo structure: %w", err)
		}
	}

	var targets []string

	switch info.ProjectType {
	case "go":
		// Prefer files in internal/ packages
		for _, pkgPath := range info.PackagePaths {
			if strings.HasPrefix(pkgPath, "internal/") {
				files, err := findGoFiles(workdir, filepath.Dir(pkgPath))
				if err != nil {
					continue
				}
				if len(files) > 0 {
					targets = append(targets, files...)
					if len(targets) >= 3 {
						break
					}
				}
			}
		}

	case "node":
		// Find largest files in src/
		if stringSliceContains(info.StandardDirs, "src") {
			files, err := findNodeFiles(workdir, "src")
			if err == nil && len(files) > 0 {
				targets = append(targets, files...)
			}
		}

	case "python":
		// Find largest Python files
		files, err := findPythonFiles(workdir)
		if err == nil && len(files) > 0 {
			targets = append(targets, files...)
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no refactor target files found")
	}

	return targets, nil
}

// Helper functions

func parseGoModuleName(workdir string) string {
	goModPath := filepath.Join(workdir, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func discoverGoPackages(workdir string) []string {
	var packages []string
	filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(workdir, path)
		if err != nil {
			return nil
		}
		if strings.Contains(relPath, "vendor") || strings.Contains(relPath, ".git") {
			return filepath.SkipDir
		}
		// Check if directory contains .go files
		files, _ := filepath.Glob(filepath.Join(path, "*.go"))
		if len(files) > 0 {
			packages = append(packages, relPath)
		}
		return nil
	})
	return packages
}

func discoverStandardGoDirs(workdir string) []string {
	var dirs []string
	standardDirs := []string{"cmd", "internal", "pkg", "api", "server", "client"}
	for _, dir := range standardDirs {
		if _, err := os.Stat(filepath.Join(workdir, dir)); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func discoverGoEntrypoints(workdir string) []string {
	var entrypoints []string
	filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.Contains(path, "vendor") || strings.Contains(path, ".git") {
			return nil
		}
		relPath, err := filepath.Rel(workdir, path)
		if err != nil {
			return nil
		}
		// Look for main.go files in cmd/
		if strings.Contains(path, "cmd") && strings.HasSuffix(path, "main.go") {
			entrypoints = append(entrypoints, relPath)
		}
		// Also look for main.go at root or common locations
		if filepath.Base(path) == "main.go" {
			entrypoints = append(entrypoints, relPath)
		}
		return nil
	})
	return entrypoints
}

func discoverNodePackages(workdir string) []string {
	var packages []string
	filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(workdir, path)
		if err != nil {
			return nil
		}
		if strings.Contains(relPath, "node_modules") || strings.Contains(relPath, ".git") {
			return filepath.SkipDir
		}
		// Check if directory contains .ts or .js files
		files, _ := filepath.Glob(filepath.Join(path, "*.ts"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(path, "*.js"))
		}
		if len(files) > 0 && relPath != "." {
			packages = append(packages, relPath)
		}
		return nil
	})
	return packages
}

func discoverNodeDirs(workdir string) []string {
	var dirs []string
	standardDirs := []string{"src", "lib", "app", "components", "pages"}
	for _, dir := range standardDirs {
		if _, err := os.Stat(filepath.Join(workdir, dir)); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func discoverPythonPackages(workdir string) []string {
	var packages []string
	filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(workdir, path)
		if err != nil {
			return nil
		}
		if strings.Contains(relPath, ".git") || strings.Contains(relPath, "__pycache__") || strings.Contains(relPath, ".venv") {
			return filepath.SkipDir
		}
		// Check if directory contains .py files and __init__.py
		if _, err := os.Stat(filepath.Join(path, "__init__.py")); err == nil {
			files, _ := filepath.Glob(filepath.Join(path, "*.py"))
			if len(files) > 0 && relPath != "." {
				packages = append(packages, relPath)
			}
		}
		return nil
	})
	return packages
}

func discoverPythonDirs(workdir string) []string {
	var dirs []string
	standardDirs := []string{"src", "lib", "app", "tests"}
	for _, dir := range standardDirs {
		if _, err := os.Stat(filepath.Join(workdir, dir)); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

func findGoFiles(workdir, searchPath string) ([]string, error) {
	var files []string
	targetDir := filepath.Join(workdir, searchPath)
	filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.Contains(path, "_test.go") {
			relPath, _ := filepath.Rel(workdir, path)
			files = append(files, relPath)
		}
		return nil
	})
	return files, nil
}

func findNodeFiles(workdir, searchPath string) ([]string, error) {
	var files []string
	targetDir := filepath.Join(workdir, searchPath)
	filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js") {
			relPath, _ := filepath.Rel(workdir, path)
			files = append(files, relPath)
		}
		return nil
	})
	return files, nil
}

func findPythonFiles(workdir string) ([]string, error) {
	var files []string
	filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".py") && !strings.Contains(path, "__pycache__") {
			relPath, _ := filepath.Rel(workdir, path)
			files = append(files, relPath)
		}
		return nil
	})
	return files, nil
}

func sanitizePackageName(s string) string {
	// Remove non-alphanumeric characters, replace with underscore
	result := ""
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result += string(r)
		} else if r == '_' || r == '-' {
			result += "_"
		}
	}
	if result == "" {
		return "impl"
	}
	// Ensure starts with letter
	if result[0] >= '0' && result[0] <= '9' {
		result = "pkg" + result
	}
	return strings.ToLower(result)
}

func getImplementationFilename(projectType, workItemID string) string {
	baseName := sanitizePackageName(workItemID)
	if baseName == "" {
		baseName = "impl"
	}
	switch projectType {
	case "go":
		return baseName + ".go"
	case "node":
		return baseName + ".ts"
	case "python":
		return baseName + ".py"
	default:
		return baseName + ".go"
	}
}

func extractKeywords(s string) []string {
	// Simple keyword extraction: split on spaces, filter common words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "as": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
		"between": true, "under": true, "again": true, "further": true, "then": true,
		"once": true, "here": true, "there": true, "when": true, "where": true,
		"why": true, "how": true, "all": true, "each": true, "few": true,
		"more": true, "most": true, "other": true, "some": true, "such": true,
		"no": true, "nor": true, "not": true, "only": true, "own": true,
		"same": true, "so": true, "than": true, "too": true, "very": true,
	}

	var keywords []string
	words := strings.Fields(s)
	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}"))
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

func fileMatchesKeywords(filepath string, keywords []string) bool {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return false
	}
	contentStr := strings.ToLower(string(content))
	matches := 0
	for _, kw := range keywords {
		if strings.Contains(contentStr, kw) {
			matches++
		}
	}
	// Require at least 2 keyword matches for relevance
	return matches >= 2
}

func stringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
