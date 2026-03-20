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

package retry

import (
	"math/rand"
	"strings"
	"time"
)

// HTTPConfig extends Config with HTTP-specific retry settings
type HTTPConfig struct {
	Config

	// RetryOnStatusCodes is a list of HTTP status codes that should trigger retry
	// Default: 429, 500, 502, 503, 504
	RetryOnStatusCodes []int

	// DontRetryOnStatusCodes is a list of HTTP status codes that should NOT trigger retry
	// Default: 400, 401, 403, 404
	DontRetryOnStatusCodes []int
}

// DefaultHTTPConfig returns a default HTTP retry configuration
func DefaultHTTPConfig() HTTPConfig {
	cfg := DefaultConfig()
	cfg.MaxAttempts = 3
	cfg.InitialDelay = 500 * time.Millisecond
	cfg.MaxDelay = 10 * time.Second

	return HTTPConfig{
		Config:                 cfg,
		RetryOnStatusCodes:     []int{429, 500, 502, 503, 504},
		DontRetryOnStatusCodes: []int{400, 401, 403, 404},
	}
}

// ShouldRetryHTTP determines if an HTTP response should be retried
func ShouldRetryHTTP(statusCode int, cfg HTTPConfig) bool {
	// Check explicit don't retry list first
	for _, code := range cfg.DontRetryOnStatusCodes {
		if statusCode == code {
			return false
		}
	}

	// Check explicit retry list
	for _, code := range cfg.RetryOnStatusCodes {
		if statusCode == code {
			return true
		}
	}

	// Default: retry on 5xx, don't retry on 4xx
	return statusCode >= 500
}

// ShouldRetry determines if an error should be retried for HTTP requests
// This is a helper that checks both network errors and HTTP status codes
func ShouldRetry(err error, attempt, maxAttempts int) bool {
	if attempt >= maxAttempts {
		return false
	}

	if err == nil {
		return false
	}

	// Check for context cancellation (non-retryable)
	errStr := err.Error()
	if strings.Contains(errStr, "context canceled") || strings.Contains(errStr, "context cancelled") {
		return false
	}

	// Network errors, timeouts, and temporary errors are retryable
	if IsTimeoutError(err) || IsTemporaryError(err) {
		return true
	}

	// Default: retry on unknown errors (conservative approach)
	return true
}

// CalculateDelay calculates the delay for the given attempt using exponential backoff
func CalculateDelay(cfg Config, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Exponential backoff: initialDelay * multiplier^(attempt-1)
	multiplier := cfg.Multiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}

	delay := float64(cfg.InitialDelay) * pow(multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add jitter if enabled
	if cfg.Jitter {
		jitterPercent := cfg.JitterPercent
		if jitterPercent <= 0 {
			jitterPercent = 0.1 // Default 10%
		}
		jitterAmount := delay * jitterPercent
		jitter := rand.Float64() * jitterAmount
		delay += jitter
	}

	return time.Duration(delay)
}

// pow calculates x^y (simple implementation for exponential backoff)
func pow(x, y float64) float64 {
	result := 1.0
	for i := 0; i < int(y); i++ {
		result *= x
	}
	// Handle fractional exponents (simplified)
	if y-float64(int(y)) > 0 {
		result *= x
	}
	return result
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
}

// IsTemporaryError checks if an error is temporary and should be retried
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	// Check for common temporary error patterns
	errStr := err.Error()
	return strings.Contains(errStr, "temporary") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "network")
}
