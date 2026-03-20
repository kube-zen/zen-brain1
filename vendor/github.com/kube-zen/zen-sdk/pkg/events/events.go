/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package events provides a generic Kubernetes event recorder wrapper.
// This package enables consistent event recording patterns across zen-gc, zen-lock, zen-watcher, and other components.
package events

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

// eventSinkWrapper wraps EventInterface to implement record.EventSink.
type eventSinkWrapper struct {
	events v1.EventInterface
}

func (e *eventSinkWrapper) Create(event *corev1.Event) (*corev1.Event, error) {
	return e.events.Create(context.Background(), event, metav1.CreateOptions{})
}

func (e *eventSinkWrapper) Update(event *corev1.Event) (*corev1.Event, error) {
	return e.events.Update(context.Background(), event, metav1.UpdateOptions{})
}

func (e *eventSinkWrapper) Patch(oldEvent *corev1.Event, data []byte) (*corev1.Event, error) {
	return e.events.Patch(context.Background(), oldEvent.Name, types.MergePatchType, data, metav1.PatchOptions{})
}

// Recorder wraps Kubernetes event recorder for controllers.
// This provides a generic interface for recording Kubernetes events.
type Recorder struct {
	recorder record.EventRecorder
}

// NewRecorder creates a new event recorder.
// component is the component name (e.g., "gc-controller", "zen-lock-controller").
func NewRecorder(client kubernetes.Interface, component string) *Recorder {
	// Create event broadcaster
	eventBroadcaster := record.NewBroadcaster()
	// StartStructuredLogging is removed as it requires klog-compatible logger.
	// Event logging is handled via StartRecordingToSink and we use sdklog for application logging.
	if client != nil {
		eventBroadcaster.StartRecordingToSink(&eventSinkWrapper{
			events: client.CoreV1().Events(""),
		})
	}

	// Create event recorder
	eventRecorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: component,
	})

	return &Recorder{
		recorder: eventRecorder,
	}
}

// Eventf records an event with formatting.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (r *Recorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Eventf(object, eventType, reason, messageFmt, args...)
}

// Event records an event.
// Events for CRDs may not be supported by all Kubernetes clusters.
// This function logs errors but does not fail if event recording fails.
func (r *Recorder) Event(object runtime.Object, eventType, reason, message string) {
	if r == nil || r.recorder == nil {
		return
	}
	r.recorder.Event(object, eventType, reason, message)
}

// GetResourceName extracts resource name from runtime.Object.
// This is a utility function for formatting event messages.
func GetResourceName(obj runtime.Object) string {
	if metaObj, ok := obj.(interface{ GetName() string }); ok {
		return metaObj.GetName()
	}
	return "unknown"
}
