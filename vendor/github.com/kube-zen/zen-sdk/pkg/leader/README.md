# zen-sdk/pkg/leader

Kubernetes leader election package for controller components.

## Overview

This package provides a unified leader election implementation for Kubernetes controllers, ensuring only one instance of a controller is active at a time. It wraps `controller-runtime`'s leader election with additional convenience functions.

## Features

- ✅ Unified leader election interface
- ✅ Integration with `controller-runtime`
- ✅ Configurable lease duration and renew deadline
- ✅ Health check support
- ✅ Context-aware cancellation

## Usage

### Basic Leader Election

```go
import (
    "context"
    "github.com/kube-zen/zen-sdk/pkg/leader"
)

func main() {
    ctx := context.Background()
    
    // Prepare leader election options
    opts, err := leader.PrepareManagerOptions(ctx, componentName, namespace)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create manager with leader election
    mgr, err := ctrl.NewManager(config, opts)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start manager (will elect leader automatically)
    if err := mgr.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### With Custom Configuration

```go
// Configure leader election
leConfig := leader.LeaderElectionConfig{
    Enabled:          true,
    ResourceName:     "zen-lock-leader",
    ResourceNamespace: "zen-system",
    LeaseDuration:    15 * time.Second,
    RenewDeadline:    10 * time.Second,
    RetryPeriod:      2 * time.Second,
}

// Prepare options with custom config
opts, err := leader.PrepareManagerOptionsWithConfig(ctx, componentName, namespace, leConfig)
if err != nil {
    log.Fatal(err)
}

mgr, err := ctrl.NewManager(config, opts)
```

## Integration with zen-sdk/pkg/logging

The leader package integrates with `zen-sdk/pkg/logging` for consistent logging:

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/leader"
    "github.com/kube-zen/zen-sdk/pkg/logging"
)

func main() {
    logger := logging.NewLogger("zen-lock")
    
    // Leader election will use controller-runtime logging
    // which is integrated with zen-sdk/pkg/logging
    opts, err := leader.PrepareManagerOptions(ctx, "zen-lock", "zen-system")
    // ...
}
```

## Environment Variables

Leader election behavior can be configured via environment variables:

- `LEADER_ELECTION` - Enable/disable leader election (default: "true" for multi-replica deployments)
- `LEADER_ELECTION_RESOURCE_NAME` - Custom resource name for lease
- `LEADER_ELECTION_NAMESPACE` - Namespace for lease resource

## Best Practices

1. **Always use leader election** for controllers that run multiple replicas
2. **Set appropriate lease durations** based on your deployment:
   - Short leases (15s) for faster failover
   - Longer leases (60s) for stable deployments
3. **Monitor leader election metrics** to detect issues
4. **Use context cancellation** for graceful shutdown

## Components Using This Package

- `zen-lock` - Lock controller
- `zen-flow` - Flow controller
- `zen-gc` - Garbage collector controller
- `zen-lead` - Leader election manager
- `zen-watcher` - Watcher controller

## License

Apache 2.0

