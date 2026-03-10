// Package contracts defines the canonical data types used across zen-brain.
package contracts

import (
	"fmt"
	"strings"
)

// IsValidWorkType returns true if w is a valid WorkType.
func IsValidWorkType(w WorkType) bool {
	switch w {
	case WorkTypeResearch, WorkTypeDesign, WorkTypeImplementation, WorkTypeDebug,
		WorkTypeRefactor, WorkTypeDocumentation, WorkTypeAnalysis, WorkTypeOperations,
		WorkTypeSecurity, WorkTypeTesting:
		return true
	default:
		return false
	}
}

// IsValidWorkDomain returns true if d is a valid WorkDomain.
func IsValidWorkDomain(d WorkDomain) bool {
	switch d {
	case DomainOffice, DomainFactory, DomainSDK, DomainPolicy, DomainMemory,
		DomainObservability, DomainInfrastructure, DomainIntegration, DomainCore:
		return true
	default:
		return false
	}
}

// IsValidPriority returns true if p is a valid Priority.
func IsValidPriority(p Priority) bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, PriorityBackground:
		return true
	default:
		return false
	}
}

// IsValidExecutionMode returns true if m is a valid ExecutionMode.
func IsValidExecutionMode(m ExecutionMode) bool {
	switch m {
	case ModeAutonomous, ModeApprovalRequired, ModeReadOnly, ModeSimulationOnly, ModeSupervised:
		return true
	default:
		return false
	}
}

// IsValidWorkStatus returns true if s is a valid WorkStatus.
func IsValidWorkStatus(s WorkStatus) bool {
	switch s {
	case StatusRequested, StatusAnalyzing, StatusAnalyzed, StatusPlanning, StatusPlanned,
		StatusPendingApproval, StatusApproved, StatusQueued, StatusRunning, StatusBlocked,
		StatusCompleted, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}

// IsValidEvidenceRequirement returns true if e is a valid EvidenceRequirement.
func IsValidEvidenceRequirement(e EvidenceRequirement) bool {
	switch e {
	case EvidenceNone, EvidenceSummary, EvidenceLogs, EvidenceDiff, EvidenceTestResults, EvidenceFullArtifact:
		return true
	default:
		return false
	}
}

// IsValidApprovalState returns true if a is a valid ApprovalState.
func IsValidApprovalState(a ApprovalState) bool {
	switch a {
	case ApprovalPending, ApprovalApproved, ApprovalRejected, ApprovalNotRequired:
		return true
	default:
		return false
	}
}

// IsValidSREDTag returns true if t is a valid SREDTag.
func IsValidSREDTag(t SREDTag) bool {
	switch t {
	case SREDU1DynamicProvisioning, SREDU2SecurityGates, SREDU3DeterministicDelivery,
		SREDU4Backpressure, SREDExperimentalGeneral:
		return true
	default:
		return false
	}
}

// ParseWorkType parses a string into WorkType. Returns error for unknown values.
func ParseWorkType(s string) (WorkType, error) {
	w := WorkType(strings.TrimSpace(strings.ToLower(s)))
	if IsValidWorkType(w) {
		return w, nil
	}
	return "", &ValidationError{Field: "work_type", Message: fmt.Sprintf("invalid work type %q", s)}
}

// ParseWorkDomain parses a string into WorkDomain. Returns error for unknown values.
func ParseWorkDomain(s string) (WorkDomain, error) {
	d := WorkDomain(strings.TrimSpace(strings.ToLower(s)))
	if IsValidWorkDomain(d) {
		return d, nil
	}
	return "", &ValidationError{Field: "work_domain", Message: fmt.Sprintf("invalid work domain %q", s)}
}

// ParsePriority parses a string into Priority. Returns error for unknown values.
func ParsePriority(s string) (Priority, error) {
	p := Priority(strings.TrimSpace(strings.ToLower(s)))
	if IsValidPriority(p) {
		return p, nil
	}
	return "", &ValidationError{Field: "priority", Message: fmt.Sprintf("invalid priority %q", s)}
}

// ParseExecutionMode parses a string into ExecutionMode. Returns error for unknown values.
func ParseExecutionMode(s string) (ExecutionMode, error) {
	m := ExecutionMode(strings.TrimSpace(strings.ToLower(s)))
	if IsValidExecutionMode(m) {
		return m, nil
	}
	return "", &ValidationError{Field: "execution_mode", Message: fmt.Sprintf("invalid execution mode %q", s)}
}

// ParseEvidenceRequirement parses a string into EvidenceRequirement. Returns error for unknown values.
func ParseEvidenceRequirement(s string) (EvidenceRequirement, error) {
	e := EvidenceRequirement(strings.TrimSpace(strings.ToLower(s)))
	if IsValidEvidenceRequirement(e) {
		return e, nil
	}
	return "", &ValidationError{Field: "evidence_requirement", Message: fmt.Sprintf("invalid evidence requirement %q", s)}
}

// ParseApprovalState parses a string into ApprovalState. Returns error for unknown values.
func ParseApprovalState(s string) (ApprovalState, error) {
	a := ApprovalState(strings.TrimSpace(strings.ToLower(s)))
	if IsValidApprovalState(a) {
		return a, nil
	}
	return "", &ValidationError{Field: "approval_state", Message: fmt.Sprintf("invalid approval state %q", s)}
}

// ValidateWorkTags validates WorkTags: no duplicates within category, valid SRED values.
// For strict category checks (e.g. human_org tags in taxonomy allowlist), use pkg/taxonomy
// from a caller that composes contracts with taxonomy to avoid import cycles.
func ValidateWorkTags(tags WorkTags) error {
	checkStringCategory := func(category string, items []string) error {
		if len(items) == 0 {
			return nil
		}
		seen := make(map[string]bool)
		for _, tag := range items {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if seen[tag] {
				return &ValidationError{Field: "tags." + category, Message: fmt.Sprintf("duplicate tag %q in category %s", tag, category)}
			}
			seen[tag] = true
		}
		return nil
	}
	if err := checkStringCategory("human_org", tags.HumanOrg); err != nil {
		return err
	}
	if err := checkStringCategory("routing", tags.Routing); err != nil {
		return err
	}
	if err := checkStringCategory("policy", tags.Policy); err != nil {
		return err
	}
	if err := checkStringCategory("analytics", tags.Analytics); err != nil {
		return err
	}
	// SRED: must be valid SREDTag enum values; no duplicates
	if len(tags.SRED) > 0 {
		sredSeen := make(map[SREDTag]bool)
		for _, t := range tags.SRED {
			if !IsValidSREDTag(t) {
				return &ValidationError{Field: "tags.sred", Message: fmt.Sprintf("invalid SRED tag %q", t)}
			}
			if sredSeen[t] {
				return &ValidationError{Field: "tags.sred", Message: fmt.Sprintf("duplicate SRED tag %q", t)}
			}
			sredSeen[t] = true
		}
	}
	return nil
}

// ValidateExecutionConstraints validates ExecutionConstraints.
func ValidateExecutionConstraints(c ExecutionConstraints) error {
	if c.MaxCostUSD < 0 {
		return &ValidationError{Field: "execution_constraints.max_cost_usd", Message: "must be >= 0"}
	}
	if c.TimeoutSeconds < 0 {
		return &ValidationError{Field: "execution_constraints.timeout_seconds", Message: "must be >= 0"}
	}
	return nil
}

// ValidateWorkItem validates a WorkItem.
func ValidateWorkItem(item *WorkItem) error {
	if item == nil {
		return &ValidationError{Message: "work item is nil"}
	}
	if strings.TrimSpace(item.ID) == "" {
		return &ValidationError{Field: "id", Message: "required"}
	}
	if strings.TrimSpace(item.Title) == "" {
		return &ValidationError{Field: "title", Message: "required"}
	}
	if item.WorkType != "" && !IsValidWorkType(item.WorkType) {
		return &ValidationError{Field: "work_type", Message: fmt.Sprintf("invalid work type %q", item.WorkType)}
	}
	if item.WorkDomain != "" && !IsValidWorkDomain(item.WorkDomain) {
		return &ValidationError{Field: "work_domain", Message: fmt.Sprintf("invalid work domain %q", item.WorkDomain)}
	}
	if item.Priority != "" && !IsValidPriority(item.Priority) {
		return &ValidationError{Field: "priority", Message: fmt.Sprintf("invalid priority %q", item.Priority)}
	}
	if item.Status != "" && !IsValidWorkStatus(item.Status) {
		return &ValidationError{Field: "status", Message: fmt.Sprintf("invalid status %q", item.Status)}
	}
	if item.EvidenceRequirement != "" && !IsValidEvidenceRequirement(item.EvidenceRequirement) {
		return &ValidationError{Field: "evidence_requirement", Message: fmt.Sprintf("invalid evidence requirement %q", item.EvidenceRequirement)}
	}
	if item.ApprovalState != "" && !IsValidApprovalState(item.ApprovalState) {
		return &ValidationError{Field: "approval_state", Message: fmt.Sprintf("invalid approval state %q", item.ApprovalState)}
	}
	if (item.Source.IssueKey != "" || item.Source.Project != "") && strings.TrimSpace(item.Source.System) == "" {
		return &ValidationError{Field: "source.system", Message: "required when source.issue_key or source.project are set"}
	}
	if err := ValidateExecutionConstraints(item.ExecutionConstraints); err != nil {
		return err
	}
	return ValidateWorkTags(item.Tags)
}

// ValidateBrainTaskSpec validates a BrainTaskSpec.
func ValidateBrainTaskSpec(spec *BrainTaskSpec) error {
	if spec == nil {
		return &ValidationError{Message: "brain task spec is nil"}
	}
	if strings.TrimSpace(spec.ID) == "" {
		return &ValidationError{Field: "id", Message: "required"}
	}
	if strings.TrimSpace(spec.Title) == "" {
		return &ValidationError{Field: "title", Message: "required"}
	}
	if strings.TrimSpace(spec.WorkItemID) == "" {
		return &ValidationError{Field: "work_item_id", Message: "required"}
	}
	if strings.TrimSpace(spec.Objective) == "" {
		return &ValidationError{Field: "objective", Message: "required"}
	}
	if !IsValidWorkType(spec.WorkType) {
		return &ValidationError{Field: "work_type", Message: fmt.Sprintf("invalid work type %q", spec.WorkType)}
	}
	if !IsValidWorkDomain(spec.WorkDomain) {
		return &ValidationError{Field: "work_domain", Message: fmt.Sprintf("invalid work domain %q", spec.WorkDomain)}
	}
	if spec.Priority != "" && !IsValidPriority(spec.Priority) {
		return &ValidationError{Field: "priority", Message: fmt.Sprintf("invalid priority %q", spec.Priority)}
	}
	if spec.EvidenceRequirement != "" && !IsValidEvidenceRequirement(spec.EvidenceRequirement) {
		return &ValidationError{Field: "evidence_requirement", Message: fmt.Sprintf("invalid evidence requirement %q", spec.EvidenceRequirement)}
	}
	if spec.TimeoutSeconds < 0 {
		return &ValidationError{Field: "timeout_seconds", Message: "must be >= 0"}
	}
	if spec.MaxRetries < 0 {
		return &ValidationError{Field: "max_retries", Message: "must be >= 0"}
	}
	if spec.EstimatedCostUSD < 0 {
		return &ValidationError{Field: "estimated_cost_usd", Message: "must be >= 0"}
	}
	for _, id := range spec.DependsOn {
		if strings.TrimSpace(id) == "" {
			return &ValidationError{Field: "depends_on", Message: "empty id in depends_on"}
		}
	}
	for _, scope := range spec.KBScopes {
		if strings.TrimSpace(scope) == "" {
			return &ValidationError{Field: "kb_scopes", Message: "empty id in kb_scopes"}
		}
	}
	for _, t := range spec.SREDTags {
		if !IsValidSREDTag(t) {
			return &ValidationError{Field: "sred_tags", Message: fmt.Sprintf("invalid SRED tag %q", t)}
		}
	}
	return nil
}

// ValidateAnalysisResult validates an AnalysisResult.
func ValidateAnalysisResult(result *AnalysisResult) error {
	if result == nil {
		return &ValidationError{Message: "analysis result is nil"}
	}
	if result.WorkItem != nil {
		if err := ValidateWorkItem(result.WorkItem); err != nil {
			return err
		}
	}
	for i := range result.BrainTaskSpecs {
		if err := ValidateBrainTaskSpec(&result.BrainTaskSpecs[i]); err != nil {
			return fmt.Errorf("brain_task_specs[%d]: %w", i, err)
		}
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		return &ValidationError{Field: "confidence", Message: "must be in [0, 1]"}
	}
	if result.EstimatedTotalCostUSD < 0 {
		return &ValidationError{Field: "estimated_total_cost_usd", Message: "must be >= 0"}
	}
	return nil
}
