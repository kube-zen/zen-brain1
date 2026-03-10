// Package taxonomy defines the canonical tag categories used across zen-brain.
// Tags are organized into categories to prevent tag sprawl and ensure consistency.
package taxonomy

import (
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// TagCategory represents a category of tags.
type TagCategory string

const (
	// HumanOrg tags are for human organization (epics, teams, quarters).
	HumanOrg TagCategory = "human_org"

	// Routing tags are for system routing decisions.
	Routing TagCategory = "routing"

	// Policy tags are for ZenGate policy evaluation.
	Policy TagCategory = "policy"

	// Analytics tags are for dashboards and reporting.
	Analytics TagCategory = "analytics"

	// SRED tags are for SR&ED/IRAP evidence categorization.
	SRED TagCategory = "sred"
)

// HumanOrgTags defines example tags for human organization.
var HumanOrgTags = []string{
	"q1-2026",
	"q2-2026",
	"q3-2026",
	"q4-2026",
	"team-platform",
	"team-product",
	"team-security",
	"epic-auth",
	"epic-observability",
	"epic-scalability",
	"sprint-23",
	"sprint-24",
}

// RoutingTags defines example tags for system routing.
var RoutingTags = []string{
	"llm-required",
	"kb-query",
	"long-running",
	"high-memory",
	"gpu-required",
	"local-only",
	"api-fallback",
	"approval-required",
	"evidence-required",
}

// PolicyTags defines example tags for policy evaluation.
var PolicyTags = []string{
	"prod-affecting",
	"requires-approval",
	"audit-trail",
	"pii-handling",
	"compliance-critical",
	"security-sensitive",
	"cost-sensitive",
	"time-sensitive",
}

// AnalyticsTags defines example tags for dashboards and reporting.
var AnalyticsTags = []string{
	"tech-debt",
	"incident",
	"feature",
	"maintenance",
	"bug-fix",
	"performance",
	"security",
	"documentation",
	"testing",
}

// SREDTags defines SR&ED uncertainty categories.
var SREDTags = []contracts.SREDTag{
	contracts.SREDU1DynamicProvisioning,
	contracts.SREDU2SecurityGates,
	contracts.SREDU3DeterministicDelivery,
	contracts.SREDU4Backpressure,
	contracts.SREDExperimentalGeneral,
}

// SREDTagDescription maps SR&ED tags to descriptions.
var SREDTagDescription = map[contracts.SREDTag]string{
	contracts.SREDU1DynamicProvisioning:   "Dynamic resource provisioning uncertainty",
	contracts.SREDU2SecurityGates:         "Security and access control uncertainty",
	contracts.SREDU3DeterministicDelivery: "Deterministic output delivery uncertainty",
	contracts.SREDU4Backpressure:          "Backpressure and flow control uncertainty",
	contracts.SREDExperimentalGeneral:     "General experimental work not tied to specific uncertainty",
}

// ValidateTag validates that a tag belongs to a known category.
func ValidateTag(category TagCategory, tag string) bool {
	switch category {
	case HumanOrg:
		return contains(HumanOrgTags, tag)
	case Routing:
		return contains(RoutingTags, tag)
	case Policy:
		return contains(PolicyTags, tag)
	case Analytics:
		return contains(AnalyticsTags, tag)
	case SRED:
		return containsSRED(tag)
	default:
		return false
	}
}

// GetCategory returns the category of a tag, or empty if not found.
func GetCategory(tag string) TagCategory {
	if contains(HumanOrgTags, tag) {
		return HumanOrg
	}
	if contains(RoutingTags, tag) {
		return Routing
	}
	if contains(PolicyTags, tag) {
		return Policy
	}
	if contains(AnalyticsTags, tag) {
		return Analytics
	}
	if containsSRED(tag) {
		return SRED
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsSRED(tag string) bool {
	for _, t := range SREDTags {
		if string(t) == tag {
			return true
		}
	}
	return false
}
