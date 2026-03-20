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

package logging

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	requestIDKey  contextKey = "request_id"
	tenantIDKey   contextKey = "tenant_id"
	clusterIDKey  contextKey = "cluster_id"
	userIDKey     contextKey = "user_id"
	traceIDKey    contextKey = "trace_id"
	spanIDKey     contextKey = "span_id"
	resourceIDKey contextKey = "resource_id"
	adapterIDKey  contextKey = "adapter_id"
	instanceIDKey contextKey = "instance_id"
)

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithTenantID adds tenant ID to context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithClusterID adds cluster ID to context
func WithClusterID(ctx context.Context, clusterID string) context.Context {
	return context.WithValue(ctx, clusterIDKey, clusterID)
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// WithTraceID adds trace ID to context (W3C TraceContext)
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithSpanID adds span ID to context (W3C TraceContext)
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, spanIDKey, spanID)
}

// WithResourceID adds a generic resource ID to context (for multi-tenant systems)
func WithResourceID(ctx context.Context, resourceID string) context.Context {
	return context.WithValue(ctx, resourceIDKey, resourceID)
}

// GetRequestID retrieves request ID from context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetTenantID retrieves tenant ID from context
func GetTenantID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(tenantIDKey).(string); ok {
		return id
	}
	return ""
}

// GetClusterID retrieves cluster ID from context
func GetClusterID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(clusterIDKey).(string); ok {
		return id
	}
	return ""
}

// GetUserID retrieves user ID from context
func GetUserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(userIDKey).(string); ok {
		return id
	}
	return ""
}

// GetTraceID retrieves trace ID from context (tries OpenTelemetry first, then custom key)
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	// Try OpenTelemetry trace context first (if available)
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() && spanCtx.HasTraceID() {
			return spanCtx.TraceID().String()
		}
	}

	// Fallback to custom context key
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

// GetSpanID retrieves span ID from context (tries OpenTelemetry first, then custom key)
func GetSpanID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	// Try OpenTelemetry span context first (if available)
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() && spanCtx.HasSpanID() {
			return spanCtx.SpanID().String()
		}
	}

	// Fallback to custom context key
	if id, ok := ctx.Value(spanIDKey).(string); ok {
		return id
	}
	return ""
}

// GetResourceID retrieves a generic resource ID from context
func GetResourceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(resourceIDKey).(string); ok {
		return id
	}
	return ""
}

// ExtractTraceContext extracts trace context from HTTP request headers
// Supports W3C TraceContext (via OpenTelemetry) and custom headers
func ExtractTraceContext(req *http.Request) context.Context {
	ctx := req.Context()

	// Use OpenTelemetry propagator if available
	prop := otel.GetTextMapPropagator()
	if prop != nil {
		ctx = prop.Extract(ctx, propagation.HeaderCarrier(req.Header))
	}

	// Fallback to custom header extraction
	if GetTraceID(ctx) == "" {
		// Try W3C TraceContext header
		traceparent := req.Header.Get("Traceparent")
		if traceparent != "" && len(traceparent) >= 36 {
			// Parse traceparent: 00-{trace-id}-{span-id}-{flags}
			// Trace ID is 32 hex chars starting at position 3
			if len(traceparent) >= 35 {
				traceID := traceparent[3:35] // Extract trace ID (32 hex chars)
				ctx = WithTraceID(ctx, traceID)
				if len(traceparent) >= 52 {
					spanID := traceparent[36:52] // Extract span ID (16 hex chars)
					ctx = WithSpanID(ctx, spanID)
				}
			}
		}
	}

	// Fallback to X-Trace-ID custom header
	if GetTraceID(ctx) == "" {
		traceID := req.Header.Get("X-Trace-ID")
		if traceID != "" {
			ctx = WithTraceID(ctx, traceID)
		}
	}

	// Extract request ID
	requestID := req.Header.Get("X-Request-ID")
	if requestID != "" {
		ctx = WithRequestID(ctx, requestID)
		// If no trace ID, use request ID as trace ID
		if GetTraceID(ctx) == "" {
			ctx = WithTraceID(ctx, requestID)
		}
	}

	return ctx
}

// PropagateTraceHeaders adds trace headers to HTTP requests for unified distributed tracing
// Supports W3C TraceContext format and custom headers
func PropagateTraceHeaders(ctx context.Context, req *http.Request) {
	if ctx == nil || req == nil {
		return
	}

	// Use OpenTelemetry propagator if available
	prop := otel.GetTextMapPropagator()
	if prop != nil {
		prop.Inject(ctx, propagation.HeaderCarrier(req.Header))
	}

	// Fallback to custom header propagation
	traceID := GetTraceID(ctx)
	if traceID != "" {
		// Set W3C TraceContext format if not already set
		if req.Header.Get("Traceparent") == "" {
			spanID := GetSpanID(ctx)
			if spanID == "" {
				spanID = "0000000000000000"
			}
			// Format: 00-{trace-id}-{span-id}-{trace-flags}
			traceparent := "00-" + traceID + "-" + spanID + "-01"
			req.Header.Set("Traceparent", traceparent)
		}
		// Also set X-Trace-ID for backward compatibility
		if req.Header.Get("X-Trace-ID") == "" {
			req.Header.Set("X-Trace-ID", traceID)
		}
	}

	// Propagate request ID
	requestID := GetRequestID(ctx)
	if requestID != "" && req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", requestID)
	}
}
