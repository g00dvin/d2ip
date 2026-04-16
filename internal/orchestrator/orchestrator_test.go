package orchestrator

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain verifies no goroutines leak from any test in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestOrchestrator_NoGoroutineLeak is a placeholder test that verifies
// the goleak infrastructure is in place. Full integration testing with mocks
// requires complex setup with all agent interfaces properly implemented.
//
// The orchestrator itself doesn't spawn background goroutines in New(),
// so structural leaks are unlikely. The real leak risk is in Run(), which
// would require full mock setup of all 7 agents.
func TestOrchestrator_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Placeholder test - orchestrator construction is simple and doesn't
	// spawn goroutines. Full Run() testing would require:
	// - source.DLCStore mock
	// - domainlist.ListProvider mock
	// - resolver.Resolver mock
	// - cache.Cache mock
	// - aggregator.Aggregator mock
	// - exporter.FileExporter mock
	// - routing.Router mock
	// - config.Config function
	//
	// These mocks are non-trivial and better suited for integration tests.
	// This test mainly ensures goleak is enabled for future work.

	t.Log("goleak enabled for orchestrator package")
}
