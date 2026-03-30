// Package workermem provides per-worker memory monitoring.
// Adapted from zen-brain 0.1 internal/workermem.
// Critical for preventing memory leaks and resource exhaustion in worker pools.
package workermem

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryStats holds memory usage information for a worker.
type MemoryStats struct {
	WorkerID       string        `json:"worker_id"`
	WorkerType     string        `json:"worker_type"`
	AllocatedMB    float64       `json:"allocated_mb"`
	TotalAllocMB   float64       `json:"total_alloc_mb"`
	SysMB          float64       `json:"sys_mb"`
	HeapAllocMB    float64       `json:"heap_alloc_mb"`
	HeapSysMB      float64       `json:"heap_sys_mb"`
	HeapInuseMB    float64       `json:"heap_inuse_mb"`
	HeapReleasedMB float64       `json:"heap_released_mb"`
	NumGC          uint32        `json:"num_gc"`
	NumGoroutines  int           `json:"num_goroutines"`
	Timestamp      time.Time     `json:"timestamp"`
	Duration       time.Duration `json:"duration"`
	MaxAllocatedMB float64       `json:"max_allocated_mb"`
	MaxSysMB       float64       `json:"max_sys_mb"`
}

// WorkerType identifies the type of worker being monitored.
type WorkerType string

const (
	WorkerTypeTask      WorkerType = "task"
	WorkerTypeScheduler WorkerType = "scheduler"
	WorkerTypeAgent     WorkerType = "agent"
	WorkerTypeGeneric   WorkerType = "generic"
)

// WorkerConfig configures memory monitoring for a worker.
type WorkerConfig struct {
	WorkerID         string
	WorkerType       WorkerType
	Name             string
	SampleInterval   time.Duration // default 1s
	AlertThresholdMB float64       // alert if memory exceeds this (0 = disabled)
}

// Monitor tracks memory usage for a worker.
type Monitor struct {
	id          string
	config      WorkerConfig
	registry    *Registry
	startTime   time.Time
	stopOnce    sync.Once
	stopChan    chan struct{}
	stats       atomic.Value // *MemoryStats
	maxAlloc    atomic.Uint64
	maxSys      atomic.Uint64
	sampleCount atomic.Uint64
	alertCount  atomic.Uint32
	closed      atomic.Bool
	onAlert     func(stats *MemoryStats)
}

// Registry tracks all active worker monitors.
type Registry struct {
	mu         sync.RWMutex
	monitors   map[string]*Monitor
	history    []MemoryStats
	maxHistory int
	metrics    *AggregatedMetrics
}

// AggregatedMetrics holds summary metrics across all workers.
type AggregatedMetrics struct {
	TotalWorkers     int       `json:"total_workers"`
	ActiveWorkers    int       `json:"active_workers"`
	TotalAllocatedMB float64   `json:"total_allocated_mb"`
	TotalSysMB       float64   `json:"total_sys_mb"`
	MaxAllocatedMB   float64   `json:"max_allocated_mb"`
	MaxSysMB         float64   `json:"max_sys_mb"`
	AvgAllocatedMB   float64   `json:"avg_allocated_mb"`
	AvgSysMB         float64   `json:"avg_sys_mb"`
	TotalGoroutines  int       `json:"total_goroutines"`
	TotalAlerts      uint32    `json:"total_alerts"`
	LastUpdate       time.Time `json:"last_update"`
}

var (
	globalRegistry     *Registry
	globalRegistryOnce sync.Once
)

// GetRegistry returns the global worker memory registry (lazy singleton).
func GetRegistry() *Registry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// NewRegistry creates a new worker memory registry.
func NewRegistry() *Registry {
	return &Registry{
		monitors:   make(map[string]*Monitor),
		maxHistory: 1000,
		metrics:    &AggregatedMetrics{LastUpdate: time.Now()},
	}
}

// NewWorkerMonitor creates a new memory monitor for a worker, registering it
// with the given registry instead of the global singleton. Use this in tests
// and for isolated worker pools.
func NewWorkerMonitor(registry *Registry, config WorkerConfig) *Monitor {
	if config.WorkerID == "" {
		config.WorkerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	if config.SampleInterval == 0 {
		config.SampleInterval = time.Second
	}

	m := &Monitor{
		id:        config.WorkerID,
		config:    config,
		startTime: time.Now(),
		stopChan:  make(chan struct{}),
	}

	initial := m.collectStats(0)
	initial.WorkerID = config.WorkerID
	initial.WorkerType = string(config.WorkerType)
	m.stats.Store(initial)
	m.registry = registry

	registry.Register(m)
	return m
}

// NewMonitor creates a new memory monitor for a worker and registers it with
// the global registry.
func NewMonitor(config WorkerConfig) *Monitor {
	return NewWorkerMonitor(GetRegistry(), config)
}

// Start begins periodic memory monitoring (blocking — run in a goroutine).
func (m *Monitor) Start(ctx context.Context) {
	if m.closed.Load() {
		return
	}

	ticker := time.NewTicker(m.config.SampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			stats := m.collectStats(time.Since(m.startTime))
			stats.WorkerID = m.config.WorkerID
			stats.WorkerType = string(m.config.WorkerType)
			m.stats.Store(stats)

			allocBytes := uint64(stats.AllocatedMB * 1024 * 1024)
			sysBytes := uint64(stats.SysMB * 1024 * 1024)
			if allocBytes > m.maxAlloc.Load() {
				m.maxAlloc.Store(allocBytes)
			}
			if sysBytes > m.maxSys.Load() {
				m.maxSys.Store(sysBytes)
			}

			m.sampleCount.Add(1)
			stats.MaxAllocatedMB = float64(m.maxAlloc.Load()) / (1024 * 1024)
			stats.MaxSysMB = float64(m.maxSys.Load()) / (1024 * 1024)

			if m.config.AlertThresholdMB > 0 && stats.AllocatedMB > m.config.AlertThresholdMB {
				m.alertCount.Add(1)
				if m.onAlert != nil {
					m.onAlert(stats)
				}
				fmt.Printf("[ALERT] Worker %s memory %.2fMB exceeds threshold %.2fMB\n",
					m.id, stats.AllocatedMB, m.config.AlertThresholdMB)
			}

			if m.registry != nil {
				m.registry.UpdateMetrics()
			}
		}
	}
}

// Stop stops memory monitoring and returns final stats.
func (m *Monitor) Stop() *MemoryStats {
	m.stopOnce.Do(func() {
		m.closed.Store(true)
		close(m.stopChan)
		if m.registry != nil {
			m.registry.Unregister(m.id)
		}
	})
	return m.GetStats()
}

// GetStats returns the latest memory stats.
func (m *Monitor) GetStats() *MemoryStats {
	v := m.stats.Load()
	if v == nil {
		return nil
	}
	return v.(*MemoryStats)
}

// GetID returns the worker ID.
func (m *Monitor) GetID() string { return m.id }

// GetSampleCount returns the number of samples collected.
func (m *Monitor) GetSampleCount() uint64 { return m.sampleCount.Load() }

// GetAlertCount returns the number of alerts triggered.
func (m *Monitor) GetAlertCount() uint32 { return m.alertCount.Load() }

// SetAlertCallback sets a callback for memory threshold alerts.
func (m *Monitor) SetAlertCallback(fn func(stats *MemoryStats)) { m.onAlert = fn }

func (m *Monitor) collectStats(duration time.Duration) *MemoryStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return &MemoryStats{
		AllocatedMB:    bytesToMB(ms.Alloc),
		TotalAllocMB:   bytesToMB(ms.TotalAlloc),
		SysMB:          bytesToMB(ms.Sys),
		HeapAllocMB:    bytesToMB(ms.HeapAlloc),
		HeapSysMB:      bytesToMB(ms.HeapSys),
		HeapInuseMB:    bytesToMB(ms.HeapInuse),
		HeapReleasedMB: bytesToMB(ms.HeapReleased),
		NumGC:          ms.NumGC,
		NumGoroutines:  runtime.NumGoroutine(),
		Timestamp:      time.Now(),
		Duration:       duration,
	}
}

// Register registers a monitor with the registry.
func (r *Registry) Register(monitor *Monitor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.monitors[monitor.id] = monitor
	r.metrics.TotalWorkers++
	r.metrics.ActiveWorkers++
	r.metrics.LastUpdate = time.Now()
}

// Unregister unregisters a monitor from the registry.
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.monitors[id]; ok {
		delete(r.monitors, id)
		r.metrics.ActiveWorkers--
		r.metrics.LastUpdate = time.Now()
	}
}

// GetMonitor returns a monitor by ID.
func (r *Registry) GetMonitor(id string) *Monitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.monitors[id]
}

// ListMonitors returns all active monitors.
func (r *Registry) ListMonitors() []*Monitor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Monitor, 0, len(r.monitors))
	for _, m := range r.monitors {
		out = append(out, m)
	}
	return out
}

// GetAggregatedMetrics returns aggregated metrics across all workers.
func (r *Registry) GetAggregatedMetrics() *AggregatedMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.metrics
}

// UpdateMetrics recalculates aggregated metrics from all monitors.
func (r *Registry) UpdateMetrics() {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := len(r.monitors)
	if n == 0 {
		r.metrics.TotalAllocatedMB = 0
		r.metrics.TotalSysMB = 0
		r.metrics.MaxAllocatedMB = 0
		r.metrics.MaxSysMB = 0
		r.metrics.AvgAllocatedMB = 0
		r.metrics.AvgSysMB = 0
		r.metrics.TotalGoroutines = runtime.NumGoroutine()
		r.metrics.ActiveWorkers = 0
		r.metrics.LastUpdate = time.Now()
		return
	}

	var totalAlloc, totalSys, maxAlloc, maxSys float64
	var totalAlerts uint32

	for _, m := range r.monitors {
		s := m.GetStats()
		if s != nil {
			totalAlloc += s.AllocatedMB
			totalSys += s.SysMB
			if s.MaxAllocatedMB > maxAlloc {
				maxAlloc = s.MaxAllocatedMB
			}
			if s.MaxSysMB > maxSys {
				maxSys = s.MaxSysMB
			}
			totalAlerts += m.alertCount.Load()
			r.history = append(r.history, *s)
		}
	}
	if len(r.history) > r.maxHistory {
		r.history = r.history[len(r.history)-r.maxHistory:]
	}

	r.metrics.TotalAllocatedMB = totalAlloc
	r.metrics.TotalSysMB = totalSys
	r.metrics.MaxAllocatedMB = maxAlloc
	r.metrics.MaxSysMB = maxSys
	r.metrics.AvgAllocatedMB = totalAlloc / float64(n)
	r.metrics.AvgSysMB = totalSys / float64(n)
	r.metrics.TotalGoroutines = runtime.NumGoroutine()
	r.metrics.TotalAlerts = totalAlerts
	r.metrics.ActiveWorkers = n
	r.metrics.LastUpdate = time.Now()
}

// GetHistory returns historical memory stats (last N entries).
func (r *Registry) GetHistory(limit int) []MemoryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	cp := make([]MemoryStats, limit)
	copy(cp, r.history[len(r.history)-limit:])
	return cp
}

// GetCurrentProcessMemory returns memory stats for the current process.
func GetCurrentProcessMemory() *MemoryStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return &MemoryStats{
		WorkerID:       "process",
		WorkerType:     "process",
		AllocatedMB:    bytesToMB(ms.Alloc),
		TotalAllocMB:   bytesToMB(ms.TotalAlloc),
		SysMB:          bytesToMB(ms.Sys),
		HeapAllocMB:    bytesToMB(ms.HeapAlloc),
		HeapSysMB:      bytesToMB(ms.HeapSys),
		HeapInuseMB:    bytesToMB(ms.HeapInuse),
		HeapReleasedMB: bytesToMB(ms.HeapReleased),
		NumGC:          ms.NumGC,
		NumGoroutines:  runtime.NumGoroutine(),
		Timestamp:      time.Now(),
	}
}

// ForceGC forces a garbage collection.
func ForceGC() { runtime.GC() }

func bytesToMB(b uint64) float64 { return float64(b) / (1024 * 1024) }
