package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRegister(t *testing.T) {
	if err := Setup(); err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_register_counter_total",
		Help: "Test counter",
	})

	err := Register(counter)
	require.NoError(t, err)
	Unregister(counter)
}

func TestMustRegister_Panic(t *testing.T) {
	oldRegistry := Registry
	Registry = nil
	defer func() { Registry = oldRegistry }()

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_panic_counter_total",
		Help: "Test counter",
	})

	assert.Panics(t, func() {
		MustRegister(counter)
	})
}

func TestUnregister_NilRegistry(t *testing.T) {
	oldRegistry := Registry
	Registry = nil
	defer func() { Registry = oldRegistry }()

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_unregister_counter_total",
		Help: "Test counter",
	})

	ok := Unregister(counter)
	assert.False(t, ok)
}
