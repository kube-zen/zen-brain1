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

package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracerProvider *sdktrace.TracerProvider
	initialized    bool
)

// Config holds OpenTelemetry configuration
type Config struct {
	ServiceName    string  // Service name (required)
	ServiceVersion string  // Service version (optional, defaults to "dev")
	Environment    string  // Deployment environment (optional)
	SamplingRate   float64 // Sampling rate (0.0-1.0, default: 0.1 = 10%)
	OTLPEndpoint   string  // OTLP endpoint (optional, uses env var or default)
	ExporterType   string  // Exporter type: "otlp", "otlphttp", "stdout" (optional, uses env var)
	Insecure       bool    // Use insecure connection for OTLP (default: true for development)
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig(serviceName string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: getVersion(),
		Environment:    os.Getenv("DEPLOYMENT_ENV"),
		SamplingRate:   0.1, // 10% sampling by default
		ExporterType:   getExporterType(),
		OTLPEndpoint:   getOTLPEndpoint(),
		Insecure:       true, // Default to insecure for development
	}
}

// Init initializes OpenTelemetry tracing for the application.
// This sets up the global tracer provider and configures trace context propagation.
//
// The exporter type can be specified in config or via OTEL_EXPORTER_TYPE environment variable:
//   - "otlp" or "otlphttp": Uses OTLP HTTP exporter
//   - "stdout" or empty: Tracing disabled (no-op)
//
// Returns a shutdown function that should be called during application shutdown,
// and an error if initialization fails.
//
//nolint:gocritic // hugeParam: config is intentionally passed by value for immutability
func Init(ctx context.Context, config Config) (func(context.Context) error, error) {
	if initialized {
		return func(context.Context) error { return nil }, nil
	}

	if config.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	var exporter sdktrace.SpanExporter
	var err error

	// Determine exporter type
	exporterType := config.ExporterType
	if exporterType == "" {
		exporterType = "otlphttp" // Default to OTLP HTTP
	}

	switch exporterType {
	case "otlp", "otlphttp":
		endpoint := config.OTLPEndpoint
		if endpoint == "" {
			endpoint = getOTLPEndpoint()
		}

		clientOpts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(endpoint),
		}

		if config.Insecure {
			clientOpts = append(clientOpts, otlptracehttp.WithInsecure())
		}

		client := otlptracehttp.NewClient(clientOpts...)
		exporter, err = otlptrace.New(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}

	case "stdout":
		// stdout exporter removed for now - use OTLP or no-op
		// Return no-op shutdown function
		initialized = true
		return func(context.Context) error { return nil }, nil

	default:
		return nil, fmt.Errorf("unsupported exporter type: %s (supported: otlp, otlphttp, stdout)", exporterType)
	}

	// Create resource with service metadata
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(config.ServiceName),
	}

	if config.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersionKey.String(config.ServiceVersion))
	}

	if config.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentKey.String(config.Environment))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	if config.SamplingRate > 0 && config.SamplingRate <= 1.0 {
		sampler = sdktrace.TraceIDRatioBased(config.SamplingRate)
	} else {
		sampler = sdktrace.AlwaysSample() // Default to always sample if invalid rate
	}

	// Create tracer provider
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tracerProvider)

	// Set global propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	initialized = true
	return tracerProvider.Shutdown, nil
}

// InitWithDefaults initializes OpenTelemetry with default configuration.
// Uses environment variables for configuration:
//   - OTEL_SERVICE_NAME (required, or use provided serviceName)
//   - OTEL_EXPORTER_TYPE (optional, defaults to "otlphttp")
//   - OTEL_EXPORTER_OTLP_ENDPOINT (optional, defaults to service-specific endpoint)
//   - DEPLOYMENT_ENV (optional, for environment tag)
func InitWithDefaults(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Allow override via env var
	if envName := os.Getenv("OTEL_SERVICE_NAME"); envName != "" {
		serviceName = envName
	}

	config := DefaultConfig(serviceName)
	return Init(ctx, config)
}

// Shutdown gracefully shuts down the tracer provider.
// This flushes any pending spans and closes connections to the exporter.
// Should be called during application shutdown to ensure all traces are exported.
func Shutdown(ctx context.Context) error {
	if tracerProvider != nil {
		return tracerProvider.Shutdown(ctx)
	}
	return nil
}

// GetTracer returns a tracer for the given name.
// The name is typically the package or component name (e.g., "zen-back/http").
// This tracer can be used to create spans for distributed tracing.
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// HTTPTracingMiddleware creates HTTP middleware that adds tracing to HTTP requests.
// It extracts trace context from headers, creates a span for the request,
// and adds span attributes for HTTP method, route, URL, status code, and duration.
func HTTPTracingMiddleware(tracerName, route string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers (W3C TraceContext)
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start span
			tr := GetTracer(tracerName)
			ctx, span := tr.Start(ctx, route)
			defer span.End()

			// Add span attributes
			span.SetAttributes(
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPRouteKey.String(route),
				semconv.HTTPURLKey.String(r.URL.String()),
			)

			// Record start time
			start := time.Now()

			// Wrap response writer to capture status code
			ww := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Execute handler with trace context
			next.ServeHTTP(ww, r.WithContext(ctx))

			// Record duration and status
			duration := time.Since(start)
			span.SetAttributes(
				attribute.Int64("http.response.duration_ms", duration.Milliseconds()),
				semconv.HTTPStatusCodeKey.Int(ww.statusCode),
			)

			// Mark span as error if status >= 400
			if ww.statusCode >= 400 {
				span.RecordError(fmt.Errorf("HTTP %d", ww.statusCode))
				// Status is automatically set by RecordError, but we can add more context
			}
		})
	}
}

// statusResponseWriter wraps http.ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Helper functions

func getExporterType() string {
	exporterType := os.Getenv("OTEL_EXPORTER_TYPE")
	if exporterType == "" {
		return "otlphttp" // Default to OTLP HTTP
	}
	return exporterType
}

func getOTLPEndpoint() string {
	// Check standard env var first
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	// Check legacy env var
	if endpoint := os.Getenv("OTEL_COLLECTOR_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	// Default to OTEL Collector service in Kubernetes
	// Components can override this by setting OTEL_EXPORTER_OTLP_ENDPOINT environment variable
	// Default assumes standard OTEL Collector deployment
	return "http://otel-collector.observability.svc.cluster.local:4318"
}

func getVersion() string {
	version := os.Getenv("VERSION")
	if version == "" {
		version = os.Getenv("OTEL_SERVICE_VERSION")
	}
	if version == "" {
		version = "dev"
	}
	return version
}
