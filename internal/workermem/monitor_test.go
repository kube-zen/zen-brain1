package workermem

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor_Defaults(t *testing.T) {
	// Use a fresh registry to avoid global state interference
	r := NewRegistry()
	m := NewWorkerMonitor(r, WorkerConfig{
		WorkerType: WorkerTypeTask,
	})
	defer m.Stop()

	assert.NotEmpty(t, m.GetID())
	assert.Equal(t, WorkerTypeTask, m.config.WorkerType)
	assert.Equal(t, time.Second, m.config.SampleInterval)
	assert.NotNil(t, m.GetStats())
}

func TestNewMonitor_CustomConfig(t *testing.T) {
	r := NewRegistry()
	m := NewWorkerMonitor(r, WorkerConfig{
		WorkerID:         "test-worker-1",
		WorkerType:       WorkerTypeAgent,
		SampleInterval:   500 * time.Millisecond,
		AlertThresholdMB: 512,
	})
	defer m.Stop()

	assert.Equal(t, "test-worker-1", m.GetID())
	assert.Equal(t, 500*time.Millisecond, m.config.SampleInterval)
}

func TestMonitor_CollectStats(t *testing.T) {
	r := NewRegistry()
	m := NewWorkerMonitor(r, WorkerConfig{WorkerID: "stats-test"})
	defer m.Stop()

	stats := m.GetStats()
	require.NotNil(t, stats)
	assert.Equal(t, "stats-test", stats.WorkerID)
	assert.Greater(t, stats.AllocatedMB, 0.0)
	assert.Greater(t, stats.SysMB, 0.0)
	assert.Greater(t, stats.NumGoroutines, 0)
	assert.NotEmpty(t, stats.Timestamp)
}

func TestMonitor_StartStop(t *testing.T) {
	r := NewRegistry()
	m := NewWorkerMonitor(r, WorkerConfig{
		WorkerID:       "lifecycle-test",
		SampleInterval: 10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go m.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	assert.GreaterOrEqual(t, m.GetSampleCount(), uint64(1))

	cancel()
	time.Sleep(20 * time.Millisecond)

	assert.Nil(t, r.GetMonitor("lifecycle-test"))
}

func TestMonitor_PeakTracking(t *testing.T) {
	r := NewRegistry()
	m := NewWorkerMonitor(r, WorkerConfig{
		WorkerID:       "peak-test",
		SampleInterval: 10 * time.Millisecond,
	})
	defer m.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	go m.Start(ctx)
	defer cancel()

	time.Sleep(30 * time.Millisecond)

	stats := m.GetStats()
	assert.GreaterOrEqual(t, stats.MaxAllocatedMB, 0.0)
}

func TestMonitor_AlertCallback(t *testing.T) {
	r := NewRegistry()
	alertCalled := false
	m := NewWorkerMonitor(r, WorkerConfig{
		WorkerID:         "alert-test",
		SampleInterval:   10 * time.Millisecond,
		AlertThresholdMB: 0.001,
	})
	m.SetAlertCallback(func(stats *MemoryStats) {
		alertCalled = true
	})

	ctx, cancel := context.WithCancel(context.Background())
	go m.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	assert.True(t, alertCalled)
	assert.GreaterOrEqual(t, m.GetAlertCount(), uint32(1))
	m.Stop()
}

func TestRegistry_RegisterUnregister(t *testing.T) {
	r := NewRegistry()
	m1 := NewWorkerMonitor(r, WorkerConfig{WorkerID: "reg-1"})
	m2 := NewWorkerMonitor(r, WorkerConfig{WorkerID: "reg-2"})

	r.Register(m1)
	r.Register(m2)
	assert.Equal(t, m1, r.GetMonitor("reg-1"))
	assert.Equal(t, m2, r.GetMonitor("reg-2"))
	assert.Len(t, r.ListMonitors(), 2)

	r.Unregister("reg-1")
	assert.Nil(t, r.GetMonitor("reg-1"))
}

func TestRegistry_AggregatedMetrics(t *testing.T) {
	r := NewRegistry()
	r.UpdateMetrics()
	metrics := r.GetAggregatedMetrics()
	require.NotNil(t, metrics)
}

func TestGetCurrentProcessMemory(t *testing.T) {
	stats := GetCurrentProcessMemory()
	require.NotNil(t, stats)
	assert.Equal(t, "process", stats.WorkerID)
	assert.Greater(t, stats.AllocatedMB, 0.0)
	assert.Greater(t, stats.NumGoroutines, 0)
}

func TestForceGC(t *testing.T) {
	// Just verify it doesn't panic
	ForceGC()
}

func TestBytesToMB(t *testing.T) {
	assert.Equal(t, 1.0, bytesToMB(1024*1024))
	assert.Equal(t, 0.5, bytesToMB(512*1024))
	assert.Equal(t, 0.0, bytesToMB(0))
}

func TestConcurrentMonitors(t *testing.T) {
	var wg sync.WaitGroup
	count := 10

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			m := NewMonitor(WorkerConfig{
				WorkerID:       id,
				SampleInterval: 10 * time.Millisecond,
			})
			ctx, cancel := context.WithCancel(context.Background())
			go m.Start(ctx)
			time.Sleep(30 * time.Millisecond)
			cancel()
			m.Stop()
		}(fmt.Sprintf("concurrent-%d", i))
	}

	wg.Wait()
}
