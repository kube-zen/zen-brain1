package integration

import (
	"os"

	"github.com/kube-zen/zen-brain1/internal/config"
	"github.com/kube-zen/zen-brain1/internal/runtime"
)

// ComponentMode represents the operating mode of an Office pipeline component.
type ComponentMode string

const (
	ModeReal     ComponentMode = "real"
	ModeStub     ComponentMode = "stub"
	ModeDisabled ComponentMode = "disabled"
	ModeDegraded ComponentMode = "degraded"
)

// ComponentStatus describes a component's mode and requirement.
type ComponentStatus struct {
	Name     string        `json:"name"`
	Mode     ComponentMode `json:"mode"`
	Required bool          `json:"required"`
	Enabled  bool          `json:"enabled"`
	Message  string        `json:"message,omitempty"`
}

// GetOfficeComponentStatus returns the status of each Office pipeline component
// based on config and environment, without initializing actual clients.
func GetOfficeComponentStatus(cfg *config.Config) []ComponentStatus {
	var statuses []ComponentStatus

	strictMode := runtime.IsStrictProfile()

	// Knowledge Base
	kbStatus := ComponentStatus{Name: "knowledge_base"}
	kbRequired := cfg != nil && cfg.KB.Required
	kbStatus.Required = kbRequired
	realKBPossible := cfg != nil && cfg.KB.DocsRepo != "" && cfg.QMD.BinaryPath != ""
	allowStubKB := os.Getenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_KB") == "1"

	if realKBPossible {
		kbStatus.Mode = ModeReal
		kbStatus.Enabled = true
	} else {
		if kbRequired {
			kbStatus.Mode = ModeDisabled
			kbStatus.Enabled = false
			kbStatus.Message = "KB required but not configured (set kb.docs_repo and qmd.binary_path)"
		} else if strictMode {
			kbStatus.Mode = ModeDisabled
			kbStatus.Enabled = false
			kbStatus.Message = "KB not configured in strict runtime"
		} else if allowStubKB {
			kbStatus.Mode = ModeStub
			kbStatus.Enabled = true
			kbStatus.Message = "explicit stub opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1"
		} else {
			kbStatus.Mode = ModeDisabled
			kbStatus.Enabled = false
			kbStatus.Message = "KB not configured; set kb.docs_repo + qmd.binary_path for real KB or ZEN_BRAIN_OFFICE_ALLOW_STUB_KB=1 for stub"
		}
	}
	statuses = append(statuses, kbStatus)

	// Ledger
	ledgerStatus := ComponentStatus{Name: "ledger"}
	ledgerRequired := cfg != nil && cfg.Ledger.Required
	ledgerEnabled := cfg != nil && cfg.Ledger.Enabled
	ledgerStatus.Required = ledgerRequired
	allowStubLedger := os.Getenv("ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER") == "1"

	if ledgerEnabled {
		ledgerStatus.Mode = ModeReal
		ledgerStatus.Enabled = true
		// Note: real ledger also requires host/port; that's validated during init.
	} else {
		if ledgerRequired {
			ledgerStatus.Mode = ModeDisabled
			ledgerStatus.Enabled = false
			ledgerStatus.Message = "Ledger required but not enabled (set ledger.enabled=true)"
		} else if strictMode {
			ledgerStatus.Mode = ModeDisabled
			ledgerStatus.Enabled = false
			ledgerStatus.Message = "Ledger not enabled in strict runtime"
		} else if allowStubLedger {
			ledgerStatus.Mode = ModeStub
			ledgerStatus.Enabled = true
			ledgerStatus.Message = "explicit stub opt-in via ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1"
		} else {
			ledgerStatus.Mode = ModeDisabled
			ledgerStatus.Enabled = false
			ledgerStatus.Message = "Ledger not enabled; set ledger.enabled=true for real ledger or ZEN_BRAIN_OFFICE_ALLOW_STUB_LEDGER=1 for stub"
		}
	}
	statuses = append(statuses, ledgerStatus)

	// Message Bus
	msgBusStatus := ComponentStatus{Name: "message_bus"}
	msgBusRequired := cfg != nil && cfg.MessageBus.Required
	msgBusEnabled := cfg != nil && cfg.MessageBus.Enabled
	msgBusStatus.Required = msgBusRequired

	if msgBusEnabled {
		msgBusStatus.Mode = ModeReal
		msgBusStatus.Enabled = true
	} else {
		if msgBusRequired {
			msgBusStatus.Mode = ModeDisabled
			msgBusStatus.Enabled = false
			msgBusStatus.Message = "Message Bus required but not enabled (set message_bus.enabled=true)"
		} else {
			msgBusStatus.Mode = ModeDisabled
			msgBusStatus.Enabled = false
			msgBusStatus.Message = "Message Bus disabled"
		}
	}
	statuses = append(statuses, msgBusStatus)

	return statuses
}