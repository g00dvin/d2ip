package resolver

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// TestMain verifies no goroutines leak from any test in this package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestResolver_NoGoroutineLeak verifies the resolver properly cleans up goroutines.
func TestResolver_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Create resolver with minimal config
	cfg := Config{
		Upstream:    "1.1.1.1:53",
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 2,
		QPS:         10,
		Retries:     1,
		BackoffBase: 100 * time.Millisecond,
		BackoffMax:  time.Second,
		FollowCNAME: false,
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}

	// Run batch resolution
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	testDomains := []string{"example.com", "google.com"}
	resultCh := r.ResolveBatch(ctx, testDomains)

	// Consume all results
	for range resultCh {
		// Just drain the channel
	}

	// Close resolver to clean up goroutines
	if err := r.Close(); err != nil {
		t.Errorf("Failed to close resolver: %v", err)
	}

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)

	// Verify no goroutines leaked
	// (goleak.VerifyNone in defer will check)
}

// TestResolver_CancelContext verifies context cancellation doesn't leak goroutines.
func TestResolver_CancelContext(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := Config{
		Upstream:    "1.1.1.1:53",
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 4,
		QPS:         20,
		Retries:     2,
		BackoffBase: 50 * time.Millisecond,
		BackoffMax:  500 * time.Millisecond,
		FollowCNAME: false,
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create resolver: %v", err)
	}
	defer r.Close()

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before starting

	resultCh := r.ResolveBatch(ctx, []string{"test1.com", "test2.com", "test3.com"})

	// Drain channel
	for range resultCh {
	}

	// Give goroutines time to exit
	time.Sleep(100 * time.Millisecond)

	// Verify no leaks even with immediate cancellation
}
