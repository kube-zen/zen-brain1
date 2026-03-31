// Package v1alpha1 defines the v1alpha1 API for zen-brain CRDs.
// DEPRECATED: GroupName "zen.kube-zen.com" is migrating to brain.zen-mesh.io (BrainTask, BrainQueue,
// BrainAgent, BrainPolicy) and platform.zen-mesh.io (ZenProject, ZenCluster).
// See docs/01-ARCHITECTURE/ADR/0010_API_GROUP_MIGRATION.md.
// GroupName must match +groupName in doc.go (zen.kube-zen.com).
package v1alpha1

const (
	// GroupName is the API group for zen-brain CRDs.
	// DEPRECATED: Will change to brain.zen-mesh.io or platform.zen-mesh.io in PATCHSET C.
	GroupName = "zen.kube-zen.com"
	// Version is the API version.
	Version = "v1alpha1"
)
