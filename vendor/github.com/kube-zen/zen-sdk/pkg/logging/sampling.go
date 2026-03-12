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
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// SamplerConfig configures log sampling behavior
type SamplerConfig struct {
	// InfoSamplingRate is the rate at which INFO level logs are sampled (0.0-1.0)
	// 0.1 means 10% of INFO logs will be logged
	InfoSamplingRate float64

	// DebugSamplingRate is the rate at which DEBUG level logs are sampled (0.0-1.0)
	DebugSamplingRate float64

	// SuccessSamplingRate is the rate at which successful operation logs are sampled (0.0-1.0)
	// This applies to INFO logs for successful operations (e.g., 200 OK responses)
	SuccessSamplingRate float64

	// ErrorSamplingRate is the rate at which ERROR level logs are sampled (0.0-1.0)
	// Typically 1.0 (100%) - we want to log all errors
	ErrorSamplingRate float64

	// WarnSamplingRate is the rate at which WARN level logs are sampled (0.0-1.0)
	// Typically 1.0 (100%) - we want to log all warnings
	WarnSamplingRate float64
}

// DefaultSamplerConfig returns a config with sensible defaults
func DefaultSamplerConfig() SamplerConfig {
	return SamplerConfig{
		InfoSamplingRate:    0.1,  // 10% of INFO logs
		DebugSamplingRate:   0.01, // 1% of DEBUG logs
		SuccessSamplingRate: 0.01, // 1% of successful operations (2xx responses)
		ErrorSamplingRate:   1.0,  // 100% of errors (log all errors)
		WarnSamplingRate:    1.0,  // 100% of warnings (log all warnings)
	}
}

// Sampler provides log sampling functionality
type Sampler struct {
	config         SamplerConfig
	infoCounter    uint64
	debugCounter   uint64
	successCounter uint64
	mu             sync.Mutex
}

// NewSampler creates a new log sampler with the given configuration
func NewSampler(config SamplerConfig) *Sampler {
	return &Sampler{
		config: config,
	}
}

// ShouldLog determines if a log entry should be logged based on sampling rate
func (s *Sampler) ShouldLog(level zapcore.Level, isSuccess bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch level {
	case zapcore.ErrorLevel:
		// Always log errors (unless explicitly configured otherwise)
		return s.config.ErrorSamplingRate >= 1.0 || s.shouldSample(s.config.ErrorSamplingRate)
	case zapcore.WarnLevel:
		// Always log warnings (unless explicitly configured otherwise)
		return s.config.WarnSamplingRate >= 1.0 || s.shouldSample(s.config.WarnSamplingRate)
	case zapcore.InfoLevel:
		if isSuccess {
			// Use success sampling rate for successful operations
			s.successCounter++
			return s.config.SuccessSamplingRate >= 1.0 || s.shouldSampleWithCounter(s.config.SuccessSamplingRate, &s.successCounter)
		}
		s.infoCounter++
		return s.config.InfoSamplingRate >= 1.0 || s.shouldSampleWithCounter(s.config.InfoSamplingRate, &s.infoCounter)
	case zapcore.DebugLevel:
		s.debugCounter++
		return s.config.DebugSamplingRate >= 1.0 || s.shouldSampleWithCounter(s.config.DebugSamplingRate, &s.debugCounter)
	default:
		// For unknown levels, log everything
		return true
	}
}

// shouldSample determines if a log should be sampled based on rate (simple probability)
func (s *Sampler) shouldSample(rate float64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}
	// Simple modulo-based sampling
	// In a real implementation, you might want to use a more sophisticated approach
	return time.Now().UnixNano()%100 < int64(rate*100)
}

// shouldSampleWithCounter uses a counter-based approach for more consistent sampling
func (s *Sampler) shouldSampleWithCounter(rate float64, counter *uint64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}
	*counter++
	// Use counter for deterministic sampling
	threshold := uint64(1.0 / rate)
	return *counter%threshold == 0
}

// RateLimiter provides rate limiting for log entries to prevent log flooding
type RateLimiter struct {
	// maxLogsPerSecond is the maximum number of logs allowed per second per key
	maxLogsPerSecond int

	// windowSize is the time window for rate limiting
	windowSize time.Duration

	// entries tracks log entries by key with timestamps
	entries map[string][]time.Time
	mu      sync.Mutex

	// cleanupInterval is how often to clean up old entries
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

// RateLimiterConfig configures rate limiting behavior
type RateLimiterConfig struct {
	// MaxLogsPerSecond is the maximum number of logs allowed per second per key
	MaxLogsPerSecond int

	// WindowSize is the time window for rate limiting (default: 1 second)
	WindowSize time.Duration

	// CleanupInterval is how often to clean up old entries (default: 1 minute)
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig returns a config with sensible defaults
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxLogsPerSecond: 10,          // Allow 10 logs per second per key
		WindowSize:       time.Second, // 1 second window
		CleanupInterval:  time.Minute, // Clean up every minute
	}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		maxLogsPerSecond: config.MaxLogsPerSecond,
		windowSize:       config.WindowSize,
		entries:          make(map[string][]time.Time),
		cleanupInterval:  config.CleanupInterval,
		lastCleanup:      time.Now(),
	}
}

// Allow checks if a log entry with the given key should be allowed (not rate limited)
// Returns true if the log should be allowed, false if it should be rate limited
func (rl *RateLimiter) Allow(key string) bool {
	if rl.maxLogsPerSecond <= 0 {
		return true // Rate limiting disabled
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Clean up old entries periodically
	if now.Sub(rl.lastCleanup) > rl.cleanupInterval {
		rl.cleanup(now)
		rl.lastCleanup = now
	}

	// Get or create entry list for this key
	timestamps, exists := rl.entries[key]
	if !exists {
		timestamps = make([]time.Time, 0, rl.maxLogsPerSecond)
	}

	// Remove timestamps outside the window
	windowStart := now.Add(-rl.windowSize)
	validTimestamps := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	timestamps = validTimestamps

	// Check if we've exceeded the rate limit
	if len(timestamps) >= rl.maxLogsPerSecond {
		// Rate limit exceeded
		return false
	}

	// Add current timestamp and update
	timestamps = append(timestamps, now)
	rl.entries[key] = timestamps

	return true
}

// cleanup removes old entries that are outside the window
func (rl *RateLimiter) cleanup(now time.Time) {
	windowStart := now.Add(-rl.windowSize * 2) // Clean up entries older than 2 windows
	for key, timestamps := range rl.entries {
		validTimestamps := timestamps[:0]
		for _, ts := range timestamps {
			if ts.After(windowStart) {
				validTimestamps = append(validTimestamps, ts)
			}
		}
		if len(validTimestamps) == 0 {
			delete(rl.entries, key)
		} else {
			rl.entries[key] = validTimestamps
		}
	}
}

// SampledLogger wraps a Logger with sampling and rate limiting
type SampledLogger struct {
	logger      *Logger
	sampler     *Sampler
	rateLimiter *RateLimiter
}

// NewSampledLogger creates a new sampled logger
func NewSampledLogger(logger *Logger, samplerConfig SamplerConfig, rateLimiterConfig RateLimiterConfig) *SampledLogger {
	return &SampledLogger{
		logger:      logger,
		sampler:     NewSampler(samplerConfig),
		rateLimiter: NewRateLimiter(rateLimiterConfig),
	}
}

// Info logs an info message with sampling and rate limiting
func (sl *SampledLogger) Info(msg string, isSuccess bool, key string, fields ...zap.Field) {
	if !sl.rateLimiter.Allow(key) {
		return // Rate limited
	}
	if !sl.sampler.ShouldLog(zapcore.InfoLevel, isSuccess) {
		return // Sampled out
	}
	sl.logger.Info(msg, fields...)
}

// Debug logs a debug message with sampling and rate limiting
func (sl *SampledLogger) Debug(msg string, key string, fields ...zap.Field) {
	if !sl.rateLimiter.Allow(key) {
		return // Rate limited
	}
	if !sl.sampler.ShouldLog(zapcore.DebugLevel, false) {
		return // Sampled out
	}
	sl.logger.Debug(msg, fields...)
}

// Warn logs a warning message with rate limiting (warnings are not sampled)
func (sl *SampledLogger) Warn(msg string, key string, fields ...zap.Field) {
	if !sl.rateLimiter.Allow(key) {
		return // Rate limited
	}
	// Warnings are not sampled (always logged if not rate limited)
	sl.logger.Warn(msg, fields...)
}

// Error logs an error message with rate limiting (errors are not sampled)
func (sl *SampledLogger) Error(err error, msg string, key string, fields ...zap.Field) {
	if !sl.rateLimiter.Allow(key) {
		return // Rate limited
	}
	// Errors are not sampled (always logged if not rate limited)
	sl.logger.Error(err, msg, fields...)
}
