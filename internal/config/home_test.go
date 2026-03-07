package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHomeDir_Default(t *testing.T) {
	// Clear env var
	os.Unsetenv(EnvHomeDir)

	home := HomeDir()
	expectedSuffix := filepath.FromSlash("/.zen-brain")
	assert.Contains(t, home, expectedSuffix)
}

func TestHomeDir_EnvOverride(t *testing.T) {
	customPath := "/tmp/custom-zen-brain"
	os.Setenv(EnvHomeDir, customPath)
	defer os.Unsetenv(EnvHomeDir)

	home := HomeDir()
	assert.Equal(t, customPath, home)
}

func TestDefaultPaths(t *testing.T) {
	os.Unsetenv(EnvHomeDir)
	paths := DefaultPaths()

	assert.NotEmpty(t, paths.Root)
	assert.Contains(t, paths.Journal, "journal")
	assert.Contains(t, paths.Context, "context")
	assert.Contains(t, paths.Cache, "cache")
	assert.Contains(t, paths.Config, "config")
	assert.Contains(t, paths.Logs, "logs")
}
