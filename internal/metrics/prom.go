package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	// Registry is the global Prometheus registry for d2ip metrics
	Registry *prometheus.Registry
)

// Setup initializes the Prometheus registry with standard collectors.
// It registers process and Go runtime collectors for basic observability.
func Setup() error {
	Registry = prometheus.NewRegistry()

	// Register standard process collector (CPU, memory, file descriptors, etc.)
	Registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Register standard Go collector (goroutines, GC stats, memory stats, etc.)
	Registry.MustRegister(collectors.NewGoCollector())

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
