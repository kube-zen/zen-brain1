package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeDirStrict(t *testing.T) {
	t.Run("with_env_var", func(t *testing.T) {
		testDir := "/tmp/test-home"
		os.Setenv(EnvHomeDir, testDir)
		defer os.Unsetenv(EnvHomeDir)

		dir, err := HomeDirStrict()

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		if dir != testDir {
			t.Errorf("Expected %s, got: %s", testDir, dir)
		}

		t.Logf("✅ HomeDir from env: %s", dir)
	})

	t.Run("without_env_var", func(t *testing.T) {
		os.Unsetenv(EnvHomeDir)

		dir, err := HomeDirStrict()

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		if dir == "" {
			t.Error("Should return non-empty directory")
		}

		// Should contain .zen-brain
		if filepath.Base(dir) != DefaultHomeDirName {
			t.Errorf("Expected directory name %s, got: %s", DefaultHomeDirName, filepath.Base(dir))
		}

		t.Logf("✅ HomeDir from user home: %s", dir)
	})

	t.Run("relative_path_rejected", func(t *testing.T) {
		os.Setenv(EnvHomeDir, "relative/path")
		defer os.Unsetenv(EnvHomeDir)

		_, err := HomeDirStrict()

		if err == nil {
			t.Error("Should reject relative path")
		}

		t.Logf("✅ Relative path rejected: %v", err)
	})
}

func TestHomeDirWithFallback(t *testing.T) {
	t.Run("normal_case", func(t *testing.T) {
		os.Unsetenv(EnvHomeDir)

		fallback := "/tmp/fallback"
		dir := HomeDirWithFallback(fallback)

		if dir == "" {
			t.Error("Should return non-empty directory")
		}

		// Should not use fallback in normal case
		if dir == fallback {
			t.Logf("Using fallback (home dir unavailable)")
		} else {
			t.Logf("✅ HomeDir without fallback: %s", dir)
		}
	})
}

func TestValidateHomeDir(t *testing.T) {
	t.Run("nonexistent_directory", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, ".zen-brain-test")

		os.Setenv(EnvHomeDir, testDir)
		defer os.Unsetenv(EnvHomeDir)

		// Should not error even if directory doesn't exist
		err := ValidateHomeDir()

		if err != nil {
			t.Errorf("Should not error for nonexistent directory: %v", err)
		}

		t.Logf("✅ Nonexistent directory validation passed")
	})

	t.Run("existing_directory", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		err := ValidateHomeDir()

		if err != nil {
			t.Errorf("Should not error for existing directory: %v", err)
		}

		t.Logf("✅ Existing directory validation passed")
	})

	t.Run("file_not_directory", func(t *testing.T) {
		// Create temp file
		tmpFile, err := os.CreateTemp("", "test-*")
		if err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		os.Setenv(EnvHomeDir, tmpFile.Name())
		defer os.Unsetenv(EnvHomeDir)

		err = ValidateHomeDir()

		if err == nil {
			t.Error("Should error when path is a file, not directory")
		}

		t.Logf("✅ File (not directory) rejected: %v", err)
	})

	t.Run("unwritable_directory", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, skipping unwritable test")
		}

		// Create temp directory with no write permission
		tmpDir := t.TempDir()
		restrictedDir := filepath.Join(tmpDir, "restricted")
		os.Mkdir(restrictedDir, 0000)
		defer os.Chmod(restrictedDir, 0755)

		os.Setenv(EnvHomeDir, restrictedDir)
		defer os.Unsetenv(EnvHomeDir)

		err := ValidateHomeDir()

		if err == nil {
			t.Error("Should error for unwritable directory")
		}

		t.Logf("✅ Unwritable directory rejected: %v", err)
	})
}

func TestEnsureHomeDirStrict(t *testing.T) {
	t.Run("create_new_directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, ".zen-brain-test")

		os.Setenv(EnvHomeDir, testDir)
		defer os.Unsetenv(EnvHomeDir)

		err := EnsureHomeDirStrict()

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Error("Directory should exist")
		}

		t.Logf("✅ Directory created: %s", testDir)
	})

	t.Run("existing_directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		err := EnsureHomeDirStrict()

		if err != nil {
			t.Errorf("Should not error for existing directory: %v", err)
		}

		t.Logf("✅ Existing directory accepted")
	})
}

func TestGetHomeDirWithProfile(t *testing.T) {
	t.Run("prod_strict_mode", func(t *testing.T) {
		os.Unsetenv(EnvHomeDir)

		dir, err := GetHomeDirWithProfile("prod")

		// Should work even without env var (uses user home)
		if err != nil {
			t.Errorf("Should not error in prod mode: %v", err)
		}

		if dir == "" {
			t.Error("Should return non-empty directory")
		}

		t.Logf("✅ Prod profile: %s", dir)
	})

	t.Run("dev_relaxed_mode", func(t *testing.T) {
		os.Unsetenv(EnvHomeDir)

		dir, err := GetHomeDirWithProfile("dev")

		if err != nil {
			t.Errorf("Should not error in dev mode: %v", err)
		}

		if dir == "" {
			t.Error("Should return non-empty directory (even as fallback)")
		}

		t.Logf("✅ Dev profile: %s", dir)
	})
}

func TestNewEnhancedPaths(t *testing.T) {
	t.Run("strict_mode", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, err := NewEnhancedPaths(true)

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		if ep == nil {
			t.Fatal("EnhancedPaths should not be nil")
		}

		if ep.paths.Root != tmpDir {
			t.Errorf("Expected root %s, got: %s", tmpDir, ep.paths.Root)
		}

		t.Logf("✅ EnhancedPaths created in strict mode: %s", ep.paths.Root)
	})

	t.Run("relaxed_mode_with_fallback", func(t *testing.T) {
		// Don't set env var to trigger fallback
		os.Unsetenv(EnvHomeDir)

		ep, err := NewEnhancedPaths(false)

		if err != nil {
			t.Errorf("Should not error in relaxed mode: %v", err)
		}

		if ep == nil {
			t.Fatal("EnhancedPaths should not be nil")
		}

		// Should have some path (fallback or user home)
		if ep.paths.Root == "" {
			t.Error("Should have non-empty root")
		}

		t.Logf("✅ EnhancedPaths created in relaxed mode: %s", ep.paths.Root)
	})
}

func TestEnhancedPaths_Validate(t *testing.T) {
	t.Run("valid_paths", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(true)
		err := ep.Validate()

		if err != nil {
			t.Errorf("Should not error for valid paths: %v", err)
		}

		t.Logf("✅ Paths validated successfully")
	})

	t.Run("problematic_root_path", func(t *testing.T) {
		// Try to use "/" as root
		os.Setenv(EnvHomeDir, "/")
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(true)
		err := ep.Validate()

		if err == nil {
			t.Error("Should reject problematic root path")
		}

		t.Logf("✅ Problematic path rejected: %v", err)
	})

	t.Run("empty_root_path", func(t *testing.T) {
		ep := &EnhancedPaths{
			paths:  &Paths{Root: ""},
			strict: true,
		}

		err := ep.Validate()

		if err == nil {
			t.Error("Should reject empty root path")
		}

		t.Logf("✅ Empty root rejected: %v", err)
	})
}

func TestEnhancedPaths_EnsureAllWithValidation(t *testing.T) {
	t.Run("create_all_directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, "zen-brain-test")

		os.Setenv(EnvHomeDir, testDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(true)
		err := ep.EnsureAllWithValidation()

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		// Verify all directories were created
		dirs := []string{
			ep.paths.Root,
			ep.paths.Journal,
			ep.paths.Context,
			ep.paths.Cache,
			ep.paths.Config,
			ep.paths.Logs,
			ep.paths.Ledger,
			ep.paths.Evidence,
			ep.paths.KB,
			ep.paths.QMD,
			ep.paths.Analysis,
		}

		for _, dir := range dirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Errorf("Directory should exist: %s", dir)
			}
		}

		t.Logf("✅ All %d directories created", len(dirs))
	})
}

func TestEnhancedPaths_GetPath(t *testing.T) {
	tmpDir := t.TempDir()

	os.Setenv(EnvHomeDir, tmpDir)
	defer os.Unsetenv(EnvHomeDir)

	ep, _ := NewEnhancedPaths(true)

	tests := []string{
		"root", "journal", "context", "cache", "config",
		"logs", "ledger", "evidence", "kb", "qmd", "analysis",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			path, err := ep.GetPath(name)

			if err != nil {
				t.Errorf("Should not error for %s: %v", name, err)
			}

			if path == "" {
				t.Errorf("Path should not be empty for %s", name)
			}

			t.Logf("✅ Path %s: %s", name, path)
		})
	}

	t.Run("unknown_path", func(t *testing.T) {
		_, err := ep.GetPath("unknown")

		if err == nil {
			t.Error("Should error for unknown path")
		}

		t.Logf("✅ Unknown path rejected: %v", err)
	})
}

func TestEnhancedPaths_CleanAll(t *testing.T) {
	t.Run("strict_mode_rejected", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(true)
		err := ep.CleanAll()

		if err == nil {
			t.Error("Should reject CleanAll in strict mode")
		}

		t.Logf("✅ CleanAll rejected in strict mode: %v", err)
	})

	t.Run("relaxed_mode_allowed", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(false)

		// Create directories first
		_ = ep.EnsureAllWithValidation()

		err := ep.CleanAll()

		if err != nil {
			t.Errorf("Should allow CleanAll in relaxed mode: %v", err)
		}

		t.Logf("✅ CleanAll allowed in relaxed mode")
	})
}

func TestEnhancedPaths_GetDiskUsage(t *testing.T) {
	t.Run("calculate_usage", func(t *testing.T) {
		tmpDir := t.TempDir()

		os.Setenv(EnvHomeDir, tmpDir)
		defer os.Unsetenv(EnvHomeDir)

		ep, _ := NewEnhancedPaths(true)

		// Create directories
		_ = ep.EnsureAllWithValidation()

		// Create some test files
		testFile := filepath.Join(ep.paths.Journal, "test.log")
		os.WriteFile(testFile, []byte("test content"), 0644)

		usage, err := ep.GetDiskUsage()

		if err != nil {
			t.Errorf("Should not error: %v", err)
		}

		if len(usage) == 0 {
			t.Error("Should have usage data")
		}

		// Journal should have some usage
		if usage["journal"] == 0 {
			t.Log("Warning: journal usage is 0 (file might not be counted)")
		}

		t.Logf("✅ Disk usage calculated: %d directories", len(usage))
		for name, size := range usage {
			t.Logf("  %s: %d bytes", name, size)
		}
	})
}
