// Package foreman exposes Prometheus metrics for the Foreman and worker pool (Block 4 observability).
package foreman

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// TasksScheduledTotal is the number of BrainTasks scheduled by the Foreman.
	TasksScheduledTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zen_foreman_tasks_scheduled_total",
		Help: "Total number of BrainTasks scheduled by the Foreman",
	})
	// TasksAdmissionDeniedTotal is the number of tasks denied by the Gate.
	TasksAdmissionDeniedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zen_foreman_tasks_admission_denied_total",
		Help: "Total number of BrainTasks denied admission by the Gate",
	})
	// ReconcileDurationSeconds is the reconcile loop duration.
	ReconcileDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "zen_foreman_reconcile_duration_seconds",
		Help:    "Reconciliation duration in seconds",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 12),
	})
	// TasksDispatchedTotal is the number of tasks enqueued to the worker pool.
	TasksDispatchedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zen_foreman_worker_tasks_dispatched_total",
		Help: "Total number of tasks dispatched to the worker pool",
	})
	// TasksCompletedTotal is the number of tasks completed successfully by workers.
	TasksCompletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zen_foreman_worker_tasks_completed_total",
		Help: "Total number of tasks completed successfully by workers",
	})
	// TasksFailedTotal is the number of tasks that failed in workers.
	TasksFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "zen_foreman_worker_tasks_failed_total",
		Help: "Total number of tasks that failed in workers",
	})
	// WorkerQueueDepth is the current worker queue backlog.
	WorkerQueueDepth = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "zen_foreman_worker_queue_depth",
		Help: "Current number of tasks waiting in the worker queue",
	})
)

func init() {
	prometheus.MustRegister(
		TasksScheduledTotal,
		TasksAdmissionDeniedTotal,
		ReconcileDurationSeconds,
		TasksDispatchedTotal,
		TasksCompletedTotal,
		TasksFailedTotal,
		WorkerQueueDepth,
	)
}
