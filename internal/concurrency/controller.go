// Package concurrency implements a dynamic, work-conserving concurrency controller
// for zen-brain's L1 factory.
//
// Policy:
//   - Keep workers busy whenever runnable work exists
//   - Always keep 2 model slots free for scheduled tasks
//   - Never push the host into sustained overload
//   - Dynamically increase/decrease concurrency instead of relying on static W values
//
// Two independent gates:
//  1. Model-slot gate: 2 reserved slots for scheduled tasks, general work gets the rest
//  2. Host-resource gate: sustained CPU below soft cap, hard stop above hard cap
//
// Conservative fallback: if telemetry is unavailable, fall back to safe minimum.
package concurrency

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Default constants
const (
	DefaultReservedSlots  = 2
	DefaultTotalSlots     = 10
	DefaultCPUSoftCapPct  = 80
	DefaultCPUHardCapPct  = 90
	DefaultHysteresisDur  = 30 * time.Second
	DefaultMinGeneral     = 1 // never go below 1 general worker if work exists
)

// CPUReader is an interface for reading host CPU pressure.
// Implemented by real readers; a no-op reader is available for testing.
type CPUReader interface {
	// CPUPercent returns the current CPU usage as a percentage (0-100).
	// Returns error if telemetry is unavailable.
	CPUPercent() (float64, error)
}

// Controller manages dynamic concurrency decisions.
type Controller struct {
	mu sync.RWMutex

	// Configuration
	totalSlots     int
	reservedSlots  int
	cpuSoftCapPct  float64
	cpuHardCapPct  float64
	hysteresisDur  time.Duration
	minGeneral     int

	// CPU reader (injected, replaceable for testing)
	cpuReader CPUReader

	// Hysteresis state
	cpuHighSince time.Time // when CPU first crossed soft cap
	throttled    bool      // whether we're currently in throttle mode

	// Metrics (observable)
	metrics ConcurrencyMetrics
}

// ConcurrencyMetrics exposes all controller state for observability.
type ConcurrencyMetrics struct {
	TotalSlots        int     `json:"total_slots"`
	ReservedSlots     int     `json:"reserved_slots"`
	UsableSlots       int     `json:"usable_slots"`
	DesiredGeneral    int     `json:"desired_general_workers"`
	CurrentRunning    int     `json:"current_running_general_workers"`
	ReadyCount        int     `json:"ready_count"`
	BacklogCount      int     `json:"backlog_count"`
	CPUPercent        float64 `json:"cpu_percent"`
	ThrottleReason    string  `json:"throttle_reason,omitempty"`
	IsThrottled       bool    `json:"is_throttled"`
	IsConservative    bool    `json:"is_conservative_fallback"`
	LastCalculation   string  `json:"last_calculation,omitempty"`
}

// Config holds controller configuration.
type Config struct {
	TotalSlots    int           `json:"total_slots"`
	ReservedSlots int           `json:"reserved_slots"`
	CPUSoftCapPct float64       `json:"cpu_soft_cap_pct"`
	CPUHardCapPct float64       `json:"cpu_hard_cap_pct"`
	HysteresisDur time.Duration `json:"hysteresis_duration"`
	MinGeneral    int           `json:"min_general_workers"`
}

// LoadConfigFromEnv creates Config from environment variables.
func LoadConfigFromEnv() Config {
	return Config{
		TotalSlots:    envIntOr("CONCURRENCY_TOTAL_SLOTS", DefaultTotalSlots),
		ReservedSlots: envIntOr("CONCURRENCY_RESERVED_SLOTS", DefaultReservedSlots),
		CPUSoftCapPct: envFloatOr("CONCURRENCY_CPU_SOFT_CAP", float64(DefaultCPUSoftCapPct)),
		CPUHardCapPct: envFloatOr("CONCURRENCY_CPU_HARD_CAP", float64(DefaultCPUHardCapPct)),
		HysteresisDur: envDurationOr("CONCURRENCY_HYSTERESIS", DefaultHysteresisDur),
		MinGeneral:    envIntOr("CONCURRENCY_MIN_GENERAL", DefaultMinGeneral),
	}
}

// NewController creates a new concurrency controller.
func NewController(cfg Config, cpuReader CPUReader) *Controller {
	if cfg.TotalSlots <= 0 {
		cfg.TotalSlots = DefaultTotalSlots
	}
	if cfg.ReservedSlots < 0 {
		cfg.ReservedSlots = 0
	}
	if cfg.ReservedSlots >= cfg.TotalSlots {
		cfg.ReservedSlots = cfg.TotalSlots - 1
	}
	if cfg.CPUSoftCapPct <= 0 {
		cfg.CPUSoftCapPct = float64(DefaultCPUSoftCapPct)
	}
	if cfg.CPUHardCapPct <= cfg.CPUSoftCapPct {
		cfg.CPUHardCapPct = cfg.CPUSoftCapPct + 10
	}
	if cfg.HysteresisDur <= 0 {
		cfg.HysteresisDur = DefaultHysteresisDur
	}
	if cfg.MinGeneral <= 0 {
		cfg.MinGeneral = DefaultMinGeneral
	}

	if cpuReader == nil {
		cpuReader = &procStatCPUReader{}
	}

	return &Controller{
		totalSlots:    cfg.TotalSlots,
		reservedSlots: cfg.ReservedSlots,
		cpuSoftCapPct: cfg.CPUSoftCapPct,
		cpuHardCapPct: cfg.CPUHardCapPct,
		hysteresisDur: cfg.HysteresisDur,
		minGeneral:    cfg.MinGeneral,
		cpuReader:     cpuReader,
	}
}

// DesiredConcurrency calculates how many general workers should be running.
//
// desired_general_workers = min(runnable_work, usable_slots, cpu_allowed_slots)
//
// Invariants enforced:
//  1. Reserved slots (2) must not be consumed by general work
//  2. CPU above hard cap → no additional dispatch
//  3. CPU above soft cap for hysteresis duration → throttle
//  4. Telemetry unavailable → conservative fallback
//  5. runnable_work=0 → desired=0 (no idle workers)
func (c *Controller) DesiredConcurrency(runnableWork int, currentRunning int) (desired int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	usableSlots := c.totalSlots - c.reservedSlots

	if usableSlots < 1 {
		usableSlots = 1 // at least 1 general slot
	}

	// Invariant 1: no work = no workers
	if runnableWork <= 0 {
		c.updateMetrics(0, currentRunning, 0, 0, 0, false, "no_runnable_work", false)
		return 0, "no_runnable_work"
	}

	// Read CPU
	cpuPct, cpuErr := c.cpuReader.CPUPercent()
	c.metrics.CPUPercent = cpuPct

	isConservative := false
	throttleReason := ""

	if cpuErr != nil {
		// Invariant 4: telemetry failure → conservative fallback
		// Use min_general instead of full capacity
		isConservative = true
		throttleReason = "cpu_telemetry_unavailable"
		log.Printf("[CONCURRENCY] ⚠️ CPU telemetry unavailable, falling back conservative (min=%d)", c.minGeneral)
		cpuAllowed := c.minGeneral
		if cpuAllowed > usableSlots {
			cpuAllowed = usableSlots
		}
		if runnableWork < cpuAllowed {
			cpuAllowed = runnableWork
		}
		c.updateMetrics(cpuAllowed, currentRunning, runnableWork, cpuPct, usableSlots, false, throttleReason, isConservative)
		return cpuAllowed, throttleReason
	}

	// Invariant 3: CPU above hard cap → no additional dispatch
	if cpuPct >= c.cpuHardCapPct {
		throttleReason = "cpu_hard_cap"
		c.cpuHighSince = now // track for hysteresis
		c.throttled = true
		// Allow current work to drain, don't add more
		desired := 0
		c.updateMetrics(desired, currentRunning, runnableWork, cpuPct, usableSlots, true, throttleReason, false)
		return desired, throttleReason
	}

	// Hysteresis check: if we've been throttled, require CPU to drop below soft cap
	// for the hysteresis duration before un-throttling
	if c.throttled {
		if cpuPct >= c.cpuSoftCapPct {
			// Still above soft cap, stay throttled
			c.cpuHighSince = now
			throttleReason = "cpu_soft_cap_hysteresis"
			desired := c.minGeneral // allow minimum during hysteresis
			if desired > usableSlots {
				desired = usableSlots
			}
			if runnableWork < desired {
				desired = runnableWork
			}
			c.updateMetrics(desired, currentRunning, runnableWork, cpuPct, usableSlots, true, throttleReason, false)
			return desired, throttleReason
		}
		// CPU dropped below soft cap
		if now.Sub(c.cpuHighSince) < c.hysteresisDur {
			// Not long enough below soft cap yet
			throttleReason = "cpu_hysteresis_waiting"
			desired := c.minGeneral
			if desired > usableSlots {
				desired = usableSlots
			}
			if runnableWork < desired {
				desired = runnableWork
			}
			c.updateMetrics(desired, currentRunning, runnableWork, cpuPct, usableSlots, true, throttleReason, false)
			return desired, throttleReason
		}
		// Hysteresis passed, un-throttle
		c.throttled = false
		c.cpuHighSince = time.Time{}
		log.Printf("[CONCURRENCY] ✅ Un-throttled (CPU=%.1f%% below soft cap for %v)",
			cpuPct, c.hysteresisDur)
	}

	// Check if CPU is approaching soft cap
	cpuAllowedSlots := usableSlots
	if cpuPct >= c.cpuSoftCapPct {
		// Start tracking for potential throttle
		if c.cpuHighSince.IsZero() {
			c.cpuHighSince = now
		}
		// Mark as throttled so hysteresis applies even if CPU drops slightly
		c.throttled = true
		// Calculate reduced slots based on CPU pressure
		cpuPressure := (cpuPct - c.cpuSoftCapPct) / (c.cpuHardCapPct - c.cpuSoftCapPct)
		reducedSlots := int(float64(usableSlots) * (1.0 - cpuPressure*0.5))
		if reducedSlots < c.minGeneral {
			reducedSlots = c.minGeneral
		}
		cpuAllowedSlots = reducedSlots
		throttleReason = "cpu_soft_cap_reducing"
	}

	// Invariant 2: enforce reserved capacity
	if cpuAllowedSlots > usableSlots {
		cpuAllowedSlots = usableSlots
	}

	// Final: min(runnable_work, usable_slots, cpu_allowed_slots)
	desired = runnableWork
	if desired > usableSlots {
		desired = usableSlots
	}
	if desired > cpuAllowedSlots {
		desired = cpuAllowedSlots
	}

	// Invariant 1 check: idle workers while runnable work exists = bug
	if desired < runnableWork && currentRunning < desired && throttleReason == "" {
		log.Printf("[CONCURRENCY] ⚠️ IDLE WORKERS BUG: desired=%d running=%d runnable=%d cpu=%.1f%%",
			desired, currentRunning, runnableWork, cpuPct)
	}

	c.updateMetrics(desired, currentRunning, runnableWork, cpuPct, usableSlots,
		throttleReason != "", throttleReason, false)
	return desired, throttleReason
}

// updateMetrics records the latest calculation for observability.
func (c *Controller) updateMetrics(desired, currentRunning, runnableWork int, cpuPct float64, usableSlots int, throttled bool, reason string, conservative bool) {
	c.metrics = ConcurrencyMetrics{
		TotalSlots:      c.totalSlots,
		ReservedSlots:   c.reservedSlots,
		UsableSlots:     usableSlots,
		DesiredGeneral:  desired,
		CurrentRunning:  currentRunning,
		ReadyCount:      runnableWork, // caller can set more specifically
		CPUPercent:      cpuPct,
		ThrottleReason:  reason,
		IsThrottled:     throttled,
		IsConservative:  conservative,
		LastCalculation: time.Now().Format(time.RFC3339),
	}
}

// Metrics returns a snapshot of current controller metrics.
func (c *Controller) Metrics() ConcurrencyMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

// SlotsAvailable returns how many more general workers can be dispatched
// given current running count and desired concurrency.
func (c *Controller) SlotsAvailable(runnableWork int, currentRunning int) int {
	desired, _ := c.DesiredConcurrency(runnableWork, currentRunning)
	if currentRunning >= desired {
		return 0
	}
	return desired - currentRunning
}

// String returns a human-readable summary of current state.
func (c *Controller) String() string {
	m := c.Metrics()
	return fmt.Sprintf("total=%d reserved=%d usable=%d desired=%d running=%d cpu=%.1f%% throttled=%v reason=%q",
		m.TotalSlots, m.ReservedSlots, m.UsableSlots, m.DesiredGeneral,
		m.CurrentRunning, m.CPUPercent, m.IsThrottled, m.ThrottleReason)
}

// env helpers
func envIntOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return fallback
}

func envFloatOr(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		if f > 0 {
			return f
		}
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
