// Package runtime tests doctor and reporting functionality.
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDoctor_PrintsSummary(t *testing.T) {
	ctx := context.Background()
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true, Message: "OK"},
		Tier1Hot:   CapabilityStatus{Name: "tier1", Mode: ModeReal, Healthy: true, Required: true, Message: "connected"},
		Tier2Warm:  CapabilityStatus{Name: "tier2", Mode: ModeMock, Healthy: true, Message: "mock ready"},
		Tier3Cold:  CapabilityStatus{Name: "tier3", Mode: ModeDisabled, Healthy: true, Message: "disabled"},
		Journal:    CapabilityStatus{Name: "journal", Mode: ModeReal, Healthy: true, Message: "active"},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: false, Message: "stub"},
		MessageBus: CapabilityStatus{Name: "messagebus", Mode: ModeReal, Healthy: true, Required: true, Message: "connected"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Doctor(ctx, nil, report)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify key fields appear in output
	if !strings.Contains(output, "Block 3 Runtime Doctor") {
		t.Error("Doctor output missing header")
	}
	if !strings.Contains(output, "ZenContext") {
		t.Error("Doctor output missing ZenContext")
	}
	if !strings.Contains(output, "Tier1 (Hot)") {
		t.Error("Doctor output missing Tier1")
	}
	if !strings.Contains(output, "healthy=true") {
		t.Error("Doctor output missing healthy status")
	}
	if !strings.Contains(output, "required=true") {
		t.Error("Doctor output missing required status")
	}
}

func TestDoctor_NilReport(t *testing.T) {
	ctx := context.Background()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Doctor(ctx, nil, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "runtime report: not available") {
		t.Error("Doctor with nil report should show 'not available'")
	}
}

func TestReportJSON_ValidReport(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{
			Name:     "zen_context",
			Mode:     ModeReal,
			Healthy:  true,
			Required: true,
			Message:  "OK",
			Metadata: map[string]interface{}{"version": "1.0"},
		},
		Tier1Hot: CapabilityStatus{Name: "tier1", Mode: ModeReal, Healthy: true, Required: true},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ReportJSON(report)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("ReportJSON unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify it's valid JSON and contains expected fields
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("ReportJSON output invalid JSON: %v", err)
	}

	if _, ok := decoded["zen_context"]; !ok {
		t.Error("ReportJSON missing zen_context field")
	}

	if _, ok := decoded["tier1_hot"]; !ok {
		t.Error("ReportJSON missing tier1_hot field")
	}
}

func TestReportJSON_NilReport(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := ReportJSON(nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("ReportJSON with nil report should not error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != "null\n" {
		t.Errorf("ReportJSON(nil) = %q, want 'null'", output)
	}
}

func TestPing_AllHealthy(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1", Mode: ModeReal, Healthy: true, Required: true},
		Tier2Warm:  CapabilityStatus{Name: "tier2", Mode: ModeMock, Healthy: true, Required: false},
		Tier3Cold:  CapabilityStatus{Name: "tier3", Mode: ModeDisabled, Healthy: true, Required: false},
		Journal:    CapabilityStatus{Name: "journal", Mode: ModeReal, Healthy: true, Required: false},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: false},
		MessageBus: CapabilityStatus{Name: "messagebus", Mode: ModeReal, Healthy: true, Required: true},
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	_, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	exitCode := Ping(report)

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if exitCode != 0 {
		t.Errorf("Ping(healthy) = %d, want 0", exitCode)
	}

	var bufOut bytes.Buffer
	bufOut.ReadFrom(rOut)
	if bufOut.String() != "ok\n" {
		t.Errorf("Ping output = %q, want 'ok'", bufOut.String())
	}
}

func TestPing_RequiredUnhealthy(t *testing.T) {
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1", Mode: ModeReal, Healthy: false, Required: true, Message: "connection failed"},
		Tier2Warm:  CapabilityStatus{Name: "tier2", Mode: ModeMock, Healthy: true, Required: false},
		Tier3Cold:  CapabilityStatus{Name: "tier3", Mode: ModeDisabled, Healthy: true, Required: false},
		Journal:    CapabilityStatus{Name: "journal", Mode: ModeReal, Healthy: true, Required: false},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: false},
		MessageBus: CapabilityStatus{Name: "messagebus", Mode: ModeReal, Healthy: true, Required: true},
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := Ping(report)

	w.Close()
	os.Stderr = oldStderr

	if exitCode != 1 {
		t.Errorf("Ping(required unhealthy) = %d, want 1", exitCode)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "tier1") || !strings.Contains(output, "unhealthy") {
		t.Error("Ping should print unhealthy capability name and message")
	}
}

func TestPing_NilReport(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := Ping(nil)

	w.Close()
	os.Stderr = oldStderr

	if exitCode != 1 {
		t.Errorf("Ping(nil) = %d, want 1", exitCode)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "no runtime report") {
		t.Error("Ping(nil) should print error message")
	}
}

func TestPing_NonRequiredUnhealthy(t *testing.T) {
	// Non-required capabilities being unhealthy should not cause failure
	report := &RuntimeReport{
		ZenContext: CapabilityStatus{Name: "zen_context", Mode: ModeReal, Healthy: true, Required: true},
		Tier1Hot:   CapabilityStatus{Name: "tier1", Mode: ModeReal, Healthy: true, Required: true},
		Tier2Warm:  CapabilityStatus{Name: "tier2", Mode: ModeMock, Healthy: false, Required: false, Message: "degraded"},
		Tier3Cold:  CapabilityStatus{Name: "tier3", Mode: ModeDisabled, Healthy: false, Required: false},
		Journal:    CapabilityStatus{Name: "journal", Mode: ModeReal, Healthy: true, Required: false},
		Ledger:     CapabilityStatus{Name: "ledger", Mode: ModeStub, Healthy: true, Required: false},
		MessageBus: CapabilityStatus{Name: "messagebus", Mode: ModeReal, Healthy: true, Required: true},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := Ping(report)

	w.Close()
	os.Stdout = oldStdout

	if exitCode != 0 {
		t.Errorf("Ping(non-required unhealthy) = %d, want 0", exitCode)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if buf.String() != "ok\n" {
		t.Errorf("Ping output = %q, want 'ok'", buf.String())
	}
}
