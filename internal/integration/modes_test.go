package integration

import (
	"os"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/config"
)

func TestGetOfficeComponentStatus_EmptyConfig(t *testing.T) {
	// No config
	statuses := GetOfficeComponentStatus(nil)
	if len(statuses) != 3 {
		t.Fatalf("expected 3 component statuses, got %d", len(statuses))
	}

	// Expect all disabled
	for _, s := range statuses {
		if s.Mode != ModeDisabled {
			t.Errorf("component %s: expected ModeDisabled, got %s", s.Name, s.Mode)
		}
		if s.Enabled {
			t.Errorf("component %s: expected enabled=false, got true", s.Name)
		}
	}
}

func TestGetOfficeComponentStatus_RealKB(t *testing.T) {
	cfg := &config.Config{
		KB: config.KBConfig{
			DocsRepo: "/path/to/docs",
			Required: true,
		},
		QMD: config.QMDConfig{
			BinaryPath: "qmd",
		},
	}

	statuses := GetOfficeComponentStatus(cfg)
	var kbStatus ComponentStatus
	for _, s := range statuses {
		if s.Name == "knowledge_base" {
			kbStatus = s
			break
		}
	}

	if kbStatus.Mode != ModeReal {
		t.Errorf("KB: expected ModeReal, got %s", kbStatus.Mode)
	}
	if !kbStatus.Enabled {
		t.Errorf("KB: expected enabled=true, got false")
	}
	if !kbStatus.Required {
		t.Errorf("KB: expected required=true, got false")
	}
}

func TestGetOfficeComponentStatus_StubKBExplicitOptIn(t *testing.T) {
	os.Setenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB", "1")
	defer os.Unsetenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB")

	cfg := &config.Config{
		KB: config.KBConfig{
			DocsRepo: "", // missing
		},
	}

	statuses := GetOfficeComponentStatus(cfg)
	var kbStatus ComponentStatus
	for _, s := range statuses {
		if s.Name == "knowledge_base" {
			kbStatus = s
			break
		}
	}

	if kbStatus.Mode != ModeStub {
		t.Errorf("KB: expected ModeStub with explicit opt-in, got %s", kbStatus.Mode)
	}
	if !kbStatus.Enabled {
		t.Errorf("KB: expected enabled=true, got false")
	}
	if kbStatus.Required {
		t.Errorf("KB: expected required=false, got true")
	}
}

func TestGetOfficeComponentStatus_StrictModeRejectsMissingKB(t *testing.T) {
	os.Setenv("ZEN_RUNTIME_PROFILE", "prod")
	defer os.Unsetenv("ZEN_RUNTIME_PROFILE")

	cfg := &config.Config{
		KB: config.KBConfig{
			Required: true,
		},
	}

	statuses := GetOfficeComponentStatus(cfg)
	var kbStatus ComponentStatus
	for _, s := range statuses {
		if s.Name == "knowledge_base" {
			kbStatus = s
			break
		}
	}

	if kbStatus.Mode != ModeDisabled {
		t.Errorf("KB: expected ModeDisabled in strict mode, got %s", kbStatus.Mode)
	}
	if kbStatus.Enabled {
		t.Errorf("KB: expected enabled=false, got true")
	}
	if !kbStatus.Required {
		t.Errorf("KB: expected required=true, got false")
	}
}

func TestGetOfficeComponentStatus_LedgerReal(t *testing.T) {
	cfg := &config.Config{
		Ledger: config.LedgerConfig{
			Enabled:  true,
			Required: true,
			Host:     "localhost",
			Port:     26257,
		},
	}

	statuses := GetOfficeComponentStatus(cfg)
	var ledgerStatus ComponentStatus
	for _, s := range statuses {
		if s.Name == "ledger" {
			ledgerStatus = s
			break
		}
	}

	if ledgerStatus.Mode != ModeReal {
		t.Errorf("Ledger: expected ModeReal, got %s", ledgerStatus.Mode)
	}
	if !ledgerStatus.Enabled {
		t.Errorf("Ledger: expected enabled=true, got false")
	}
	if !ledgerStatus.Required {
		t.Errorf("Ledger: expected required=true, got false")
	}
}

func TestGetOfficeComponentStatus_MessageBusDisabled(t *testing.T) {
	cfg := &config.Config{
		MessageBus: config.MessageBusConfig{
			Enabled:  false,
			Required: false,
		},
	}

	statuses := GetOfficeComponentStatus(cfg)
	var msgBusStatus ComponentStatus
	for _, s := range statuses {
		if s.Name == "message_bus" {
			msgBusStatus = s
			break
		}
	}

	if msgBusStatus.Mode != ModeDisabled {
		t.Errorf("MessageBus: expected ModeDisabled, got %s", msgBusStatus.Mode)
	}
	if msgBusStatus.Enabled {
		t.Errorf("MessageBus: expected enabled=false, got true")
	}
	if msgBusStatus.Required {
		t.Errorf("MessageBus: expected required=false, got true")
	}
}

// TestOfficeComponentStatusMessageClarity ensures component status messages
// are clear and actionable for operators.
func TestOfficeComponentStatusMessageClarity(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func()
		cleanup  func()
		config   *config.Config
		check    func(*testing.T, []ComponentStatus)
	}{
		{
			name: "KB not configured shows guidance",
			setupEnv: func() {
				os.Setenv("ZEN_RUNTIME_PROFILE", "dev")
			},
			cleanup: func() {
				os.Unsetenv("ZEN_RUNTIME_PROFILE")
			},
			config: &config.Config{
				KB: config.KBConfig{
					DocsRepo: "", // explicitly empty to test guidance
				},
				QMD: config.QMDConfig{
					BinaryPath: "", // explicitly empty
				},
			},
			check: func(t *testing.T, statuses []ComponentStatus) {
				var kbStatus ComponentStatus
				for _, s := range statuses {
					if s.Name == "knowledge_base" {
						kbStatus = s
						break
					}
				}

				if kbStatus.Message == "" {
					t.Error("KB status message should not be empty")
				}

				// Message should contain actionable guidance
				if !containsString(kbStatus.Message, "kb.docs_repo") && !containsString(kbStatus.Message, "ZEN_BRAIN_OFFICE_ALLOW_STUB_KB") {
					t.Errorf("KB message should contain actionable guidance: %s", kbStatus.Message)
				}
			},
		},
		{
			name: "Stub KB opt-in message explicit",
			setupEnv: func() {
				os.Setenv("ZEN_RUNTIME_PROFILE", "dev")
				os.Setenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB", "1")
			},
			cleanup: func() {
				os.Unsetenv("ZEN_RUNTIME_PROFILE")
				os.Unsetenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB")
			},
			config: &config.Config{
				KB: config.KBConfig{
					DocsRepo: "", // empty to force stub when opt-in is set
				},
				QMD: config.QMDConfig{
					BinaryPath: "", // empty
				},
			},
			check: func(t *testing.T, statuses []ComponentStatus) {
				var kbStatus ComponentStatus
				for _, s := range statuses {
					if s.Name == "knowledge_base" {
						kbStatus = s
						break
					}
				}

				if kbStatus.Mode != ModeStub {
					t.Errorf("KB should be in stub mode, got %s", kbStatus.Mode)
				}

				// Message should mention explicit opt-in
				if !containsString(kbStatus.Message, "ZEN_BRAIN_OFFICE_ALLOW_STUB_KB") {
					t.Errorf("Stub KB message should mention opt-in env var: %s", kbStatus.Message)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupEnv()
			defer tc.cleanup()

			statuses := GetOfficeComponentStatus(tc.config)

			tc.check(t, statuses)
		})
	}
}

// containsString is a helper to check if a string contains a substring
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
