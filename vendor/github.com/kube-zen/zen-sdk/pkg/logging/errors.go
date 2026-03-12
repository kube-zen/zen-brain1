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
	"errors"
	"fmt"
	"runtime"
	"strings"

	"go.uber.org/zap"
)

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// ErrorCategoryUnknown represents an unknown/unclassified error
	ErrorCategoryUnknown ErrorCategory = "unknown"
	// ErrorCategoryValidation represents a validation error (user input)
	ErrorCategoryValidation ErrorCategory = "validation"
	// ErrorCategoryAuthentication represents an authentication error
	ErrorCategoryAuthentication ErrorCategory = "authentication"
	// ErrorCategoryAuthorization represents an authorization error (permissions)
	ErrorCategoryAuthorization ErrorCategory = "authorization"
	// ErrorCategoryNotFound represents a resource not found error
	ErrorCategoryNotFound ErrorCategory = "not_found"
	// ErrorCategoryConflict represents a conflict error (e.g., duplicate resource)
	ErrorCategoryConflict ErrorCategory = "conflict"
	// ErrorCategoryRateLimit represents a rate limiting error
	ErrorCategoryRateLimit ErrorCategory = "rate_limit"
	// ErrorCategoryTimeout represents a timeout error
	ErrorCategoryTimeout ErrorCategory = "timeout"
	// ErrorCategoryNetwork represents a network error
	ErrorCategoryNetwork ErrorCategory = "network"
	// ErrorCategoryDatabase represents a database error
	ErrorCategoryDatabase ErrorCategory = "database"
	// ErrorCategoryExternal represents an external service error
	ErrorCategoryExternal ErrorCategory = "external"
	// ErrorCategoryInternal represents an internal/system error
	ErrorCategoryInternal ErrorCategory = "internal"
	// ErrorCategoryConfig represents a configuration error
	ErrorCategoryConfig ErrorCategory = "config"
	// ErrorCategoryTemporary represents a temporary/transient error
	ErrorCategoryTemporary ErrorCategory = "temporary"
)

// ErrorContext holds enhanced error context information
type ErrorContext struct {
	Category   ErrorCategory
	Code       string
	Message    string
	Stack      string
	WrappedErr error
	Fields     []zap.Field
}

// CategorizeError attempts to categorize an error based on its message and type
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	errMsg := strings.ToLower(err.Error())

	// Check for common error patterns
	switch {
	case strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "does not exist"):
		return ErrorCategoryNotFound
	case strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate") || strings.Contains(errMsg, "conflict"):
		return ErrorCategoryConflict
	case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "authentication failed") || strings.Contains(errMsg, "invalid token"):
		return ErrorCategoryAuthentication
	case strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "access denied"):
		return ErrorCategoryAuthorization
	case strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "malformed"):
		return ErrorCategoryValidation
	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded"):
		return ErrorCategoryTimeout
	case strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "too many requests"):
		return ErrorCategoryRateLimit
	case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "connection reset") || strings.Contains(errMsg, "network"):
		return ErrorCategoryNetwork
	case strings.Contains(errMsg, "database") || strings.Contains(errMsg, "sql") || strings.Contains(errMsg, "transaction"):
		return ErrorCategoryDatabase
	case strings.Contains(errMsg, "config") || strings.Contains(errMsg, "configuration"):
		return ErrorCategoryConfig
	default:
		return ErrorCategoryInternal
	}
}

// GetStackTrace returns the stack trace for the current goroutine
// Only includes stack frames up to the caller (skips logging package frames)
func GetStackTrace(skip int) string {
	if !isDevelopment() {
		return "" // Stack traces only in development/debug mode
	}

	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	if n == 0 {
		return ""
	}

	lines := strings.Split(string(buf[:n]), "\n")
	// Skip first line (goroutine info) and skip logging package frames
	start := 2 + skip*2 // Each frame is 2 lines
	if start >= len(lines) {
		return ""
	}

	// Include up to 10 frames
	maxFrames := start + 20 // 10 frames * 2 lines each
	if maxFrames > len(lines) {
		maxFrames = len(lines)
	}

	return strings.Join(lines[start:maxFrames], "\n")
}

// ExtractErrorContext extracts enhanced context from an error
func ExtractErrorContext(err error, skipStack int) ErrorContext {
	if err == nil {
		return ErrorContext{
			Category: ErrorCategoryUnknown,
			Message:  "",
		}
	}

	category := CategorizeError(err)
	stack := GetStackTrace(skipStack + 1) // +1 to skip this function

	// Try to unwrap the error to get the underlying error
	wrappedErr := err
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		wrappedErr = unwrapped
	}

	return ErrorContext{
		Category:   category,
		Message:    err.Error(),
		Stack:      stack,
		WrappedErr: wrappedErr,
	}
}

// ErrorCategoryField creates a zap field for error category
func ErrorCategoryField(category ErrorCategory) zap.Field {
	return zap.String("error_category", string(category))
}

// ErrorStackField creates a zap field for error stack trace (only in debug mode)
func ErrorStackField(stack string) zap.Field {
	if stack == "" || !isDevelopment() {
		return zap.Skip()
	}
	return zap.String("error_stack", stack)
}

// ErrorFields extracts all error-related fields for logging
// This is a convenience function that extracts category, code, and optionally stack trace
func ErrorFields(err error, errorCode string) []zap.Field {
	if err == nil {
		return []zap.Field{}
	}

	ctx := ExtractErrorContext(err, 2) // Skip 2 frames (ErrorFields -> caller)
	fields := []zap.Field{
		zap.Error(err),
		ErrorCategoryField(ctx.Category),
	}

	if errorCode != "" {
		fields = append(fields, ErrorCode(errorCode))
	}

	if ctx.Stack != "" {
		fields = append(fields, ErrorStackField(ctx.Stack))
	}

	return fields
}

// EnhancedErrorLogger is a helper for logging errors with enhanced context
type EnhancedErrorLogger struct {
	logger *Logger
}

// NewEnhancedErrorLogger creates a new enhanced error logger
func NewEnhancedErrorLogger(logger *Logger) *EnhancedErrorLogger {
	return &EnhancedErrorLogger{logger: logger}
}

// LogError logs an error with enhanced context
func (e *EnhancedErrorLogger) LogError(err error, msg string, errorCode string, fields ...zap.Field) {
	if err == nil {
		return
	}

	ctx := ExtractErrorContext(err, 3) // Skip frames for LogError -> caller
	//nolint:gocritic // appendAssign: fields slice is intentionally not modified, we create a new slice
	allFields := append(fields,
		zap.Error(err),
		ErrorCategoryField(ctx.Category),
	)

	if errorCode != "" {
		allFields = append(allFields, ErrorCode(errorCode))
	}

	if ctx.Stack != "" {
		allFields = append(allFields, ErrorStackField(ctx.Stack))
	}

	e.logger.Error(err, msg, allFields...)
}

// WrapError wraps an error with additional context
// Returns a new error that includes the original error and context
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// WrapErrorf wraps an error with formatted additional context
func WrapErrorf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// isDevelopment is defined in logging.go

// ErrorWithCode creates an error with an associated error code
type ErrorWithCode struct {
	Code    string
	Message string
	Err     error
}

func (e *ErrorWithCode) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ErrorWithCode) Unwrap() error {
	return e.Err
}

// NewErrorWithCode creates a new error with an associated code
func NewErrorWithCode(code, message string) error {
	return &ErrorWithCode{
		Code:    code,
		Message: message,
	}
}

// NewErrorWithCodeAndCause creates a new error with code and underlying cause
func NewErrorWithCodeAndCause(code, message string, cause error) error {
	return &ErrorWithCode{
		Code:    code,
		Message: message,
		Err:     cause,
	}
}

// ExtractErrorCode extracts the error code from an error if it implements ErrorWithCode
func ExtractErrorCode(err error) string {
	var errWithCode *ErrorWithCode
	if errors.As(err, &errWithCode) {
		return errWithCode.Code
	}
	return ""
}

// IsRetryableError checks if an error is likely retryable based on its category
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	category := CategorizeError(err)
	return category == ErrorCategoryTemporary ||
		category == ErrorCategoryTimeout ||
		category == ErrorCategoryNetwork ||
		category == ErrorCategoryExternal ||
		(category == ErrorCategoryRateLimit) // Rate limits are retryable after backoff
}

// IsClientError checks if an error is a client error (4xx)
func IsClientError(err error) bool {
	if err == nil {
		return false
	}
	category := CategorizeError(err)
	return category == ErrorCategoryValidation ||
		category == ErrorCategoryAuthentication ||
		category == ErrorCategoryAuthorization ||
		category == ErrorCategoryNotFound ||
		category == ErrorCategoryConflict
}

// IsServerError checks if an error is a server error (5xx)
func IsServerError(err error) bool {
	if err == nil {
		return false
	}
	category := CategorizeError(err)
	return category == ErrorCategoryInternal ||
		category == ErrorCategoryDatabase ||
		category == ErrorCategoryExternal ||
		category == ErrorCategoryConfig
}

// WithZapFields adds zap fields to an error context for structured logging
func (ctx *ErrorContext) WithZapFields(fields ...zap.Field) *ErrorContext {
	ctx.Fields = append(ctx.Fields, fields...)
	return ctx
}
