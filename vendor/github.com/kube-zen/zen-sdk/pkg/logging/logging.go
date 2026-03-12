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

// Package logging provides structured logging configuration for Kubernetes controllers.
// It standardizes logging across all Zen tools using zap, providing consistent
// formatting, component context, and development mode detection.
//
// Usage:
//
//	logger := logging.NewLogger("my-controller")
//	logger.Info("Controller started", "namespace", "default")
//	logger.Error(err, "Reconciliation failed", "resource", "my-resource")
//
// The logger automatically includes component name context and adapts to development
// or production environments based on the LOG_DEV environment variable.
package logging

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Logger wraps zap.Logger with component-specific context
type Logger struct {
	*zap.Logger
	componentName string
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Error logs an error message with enhanced context
// If fields don't already include error_code or error_category, they are automatically added
func (l *Logger) Error(err error, msg string, fields ...zap.Field) {
	if err == nil {
		l.Logger.Error(msg, fields...)
		return
	}

	// Check if error_category already present
	hasCategory := false
	for _, f := range fields {
		if f.Key == "error_category" {
			hasCategory = true
			break
		}
	}

	// Add error and enhanced context if not already present
	errorFields := []zap.Field{zap.Error(err)}
	if !hasCategory {
		category := CategorizeError(err)
		errorFields = append(errorFields, ErrorCategoryField(category))
	}
	// Don't auto-add error_code - caller should specify explicitly

	// Add stack trace in debug mode (if not already present)
	if !hasCategory && isDevelopment() {
		stack := GetStackTrace(3) // Skip: GetStackTrace -> Error -> caller
		if stack != "" {
			errorFields = append(errorFields, ErrorStackField(stack))
		}
	}

	l.Logger.Error(msg, append(fields, errorFields...)...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// LoggerConfig holds configuration for logger creation
type LoggerConfig struct {
	// ComponentName is the name of the component
	ComponentName string
	// Development enables development mode (pretty console logs, stack traces)
	Development bool
	// LogLevel sets the minimum log level (debug, info, warn, error)
	// If empty, uses environment variable LOG_LEVEL or defaults to info
	LogLevel string
	// EnableStackTraces enables stack traces for errors (even in production)
	EnableStackTraces bool
}

// NewLogger creates a new structured logger for a component with default configuration
func NewLogger(componentName string) *Logger {
	config := LoggerConfig{
		ComponentName: componentName,
		Development:   isDevelopment(),
	}
	return NewLoggerWithConfig(config)
}

// NewLoggerWithConfig creates a new structured logger with custom configuration
func NewLoggerWithConfig(config LoggerConfig) *Logger {
	if config.ComponentName == "" {
		config.ComponentName = "unknown"
	}

	// Determine development mode
	devMode := config.Development
	if !devMode {
		devMode = isDevelopment()
	}

	// Determine log level
	logLevel := getLogLevel(config.LogLevel)

	// Build zap logger options
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if devMode {
		// Development: pretty console encoder with color
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		// Production: JSON encoder for log aggregation
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Set core with log level
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		logLevel,
	)

	// Add caller info in development, optional in production
	options := []zap.Option{
		zap.AddCaller(),
	}

	if devMode || config.EnableStackTraces {
		// Always include stack traces in development or if explicitly enabled
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	baseLogger := zap.New(core, options...)
	componentLogger := baseLogger.With(zap.String("component", config.ComponentName))

	// Configure controller-runtime logger
	ctrlOpts := ctrlzap.Options{
		Development: devMode,
		EncoderConfigOptions: []ctrlzap.EncoderConfigOption{
			func(cfg *zapcore.EncoderConfig) {
				*cfg = encoderConfig
			},
		},
	}
	ctrlLogger := ctrlzap.New(ctrlzap.UseFlagOptions(&ctrlOpts))
	ctrl.SetLogger(ctrlLogger)

	return &Logger{
		Logger:        componentLogger,
		componentName: config.ComponentName,
	}
}

// getLogLevel parses log level from string or environment variable
func getLogLevel(level string) zapcore.Level {
	if level == "" {
		level = os.Getenv("LOG_LEVEL")
	}
	if level == "" {
		level = "info" // Default to info
	}

	switch level {
	case "debug", "DEBUG":
		return zapcore.DebugLevel
	case "info", "INFO":
		return zapcore.InfoLevel
	case "warn", "WARN", "warning", "WARNING":
		return zapcore.WarnLevel
	case "error", "ERROR":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// WithComponent adds component name to log context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger:        l.Logger.With(zap.String("component", component)),
		componentName: component,
	}
}

// WithField adds a field to the log context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		Logger:        l.Logger.With(zap.Any(key, value)),
		componentName: l.componentName,
	}
}

// WithFields adds multiple fields to the log context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	return &Logger{
		Logger:        l.Logger.With(zapFields...),
		componentName: l.componentName,
	}
}

// WithContext creates a logger with context values automatically extracted
// Extracts standard context values: request_id, tenant_id, user_id, cluster_id, trace_id, span_id
// This follows Kubernetes logging best practices for context propagation
func (l *Logger) WithContext(ctx context.Context) *Logger {
	if ctx == nil {
		return l
	}

	var fields []zap.Field

	// Extract standard context values (generic, useful for any Kubernetes application)
	if requestID := GetRequestID(ctx); requestID != "" {
		fields = append(fields, RequestID(requestID))
	}
	if traceID := GetTraceID(ctx); traceID != "" {
		fields = append(fields, TraceID(traceID))
	}
	if spanID := GetSpanID(ctx); spanID != "" {
		fields = append(fields, SpanID(spanID))
	}
	if tenantID := GetTenantID(ctx); tenantID != "" {
		// Mask UUID by default for security (caller can override if needed)
		fields = append(fields, TenantID(tenantID, true))
	}
	if userID := GetUserID(ctx); userID != "" {
		fields = append(fields, UserID(userID, true))
	}
	if clusterID := GetClusterID(ctx); clusterID != "" {
		fields = append(fields, ClusterID(clusterID))
	}
	if resourceID := GetResourceID(ctx); resourceID != "" {
		fields = append(fields, ResourceID(resourceID))
	}

	if len(fields) == 0 {
		return l
	}

	return &Logger{
		Logger:        l.Logger.With(fields...),
		componentName: l.componentName,
	}
}

// Context-aware logging methods (generic, follows Kubernetes patterns)

// InfoC logs an info message with context (extracts context values automatically)
func (l *Logger) InfoC(ctx context.Context, msg string, fields ...zap.Field) {
	logger := l.WithContext(ctx)
	logger.Info(msg, fields...)
}

// DebugC logs a debug message with context
func (l *Logger) DebugC(ctx context.Context, msg string, fields ...zap.Field) {
	logger := l.WithContext(ctx)
	logger.Debug(msg, fields...)
}

// WarnC logs a warning message with context
func (l *Logger) WarnC(ctx context.Context, msg string, fields ...zap.Field) {
	logger := l.WithContext(ctx)
	logger.Warn(msg, fields...)
}

// ErrorC logs an error message with context
func (l *Logger) ErrorC(ctx context.Context, err error, msg string, fields ...zap.Field) {
	logger := l.WithContext(ctx)
	logger.Error(err, msg, fields...)
}

// isDevelopment checks if we're in development mode
func isDevelopment() bool {
	return os.Getenv("LOG_LEVEL") == "debug" ||
		os.Getenv("DEVELOPMENT") == "true" ||
		os.Getenv("ENV") == "development"
}
