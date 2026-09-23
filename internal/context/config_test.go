package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/context/tier1"
)

// TestDefaultZenContextConfig tests the default configuration is properly initialized.
func TestDefaultZenContextConfig(t *testing.T) {
	config := DefaultZenContextConfig()

	if config.ClusterID != "default" {
		t.Errorf("ClusterID should be 'default', got: %s", config.ClusterID)
	}

	if config.Tier1Redis == nil {
		t.Error("Tier1Redis should not be nil")
	} else {
		redisConfig := tier1.DefaultRedisConfig()
		if config.Tier1Redis.Addr != redisConfig.Addr {
			t.Errorf("Tier1Redis.Addr should match default, got: %s vs %s",
				config.Tier1Redis.Addr, redisConfig.Addr)
		}
	}

	if config.Tier2QMD == nil {
		t.Error("Tier2QMD should not be nil")
	} else {
		wantRepo := filepath.Join(os.Getenv("HOME"), ".zen", "zen-brain1", "zen-docs")
		if config.Tier2QMD.RepoPath != wantRepo {
			t.Errorf("Tier2QMD.RepoPath should be '%s' (fail-closed HomeDir anchoring), got: %s",
				wantRepo, config.Tier2QMD.RepoPath)
		}
		if config.Tier2QMD.Verbose != false {
			t.Errorf("Tier2QMD.Verbose should be false, got: %v", config.Tier2QMD.Verbose)
		}
	}

	if config.Tier3S3 == nil {
		t.Error("Tier3S3 should not be nil")
	}

	if config.Journal == nil {
		t.Error("Journal should not be nil")
	} else {
		wantJournal := filepath.Join(os.Getenv("HOME"), ".zen", "zen-brain1", "journal")
		if config.Journal.JournalPath != wantJournal {
			t.Errorf("Journal.JournalPath should be '%s' (fail-closed HomeDir anchoring), got: %s",
				wantJournal, config.Journal.JournalPath)
		}
		if config.Journal.EnableQueryIndex != true {
			t.Errorf("Journal.EnableQueryIndex should be true, got: %v",
				config.Journal.EnableQueryIndex)
		}
	}

	if config.Verbose != false {
		t.Errorf("Verbose should be false, got: %v", config.Verbose)
	}
}
