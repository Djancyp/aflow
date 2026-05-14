// Package metrics registers all Prometheus metrics for aflow.
// Import this package for its side-effect (registration) and then call
// the exported vars directly — no dependency injection needed.
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// HTTP

	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aflow",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests partitioned by method, path template, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aflow",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Executions

	ExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aflow",
			Name:      "executions_total",
			Help:      "Total workflow executions partitioned by final status.",
		},
		[]string{"status"},
	)

	ExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aflow",
			Name:      "execution_duration_seconds",
			Help:      "End-to-end workflow execution duration in seconds.",
			Buckets:   []float64{.05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"status"},
	)

	// Nodes

	NodeExecutionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aflow",
			Name:      "node_executions_total",
			Help:      "Total node executions partitioned by node type and status.",
		},
		[]string{"node_type", "status"},
	)

	NodeExecutionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aflow",
			Name:      "node_execution_duration_seconds",
			Help:      "Per-node execution duration in seconds.",
			Buckets:   []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"node_type"},
	)

	// Queue

	QueueJobsEnqueued = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aflow",
			Name:      "queue_jobs_enqueued_total",
			Help:      "Total jobs enqueued by kind.",
		},
		[]string{"kind"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		ExecutionsTotal,
		ExecutionDuration,
		NodeExecutionsTotal,
		NodeExecutionDuration,
		QueueJobsEnqueued,
	)
}
