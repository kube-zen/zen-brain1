// Copyright 2025 The Zen Watcher Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package dedup provides deduplication strategies for Kubernetes event processing.
// It implements multiple deduplication algorithms (hash-based, time-window, etc.)
// to prevent duplicate Observations from being created across different event sources.
//
// Usage:
//
//	deduper := dedup.NewHashDeduper(ttl)
//	if !deduper.Seen(key, data) {
//	    // Process new event
//	}
//
// This package is used by zen-watcher to ensure idempotent event processing.
package dedup

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// DedupKey represents the minimal metadata for deduplication
type DedupKey struct {
	Source      string
	Namespace   string
	Kind        string
	Name        string
	Reason      string
	MessageHash string // SHA256 hash of message (first 16 chars for efficiency)
}

// String returns a string representation of the dedup key
func (k DedupKey) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", k.Source, k.Namespace, k.Kind, k.Name, k.Reason, k.MessageHash)
}

// entry represents a cache entry with timestamp
type entry struct {
	key       string
	timestamp time.Time
}

// timeBucket represents a time-based bucket for organizing events
type timeBucket struct {
	startTime    time.Time
	keys         map[string]time.Time // dedup key -> last seen time
	fingerprints map[string]time.Time // fingerprint -> last seen time
}

// fingerprint represents a content-based fingerprint
type fingerprint struct {
	hash      string
	timestamp time.Time
	count     int // number of times seen
}

// rateLimitTracker tracks rate limiting per source using token bucket algorithm
type rateLimitTracker struct {
	mu         sync.Mutex
	tokens     int       // current tokens available
	lastRefill time.Time // last token refill time
	maxTokens  int       // maximum tokens (burst capacity)
	refillRate float64   // tokens per second
}

// aggregatedEvent represents an aggregated event within a rolling window
type aggregatedEvent struct {
	firstSeen   time.Time
	lastSeen    time.Time
	count       int
	fingerprint string
}

// Deduper provides enhanced deduplication with:
// - Time-based buckets for efficient cleanup
// - Content-based fingerprinting
// - Rate limiting per source
// - Event aggregation in rolling window
// - Per-source deduplication windows
type Deduper struct {
	mu sync.RWMutex

	// Original cache for backward compatibility
	cache                map[string]*entry // key -> entry
	lruList              []string          // LRU list (most recent at end)
	maxSize              int               // Maximum cache size (LRU eviction)
	windowSeconds        int               // Default sliding window in seconds (for backward compatibility)
	defaultWindowSeconds int               // Default window for sources not in sourceWindows map
	sourceWindows        map[string]int    // Per-source deduplication windows (source -> seconds)
	ttl                  time.Duration     // TTL for entries (default)

	// Enhanced features
	// Time-based buckets
	buckets           map[int64]*timeBucket // bucket key (unix timestamp) -> bucket
	bucketSizeSeconds int                   // size of each bucket in seconds

	// Fingerprint-based dedup
	fingerprints map[string]*fingerprint // fingerprint hash -> fingerprint metadata

	// Rate limiting per source
	rateLimits       map[string]*rateLimitTracker // source -> rate limit tracker
	maxRatePerSource int                          // maximum events per source per second
	maxRateBurst     int                          // burst capacity

	// Event aggregation
	aggregatedEvents  map[string]*aggregatedEvent // fingerprint -> aggregated event
	enableAggregation bool                        // whether aggregation is enabled

	// Cleanup control
	stopCh      chan struct{}  // Stop channel for cleanup loop
	wg          sync.WaitGroup // Wait group for cleanup goroutine
	stopOnce    sync.Once      // Ensure Stop() is only called once
	lastCleanup time.Time      // Last time cleanup was performed (for optimization)
}

// parseSourceWindows parses per-source windows from environment variable
func parseSourceWindows(defaultWindow int) (map[string]int, int) {
	sourceWindows := make(map[string]int)
	defaultWindowSeconds := defaultWindow

	if sourceWindowsStr := os.Getenv("DEDUP_WINDOW_BY_SOURCE"); sourceWindowsStr != "" {
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(sourceWindowsStr), &config); err == nil {
			for source, windowVal := range config {
				if windowInt, ok := windowVal.(float64); ok {
					windowSecondsInt := int(windowInt)
					if windowSecondsInt > 0 {
						if source == "default" {
							defaultWindowSeconds = windowSecondsInt
						} else {
							sourceWindows[source] = windowSecondsInt
						}
					}
				}
			}
		}
	}

	return sourceWindows, defaultWindowSeconds
}

// parseBucketSize reads bucket size from environment
func parseBucketSize(defaultWindowSeconds int) int {
	bucketSizeSeconds := defaultWindowSeconds / 10
	if bucketSizeSeconds < 10 {
		bucketSizeSeconds = 10
	}
	if bucketStr := os.Getenv("DEDUP_BUCKET_SIZE_SECONDS"); bucketStr != "" {
		if b, err := strconv.Atoi(bucketStr); err == nil && b > 0 {
			bucketSizeSeconds = b
		}
	}
	return bucketSizeSeconds
}

// parseRateLimits reads rate limit configuration from environment
func parseRateLimits() (int, int) {
	maxRatePerSource := 100
	if rateStr := os.Getenv("DEDUP_MAX_RATE_PER_SOURCE"); rateStr != "" {
		if r, err := strconv.Atoi(rateStr); err == nil && r > 0 {
			maxRatePerSource = r
		}
	}

	maxRateBurst := maxRatePerSource * 2
	if burstStr := os.Getenv("DEDUP_RATE_BURST"); burstStr != "" {
		if b, err := strconv.Atoi(burstStr); err == nil && b > 0 {
			maxRateBurst = b
		}
	}

	return maxRatePerSource, maxRateBurst
}

// parseAggregationFlag reads aggregation enable flag from environment
func parseAggregationFlag() bool {
	enableAggregation := true
	if aggStr := os.Getenv("DEDUP_ENABLE_AGGREGATION"); aggStr != "" {
		if aggStr == "false" || aggStr == "0" {
			enableAggregation = false
		}
	}
	return enableAggregation
}

// NewDeduper creates a new deduper with specified configuration and enhanced features
func NewDeduper(windowSeconds, maxSize int) *Deduper {
	if windowSeconds <= 0 {
		windowSeconds = 60 // Default 60 seconds
	}
	if maxSize <= 0 {
		maxSize = 10000 // Default 10k entries
	}

	sourceWindows, defaultWindowSeconds := parseSourceWindows(windowSeconds)
	bucketSizeSeconds := parseBucketSize(defaultWindowSeconds)
	maxRatePerSource, maxRateBurst := parseRateLimits()
	enableAggregation := parseAggregationFlag()

	deduper := &Deduper{
		cache:                make(map[string]*entry),
		lruList:              make([]string, 0, maxSize),
		maxSize:              maxSize,
		windowSeconds:        windowSeconds, // Keep for backward compatibility
		defaultWindowSeconds: defaultWindowSeconds,
		sourceWindows:        sourceWindows,
		ttl:                  time.Duration(defaultWindowSeconds) * time.Second,
		buckets:              make(map[int64]*timeBucket),
		bucketSizeSeconds:    bucketSizeSeconds,
		fingerprints:         make(map[string]*fingerprint),
		rateLimits:           make(map[string]*rateLimitTracker),
		maxRatePerSource:     maxRatePerSource,
		maxRateBurst:         maxRateBurst,
		aggregatedEvents:     make(map[string]*aggregatedEvent),
		enableAggregation:    enableAggregation,
		stopCh:               make(chan struct{}),
		lastCleanup:          time.Now(), // Initialize to current time
	}

	// Start background cleanup goroutine for enhanced features
	deduper.wg.Add(1)
	go deduper.cleanupLoop()

	return deduper
}

// HashMessage creates a short hash of the message for deduplication
func HashMessage(message string) string {
	if message == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(message))
	// Use first 16 characters of hex representation for efficiency
	return fmt.Sprintf("%x", hash[:8])
}

// GenerateFingerprint creates a content-based fingerprint from observation content
// Includes: source, category, severity, eventType, resource, and critical details
func GenerateFingerprint(content map[string]interface{}) string {
	normalized := make(map[string]interface{})

	// Extract key fields for fingerprinting
	if spec, ok := content["spec"].(map[string]interface{}); ok {
		// Include source, category, severity, eventType
		if source, ok := spec["source"].(string); ok {
			normalized["source"] = source
		}
		if category, ok := spec["category"].(string); ok {
			normalized["category"] = category
		}
		if severity, ok := spec["severity"].(string); ok {
			normalized["severity"] = severity
		}
		if eventType, ok := spec["eventType"].(string); ok {
			normalized["eventType"] = eventType
		}
		// Normalize resource for fingerprinting
		if resource, ok := spec["resource"].(map[string]interface{}); ok {
			normResource := make(map[string]interface{})
			if kind, ok := resource["kind"].(string); ok {
				normResource["kind"] = kind
			}
			if name, ok := resource["name"].(string); ok {
				normResource["name"] = name
			}
			if ns, ok := resource["namespace"].(string); ok {
				normResource["namespace"] = ns
			}
			normalized["resource"] = normResource
		}
		// Include critical details for fingerprinting (vulnerability ID, rule, etc.)
		if details, ok := spec["details"].(map[string]interface{}); ok {
			normDetails := make(map[string]interface{})
			// Include fields that uniquely identify the event
			for _, field := range []string{"vulnerabilityID", "rule", "policy", "reason", "auditID", "checkId"} {
				if val, ok := details[field]; ok {
					normDetails[field] = val
				}
			}
			if len(normDetails) > 0 {
				normalized["details"] = normDetails
			}
		}
	}

	// Serialize to JSON for consistent hashing
	jsonBytes, err := json.Marshal(normalized)
	if err != nil {
		// Fallback: hash the string representation
		jsonBytes = []byte(fmt.Sprintf("%v", normalized))
	}

	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for fingerprint
}

// getWindowForSource returns the deduplication window in seconds for a given source
// Returns source-specific window if configured, otherwise default window
// Must be called with lock held
//
//nolint:unused // Reserved for future use when external callers need locked version
func (d *Deduper) getWindowForSource(source string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getWindowForSourceUnlocked(source)
}

// getWindowForSourceUnlocked returns source-specific window without lock (caller must hold lock)
func (d *Deduper) getWindowForSourceUnlocked(source string) int {
	if source == "" {
		return d.defaultWindowSeconds
	}
	if window, exists := d.sourceWindows[source]; exists {
		return window
	}
	return d.defaultWindowSeconds
}

// getBucketKey returns the bucket key for a given time
func (d *Deduper) getBucketKey(t time.Time) int64 {
	return t.Unix() / int64(d.bucketSizeSeconds)
}

// checkRateLimit checks if the source is within rate limits (called with lock held)
func (d *Deduper) checkRateLimit(source string, now time.Time) bool {
	tracker, exists := d.rateLimits[source]
	if !exists {
		tracker = &rateLimitTracker{
			tokens:     d.maxRateBurst,
			lastRefill: now,
			maxTokens:  d.maxRateBurst,
			refillRate: float64(d.maxRatePerSource),
		}
		d.rateLimits[source] = tracker
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Refill tokens based on time elapsed
	elapsed := now.Sub(tracker.lastRefill).Seconds()
	tokensToAdd := int(elapsed * tracker.refillRate)
	if tokensToAdd > 0 {
		tracker.tokens += tokensToAdd
		if tracker.tokens > tracker.maxTokens {
			tracker.tokens = tracker.maxTokens
		}
		tracker.lastRefill = now
	}

	// Check if we have tokens
	if tracker.tokens <= 0 {
		return false
	}

	// Consume one token
	tracker.tokens--
	return true
}

// isDuplicateInBucket checks if this key was seen in the current time bucket (called with lock held)
func (d *Deduper) isDuplicateInBucket(keyStr, fingerprintHash string, now time.Time) bool {
	bucketKey := d.getBucketKey(now)
	bucket, exists := d.buckets[bucketKey]
	if !exists {
		return false
	}

	// Check by key
	if lastSeen, exists := bucket.keys[keyStr]; exists {
		bucketDuration := time.Duration(d.bucketSizeSeconds) * time.Second
		if now.Sub(lastSeen) < bucketDuration {
			return true
		}
	}

	// Check by fingerprint (only if fingerprint is provided)
	if fingerprintHash != "" {
		if lastSeen, exists := bucket.fingerprints[fingerprintHash]; exists {
			bucketDuration := time.Duration(d.bucketSizeSeconds) * time.Second
			if now.Sub(lastSeen) < bucketDuration {
				return true
			}
		}
	}

	return false
}

// addToBucket adds the key to the appropriate time bucket (must be called with lock held)
//
//nolint:unused // Reserved for future use when external callers need locked version
func (d *Deduper) addToBucket(keyStr, fingerprintHash string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addToBucketUnlocked(keyStr, fingerprintHash, now)
}

// addToBucketUnlocked adds the key to the appropriate time bucket (caller must hold lock)
func (d *Deduper) addToBucketUnlocked(keyStr, fingerprintHash string, now time.Time) {
	bucketKey := d.getBucketKey(now)
	bucket, exists := d.buckets[bucketKey]
	if !exists {
		startTime := now.Truncate(time.Duration(d.bucketSizeSeconds) * time.Second)
		bucket = &timeBucket{
			startTime:    startTime,
			keys:         make(map[string]time.Time),
			fingerprints: make(map[string]time.Time),
		}
		d.buckets[bucketKey] = bucket
	}

	bucket.keys[keyStr] = now
	if fingerprintHash != "" {
		bucket.fingerprints[fingerprintHash] = now
	}
}

// isDuplicateFingerprintForSource checks if this fingerprint was seen recently for a specific source (must be called with lock held)
// isDuplicateFingerprintForSource checks if this fingerprint was seen recently (read-only check)
// Returns (isDuplicate, needsCleanup) where needsCleanup indicates if expired fingerprint needs cleanup
func (d *Deduper) isDuplicateFingerprintForSource(fingerprintHash, source string, now time.Time) (bool, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isDuplicateFingerprintForSourceUnlocked(fingerprintHash, source, now)
}

// isDuplicateFingerprintForSourceUnlocked checks if this fingerprint was seen recently (caller must hold read lock)
// Returns (isDuplicate, needsCleanup) where needsCleanup indicates if expired fingerprint needs cleanup
// NOTE: This function does NOT modify data structures - cleanup must be done separately with write lock
func (d *Deduper) isDuplicateFingerprintForSourceUnlocked(fingerprintHash, source string, now time.Time) (bool, bool) {
	fp, exists := d.fingerprints[fingerprintHash]
	if !exists {
		return false, false
	}

	// Get source-specific window (caller must hold lock)
	windowSeconds := d.getWindowForSourceUnlocked(source)

	// Check if fingerprint is still within window
	age := now.Sub(fp.timestamp)
	if age >= time.Duration(windowSeconds)*time.Second {
		// Expired, needs cleanup (but don't do it here - caller must do it with write lock)
		return false, true // Expired, not a duplicate, but needs cleanup
	}

	// Update count and timestamp (read-only access, actual update happens in addFingerprintUnlocked)
	// Note: We can't modify fp here since we only hold read lock
	// The timestamp update will happen later when we add the fingerprint
	return true, false // Duplicate, no cleanup needed
}

// cleanupExpiredFingerprintUnlocked removes an expired fingerprint and its cache entry (caller must hold write lock)
func (d *Deduper) cleanupExpiredFingerprintUnlocked(fingerprintHash string) {
	// Remove from fingerprints
	delete(d.fingerprints, fingerprintHash)
	// Also remove from cache if it exists (cache key is fingerprint for fingerprint-based dedup)
	if _, exists := d.cache[fingerprintHash]; exists {
		delete(d.cache, fingerprintHash)
		d.removeFromLRUUnlocked(fingerprintHash)
	}
}

// addFingerprint adds or updates a fingerprint (must be called with lock held)
//
//nolint:unused // Reserved for future use when external callers need locked version
func (d *Deduper) addFingerprint(fingerprintHash string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addFingerprintUnlocked(fingerprintHash, now)
}

// addFingerprintUnlocked adds or updates a fingerprint (caller must hold lock)
func (d *Deduper) addFingerprintUnlocked(fingerprintHash string, now time.Time) {
	// Check if we need to evict old fingerprints
	if len(d.fingerprints) >= d.maxSize {
		// Evict oldest fingerprint
		var oldestKey string
		var oldestTime time.Time = now
		for k, v := range d.fingerprints {
			if v.timestamp.Before(oldestTime) {
				oldestTime = v.timestamp
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(d.fingerprints, oldestKey)
		}
	}

	d.fingerprints[fingerprintHash] = &fingerprint{
		hash:      fingerprintHash,
		timestamp: now,
		count:     1,
	}
}

// updateAggregation updates the aggregated event counter (must be called with lock held)
// This is a public API wrapper for external use; internal code uses updateAggregationUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) updateAggregation(fingerprintHash string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.updateAggregationUnlocked(fingerprintHash, now)
}

// updateAggregationUnlocked updates the aggregated event counter (caller must hold lock)
func (d *Deduper) updateAggregationUnlocked(fingerprintHash string, now time.Time) {
	if !d.enableAggregation {
		return
	}

	agg, exists := d.aggregatedEvents[fingerprintHash]
	if !exists {
		agg = &aggregatedEvent{
			firstSeen:   now,
			lastSeen:    now,
			count:       0,
			fingerprint: fingerprintHash,
		}
		d.aggregatedEvents[fingerprintHash] = agg
	}

	agg.lastSeen = now
	agg.count++
}

// cleanupOldBuckets removes buckets that are outside the window (must be called with lock held)
// Uses the maximum window across all sources to ensure we don't delete buckets too early
// This is a public API wrapper for external use; internal code uses cleanupOldBucketsUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) cleanupOldBuckets(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanupOldBucketsUnlocked(now)
}

// cleanupOldBucketsUnlocked removes buckets that are outside the window (caller must hold lock)
func (d *Deduper) cleanupOldBucketsUnlocked(now time.Time) {
	// Find the maximum window to ensure we keep buckets for all sources
	maxWindowSeconds := d.defaultWindowSeconds
	for _, window := range d.sourceWindows {
		if window > maxWindowSeconds {
			maxWindowSeconds = window
		}
	}

	cutoffTime := now.Add(-time.Duration(maxWindowSeconds) * time.Second)
	cutoffBucket := d.getBucketKey(cutoffTime)

	for bucketKey := range d.buckets {
		if bucketKey < cutoffBucket {
			delete(d.buckets, bucketKey)
		}
	}

	// Limit number of buckets (max window/bucket size + 2 for safety)
	maxBuckets := (maxWindowSeconds / d.bucketSizeSeconds) + 2
	if len(d.buckets) > maxBuckets {
		// Remove oldest buckets
		keys := make([]int64, 0, len(d.buckets))
		for k := range d.buckets {
			keys = append(keys, k)
		}
		// Sort and remove oldest
		for len(d.buckets) > maxBuckets && len(keys) > 0 {
			// Find oldest
			oldestIdx := 0
			for i, k := range keys {
				if k < keys[oldestIdx] {
					oldestIdx = i
				}
			}
			delete(d.buckets, keys[oldestIdx])
			keys = append(keys[:oldestIdx], keys[oldestIdx+1:]...)
		}
	}
}

// cleanupOldFingerprints removes fingerprints outside the window (must be called with lock held)
// Uses the maximum window across all sources to ensure we don't delete fingerprints too early
// This is a public API wrapper for external use; internal code uses cleanupOldFingerprintsUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) cleanupOldFingerprints(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanupOldFingerprintsUnlocked(now)
}

// cleanupOldFingerprintsUnlocked removes fingerprints outside the window (caller must hold lock)
func (d *Deduper) cleanupOldFingerprintsUnlocked(now time.Time) {
	// Find the maximum window to ensure we keep fingerprints for all sources
	maxWindowSeconds := d.defaultWindowSeconds
	for _, window := range d.sourceWindows {
		if window > maxWindowSeconds {
			maxWindowSeconds = window
		}
	}

	cutoff := now.Add(-time.Duration(maxWindowSeconds) * time.Second)
	for hash, fp := range d.fingerprints {
		if fp.timestamp.Before(cutoff) {
			delete(d.fingerprints, hash)
		}
	}
}

// cleanupOldAggregations removes aggregated events outside the window (must be called with lock held)
// Uses the maximum window across all sources to ensure we don't delete aggregations too early
// This is a public API wrapper for external use; internal code uses cleanupOldAggregationsUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) cleanupOldAggregations(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanupOldAggregationsUnlocked(now)
}

// cleanupOldAggregationsUnlocked removes aggregated events outside the window (caller must hold lock)
func (d *Deduper) cleanupOldAggregationsUnlocked(now time.Time) {
	if !d.enableAggregation {
		return
	}

	// Find the maximum window to ensure we keep aggregations for all sources
	maxWindowSeconds := d.defaultWindowSeconds
	for _, window := range d.sourceWindows {
		if window > maxWindowSeconds {
			maxWindowSeconds = window
		}
	}

	cutoff := now.Add(-time.Duration(maxWindowSeconds) * time.Second)
	for hash, agg := range d.aggregatedEvents {
		if agg.lastSeen.Before(cutoff) {
			delete(d.aggregatedEvents, hash)
		}
	}
}

// cleanupLoop runs periodic cleanup in background
func (d *Deduper) cleanupLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(time.Duration(d.bucketSizeSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.mu.Lock()
			now := time.Now()
			// Use Unlocked versions since we already hold the lock
			d.cleanupOldBucketsUnlocked(now)
			d.cleanupOldFingerprintsUnlocked(now)
			d.cleanupOldAggregationsUnlocked(now)
			d.mu.Unlock()
		}
	}
}

// Stop stops the deduper cleanup goroutine and waits for it to finish
// This method is safe to call multiple times (idempotent)
func (d *Deduper) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
	})
	d.wg.Wait()
}

// ShouldCreate checks if an observation should be created (backward compatible)
// Returns true if this is the first event (should create), false if duplicate within window
//
//nolint:gocritic // hugeParam: key is intentionally passed by value for immutability
func (d *Deduper) ShouldCreate(key DedupKey) bool {
	return d.ShouldCreateWithContent(key, nil)
}

// ShouldCreateWithContent checks if an observation should be created with enhanced features
// If content is provided, uses fingerprint-based dedup and all enhanced features
// Returns true if this is the first event (should create), false if duplicate within window
// Uses source-specific deduplication windows if configured
// Optimized with fine-grained locking for better concurrent performance
//
//nolint:gocritic // hugeParam: key is intentionally passed by value for immutability
func (d *Deduper) ShouldCreateWithContent(key DedupKey, content map[string]interface{}) bool {
	keyStr := key.String()
	now := time.Now()
	source := key.Source

	// Get source-specific window (read-only, use RLock)
	d.mu.RLock()
	windowSeconds := d.getWindowForSourceUnlocked(source)
	d.mu.RUnlock()
	ttl := time.Duration(windowSeconds) * time.Second

	// 1. Rate limiting check (only if we have a source) - needs write lock
	if source != "" {
		d.mu.Lock()
		allowed := d.checkRateLimit(source, now)
		d.mu.Unlock()
		if !allowed {
			return false // Rate limit exceeded
		}
	}

	// 2. Generate fingerprint if content is provided (no lock needed)
	fingerprintHash := ""
	fingerprintExpired := false // Track if fingerprint was expired and removed
	if content != nil {
		fingerprintHash = GenerateFingerprint(content)

		// Check fingerprint-based dedup first (more accurate) with source-specific window
		// This check also removes expired fingerprints and their cache entries
		d.mu.RLock()
		isDup, needsCleanup := d.isDuplicateFingerprintForSource(fingerprintHash, source, now)
		d.mu.RUnlock()

		// Cleanup expired fingerprints if needed (requires write lock)
		if needsCleanup {
			d.mu.Lock()
			d.cleanupExpiredFingerprintUnlocked(fingerprintHash)
			d.mu.Unlock()
		}

		if isDup {
			// Update aggregation (needs write lock)
			d.mu.Lock()
			d.updateAggregationUnlocked(fingerprintHash, now)
			d.mu.Unlock()
			return false // Duplicate fingerprint
		}
		// If fingerprint was expired and removed, skip cache check (cache entry already removed)
		// Check if fingerprint still exists (if not, it was expired and removed)
		d.mu.RLock()
		_, fpExists := d.fingerprints[fingerprintHash]
		d.mu.RUnlock()
		if !fpExists {
			// Fingerprint was expired and removed, cache entry was also removed
			fingerprintExpired = true
		}
	}

	// 3. Check time-based bucket dedup (read-only)
	// Only check bucket if we have a fingerprint and it wasn't expired (for fingerprint-based dedup)
	// For key-only dedup, we rely on the cache check below
	if fingerprintHash != "" && !fingerprintExpired {
		d.mu.RLock()
		isDup := d.isDuplicateInBucket(keyStr, fingerprintHash, now)
		d.mu.RUnlock()
		if isDup {
			// Update aggregation (needs write lock)
			d.mu.Lock()
			d.updateAggregationUnlocked(fingerprintHash, now)
			d.mu.Unlock()
			return false // Duplicate in bucket
		}
	}

	// 4. Check original cache-based dedup (read-only first)
	// For fingerprint-based dedup, check cache by fingerprint; for key-based, check by key
	// Skip cache check if fingerprint was expired (cache entry already removed)
	cacheKey := keyStr
	if fingerprintHash != "" {
		cacheKey = fingerprintHash // Use fingerprint as cache key for fingerprint-based dedup
	}

	// Skip cache check if fingerprint was expired and removed
	if !fingerprintExpired {
		d.mu.RLock()
		ent, exists := d.cache[cacheKey]
		if exists {
			isExpired := now.Sub(ent.timestamp) >= ttl
			d.mu.RUnlock()
			if !isExpired {
				// Update LRU and timestamp (needs write lock)
				d.mu.Lock()
				// Double-check after acquiring write lock
				if ent, stillExists := d.cache[cacheKey]; stillExists && now.Sub(ent.timestamp) < ttl {
					d.updateLRUUnlocked(cacheKey)
					ent.timestamp = now
					d.mu.Unlock()
					return false // Duplicate in original cache
				}
				// Entry expired or removed, continue to add
				// Note: We still hold the write lock here from line above
				if ent, stillExists := d.cache[cacheKey]; stillExists {
					delete(d.cache, cacheKey)
					d.removeFromLRUUnlocked(cacheKey)
					_ = ent // Use ent to avoid unused variable warning
				}
				d.mu.Unlock()
			} else {
				// Entry expired, remove it (needs write lock)
				d.mu.Lock()
				if stillExists := d.cache[cacheKey]; stillExists != nil {
					delete(d.cache, cacheKey)
					d.removeFromLRUUnlocked(cacheKey)
				}
				d.mu.Unlock()
			}
		} else {
			d.mu.RUnlock()
		}
	}

	// 5. Cleanup old buckets and fingerprints (periodic, needs write lock)
	// Only cleanup occasionally to avoid lock contention (every 100th call or when >1s since last cleanup)
	// Full cleanup of all sources happens in the background cleanup loop
	shouldCleanup := false
	d.mu.RLock()
	lastCleanup := d.lastCleanup
	d.mu.RUnlock()
	if now.Sub(lastCleanup) > time.Second {
		shouldCleanup = true
	}

	// 6. Add to all structures (write operations) - acquire lock once for all operations
	d.mu.Lock()

	// Perform cleanup if needed (while we already hold the lock)
	if shouldCleanup {
		// Only cleanup expired entries for the current source to avoid removing unrelated entries
		d.cleanupExpiredForSourceUnlocked(source, now)
		d.lastCleanup = now
	}

	// Add to all structures
	d.addToBucketUnlocked(keyStr, fingerprintHash, now)
	if fingerprintHash != "" {
		d.addFingerprintUnlocked(fingerprintHash, now)
		d.updateAggregationUnlocked(fingerprintHash, now)
	}
	// Use fingerprint as cache key for fingerprint-based dedup, key for key-based dedup
	// cacheKey is already declared above, just reuse it
	if fingerprintHash != "" {
		cacheKey = fingerprintHash
	} else {
		cacheKey = keyStr
	}
	d.addToCacheUnlocked(cacheKey, now)
	d.mu.Unlock()

	return true // First event, should create
}

// cleanupExpiredForSource removes expired entries for a specific source (must be called with lock held)
// Uses source-specific window if configured
//
//nolint:unused // Reserved for future use when external callers need locked version
func (d *Deduper) cleanupExpiredForSource(source string, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanupExpiredForSourceUnlocked(source, now)
}

// cleanupExpiredForSourceUnlocked removes expired entries for a specific source (caller must hold lock)
func (d *Deduper) cleanupExpiredForSourceUnlocked(source string, now time.Time) {
	expired := make([]string, 0)

	// Parse source from cache keys to check expiration with source-specific window
	// Cache keys format: "source/namespace/kind/name/reason/messageHash"
	for keyStr, ent := range d.cache {
		// Extract source from key (first part before /)
		keySource := ""
		for idx := 0; idx < len(keyStr); idx++ {
			if keyStr[idx] == '/' {
				keySource = keyStr[:idx]
				break
			}
		}

		// Use source-specific window if this entry matches the source
		entryWindowSeconds := d.getWindowForSourceUnlocked(keySource)
		entryTTL := time.Duration(entryWindowSeconds) * time.Second

		if now.Sub(ent.timestamp) >= entryTTL {
			expired = append(expired, keyStr)
		}
	}

	for _, keyStr := range expired {
		delete(d.cache, keyStr)
		d.removeFromLRUUnlocked(keyStr)
	}
}

// addToCache adds a new entry to cache with LRU eviction if needed (must be called with lock held)
//
//nolint:unused // Reserved for future use when external callers need locked version
func (d *Deduper) addToCache(keyStr string, timestamp time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addToCacheUnlocked(keyStr, timestamp)
}

// addToCacheUnlocked adds a new entry to cache with LRU eviction if needed (caller must hold lock)
func (d *Deduper) addToCacheUnlocked(keyStr string, timestamp time.Time) {
	// If at capacity, evict LRU (oldest)
	if len(d.cache) >= d.maxSize && len(d.lruList) > 0 {
		// Remove oldest (first in list)
		oldest := d.lruList[0]
		delete(d.cache, oldest)
		d.lruList = d.lruList[1:]
	}

	// Add new entry
	d.cache[keyStr] = &entry{
		key:       keyStr,
		timestamp: timestamp,
	}
	d.lruList = append(d.lruList, keyStr)
}

// updateLRU moves key to end of LRU list (most recent) (must be called with lock held)
// This is a public API wrapper for external use; internal code uses updateLRUUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) updateLRU(keyStr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.updateLRUUnlocked(keyStr)
}

// updateLRUUnlocked moves key to end of LRU list (caller must hold lock)
func (d *Deduper) updateLRUUnlocked(keyStr string) {
	// Find and remove from current position
	for i, k := range d.lruList {
		if k == keyStr {
			// Remove from current position
			d.lruList = append(d.lruList[:i], d.lruList[i+1:]...)
			break
		}
	}
	// Add to end (most recent)
	d.lruList = append(d.lruList, keyStr)
}

// removeFromLRU removes key from LRU list (must be called with lock held)
// This is a public API wrapper for external use; internal code uses removeFromLRUUnlocked directly.
//
//nolint:unused // Public API for external use
func (d *Deduper) removeFromLRU(keyStr string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.removeFromLRUUnlocked(keyStr)
}

// removeFromLRUUnlocked removes key from LRU list (caller must hold lock)
func (d *Deduper) removeFromLRUUnlocked(keyStr string) {
	for i, k := range d.lruList {
		if k == keyStr {
			d.lruList = append(d.lruList[:i], d.lruList[i+1:]...)
			return
		}
	}
}

// Stats returns current cache statistics
func (d *Deduper) Stats() (size int, maxSize int, windowSeconds int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.cache), d.maxSize, d.defaultWindowSeconds
}

// GetSourceWindows returns the per-source window configuration
func (d *Deduper) GetSourceWindows() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range d.sourceWindows {
		result[k] = v
	}
	return result
}

// GetDefaultWindow returns the default deduplication window
func (d *Deduper) GetDefaultWindow() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.defaultWindowSeconds
}

// UpdateSourceWindows updates the per-source window configuration dynamically
// This is thread-safe and can be called at runtime to update configuration
func (d *Deduper) UpdateSourceWindows(sourceWindows map[string]int, defaultWindow int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update default window if provided
	if defaultWindow > 0 {
		d.defaultWindowSeconds = defaultWindow
		// Update TTL based on new default (used for backward compatibility)
		d.ttl = time.Duration(d.defaultWindowSeconds) * time.Second
	}

	// Update source windows
	d.sourceWindows = make(map[string]int)
	for source, window := range sourceWindows {
		if window > 0 {
			d.sourceWindows[source] = window
		}
	}

	// Update windowSeconds for backward compatibility (use default or max)
	d.windowSeconds = d.defaultWindowSeconds
	for _, window := range d.sourceWindows {
		if window > d.windowSeconds {
			d.windowSeconds = window
		}
	}
}

// EnhancedStats returns enhanced statistics including buckets, fingerprints, and aggregations
func (d *Deduper) EnhancedStats() (buckets int, fingerprints int, aggregated int, rateLimits int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.buckets), len(d.fingerprints), len(d.aggregatedEvents), len(d.rateLimits)
}

// Clear removes all entries from cache
func (d *Deduper) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*entry)
	d.lruList = make([]string, 0, d.maxSize)
}

// SetMaxSize updates the maximum cache size (for HA adaptive cache sizing)
func (d *Deduper) SetMaxSize(newMaxSize int) {
	if newMaxSize <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	oldMaxSize := d.maxSize
	d.maxSize = newMaxSize

	// If new size is smaller, evict oldest entries
	if newMaxSize < oldMaxSize && len(d.cache) > newMaxSize {
		entriesToRemove := len(d.cache) - newMaxSize
		for i := 0; i < entriesToRemove && len(d.lruList) > 0; i++ {
			oldest := d.lruList[0]
			delete(d.cache, oldest)
			d.lruList = d.lruList[1:]
		}
	}
}

// SetDefaultWindow updates the default deduplication window (for HA dynamic dedup optimization)
func (d *Deduper) SetDefaultWindow(windowSeconds int) {
	if windowSeconds <= 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.defaultWindowSeconds = windowSeconds
	d.windowSeconds = windowSeconds // Also update for backward compatibility
}
