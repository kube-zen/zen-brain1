// Package intelligence provides proof-of-work mining and pattern learning capabilities.
package intelligence

import (
	"time"
)

const (
	// recentWindowDays is the time window (in days) for considering runs as "recent".
	recentWindowDays = 30
)

// recentWindow is the time duration for considering runs as "recent".
var recentWindow = time.Duration(recentWindowDays) * 24 * time.Hour

// isRecent returns true if the given timestamp is within the recent window.
func isRecent(ts time.Time) bool {
	if ts.IsZero() {
		return false
	}
	return time.Since(ts) < recentWindow
}

// calculateFreshnessFactor calculates a freshness factor based on last seen time.
// Returns a value between 0.0 and 1.0:
//   - last seen <= 7d => 1.0
//   - last seen <= 30d => 0.85
//   - last seen <= 90d => 0.60
//   - older => 0.35
func calculateFreshnessFactor(lastSeen time.Time) float64 {
	if lastSeen.IsZero() {
		return 0.35 // Stale/unknown
	}

	elapsed := time.Since(lastSeen)
	days := elapsed.Hours() / 24

	switch {
	case days <= 7:
		return 1.0
	case days <= 30:
		return 0.85
	case days <= 90:
		return 0.60
	default:
		return 0.35
	}
}

// getDaysSince returns the number of days since the given timestamp.
func getDaysSince(ts time.Time) int {
	if ts.IsZero() {
		return -1
	}
	return int(time.Since(ts).Hours() / 24)
}
