// Package labels defines label and annotation key constants for zen-brain CRDs.
//
// Migration note: keys are being migrated from zen.kube-zen.com to brain.zen-mesh.io.
// During the transition period, code reads new keys first and falls back to old keys.
// Write paths use new keys only (unless dual-write is explicitly needed).
//
// See ADR-0010 (docs/01-ARCHITECTURE/ADR/0010_API_GROUP_MIGRATION.md) for full migration plan.
package labels

// ---------------------------------------------------------------------------
// Label keys
// ---------------------------------------------------------------------------

// LabelReportedToJira marks a BrainTask as already reported to Jira.
// READ: checks new key first, falls back to old key.
// WRITE: writes new key only.
const (
	LabelReportedToJira    = "brain.zen-mesh.io/reported-to-jira"
	LabelReportedToJiraOld = "zen.kube-zen.com/reported-to-jira" // DEPRECATED: remove in PATCHSET C cleanup
)

// GetReportedToJira checks both new and legacy label keys.
// Returns true if either key has value "true".
// Precedence: new key wins. If both present with different values, new key takes priority
// and a warning should be logged by the caller if a logger is available.
func GetReportedToJira(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	// New key takes precedence
	if v, ok := labels[LabelReportedToJira]; ok {
		return v == "true"
	}
	// Legacy fallback
	if v, ok := labels[LabelReportedToJiraOld]; ok {
		return v == "true"
	}
	return false
}

// EnsureLabels returns the labels map, initializing if nil.
// Use this before calling SetReportedToJira when the map might be nil.
func EnsureLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return make(map[string]string)
	}
	return labels
}

// SetReportedToJira sets the new label key to "true".
// Does NOT write the old key — migration is additive-only on the read side.
// Note: if labels is nil, this will panic. Call EnsureLabels first.
func SetReportedToJira(labels map[string]string) {
	labels[LabelReportedToJira] = "true"
}

// ---------------------------------------------------------------------------
// Annotation keys
// ---------------------------------------------------------------------------

// AnnotationPlannedModel identifies the LLM model chosen for a task during planning.
// READ: checks new key first, falls back to old key.
// WRITE: writes new key only.
const (
	AnnotationPlannedModel    = "brain.zen-mesh.io/planned-model"
	AnnotationPlannedModelOld = "zen.kube-zen.com/planned-model" // DEPRECATED: remove in PATCHSET C cleanup
)

// GetPlannedModel checks both new and legacy annotation keys.
// Returns the new key value if present, otherwise the old key value.
// Returns empty string if neither is set.
// If both are present with different values, the new key wins and the caller
// should log a warning about the conflict.
func GetPlannedModel(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	if v, ok := annotations[AnnotationPlannedModel]; ok {
		return v
	}
	return annotations[AnnotationPlannedModelOld]
}

// ---------------------------------------------------------------------------
// Factory annotations — DEPRECATED, deferred migration
// ---------------------------------------------------------------------------
// The following factory-* annotation keys remain under zen.kube-zen.com for now.
// They are internal metadata written by the factory worker and read by tests.
// They are NOT used as selectors, policy inputs, or inter-component contracts.
// Migration will happen in a future PATCHSET if they become blockers.
//
// Full list (for reference):
//   zen.kube-zen.com/factory-workspace
//   zen.kube-zen.com/factory-proof
//   zen.kube-zen.com/factory-template
//   zen.kube-zen.com/factory-files-changed
//   zen.kube-zen.com/factory-duration-seconds
//   zen.kube-zen.com/factory-recommendation
//   zen.kube-zen.com/factory-execution-mode
//   zen.kube-zen.com/jira-key
//   zen.kube-zen.com/source
//   zen.kube-zen.com/work-type
//   zen.kube-zen.com/work-domain
