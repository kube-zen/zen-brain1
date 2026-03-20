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

// MonitoringLogger provides logging helpers that integrate with monitoring systems
// These helpers ensure logs contain fields that are useful for metrics extraction
type MonitoringLogger struct {
	logger *Logger
}

// NewMonitoringLogger creates a new monitoring logger
func NewMonitoringLogger(logger *Logger) *MonitoringLogger {
	return &MonitoringLogger{logger: logger}
}

// LogMetric logs a metric event that can be extracted for monitoring
// Use this for custom metrics that aren't covered by standard performance logging
func (ml *MonitoringLogger) LogMetric(ctx context.Context, metricName string, value float64, unit string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("metric_name", metricName),
		zap.Float64("metric_value", value),
		zap.String("metric_unit", unit),
		zap.String("event_type", "metric"),
	}
	allFields = append(allFields, fields...)

	ml.logger.WithContext(ctx).Info("Metric event", allFields...)
}

// LogCounter logs a counter increment event
func (ml *MonitoringLogger) LogCounter(ctx context.Context, counterName string, increment int64, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("counter_name", counterName),
		zap.Int64("counter_increment", increment),
		zap.String("event_type", "counter"),
	}
	allFields = append(allFields, fields...)

	ml.logger.WithContext(ctx).Info("Counter event", allFields...)
}

// LogHistogram logs a histogram value for latency/request size distribution
func (ml *MonitoringLogger) LogHistogram(ctx context.Context, histogramName string, value float64, bucket string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("histogram_name", histogramName),
		zap.Float64("histogram_value", value),
		zap.String("histogram_bucket", bucket),
		zap.String("event_type", "histogram"),
	}
	allFields = append(allFields, fields...)

	ml.logger.WithContext(ctx).Info("Histogram event", allFields...)
}

// LogGauge logs a gauge value (current state)
func (ml *MonitoringLogger) LogGauge(ctx context.Context, gaugeName string, value float64, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("gauge_name", gaugeName),
		zap.Float64("gauge_value", value),
		zap.String("event_type", "gauge"),
	}
	allFields = append(allFields, fields...)

	ml.logger.WithContext(ctx).Info("Gauge event", allFields...)
}

// LogHealthCheck logs a health check result
func (ml *MonitoringLogger) LogHealthCheck(ctx context.Context, checkName string, healthy bool, duration time.Duration, err error, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("health_check_name", checkName),
		zap.Bool("health_check_healthy", healthy),
		Latency(duration),
		zap.String("event_type", "health_check"),
	}
	allFields = append(allFields, fields...)

	if err != nil {
		allFields = append(allFields, zap.String("health_check_error", err.Error()))
		ml.logger.WithContext(ctx).Error(err, "Health check failed", allFields...)
	} else {
		ml.logger.WithContext(ctx).Info("Health check completed", allFields...)
	}
}

// LogCriticalEvent logs a critical event that should trigger alerts
func (ml *MonitoringLogger) LogCriticalEvent(ctx context.Context, eventName, severity string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String("critical_event_name", eventName),
		zap.String("critical_event_severity", severity),
		zap.String("event_type", "critical_event"),
		zap.Bool("alert", true), // Flag for alerting systems
	}
	allFields = append(allFields, fields...)

	ml.logger.WithContext(ctx).Error(nil, "Critical event", allFields...)
}

// Monitoring field helpers

// MetricName creates a metric_name field
func MetricName(name string) zap.Field {
	return zap.String("metric_name", name)
}

// MetricValue creates a metric_value field
func MetricValue(value float64) zap.Field {
	return zap.Float64("metric_value", value)
}

// MetricUnit creates a metric_unit field
func MetricUnit(unit string) zap.Field {
	return zap.String("metric_unit", unit)
}

// HealthCheckName creates a health_check_name field
func HealthCheckName(name string) zap.Field {
	return zap.String("health_check_name", name)
}

// HealthCheckHealthy creates a health_check_healthy field
func HealthCheckHealthy(healthy bool) zap.Field {
	return zap.Bool("health_check_healthy", healthy)
}

// EventType creates an event_type field (for log classification)
func EventType(eventType string) zap.Field {
	return zap.String("event_type", eventType)
}

// AlertFlag creates an alert field (marks logs that should trigger alerts)
func AlertFlag(shouldAlert bool) zap.Field {
	return zap.Bool("alert", shouldAlert)
}

// Severity creates a severity field (for critical events)
func Severity(severity string) zap.Field {
	return zap.String("severity", severity)
}
