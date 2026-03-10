// Package runtime provides Block 3 canonical bootstrap and capability reporting.
package runtime

// DependencyMode indicates how a capability is provided.
type DependencyMode string

const (
	ModeReal     DependencyMode = "real"
	ModeMock     DependencyMode = "mock"
	ModeStub     DependencyMode = "stub"
	ModeDisabled DependencyMode = "disabled"
	ModeDegraded DependencyMode = "degraded"
)

// CapabilityStatus describes one Block 3 capability.
type CapabilityStatus struct {
	Name     string                 `json:"name"`
	Mode     DependencyMode         `json:"mode"`
	Healthy  bool                   `json:"healthy"`
	Required bool                   `json:"required"`
	Message  string                 `json:"message,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RuntimeReport is the unified Block 3 capability report.
type RuntimeReport struct {
	ZenContext CapabilityStatus `json:"zen_context"`
	Tier1Hot   CapabilityStatus  `json:"tier1_hot"`
	Tier2Warm  CapabilityStatus  `json:"tier2_warm"`
	Tier3Cold  CapabilityStatus  `json:"tier3_cold"`
	Journal    CapabilityStatus  `json:"journal"`
	Ledger     CapabilityStatus  `json:"ledger"`
	MessageBus CapabilityStatus  `json:"message_bus"`
}
