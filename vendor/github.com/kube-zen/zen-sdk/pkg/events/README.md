# Events Package

**Generic Kubernetes event recorder wrapper for controllers**

## Overview

This package provides a generic wrapper around Kubernetes event recording that can be used across zen-gc, zen-lock, zen-watcher, and other Kubernetes controller components. It enables consistent event recording patterns.

## Features

- ✅ **Generic Interface**: Simple wrapper around controller-runtime's event recorder
- ✅ **Component Naming**: Automatic component name in events
- ✅ **Nil-Safe**: Handles nil clients gracefully
- ✅ **CRD Support**: Works with CRDs (may fail silently on some clusters)

## Quick Start

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/events"
    "k8s.io/client-go/kubernetes"
)

// Create event recorder
recorder := events.NewRecorder(kubeClient, "gc-controller")

// Record an event
recorder.Eventf(
    policy,
    corev1.EventTypeNormal,
    "PolicyEvaluated",
    "Evaluated policy: matched=%d, deleted=%d",
    matched, deleted,
)
```

## Usage Examples

### Basic Event Recording

```go
recorder := events.NewRecorder(kubeClient, "my-controller")

// Record normal event
recorder.Eventf(
    resource,
    corev1.EventTypeNormal,
    "ResourceProcessed",
    "Processed resource %s",
    resourceName,
)

// Record warning event
recorder.Event(
    resource,
    corev1.EventTypeWarning,
    "ProcessingFailed",
    "Failed to process resource",
)
```

### Component-Specific Wrappers

Components can extend the generic recorder for their specific needs:

```go
// In zen-gc
type EventRecorder struct {
    *events.Recorder
}

func NewEventRecorder(client kubernetes.Interface) *EventRecorder {
    return &EventRecorder{
        Recorder: events.NewRecorder(client, "gc-controller"),
    }
}

func (er *EventRecorder) RecordPolicyEvaluated(policy *v1alpha1.GarbageCollectionPolicy, matched, deleted, pending int64) {
    er.Eventf(
        policy,
        corev1.EventTypeNormal,
        "PolicyEvaluated",
        "Evaluated policy: matched=%d, deleted=%d, pending=%d",
        matched, deleted, pending,
    )
}
```

## API Reference

### Functions

- `NewRecorder(client kubernetes.Interface, component string) *Recorder` - Create a new event recorder
- `GetResourceName(obj runtime.Object) string` - Extract resource name from runtime.Object

### Methods

- `Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...interface{})` - Record event with formatting
- `Event(object runtime.Object, eventType, reason, message string)` - Record event

## Notes

- **CRD Events**: Events for CRDs may not be supported by all Kubernetes clusters. The recorder will attempt to record events but failures are silently ignored.
- **Nil Client**: If client is nil, events are not recorded but the recorder won't panic.
- **Component Name**: The component name is automatically included in all events.

## Migration Guide

### From Component-Specific Event Recorders

If you have component-specific event recorders, you can migrate to the generic wrapper:

**Before:**
```go
type EventRecorder struct {
    recorder record.EventRecorder
}

func NewEventRecorder(client kubernetes.Interface) *EventRecorder {
    // Component-specific setup
}
```

**After:**
```go
import "github.com/kube-zen/zen-sdk/pkg/events"

type EventRecorder struct {
    *events.Recorder
}

func NewEventRecorder(client kubernetes.Interface) *EventRecorder {
    return &EventRecorder{
        Recorder: events.NewRecorder(client, "my-controller"),
    }
}
```

## Related

- [Kubernetes Events](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#event-v1-core) - Kubernetes Event API
- [controller-runtime EventRecorder](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/recorder) - Underlying event recorder

