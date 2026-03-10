package apiserver

import (
	"net/http"
	"testing"

	"github.com/kube-zen/zen-brain1/internal/runtime"
)

func TestRuntimeChecker_ReadinessPass(t *testing.T) {
	report := &runtime.RuntimeReport{
		ZenContext: runtime.CapabilityStatus{Name: "zen_context", Healthy: true, Required: true},
		Tier1Hot:   runtime.CapabilityStatus{Name: "tier1_hot", Healthy: true, Required: true},
		Ledger:     runtime.CapabilityStatus{Name: "ledger", Healthy: true, Required: false},
		MessageBus: runtime.CapabilityStatus{Name: "message_bus", Mode: runtime.ModeDisabled, Healthy: false, Required: false},
	}
	c := NewRuntimeChecker(report)
	if err := c.ReadinessCheck(nil); err != nil {
		t.Errorf("ReadinessCheck should pass: %v", err)
	}
}

func TestRuntimeChecker_ReadinessFailRequiredUnhealthy(t *testing.T) {
	report := &runtime.RuntimeReport{
		ZenContext: runtime.CapabilityStatus{Name: "zen_context", Healthy: false, Required: true, Message: "redis down"},
		Tier1Hot:   runtime.CapabilityStatus{Name: "tier1_hot", Healthy: false, Required: true},
	}
	c := NewRuntimeChecker(report)
	if err := c.ReadinessCheck(nil); err == nil {
		t.Error("ReadinessCheck should fail when required capability unhealthy")
	}
}

func TestRuntimeChecker_Liveness(t *testing.T) {
	c := NewRuntimeChecker(nil)
	if err := c.LivenessCheck((*http.Request)(nil)); err != nil {
		t.Errorf("LivenessCheck should always pass: %v", err)
	}
}
