// Package v1alpha1 provides conversion between API types and pkg/contracts canonical types.
package v1alpha1

import (
	"github.com/kube-zen/zen-brain1/pkg/contracts"
)

// BrainTaskSpecFromContract converts a canonical BrainTaskSpec to the API type.
// Callers should set SessionID and QueueName on the result when creating a BrainTask.
// CreatedAt/UpdatedAt are not in the CRD and are not set.
func BrainTaskSpecFromContract(in *contracts.BrainTaskSpec) *BrainTaskSpec {
	if in == nil {
		return nil
	}
	out := &BrainTaskSpec{
		ID:                  in.ID,
		WorkItemID:          in.WorkItemID,
		SourceKey:           in.SourceKey,
		Title:               in.Title,
		Description:         in.Description,
		WorkType:             in.WorkType,
		WorkDomain:          in.WorkDomain,
		Priority:            in.Priority,
		Objective:           in.Objective,
		AcceptanceCriteria:  append([]string(nil), in.AcceptanceCriteria...),
		Constraints:         append([]string(nil), in.Constraints...),
		EvidenceRequirement:  in.EvidenceRequirement,
		Hypothesis:          in.Hypothesis,
		TimeoutSeconds:      in.TimeoutSeconds,
		MaxRetries:          in.MaxRetries,
		EstimatedCostUSD:    in.EstimatedCostUSD,
		DependsOn:           append([]string(nil), in.DependsOn...),
		KBScopes:            append([]string(nil), in.KBScopes...),
	}
	if len(in.SREDTags) > 0 {
		out.SREDTags = make([]contracts.SREDTag, len(in.SREDTags))
		copy(out.SREDTags, in.SREDTags)
	}
	return out
}

// ToContract converts the API BrainTaskSpec to the canonical type.
// SessionID and QueueName are CRD-only and are not present on contracts.BrainTaskSpec.
// CreatedAt/UpdatedAt are zero in the result (not stored in CRD).
func (in *BrainTaskSpec) ToContract() *contracts.BrainTaskSpec {
	if in == nil {
		return nil
	}
	out := &contracts.BrainTaskSpec{
		ID:                 in.ID,
		WorkItemID:         in.WorkItemID,
		SourceKey:          in.SourceKey,
		Title:              in.Title,
		Description:        in.Description,
		WorkType:           in.WorkType,
		WorkDomain:         in.WorkDomain,
		Priority:           in.Priority,
		Objective:          in.Objective,
		AcceptanceCriteria: append([]string(nil), in.AcceptanceCriteria...),
		Constraints:        append([]string(nil), in.Constraints...),
		EvidenceRequirement: in.EvidenceRequirement,
		Hypothesis:         in.Hypothesis,
		TimeoutSeconds:     in.TimeoutSeconds,
		MaxRetries:         in.MaxRetries,
		EstimatedCostUSD:   in.EstimatedCostUSD,
		DependsOn:          append([]string(nil), in.DependsOn...),
		KBScopes:           append([]string(nil), in.KBScopes...),
	}
	if len(in.SREDTags) > 0 {
		out.SREDTags = make([]contracts.SREDTag, len(in.SREDTags))
		copy(out.SREDTags, in.SREDTags)
	}
	return out
}
