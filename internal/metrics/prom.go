package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	// Registry is the global Prometheus registry for d2ip metrics
	Registry *prometheus.Registry

	// Resolver metrics
	DNSResolveTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dns_resolve_total",
			Help: "Total number of DNS resolution attempts by status (success, failed, nxdomain)",
		},
		[]string{"status"},
	)

	DNSResolveDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dns_resolve_duration_seconds",
			Help:    "Duration of DNS queries in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
	)

	// Orchestrator pipeline metrics
	PipelineRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_runs_total",
			Help: "Total number of pipeline runs by status (success, failed)",
		},
		[]string{"status"},
	)

	PipelineStepDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pipeline_step_duration_seconds",
			Help:    "Duration of pipeline steps in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120},
		},
		[]string{"step"},
	)

	PipelineLastSuccess = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "pipeline_last_success_timestamp",
			Help: "Unix timestamp of last successful pipeline run",
		},
	)
)

// Setup initializes the Prometheus registry with standard collectors.
// It registers process and Go runtime collectors for basic observability.
func Setup() error {
	Registry = prometheus.NewRegistry()

	// Register standard process collector (CPU, memory, file descriptors, etc.)
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Register standard Go collector (goroutines, GC stats, memory stats, etc.)
	Registry.MustRegister(collectors.NewGoCollector())

	// Register resolver metrics
	Registry.MustRegister(DNSResolveTotal)
	Registry.MustRegister(DNSResolveDuration)

	// Register orchestrator pipeline metrics
	Registry.MustRegister(PipelineRunsTotal)
	Registry.MustRegister(PipelineStepDuration)
	Registry.MustRegister(PipelineLastSuccess)

	return nil
}

// MustRegister is a convenience function to register collectors with the global registry.
// It panics if registration fails (similar to prometheus.MustRegister).
func MustRegister(cs ...prometheus.Collector) {
	if Registry == nil {
		panic("metrics registry not initialized; call metrics.Setup() first")
	}
	Registry.MustRegister(cs...)
}

// Register is a convenience function to register collectors with the global registry.
// It returns an error if registration fails.
func Register(c prometheus.Collector) error {
	if Registry == nil {
		return prometheus.AlreadyRegisteredError{}
	}
	return Registry.Register(c)
}

// Unregister removes a collector from the global registry.
func Unregister(c prometheus.Collector) bool {
	if Registry == nil {
		return false
	}
	return Registry.Unregister(c)
}
