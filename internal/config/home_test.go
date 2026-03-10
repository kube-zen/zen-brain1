package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeDir_Default(t *testing.T) {
	// Clear env var
	os.Unsetenv(EnvHomeDir)

	home := HomeDir()
	expectedSuffix := filepath.FromSlash("/.zen-brain")
	if !strings.Contains(home, expectedSuffix) {
		t.Errorf("HomeDir() = %q, expected to contain %q", home, expectedSuffix)
	}
}

func TestHomeDir_EnvOverride(t *testing.T) {
	customPath := "/tmp/custom-zen-brain"
	os.Setenv(EnvHomeDir, customPath)
	defer os.Unsetenv(EnvHomeDir)

	home := HomeDir()
	if home != customPath {
		t.Errorf("HomeDir() = %q, expected %q", home, customPath)
	}
}

func TestDefaultPaths(t *testing.T) {
	os.Unsetenv(EnvHomeDir)
	paths := DefaultPaths()

	if paths.Root == "" {
		t.Error("Root should not be empty")
	}
	checkContains(t, paths.Journal, "journal")
	checkContains(t, paths.Context, "context")
	checkContains(t, paths.Cache, "cache")
	checkContains(t, paths.Config, "config")
	checkContains(t, paths.Logs, "logs")
	checkContains(t, paths.Ledger, "ledger")
	checkContains(t, paths.Evidence, "evidence")
	checkContains(t, paths.KB, "kb")
	checkContains(t, paths.QMD, "qmd")
}

func checkContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("%q should contain %q", s, substr)
	}
}
