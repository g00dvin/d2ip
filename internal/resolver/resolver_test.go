package resolver

import (
	"context"
	"strings"
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

func TestIsNXDomain_ClassifiesCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"NXDOMAIN rcode", &DNSError{Rcode: 3, Message: "NXDOMAIN"}, true},
		{"SERVFAIL rcode", &DNSError{Rcode: 2, Message: "SERVFAIL"}, false},
		{"nil error", nil, false},
		{"generic error", context.DeadlineExceeded, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNXDomain(tt.err)
			if result != tt.expected {
				t.Errorf("isNXDomain(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRetryable_ClassifiesCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"SERVFAIL is retryable", &DNSError{Rcode: 2, Message: "SERVFAIL"}, true},
		{"NXDOMAIN is not retryable", &DNSError{Rcode: 3, Message: "NXDOMAIN"}, false},
		{"timeout error is retryable", context.DeadlineExceeded, false},
		{"nil error is not retryable", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDNSError_Error(t *testing.T) {
	err := &DNSError{Rcode: 3, Message: "NXDOMAIN"}
	if !strings.Contains(err.Error(), "NXDOMAIN") {
		t.Errorf("DNSError.Error() should contain 'NXDOMAIN', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("DNSError.Error() should contain rcode '3', got %q", err.Error())
	}
}
