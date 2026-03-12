# zen-sdk/pkg/logging

Unified, professional logging package for all `zen` components with context-aware structured logging, OpenTelemetry integration, and production-ready features.

## Features

- ✅ **Context-aware logging** - Automatic extraction of request_id, tenant_id, user_id, trace_id, span_id
- ✅ **Structured logging** - JSON output in production, pretty console in development
- ✅ **OpenTelemetry integration** - Automatic trace/span ID extraction from context
- ✅ **Enhanced error handling** - Automatic categorization, stack traces in debug mode
- ✅ **Security features** - UUID masking, token masking, password redaction, PII protection
- ✅ **Performance logging** - Standardized helpers for requests, DB, cache, external APIs
- ✅ **Audit logging** - Security-compliant audit helpers for compliance
- ✅ **Sampling & rate limiting** - Prevent log flooding in production
- ✅ **Dev vs Production optimizations** - Pretty logs in dev, optimized JSON in production

## Quick Start

```go
import "github.com/kube-zen/zen-sdk/pkg/logging"

// Create a logger
logger := logging.NewLogger("my-component")

// Log with context (extracts request_id, tenant_id, trace_id automatically)
ctx := context.WithValue(context.Background(), "request_id", "req-123")
logger.WithContext(ctx).Info("Processing request",
    logging.Operation("handle_request"),
    logging.String("status", "success"),
)
```

## Basic Usage

### Creating a Logger

```go
// Simple logger with defaults
logger := logging.NewLogger("zen-back")

// Logger with custom configuration
config := logging.LoggerConfig{
    ComponentName:     "zen-back",
    Development:       false,
    LogLevel:          "info",
    EnableStackTraces: false,
}
logger := logging.NewLoggerWithConfig(config)
```

### Logging Levels

```go
logger.WithContext(ctx).Debug("Debug message", logging.Operation("debug_op"))
logger.WithContext(ctx).Info("Info message", logging.Operation("info_op"))
logger.WithContext(ctx).Warn("Warning message", logging.Operation("warn_op"))
logger.WithContext(ctx).Error(err, "Error message", logging.Operation("error_op"))
```

### Context-Aware Logging

The logger automatically extracts context values:

```go
ctx := context.Background()
ctx = logging.WithRequestID(ctx, "req-123")
ctx = logging.WithTenantID(ctx, "tenant-456")
ctx = logging.WithUserID(ctx, "user-789")
ctx = logging.WithClusterID(ctx, "cluster-abc")
ctx = logging.WithTraceID(ctx, "trace-789")
ctx = logging.WithResourceID(ctx, "resource-xyz")

// Kubernetes context helpers
ctx = logging.WithNamespace(ctx, "default")
ctx = logging.WithName(ctx, "my-resource")
ctx = logging.WithKind(ctx, "Pod")

logger.WithContext(ctx).Info("Request processed")
// Logs include: request_id, tenant_id, user_id, cluster_id, trace_id, resource_id automatically
```

Available context helpers:
- `WithRequestID(ctx, id)` - Add request ID
- `WithTenantID(ctx, id)` - Add tenant ID
- `WithUserID(ctx, id)` - Add user ID
- `WithClusterID(ctx, id)` - Add cluster ID
- `WithResourceID(ctx, id)` - Add generic resource ID
- `WithNamespace(ctx, ns)` - Add Kubernetes namespace
- `WithName(ctx, name)` - Add Kubernetes resource name
- `WithKind(ctx, kind)` - Add Kubernetes resource kind

## Error Logging

### Automatic Error Enhancement

Errors are automatically categorized and include stack traces in debug mode:

```go
err := fmt.Errorf("user not found")
logger.WithContext(ctx).Error(err, "Operation failed",
    logging.Operation("fetch_user"),
    logging.ErrorCode("USER_NOT_FOUND"),
)
// Automatically adds: error_category, error_stack (if debug mode)
```

### Error Categories

13 predefined categories:
- `validation`, `authentication`, `authorization`
- `not_found`, `conflict`, `rate_limit`
- `timeout`, `network`, `database`
- `external`, `internal`, `config`, `temporary`

### Error Helpers

```go
// Check if error is retryable
if logging.IsRetryableError(err) {
    // Retry logic
}

// Check error type
if logging.IsClientError(err) {
    // 4xx error handling
}
if logging.IsServerError(err) {
    // 5xx error handling
}

// Create error with code
err := logging.NewErrorWithCode("USER_NOT_FOUND", "User not found")
code := logging.ExtractErrorCode(err)
```

## Performance Logging

### Performance Logger

```go
perfLogger := logging.NewPerformanceLogger(logger)

// HTTP request
perfLogger.LogRequestProcessed(ctx, "user_list",
    duration,
    200,
    requestSize,
    responseSize,
    logging.RequestID(requestID),
)

// Database operation
perfLogger.LogDBCall(ctx, "select_users", query, duration, rowsAffected, err)

// Cache operation
perfLogger.LogCacheOperation(ctx, "get", cacheKey, hit, duration, err)

// External API call
perfLogger.LogExternalAPICall(ctx, "payment-service", "/process", "POST",
    statusCode, duration, requestSize, responseSize, err,
)

// Message queue operation
perfLogger.LogMessageQueueOperation(ctx, "publish", "event-queue",
    messageCount, duration, err,
)

// File operation
perfLogger.LogFileOperation(ctx, "read", "/path/to/file", fileSize, duration, err)

// Measure duration
err := perfLogger.MeasureDuration(ctx, "complex_operation", func() error {
    // Do work
    return nil
})

// Measure duration with custom fields
err := perfLogger.MeasureDurationWithFields(ctx, "operation", []zap.Field{
    logging.String("custom_field", "value"),
}, func() error {
    // Do work
    return nil
})
```

## Audit Logging

### Audit Logger

```go
auditLogger := logging.NewAuditLogger(logger)

// User actions
auditLogger.LogUserAction(ctx, logging.AuditActionCreate, "user", userID,
    logging.AuditResultSuccess,
    logging.UserID(userID, true),
)

// Authentication
auditLogger.LogLogin(ctx, logging.AuditResultSuccess, ipAddress, userAgent)

// Authorization decisions
auditLogger.LogAuthorization(ctx, "resource", resourceID, "read",
    logging.AuditResultDenied,
)

// Configuration changes
auditLogger.LogConfigChange(ctx, "max_connections", oldValue, newValue)

// Data access (PII)
auditLogger.LogDataAccess(ctx, "user_profile", userID)
```

## Sampling and Rate Limiting

For high-volume operations, use sampled logging:

```go
samplerConfig := logging.DefaultSamplerConfig()
rateLimiterConfig := logging.DefaultRateLimiterConfig()

sampledLogger := logging.NewSampledLogger(logger, samplerConfig, rateLimiterConfig)

// Info logs are sampled (10% by default)
sampledLogger.Info("Request processed", true, "http_request",
    logging.HTTPStatus(200),
)

// Errors are always logged (not sampled)
sampledLogger.Error(err, "Request failed", "http_error",
    logging.HTTPStatus(500),
)
```

## Security Features

### Data Masking

```go
// UUID masking
masked := logging.MaskUUID("123e4567-e89b-12d3-a456-426614174000")
// Result: "123e4567-e89b-****-a456-426614174000"

// Token masking
masked := logging.MaskToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")
// Result: "eyJh****...J9"

// Email masking
masked := logging.MaskEmail("user@example.com")
// Result: "u***@example.com"

// IP masking
masked := logging.MaskIP("192.168.1.100")
// Result: "192.168.1.***"
```

### Field Helpers with Masking

```go
// Automatically masks UUIDs
logging.TenantID(tenantID, true)  // maskUUID=true
logging.UserID(userID, true)      // maskUUID=true

// Automatically sanitizes SQL
logging.DBQuery("SELECT * FROM users WHERE password = 'secret'")
// Result: sanitized query with masked sensitive values
```

## Structured Fields

Standard field helpers following Kubernetes logging conventions:

```go
// Request/Correlation
logging.RequestID(id)
logging.TraceID(id)
logging.SpanID(id)

// Resource identification
logging.TenantID(id, maskUUID)
logging.UserID(id, maskUUID)
logging.ClusterID(id)
logging.ResourceID(id)
logging.ResourceType(type)

// HTTP
logging.HTTPMethod(method)
logging.HTTPPath(path)
logging.HTTPStatus(status)

// Performance
logging.Latency(duration)
logging.LatencyMs(ms)

// Kubernetes
logging.Namespace(ns)
logging.Pod(pod)
logging.Node(node)
logging.Kind(kind)
logging.Name(name)

// Operations
logging.Operation(op)
logging.ErrorCode(code)
logging.Component(name)

// Additional helpers
logging.RetryCount(count)
logging.CacheHit(hit)
logging.RemoteAddr(addr)
logging.UserAgent(ua)
logging.DBQuery(query)      // Automatically sanitizes SQL
logging.DBDurationMs(ms)
logging.Strings(key, values)
logging.Duration(key, duration)
```

## Environment Configuration

### Environment Variables

- `LOG_LEVEL` - Log level: debug, info, warn, error (default: info)
- `DEVELOPMENT` - Set to "true" for development mode
- `ENV` - Set to "development" for development mode
- `DEBUG` - Set to "true" for debug mode (enables stack traces)

### Development vs Production

**Development Mode** (pretty console logs):
- Colored output
- Human-readable format
- Stack traces for errors
- Caller information

**Production Mode** (JSON logs):
- Structured JSON output
- Optimized for log aggregation
- Minimal verbosity
- Stack traces only if explicitly enabled

## Best Practices

### 1. Always Use Context

```go
// ✅ Good
logger.WithContext(ctx).Info("Operation completed", logging.Operation("op"))

// ❌ Bad
logger.Info("Operation completed") // Missing context
```

### 2. Use Structured Fields

```go
// ✅ Good
logger.WithContext(ctx).Info("User created",
    logging.UserID(userID, true),
    logging.ResourceType("user"),
    logging.Operation("create_user"),
)

// ❌ Bad
logger.WithContext(ctx).Info(fmt.Sprintf("User %s created", userID))
```

### 3. Error Logging

```go
// ✅ Good
logger.WithContext(ctx).Error(err, "Failed to create user",
    logging.Operation("create_user"),
    logging.ErrorCode("USER_CREATE_FAILED"),
)

// ❌ Bad
logger.WithContext(ctx).Error(err, fmt.Sprintf("Failed: %v", err))
```

### 4. Performance Logging

```go
// ✅ Good - use PerformanceLogger for standard metrics
perfLogger.LogRequestProcessed(ctx, "user_list", duration, statusCode, reqSize, respSize)

// ❌ Bad - manual logging without standardization
logger.Info(fmt.Sprintf("Request took %v", duration))
```

### 5. Security

```go
// ✅ Good - masks sensitive data
logger.WithContext(ctx).Info("User logged in",
    logging.UserID(userID, true),      // Masked
    logging.RemoteAddr(logging.MaskIP(ip)), // Masked
)

// ❌ Bad - logs sensitive data
logger.WithContext(ctx).Info("User logged in",
    logging.String("user_id", userID),  // Not masked!
    logging.String("token", token),     // Exposed!
)
```

### 6. Audit Logging

```go
// ✅ Good - use AuditLogger for security-sensitive operations
auditLogger.LogUserAction(ctx, logging.AuditActionDelete, "user", userID,
    logging.AuditResultSuccess,
)

// ❌ Bad - regular logging for audit events
logger.Info("User deleted") // Missing audit context
```

## Integration with OpenTelemetry

The logger automatically extracts trace and span IDs from OpenTelemetry context:

```go
// After initializing OpenTelemetry
import "github.com/kube-zen/zen-sdk/pkg/observability"

observability.InitWithDefaults(ctx, "my-service")

// Logger automatically includes trace_id and span_id in logs
logger.WithContext(ctx).Info("Processing request")
// Logs include: trace_id, span_id (extracted from OTEL context)
```

## Examples

See:
- `performance_example.go` - Performance logging examples
- `sampling_example.go` - Sampling and rate limiting examples
- `PATTERNS.md` - Component-specific patterns and examples
- `INTEGRATION.md` - Integration with monitoring and log aggregation
- Component codebases for real-world usage

## Migration Guide

### From Old Logging

If migrating from older logging implementations:

1. Replace `logger.Info(ctx, msg, fields...)` with `logger.WithContext(ctx).Info(msg, fields...)`
2. Replace error logging: `logger.Error(ctx, msg, err)` → `logger.WithContext(ctx).Error(err, msg)`
3. Update field helpers to use `logging.*` functions (e.g., `logging.Operation()`)
4. Use `logging.String("key", value)` instead of custom field functions

## License

Apache 2.0

