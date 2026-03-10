# Retry Package

**Package:** `github.com/kube-zen/zen-sdk/pkg/retry`

**Purpose:** Exponential backoff retry logic with context cancellation support

## Overview

The `retry` package provides utilities for executing functions with exponential backoff retry logic, specifically designed for Kubernetes API operations and other transient failures.

## Features

- ✅ **Exponential Backoff** - Configurable backoff with jitter
- ✅ **Context Cancellation** - Respects context cancellation
- ✅ **Kubernetes Error Handling** - Built-in retryable error detection for K8s API errors
- ✅ **Generic Support** - Type-safe generic functions for functions with return values
- ✅ **Configurable** - Customizable retry attempts, delays, and error classification

## Quick Start

```go
import "github.com/kube-zen/zen-sdk/pkg/retry"

// Simple retry
err := retry.Do(ctx, retry.DefaultConfig(), func() error {
    return someOperation()
})

// Retry with result
result, err := retry.DoWithResult(ctx, retry.DefaultConfig(), func() (string, error) {
    return someOperationWithResult()
})
```

## API Reference

### `Do(ctx, config, fn)`

Executes a function with exponential backoff retry logic.

```go
err := retry.Do(ctx, retry.DefaultConfig(), func() error {
    return client.Create(ctx, obj)
})
```

**Parameters:**
- `ctx` - Context for cancellation
- `config` - Retry configuration
- `fn` - Function to execute

**Returns:**
- `error` - Last error if all attempts failed, or nil on success

### `DoWithResult[T](ctx, config, fn)`

Executes a function that returns a result with exponential backoff retry logic.

```go
result, err := retry.DoWithResult(ctx, retry.DefaultConfig(), func() (string, error) {
    return client.Get(ctx, name)
})
```

**Parameters:**
- `ctx` - Context for cancellation
- `config` - Retry configuration
- `fn` - Function to execute (returns `(T, error)`)

**Returns:**
- `T` - Result value
- `error` - Last error if all attempts failed, or nil on success

### `Config`

Retry configuration structure.

```go
type Config struct {
    MaxAttempts    int              // Maximum retry attempts (default: 3)
    InitialDelay   time.Duration    // Initial delay (default: 100ms)
    MaxDelay       time.Duration    // Maximum delay (default: 5s)
    Multiplier     float64          // Exponential multiplier (default: 2.0)
    RetryableErrors func(error) bool // Error classification function
}
```

### `DefaultConfig()`

Returns a default retry configuration with sensible defaults for Kubernetes operations.

**Default Values:**
- MaxAttempts: 3
- InitialDelay: 100ms
- MaxDelay: 5s
- Multiplier: 2.0
- RetryableErrors: Handles K8s API errors (timeout, too many requests, conflict, internal errors)

### `IsRetryableError(err)`

Checks if an error is retryable using the default retryable error function.

```go
if retry.IsRetryableError(err) {
    // Retry the operation
}
```

## Examples

### Example 1: Basic Retry

```go
import (
    "context"
    "github.com/kube-zen/zen-sdk/pkg/retry"
)

func createResource(ctx context.Context, client Client, obj Object) error {
    return retry.Do(ctx, retry.DefaultConfig(), func() error {
        return client.Create(ctx, obj)
    })
}
```

### Example 2: Custom Configuration

```go
config := retry.Config{
    MaxAttempts:  5,
    InitialDelay: 200 * time.Millisecond,
    MaxDelay:     10 * time.Second,
    Multiplier:   1.5,
    RetryableErrors: func(err error) bool {
        // Custom error classification
        return k8serrors.IsServerTimeout(err) || 
               k8serrors.IsTooManyRequests(err)
    },
}

err := retry.Do(ctx, config, func() error {
    return operation()
})
```

### Example 3: Retry with Result

```go
result, err := retry.DoWithResult(ctx, retry.DefaultConfig(), func() (string, error) {
    return client.Get(ctx, name)
})
if err != nil {
    return "", err
}
return result, nil
```

### Example 4: Context Cancellation

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.Do(ctx, retry.DefaultConfig(), func() error {
    return longRunningOperation()
})
// Will respect context timeout and cancel retries
```

## Retryable Errors

By default, the following Kubernetes API errors are considered retryable:

- **Server Timeout** (`IsServerTimeout`)
- **Timeout** (`IsTimeout`)
- **Too Many Requests** (`IsTooManyRequests`)
- **Internal Error** (`IsInternalError`)
- **Conflict** (`IsConflict`) - For optimistic concurrency control

## Migration Guide

### Before (zen-lock)

```go
import "github.com/kube-zen/zen-lock/pkg/common"

err := common.Retry(ctx, common.DefaultRetryConfig(), func() error {
    return operation()
})
```

### After (zen-sdk)

```go
import "github.com/kube-zen/zen-sdk/pkg/retry"

err := retry.Do(ctx, retry.DefaultConfig(), func() error {
    return operation()
})
```

## Benefits

- ✅ **Consistency** - Same retry logic across all OSS components
- ✅ **Type Safety** - Generic support for functions with return values
- ✅ **Context Aware** - Respects context cancellation
- ✅ **Kubernetes Optimized** - Built-in K8s error handling
- ✅ **Less Code** - No need to write retry logic in each component

## See Also

- [Zen SDK README](../../README.md)
- [Migration Guide](../../docs/MIGRATION_GUIDE.md)

