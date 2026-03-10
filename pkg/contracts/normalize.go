// Package contracts defines the canonical data types used across zen-brain.
package contracts

import (
	"sort"
	"strings"
)

// NormalizeWorkItem trims strings, sorts and dedupes slices. Does not invent enum values.
func NormalizeWorkItem(item *WorkItem) {
	if item == nil {
		return
	}
	item.ID = strings.TrimSpace(item.ID)
	item.Title = strings.TrimSpace(item.Title)
	item.Summary = strings.TrimSpace(item.Summary)
	item.Body = strings.TrimSpace(item.Body)
	item.ClusterID = strings.TrimSpace(item.ClusterID)
	item.ProjectID = strings.TrimSpace(item.ProjectID)
	item.WorkingDir = strings.TrimSpace(item.WorkingDir)
	item.ParentID = strings.TrimSpace(item.ParentID)
	item.RequestedBy = strings.TrimSpace(item.RequestedBy)
	item.PolicyClass = strings.TrimSpace(item.PolicyClass)
	normalizeWorkTags(&item.Tags)
	item.DependsOn = sortDedupeStrings(item.DependsOn)
	item.EvidenceRefs = sortDedupeStrings(item.EvidenceRefs)
	item.SourceRefs = sortDedupeStrings(item.SourceRefs)
	item.KBScopes = sortDedupeStrings(item.KBScopes)
	normalizeExecutionConstraints(&item.ExecutionConstraints)
}

// NormalizeBrainTaskSpec trims strings, sorts and dedupes DependsOn, KBScopes. Does not invent enum values.
func NormalizeBrainTaskSpec(spec *BrainTaskSpec) {
	if spec == nil {
		return
	}
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Title = strings.TrimSpace(spec.Title)
	spec.Description = strings.TrimSpace(spec.Description)
	spec.WorkItemID = strings.TrimSpace(spec.WorkItemID)
	spec.SourceKey = strings.TrimSpace(spec.SourceKey)
	spec.Objective = strings.TrimSpace(spec.Objective)
	spec.Hypothesis = strings.TrimSpace(spec.Hypothesis)
	spec.DependsOn = sortDedupeStrings(spec.DependsOn)
	spec.KBScopes = sortDedupeStrings(spec.KBScopes)
	spec.SREDTags = sortDedupeSREDTags(spec.SREDTags)
	for i := range spec.AcceptanceCriteria {
		spec.AcceptanceCriteria[i] = strings.TrimSpace(spec.AcceptanceCriteria[i])
	}
	for i := range spec.Constraints {
		spec.Constraints[i] = strings.TrimSpace(spec.Constraints[i])
	}
}

func normalizeWorkTags(tags *WorkTags) {
	if tags == nil {
		return
	}
	tags.HumanOrg = sortDedupeStrings(tags.HumanOrg)
	tags.Routing = sortDedupeStrings(tags.Routing)
	tags.Policy = sortDedupeStrings(tags.Policy)
	tags.Analytics = sortDedupeStrings(tags.Analytics)
	tags.SRED = sortDedupeSREDTags(tags.SRED)
}

func normalizeExecutionConstraints(c *ExecutionConstraints) {
	if c == nil {
		return
	}
	c.AllowedClusters = sortDedupeStrings(c.AllowedClusters)
}

func sortDedupeStrings(s []string) []string {
	if len(s) == 0 {
		return s
	}
	seen := make(map[string]bool)
	var out []string
	for _, v := range s {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func sortDedupeSREDTags(s []SREDTag) []SREDTag {
	if len(s) == 0 {
		return s
	}
	seen := make(map[SREDTag]bool)
	var out []SREDTag
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}
