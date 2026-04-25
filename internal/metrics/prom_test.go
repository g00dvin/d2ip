package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSetup(t *testing.T) {
	if err := Setup(); err != nil {
		t.Fatalf("Setup error: %v", err)
	}
	if Registry == nil {
		t.Error("Registry is nil after Setup")
	}
}

func TestMustRegisterAndUnregister(t *testing.T) {
	if err := Setup(); err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_counter_total",
		Help: "Test counter",
	})

	MustRegister(counter)
	Unregister(counter)
	// If we get here without panic, test passes
}
