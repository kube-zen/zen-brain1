// Package config provides configuration management for zen-brain.
// This includes configurable home directory paths and runtime settings.
package config

import (
	"os"
	"path/filepath"
)

const (
	// DefaultHomeDirName is the default directory name under user home.
	DefaultHomeDirName = ".zen-brain"

	// EnvHomeDir is the environment variable for overriding the home directory.
	EnvHomeDir = "ZEN_BRAIN_HOME"
)

// HomeDir returns the configured home directory for zen-brain.
// Priority: ZEN_BRAIN_HOME env var > ~/.zen-brain
func HomeDir() string {
	if env := os.Getenv(EnvHomeDir); env != "" {
		return env
	}

	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home cannot be determined
		return DefaultHomeDirName
	}

	return filepath.Join(home, DefaultHomeDirName)
}

// EnsureHomeDir creates the home directory if it doesn't exist.
func EnsureHomeDir() error {
	dir := HomeDir()
	return os.MkdirAll(dir, 0755)
}
