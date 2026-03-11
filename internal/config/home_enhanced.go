package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// HomeDirStrict returns the home directory with strict validation.
// In strict mode, it fails if home directory cannot be determined.
func HomeDirStrict() (string, error) {
	// Priority 1: Environment variable
	if env := os.Getenv(EnvHomeDir); env != "" {
		// Validate path is absolute
		if !filepath.IsAbs(env) {
			return "", fmt.Errorf("ZEN_BRAIN_HOME must be absolute path, got: %s", env)
		}
		return env, nil
	}

	// Priority 2: User home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user home directory: %w", err)
	}

	if home == "" {
		return "", fmt.Errorf("user home directory is empty")
	}

	return filepath.Join(home, DefaultHomeDirName), nil
}

// HomeDirWithFallback returns home directory with explicit fallback strategy.
// This is safer than the original HomeDir() which silently falls back.
func HomeDirWithFallback(fallback string) string {
	dir, err := HomeDirStrict()
	if err != nil {
		// Log the error but use fallback
		fmt.Fprintf(os.Stderr, "[WARN] HomeDir: %v, using fallback: %s\n", err, fallback)
		return fallback
	}
	return dir
}

// ValidateHomeDir checks if home directory is valid and accessible.
func ValidateHomeDir() error {
	dir, err := HomeDirStrict()
	if err != nil {
		return err
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist - that's OK, will be created
			return nil
		}
		return fmt.Errorf("cannot access home directory %s: %w", dir, err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("home path %s is not a directory", dir)
	}

	// Check if we can write to it
	testFile := filepath.Join(dir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to home directory %s: %w", dir, err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// EnsureHomeDirStrict creates the home directory with strict validation.
func EnsureHomeDirStrict() error {
	dir, err := HomeDirStrict()
	if err != nil {
		return err
	}

	// Validate before creating
	if err := ValidateHomeDir(); err != nil {
		return err
	}

	// Create with proper permissions
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create home directory %s: %w", dir, err)
	}

	return nil
}

// GetHomeDirWithProfile returns home directory based on runtime profile.
// In prod/staging: strict mode (fails on errors)
// In dev/test: relaxed mode (uses fallback)
func GetHomeDirWithProfile(profile string) (string, error) {
	switch profile {
	case "prod", "staging":
		// Strict mode - no fallback
		return HomeDirStrict()
	default:
		// Relaxed mode - use current directory as fallback
		dir, err := HomeDirStrict()
		if err != nil {
			// Fallback to current directory in dev/test mode
			cwd, _ := os.Getwd()
			return filepath.Join(cwd, DefaultHomeDirName), nil
		}
		return dir, nil
	}
}
