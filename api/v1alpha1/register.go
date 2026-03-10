package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is the group version for BrainTask.
	GroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
	// SchemeBuilder is used to add types to the scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	// AddToScheme adds BrainTask types to the scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&BrainTask{}, &BrainTaskList{})
	SchemeBuilder.Register(&BrainAgent{}, &BrainAgentList{})
	SchemeBuilder.Register(&BrainQueue{}, &BrainQueueList{})
	SchemeBuilder.Register(&BrainPolicy{}, &BrainPolicyList{})
}
