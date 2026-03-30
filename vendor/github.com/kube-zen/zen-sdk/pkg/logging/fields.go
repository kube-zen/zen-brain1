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
	"time"

	"go.uber.org/zap"
)

// Field is a zap.Field for backward compatibility and convenience
type Field = zap.Field

// Standard field helpers following Kubernetes logging conventions
// These are generic and useful for any Kubernetes-based application

// RequestID creates a request_id field (standard HTTP correlation)
func RequestID(id string) zap.Field {
	return zap.String("request_id", id)
}

// TraceID creates a trace_id field (W3C TraceContext)
func TraceID(id string) zap.Field {
	return zap.String("trace_id", id)
}

// SpanID creates a span_id field (W3C TraceContext)
func SpanID(id string) zap.Field {
	return zap.String("span_id", id)
}

// TenantID creates a tenant_id field with optional masking
// maskUUID: if true, applies UUID masking for security
func TenantID(id string, maskUUID bool) zap.Field {
	if maskUUID && id != "" {
		id = MaskUUID(id)
	}
	return zap.String("tenant_id", id)
}

// UserID creates a user_id field with optional masking
func UserID(id string, maskUUID bool) zap.Field {
	if maskUUID && id != "" {
		id = MaskUUID(id)
	}
	return zap.String("user_id", id)
}

// ClusterID creates a cluster_id field (generic resource identifier)
func ClusterID(id string) zap.Field {
	return zap.String("cluster_id", id)
}

// ResourceID creates a resource_id field (generic resource identifier)
func ResourceID(id string) zap.Field {
	return zap.String("resource_id", id)
}

// ResourceType creates a resource_type field
func ResourceType(resourceType string) zap.Field {
	return zap.String("resource_type", resourceType)
}

// Operation creates an operation field (standard Kubernetes pattern)
func Operation(op string) zap.Field {
	return zap.String("operation", op)
}

// HTTPMethod creates an http_method field
func HTTPMethod(method string) zap.Field {
	return zap.String("http_method", method)
}

// HTTPPath creates an http_path field
func HTTPPath(path string) zap.Field {
	return zap.String("http_path", path)
}

// HTTPStatus creates an http_status field
func HTTPStatus(status int) zap.Field {
	return zap.Int("http_status", status)
}

// Latency creates a latency_ms field from duration
func Latency(d time.Duration) zap.Field {
	return zap.Int64("latency_ms", d.Milliseconds())
}

// LatencyMs creates a latency_ms field
func LatencyMs(ms int64) zap.Field {
	return zap.Int64("latency_ms", ms)
}

// ErrorCode creates an error_code field
func ErrorCode(code string) zap.Field {
	return zap.String("error_code", code)
}

// RemoteAddr creates a remote_addr field (IP address should be masked if sensitive)
func RemoteAddr(addr string) zap.Field {
	return zap.String("remote_addr", addr)
}

// UserAgent creates a user_agent field
func UserAgent(ua string) zap.Field {
	return zap.String("user_agent", ua)
}

// Component creates a component field (standard Kubernetes pattern)
func Component(name string) zap.Field {
	return zap.String("component", name)
}

// Namespace creates a namespace field (standard Kubernetes pattern)
func Namespace(ns string) zap.Field {
	return zap.String("namespace", ns)
}

// Pod creates a pod field (standard Kubernetes pattern)
func Pod(pod string) zap.Field {
	return zap.String("pod", pod)
}

// Node creates a node field (standard Kubernetes pattern)
func Node(node string) zap.Field {
	return zap.String("node", node)
}

// Kind creates a kind field (Kubernetes resource kind)
func Kind(kind string) zap.Field {
	return zap.String("kind", kind)
}

// Name creates a name field (Kubernetes resource name)
func Name(name string) zap.Field {
	return zap.String("name", name)
}

// RetryCount creates a retry_count field
func RetryCount(count int) zap.Field {
	return zap.Int("retry_count", count)
}

// CacheHit creates a cache_hit field
func CacheHit(hit bool) zap.Field {
	return zap.Bool("cache_hit", hit)
}

// Custom field helpers (generic, useful for any application)

// String creates a custom string field
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int creates a custom int field
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Int64 creates a custom int64 field
func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

// Float64 creates a custom float64 field
func Float64(key string, value float64) zap.Field {
	return zap.Float64(key, value)
}

// Bool creates a custom bool field
func Bool(key string, value bool) zap.Field {
	return zap.Bool(key, value)
}

// Duration creates a custom duration field (stored as milliseconds)
func Duration(key string, value time.Duration) zap.Field {
	return zap.Int64(key, value.Milliseconds())
}

// Strings creates a custom string slice field
func Strings(key string, values []string) zap.Field {
	return zap.Strings(key, values)
}

// DBQuery creates a db_query field with SQL sanitization
func DBQuery(query string) zap.Field {
	return zap.String("db_query", SanitizeSQL(query))
}

// DBDurationMs creates a db_duration_ms field
func DBDurationMs(ms int64) zap.Field {
	return zap.Int64("db_duration_ms", ms)
}

// Error creates an error field
func Error(err error) zap.Field {
	return zap.Error(err)
}
