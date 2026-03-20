# OpenTelemetry Observability Package

This package provides a unified OpenTelemetry tracing implementation for all `zen` components.

## Features

- ✅ Consolidated OTEL initialization
- ✅ Configurable sampling rates
- ✅ HTTP middleware for automatic request tracing
- ✅ Proper shutdown handling
- ✅ Environment variable configuration
- ✅ Support for OTLP HTTP exporter
- ✅ W3C TraceContext propagation

## Usage

### Basic Initialization

```go
import (
    "context"
    "github.com/kube-zen/zen-sdk/pkg/observability"
)

func main() {
    ctx := context.Background()
    
    // Initialize with defaults (uses environment variables)
    shutdown, err := observability.InitWithDefaults(ctx, "my-service")
    if err != nil {
        log.Fatal(err)
    }
    defer shutdown(ctx)
    
    // ... rest of application
}
```

### Custom Configuration

```go
config := observability.Config{
    ServiceName:      "zen-back",
    ServiceVersion:   "1.0.0",
    Environment:      "production",
    SamplingRate:     0.1, // 10% sampling
    OTLPEndpoint:     "http://otel-collector:4318",
    Insecure:         false, // Use TLS in production
}

shutdown, err := observability.Init(ctx, config)
if err != nil {
    log.Fatal(err)
}
defer shutdown(ctx)
```

### HTTP Middleware

```go
import (
    "net/http"
    "github.com/kube-zen/zen-sdk/pkg/observability"
)

func setupRoutes() {
    mux := http.NewServeMux()
    
    // Add tracing middleware (wraps handler with automatic span creation)
    // Parameters: tracerName, route
    handler := observability.HTTPTracingMiddleware("zen-back/http", "/api/users")(myHandler)
    mux.Handle("/api/users", handler)
    
    // For multiple routes, create middleware for each route
    usersHandler := observability.HTTPTracingMiddleware("zen-back/http", "/api/users")(handleUsers)
    ordersHandler := observability.HTTPTracingMiddleware("zen-back/http", "/api/orders")(handleOrders)
    mux.Handle("/api/users", usersHandler)
    mux.Handle("/api/orders", ordersHandler)
}
```

### Manual Span Creation

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/observability"
)

func myFunction(ctx context.Context) {
    tracer := observability.GetTracer("my-package")
    ctx, span := tracer.Start(ctx, "my-operation")
    defer span.End()
    
    // ... do work
}
```

## Environment Variables

- `OTEL_SERVICE_NAME` - Service name (overrides provided serviceName)
- `OTEL_EXPORTER_TYPE` - Exporter type: "otlp", "otlphttp", "stdout" (default: "otlphttp")
- `OTEL_EXPORTER_OTLP_ENDPOINT` - OTLP endpoint URL (default: service-specific)
- `OTEL_COLLECTOR_ENDPOINT` - Legacy endpoint variable (fallback)
- `DEPLOYMENT_ENV` - Deployment environment (e.g., "production", "staging")
- `VERSION` - Service version (or `OTEL_SERVICE_VERSION`)

## Configuration Best Practices

1. **Sampling Rate**: Use 10% (0.1) for high-volume services, 100% (1.0) for low-volume critical services
2. **Security**: Set `Insecure: false` in production, use TLS
3. **Service Name**: Use consistent naming: `zen-{component}` (e.g., `zen-back`, `zen-bff`)
4. **Shutdown**: Always call the shutdown function during graceful shutdown

## Integration with zen-sdk/pkg/logging

This package works seamlessly with `zen-sdk/pkg/logging`. Trace IDs and Span IDs are automatically extracted from context by the logging package:

```go
import (
    "github.com/kube-zen/zen-sdk/pkg/logging"
    "github.com/kube-zen/zen-sdk/pkg/observability"
)

func handler(ctx context.Context) {
    // Create span
    tracer := observability.GetTracer("my-component")
    ctx, span := tracer.Start(ctx, "my-operation")
    defer span.End()
    
    // Log with automatic trace context extraction
    logger := logging.NewLogger("my-component")
    logger.WithContext(ctx).Info("Operation started")
    // Trace ID and Span ID are automatically added to logs
}
```

