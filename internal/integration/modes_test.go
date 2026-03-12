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