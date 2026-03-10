# Health Package

**Generic health check interfaces and utilities for Kubernetes controllers**

## Overview

This package provides generic health check interfaces and implementations that can be used across zen-gc, zen-lock, zen-watcher, and other Kubernetes controller components. It enables consistent health check patterns for readiness, liveness, and startup probes.

## Features

- ✅ **Generic Interface**: `Checker` interface for all health checks
- ✅ **Informer Sync Checker**: Verify Kubernetes informer sync status
- ✅ **Activity Checker**: Track component activity over time
- ✅ **Composite Checker**: Combine multiple health checkers
- ✅ **Standard Probes**: Readiness, liveness, and startup checks

## Quick Start

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/health"
    "sigs.k8s.io/controller-runtime/pkg/manager"
)

// Create informer sync checker
informerChecker := health.NewInformerSyncChecker(func() map[string]func() bool {
    return map[string]func() bool{
        "policy-informer": func() bool { return policyInformer.HasSynced() },
        "resource-informer": func() bool { return resourceInformer.HasSynced() },
    }
})

// Register health checks with controller-runtime manager
mgr.AddHealthzCheck("readiness", informerChecker.ReadinessCheck)
mgr.AddReadyzCheck("readiness", informerChecker.ReadinessCheck)
mgr.AddHealthzCheck("startup", informerChecker.StartupCheck)
```

## Usage Examples

### Informer Sync Checker

For controllers that use Kubernetes informers:

```go
// Create checker
checker := health.NewInformerSyncChecker(func() map[string]func() bool {
    informers := make(map[string]func() bool)
    for uid, informer := range reconciler.resourceInformers {
        informers[string(uid)] = func() bool { return informer.HasSynced() }
    }
    return informers
})

// Use with controller-runtime
mgr.AddReadyzCheck("informer-sync", checker.ReadinessCheck)
mgr.AddHealthzCheck("liveness", checker.LivenessCheck)
mgr.AddHealthzCheck("startup", checker.StartupCheck)
```

### Activity Checker

For controllers that need to track activity:

```go
var lastActivity time.Time
var mu sync.RWMutex

updateActivity := func() {
    mu.Lock()
    defer mu.Unlock()
    lastActivity = time.Now()
}

getLastActivity := func() time.Time {
    mu.RLock()
    defer mu.RUnlock()
    return lastActivity
}

checker := health.NewActivityChecker(getLastActivity, 5*time.Minute)

// Update activity when processing
updateActivity()

// Use with controller-runtime
mgr.AddHealthzCheck("activity", checker.LivenessCheck)
```

### Composite Checker

Combine multiple checkers:

```go
informerChecker := health.NewInformerSyncChecker(getInformers)
activityChecker := health.NewActivityChecker(getLastActivity, 5*time.Minute)

composite := health.NewCompositeChecker(informerChecker, activityChecker)

mgr.AddReadyzCheck("composite", composite.ReadinessCheck)
mgr.AddHealthzCheck("composite", composite.LivenessCheck)
```

## API Reference

### Interfaces

#### `Checker`

```go
type Checker interface {
    ReadinessCheck(req *http.Request) error
    LivenessCheck(req *http.Request) error
    StartupCheck(req *http.Request) error
}
```

### Implementations

#### `InformerSyncChecker`

Checks if Kubernetes informers are synced.

```go
func NewInformerSyncChecker(getInformers func() map[string]func() bool) *InformerSyncChecker
```

#### `ActivityChecker`

Tracks component activity and considers unhealthy if no activity within time window.

```go
func NewActivityChecker(getLastActivityTime func() time.Time, maxTimeSinceActivity time.Duration) *ActivityChecker
```

#### `CompositeChecker`

Combines multiple checkers - all must pass.

```go
func NewCompositeChecker(checkers ...Checker) *CompositeChecker
func (c *CompositeChecker) AddChecker(checker Checker)
```

## Migration Guide

### From Component-Specific Health Checks

If you have component-specific health checkers, you can migrate to the generic interfaces:

**Before:**
```go
type HealthChecker struct {
    reconciler *GCPolicyReconciler
}

func (h *HealthChecker) ReadinessCheck(req *http.Request) error {
    // Component-specific logic
}
```

**After:**
```go
import "github.com/kube-zen/zen-sdk/pkg/health"

checker := health.NewInformerSyncChecker(func() map[string]func() bool {
    // Return informers from reconciler
})
```

## Related

- [zen-sdk/pkg/lifecycle](../lifecycle/README.md) - Lifecycle management
- [zen-sdk/pkg/logging](../logging/README.md) - Logging utilities

