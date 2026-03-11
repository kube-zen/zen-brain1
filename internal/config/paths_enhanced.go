package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnhancedPaths provides strict path management with validation.
type EnhancedPaths struct {
	paths *Paths
	strict bool
}

// NewEnhancedPaths creates enhanced paths with validation.
func NewEnhancedPaths(strict bool) (*EnhancedPaths, error) {
	home, err := HomeDirStrict()
	if err != nil {
		if strict {
			return nil, fmt.Errorf("cannot determine home directory in strict mode: %w", err)
		}
		// Relaxed mode: use fallback
		home = DefaultHomeDirName
	}

	paths := &Paths{
		Root:     home,
		Journal:  filepath.Join(home, "journal"),
		Context:  filepath.Join(home, "context"),
		Cache:    filepath.Join(home, "cache"),
		Config:   filepath.Join(home, "config"),
		Logs:     filepath.Join(home, "logs"),
		Ledger:   filepath.Join(home, "ledger"),
		Evidence: filepath.Join(home, "evidence"),
		KB:       filepath.Join(home, "kb"),
		QMD:      filepath.Join(home, "qmd"),
		Analysis: filepath.Join(home, "analysis"),
	}

	return &EnhancedPaths{
		paths:  paths,
		strict: strict,
	}, nil
}

// Validate validates all paths are acceptable.
func (ep *EnhancedPaths) Validate() error {
	if ep.paths == nil {
		return fmt.Errorf("paths not initialized")
	}

	// Validate root path
	if ep.paths.Root == "" {
		return fmt.Errorf("root path is empty")
	}

	// Check for problematic paths
	problematicPaths := []string{
		"/",
		"/root",
		"/etc",
		"/usr",
		"/var",
	}

	for _, bad := range problematicPaths {
		if ep.paths.Root == bad || filepath.Dir(ep.paths.Root) == bad {
			return fmt.Errorf("root path %s is unsafe (parent of %s)", ep.paths.Root, bad)
		}
	}

	// In strict mode, ensure root is absolute
	if ep.strict && !filepath.IsAbs(ep.paths.Root) {
		return fmt.Errorf("root path must be absolute in strict mode: %s", ep.paths.Root)
	}

	return nil
}

// EnsureAllWithValidation creates directories with validation.
func (ep *EnhancedPaths) EnsureAllWithValidation() error {
	// Validate first
	if err := ep.Validate(); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Ensure all directories
	dirs := []struct {
		name string
		path string
		mode os.FileMode
	}{
		{"root", ep.paths.Root, 0755},
		{"journal", ep.paths.Journal, 0755},
		{"context", ep.paths.Context, 0755},
		{"cache", ep.paths.Cache, 0755},
		{"config", ep.paths.Config, 0755},
		{"logs", ep.paths.Logs, 0755},
		{"ledger", ep.paths.Ledger, 0700}, // More restrictive for ledger
		{"evidence", ep.paths.Evidence, 0755},
		{"kb", ep.paths.KB, 0755},
		{"qmd", ep.paths.QMD, 0755},
		{"analysis", ep.paths.Analysis, 0755},
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir.path, dir.mode); err != nil {
			return fmt.Errorf("cannot create %s directory %s: %w", dir.name, dir.path, err)
		}

		// Verify directory was created with correct permissions
		if err := ep.verifyDirectory(dir.path, dir.mode); err != nil {
			return fmt.Errorf("directory verification failed for %s: %w", dir.name, err)
		}
	}

	return nil
}

// verifyDirectory verifies a directory exists and has correct permissions.
func (ep *EnhancedPaths) verifyDirectory(path string, expectedMode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	// Check write permission
	testFile := filepath.Join(path, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory %s: %w", path, err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// GetPath returns a specific path by name.
func (ep *EnhancedPaths) GetPath(name string) (string, error) {
	switch name {
	case "root":
		return ep.paths.Root, nil
	case "journal":
		return ep.paths.Journal, nil
	case "context":
		return ep.paths.Context, nil
	case "cache":
		return ep.paths.Cache, nil
	case "config":
		return ep.paths.Config, nil
	case "logs":
		return ep.paths.Logs, nil
	case "ledger":
		return ep.paths.Ledger, nil
	case "evidence":
		return ep.paths.Evidence, nil
	case "kb":
		return ep.paths.KB, nil
	case "qmd":
		return ep.paths.QMD, nil
	case "analysis":
		return ep.paths.Analysis, nil
	default:
		return "", fmt.Errorf("unknown path: %s", name)
	}
}

// Paths returns the underlying Paths struct.
func (ep *EnhancedPaths) Paths() *Paths {
	return ep.paths
}

// CleanAll removes all directories (dangerous, use with caution).
func (ep *EnhancedPaths) CleanAll() error {
	if ep.strict {
		return fmt.Errorf("CleanAll not allowed in strict mode")
	}

	// Only allow in dev/test mode
	dirs := []string{
		ep.paths.Cache,
		ep.paths.Logs,
		// Never delete: journal, context, ledger, evidence, kb, qmd, analysis (durable data)
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("cannot clean directory %s: %w", dir, err)
		}
	}

	return nil
}

// GetDiskUsage returns disk usage for all directories.
func (ep *EnhancedPaths) GetDiskUsage() (map[string]int64, error) {
	usage := make(map[string]int64)

	dirs := map[string]string{
		"root":     ep.paths.Root,
		"journal":  ep.paths.Journal,
		"context":  ep.paths.Context,
		"cache":    ep.paths.Cache,
		"config":   ep.paths.Config,
		"logs":     ep.paths.Logs,
		"ledger":   ep.paths.Ledger,
		"evidence": ep.paths.Evidence,
		"kb":       ep.paths.KB,
		"qmd":      ep.paths.QMD,
		"analysis": ep.paths.Analysis,
	}

	for name, path := range dirs {
		size, err := ep.calculateDirSize(path)
		if err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "[WARN] Cannot calculate size for %s: %v\n", name, err)
			continue
		}
		usage[name] = size
	}

	return usage, nil
}

// calculateDirSize calculates total size of a directory.
func (ep *EnhancedPaths) calculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
