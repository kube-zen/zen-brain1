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
	"time"

	"go.uber.org/zap"
)

// PerformanceLogger provides standardized performance logging helpers
type PerformanceLogger struct {
	logger *Logger
}

// NewPerformanceLogger creates a new performance logger
func NewPerformanceLogger(logger *Logger) *PerformanceLogger {
	return &PerformanceLogger{logger: logger}
}

// LogRequestProcessed logs a processed request with standardized fields
func (pl *PerformanceLogger) LogRequestProcessed(ctx context.Context, operation string, duration time.Duration, statusCode int, requestSize, responseSize int64, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation(operation),
		Latency(duration),
		HTTPStatus(statusCode),
		Int64("request_size_bytes", requestSize),
		Int64("response_size_bytes", responseSize),
	}
	allFields = append(allFields, fields...)

	pl.logger.WithContext(ctx).Info("Request processed", allFields...)
}

// LogDBCall logs a database operation with standardized fields
func (pl *PerformanceLogger) LogDBCall(ctx context.Context, operation, query string, duration time.Duration, rowsAffected int64, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation(operation),
		DBQuery(query),
		DBDurationMs(duration.Milliseconds()),
		Int64("rows_affected", rowsAffected),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "Database operation failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("Database operation completed", allFields...)
	}
}

// LogCacheOperation logs a cache operation with standardized fields
func (pl *PerformanceLogger) LogCacheOperation(ctx context.Context, operation, key string, hit bool, duration time.Duration, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation(operation),
		String("cache_key", key),
		CacheHit(hit),
		Latency(duration),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "Cache operation failed", allFields...)
	} else {
		cacheStatus := "miss"
		if hit {
			cacheStatus = "hit"
		}
		pl.logger.WithContext(ctx).Info("Cache operation completed",
			append(allFields, String("cache_status", cacheStatus))...)
	}
}

// LogExternalAPICall logs an external API call with standardized fields
func (pl *PerformanceLogger) LogExternalAPICall(ctx context.Context, service, endpoint, method string, statusCode int, duration time.Duration, requestSize, responseSize int64, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation("external_api_call"),
		String("external_service", service),
		String("external_endpoint", endpoint),
		HTTPMethod(method),
		HTTPStatus(statusCode),
		Latency(duration),
		Int64("request_size_bytes", requestSize),
		Int64("response_size_bytes", responseSize),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "External API call failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("External API call completed", allFields...)
	}
}

// LogMessageQueueOperation logs a message queue operation with standardized fields
func (pl *PerformanceLogger) LogMessageQueueOperation(ctx context.Context, operation, queueName string, messageCount int, duration time.Duration, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation(operation),
		String("queue_name", queueName),
		Int("message_count", messageCount),
		Latency(duration),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "Message queue operation failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("Message queue operation completed", allFields...)
	}
}

// LogFileOperation logs a file operation with standardized fields
func (pl *PerformanceLogger) LogFileOperation(ctx context.Context, operation, filePath string, fileSize int64, duration time.Duration, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		Operation(operation),
		String("file_path", filePath),
		Int64("file_size_bytes", fileSize),
		Latency(duration),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "File operation failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("File operation completed", allFields...)
	}
}

// MeasureDuration is a helper function that measures duration and logs it
func (pl *PerformanceLogger) MeasureDuration(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	allFields := []zap.Field{
		Operation(operation),
		Latency(duration),
	}

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "Operation failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("Operation completed", allFields...)
	}

	return err
}

// MeasureDurationWithFields is like MeasureDuration but accepts additional fields
func (pl *PerformanceLogger) MeasureDurationWithFields(ctx context.Context, operation string, fields []zap.Field, fn func() error) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	allFields := []zap.Field{
		Operation(operation),
		Latency(duration),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		pl.logger.WithContext(ctx).Error(err, "Operation failed", allFields...)
	} else {
		pl.logger.WithContext(ctx).Info("Operation completed", allFields...)
	}

	return err
}

// Performance metrics field helpers (for manual logging)

// RequestSizeBytes creates a request_size_bytes field
func RequestSizeBytes(size int64) zap.Field {
	return Int64("request_size_bytes", size)
}

// ResponseSizeBytes creates a response_size_bytes field
func ResponseSizeBytes(size int64) zap.Field {
	return Int64("response_size_bytes", size)
}

// RowsAffected creates a rows_affected field
func RowsAffected(count int64) zap.Field {
	return Int64("rows_affected", count)
}

// QueueName creates a queue_name field
func QueueName(name string) zap.Field {
	return String("queue_name", name)
}

// MessageCount creates a message_count field
func MessageCount(count int) zap.Field {
	return Int("message_count", count)
}

// ExternalService creates an external_service field
func ExternalService(service string) zap.Field {
	return String("external_service", service)
}

// ExternalEndpoint creates an external_endpoint field
func ExternalEndpoint(endpoint string) zap.Field {
	return String("external_endpoint", endpoint)
}

// FilePath creates a file_path field (be careful with sensitive paths)
func FilePath(path string) zap.Field {
	return String("file_path", path)
}

// FileSizeBytes creates a file_size_bytes field
func FileSizeBytes(size int64) zap.Field {
	return Int64("file_size_bytes", size)
}

// ThroughputBytesPerSecond creates a throughput_bytes_per_second field
func ThroughputBytesPerSecond(throughput float64) zap.Field {
	return Float64("throughput_bytes_per_second", throughput)
}

// ConcurrentOperations creates a concurrent_operations field
func ConcurrentOperations(count int) zap.Field {
	return Int("concurrent_operations", count)
}
