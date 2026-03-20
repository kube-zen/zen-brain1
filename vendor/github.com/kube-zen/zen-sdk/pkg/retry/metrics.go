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
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RetryAttemptsTotal counts total retry attempts
	RetryAttemptsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_retry_attempts_total",
			Help: "Total number of retry attempts",
		},
		[]string{"operation"},
	)

	// RetrySuccessesTotal counts successful retries (operation succeeded after retry)
	RetrySuccessesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_retry_successes_total",
			Help: "Total number of successful retries (operation succeeded after retry)",
		},
		[]string{"operation"},
	)

	// RetryFailuresTotal counts failed retries (operation failed after max attempts)
	RetryFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zen_retry_failures_total",
			Help: "Total number of failed retries (operation failed after max attempts)",
		},
		[]string{"operation"},
	)

	// RetryDelaySeconds tracks retry delay durations
	RetryDelaySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zen_retry_delay_seconds",
			Help:    "Retry delay duration in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0}, // seconds
		},
		[]string{"operation"},
	)
)

// DoWithOperation executes a function with exponential backoff retry logic and metrics tracking
// The operation name is used for Prometheus metrics labeling
// This is a wrapper around Do() that adds metrics tracking
func DoWithOperation(ctx context.Context, config Config, operation string, fn func() error) error {
	if operation == "" {
		operation = "unknown"
	}

	// Set defaults
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = DefaultConfig().RetryableErrors
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			RetryFailuresTotal.WithLabelValues(operation).Inc()
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Track attempt (only count retries, not initial attempt)
		if attempt > 0 {
			RetryAttemptsTotal.WithLabelValues(operation).Inc()
		}

		// Execute the function
		err := fn()
		if err == nil {
			// Success - if this was a retry, record success
			if attempt > 0 {
				RetrySuccessesTotal.WithLabelValues(operation).Inc()
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			RetryFailuresTotal.WithLabelValues(operation).Inc()
			return err
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts-1 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * math.Pow(config.Multiplier, float64(attempt)))
			if backoffDelay > config.MaxDelay {
				backoffDelay = config.MaxDelay
			}

			// Add jitter if enabled
			if config.Jitter {
				jitterPercent := config.JitterPercent
				if jitterPercent <= 0 {
					jitterPercent = 0.1 // Default 10%
				}
				jitterAmount := float64(backoffDelay) * jitterPercent
				jitter := time.Duration(rand.Float64() * jitterAmount)
				backoffDelay += jitter
			}

			// Record delay metric
			RetryDelaySeconds.WithLabelValues(operation).Observe(backoffDelay.Seconds())

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				RetryFailuresTotal.WithLabelValues(operation).Inc()
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	RetryFailuresTotal.WithLabelValues(operation).Inc()
	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// DoWithResultAndOperation executes a function that returns a result with exponential backoff retry logic and metrics tracking
// The operation name is used for Prometheus metrics labeling
// This is a wrapper around DoWithResult() that adds metrics tracking
func DoWithResultAndOperation[T any](ctx context.Context, config Config, operation string, fn func() (T, error)) (T, error) {
	var zero T
	if operation == "" {
		operation = "unknown"
	}

	// Set defaults
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = DefaultConfig().RetryableErrors
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			RetryFailuresTotal.WithLabelValues(operation).Inc()
			return zero, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Track attempt (only count retries, not initial attempt)
		if attempt > 0 {
			RetryAttemptsTotal.WithLabelValues(operation).Inc()
		}

		// Execute the function
		result, err := fn()
		if err == nil {
			// Success - if this was a retry, record success
			if attempt > 0 {
				RetrySuccessesTotal.WithLabelValues(operation).Inc()
			}
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			RetryFailuresTotal.WithLabelValues(operation).Inc()
			return zero, err
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts-1 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * math.Pow(config.Multiplier, float64(attempt)))
			if backoffDelay > config.MaxDelay {
				backoffDelay = config.MaxDelay
			}

			// Add jitter if enabled
			if config.Jitter {
				jitterPercent := config.JitterPercent
				if jitterPercent <= 0 {
					jitterPercent = 0.1 // Default 10%
				}
				jitterAmount := float64(backoffDelay) * jitterPercent
				jitter := time.Duration(rand.Float64() * jitterAmount)
				backoffDelay += jitter
			}

			// Record delay metric
			RetryDelaySeconds.WithLabelValues(operation).Observe(backoffDelay.Seconds())

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				RetryFailuresTotal.WithLabelValues(operation).Inc()
				return zero, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	RetryFailuresTotal.WithLabelValues(operation).Inc()
	return zero, fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}
