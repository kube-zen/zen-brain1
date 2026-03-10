// doc_sync_test.go provides a cheap assertion that contract docs still mention key concepts.
// It does not parse markdown; it catches obvious drift when someone changes docs or contracts.
package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataModelDocMentionsKeyConcepts(t *testing.T) {
	// Find module root (directory containing go.mod)
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("Getwd: %v", err)
		return
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("go.mod not found")
			return
		}
		dir = parent
	}
	path := filepath.Join(dir, "docs", "02-CONTRACTS", "DATA_MODEL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("DATA_MODEL.md not found or unreadable: %v", err)
		return
	}
	content := string(data)
	checks := []struct {
		name  string
		substr string
	}{
		{"WorkType", "WorkType"},
		{"WorkDomain", "WorkDomain"},
		{"WorkTags", "WorkTags"},
		{"EvidenceRequirement", "EvidenceRequirement"},
	}
	for _, c := range checks {
		if !strings.Contains(content, c.substr) {
			t.Errorf("docs/02-CONTRACTS/DATA_MODEL.md should mention %s", c.name)
		}
	}
	// Document should list WorkType values (at least one)
	if !strings.Contains(content, "implementation") {
		t.Error("DATA_MODEL.md should list WorkType values (e.g. implementation)")
	}
	if !strings.Contains(content, "core") {
		t.Error("DATA_MODEL.md should list WorkDomain values (e.g. core)")
	}
}
