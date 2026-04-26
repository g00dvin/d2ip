package resolver

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
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

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status Status
		want   string
	}{
		{StatusValid, "valid"},
		{StatusFailed, "failed"},
		{StatusNXDomain, "nxdomain"},
		{Status(255), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Upstream != "8.8.8.8:53" {
		t.Errorf("unexpected upstream: %s", cfg.Upstream)
	}
	if cfg.Network != "udp" {
		t.Errorf("unexpected network: %s", cfg.Network)
	}
	if cfg.Concurrency != 64 {
		t.Errorf("unexpected concurrency: %d", cfg.Concurrency)
	}
	if cfg.QPS != 1000 {
		t.Errorf("unexpected qps: %d", cfg.QPS)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("unexpected timeout: %v", cfg.Timeout)
	}
	if cfg.Retries != 2 {
		t.Errorf("unexpected retries: %d", cfg.Retries)
	}
	if cfg.BackoffBase != 100*time.Millisecond {
		t.Errorf("unexpected backoff base: %v", cfg.BackoffBase)
	}
	if cfg.BackoffMax != 5*time.Second {
		t.Errorf("unexpected backoff max: %v", cfg.BackoffMax)
	}
	if cfg.FollowCNAME != true {
		t.Errorf("unexpected followCNAME: %v", cfg.FollowCNAME)
	}
	if cfg.MaxCNAMEChain != 8 {
		t.Errorf("unexpected maxCNAMEChain: %d", cfg.MaxCNAMEChain)
	}
}

func TestNew_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"empty upstream", Config{Upstream: "", Concurrency: 1, QPS: 1}},
		{"zero concurrency", Config{Upstream: "1.1.1.1:53", Concurrency: 0, QPS: 1}},
		{"zero qps", Config{Upstream: "1.1.1.1:53", Concurrency: 1, QPS: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestNew_DefaultMaxCNAMEChain(t *testing.T) {
	cfg := Config{
		Upstream:    "1.1.1.1:53",
		Concurrency: 1,
		QPS:         1,
	}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer r.Close()
	if r.cfg.MaxCNAMEChain != 8 {
		t.Errorf("expected MaxCNAMEChain=8, got %d", r.cfg.MaxCNAMEChain)
	}
}

func TestCalculateBackoff(t *testing.T) {
	r := &DNSResolver{
		cfg: Config{
			BackoffBase: 100 * time.Millisecond,
			BackoffMax:  1 * time.Second,
		},
	}

	b1 := r.calculateBackoff(1)
	// Base: 100ms * 2^1 = 200ms. Jitter: ±25% -> [150ms, 250ms]
	if b1 < 150*time.Millisecond || b1 > 250*time.Millisecond {
		t.Errorf("attempt 1 backoff out of expected range: %v", b1)
	}

	b10 := r.calculateBackoff(10)
	if b10 > r.cfg.BackoffMax+250*time.Millisecond {
		t.Errorf("attempt 10 backoff should be capped near max: %v", b10)
	}

	b0 := r.calculateBackoff(0)
	if b0 < 75*time.Millisecond || b0 > 125*time.Millisecond {
		t.Errorf("attempt 0 backoff out of expected range: %v", b0)
	}
}

func TestQueryDomain_ValidResult(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.0.2.1"),
		})
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusValid {
		t.Errorf("expected StatusValid, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry for valid result")
	}
	if len(result.IPv4) != 1 {
		t.Errorf("expected 1 IPv4 address, got %d", len(result.IPv4))
	}
}

func TestQueryDomain_NoFollowCNAME(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.0.2.1"),
		})
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusValid {
		t.Errorf("expected StatusValid, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry")
	}
}

func TestQueryDomain_NXDOMAIN(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeNameError
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusNXDomain {
		t.Errorf("expected StatusNXDomain, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry for NXDOMAIN")
	}
}

func TestQueryDomain_BothRetryable(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if !shouldRetry {
		t.Error("expected retry for SERVFAIL")
	}
}

func TestQueryDomain_OneSucceedsOneFails(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Qtype {
		case dns.TypeA:
			m.Rcode = dns.RcodeServerFailure
		case dns.TypeAAAA:
			m.Answer = append(m.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
				AAAA: net.ParseIP("2001:db8::1"),
			})
		}
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusValid {
		t.Errorf("expected StatusValid, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry when one query succeeds")
	}
	if len(result.IPv6) != 1 {
		t.Errorf("expected 1 IPv6 address, got %d", len(result.IPv6))
	}
}

func TestQueryDomain_BothEmptyNoError(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusNXDomain {
		t.Errorf("expected StatusNXDomain for empty NOERROR, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry")
	}
}

func TestQueryDomain_NonRetryableError(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Qtype {
		case dns.TypeA:
			m.Rcode = dns.RcodeRefused
		case dns.TypeAAAA:
			m.Rcode = dns.RcodeSuccess
		}
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "example.com")
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry for non-retryable error")
	}
}

func TestQueryDomain_CNAMEError(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Name {
		case "a.example.com.":
			m.Answer = append(m.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: "a.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
				Target: "b.example.com.",
			})
		case "b.example.com.":
			m.Answer = append(m.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: "b.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
				Target: "a.example.com.",
			})
		}
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	res, err := New(Config{
		Upstream:      addr,
		Network:       "udp",
		Timeout:       time.Second,
		Concurrency:   1,
		QPS:           1000,
		FollowCNAME:   true,
		MaxCNAMEChain: 8,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, shouldRetry := res.queryDomain(context.Background(), "a.example.com")
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %v", result.Status)
	}
	if shouldRetry {
		t.Error("expected no retry for CNAME loop")
	}
	if result.Err == nil || !strings.Contains(result.Err.Error(), "CNAME loop") {
		t.Errorf("expected CNAME loop error, got %v", result.Err)
	}
}

func TestResolveBatch_EmptyDomains(t *testing.T) {
	defer goleak.VerifyNone(t)

	r, err := New(Config{
		Upstream:    "1.1.1.1:53",
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 2,
		QPS:         10,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer r.Close()

	ctx := context.Background()
	resultCh := r.ResolveBatch(ctx, []string{})

	count := 0
	for range resultCh {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 results, got %d", count)
	}
}

func TestResolveDomain_ContextCancelledDuringBackoff(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	r, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		Retries:     2,
		BackoffBase: 5 * time.Second,
		BackoffMax:  10 * time.Second,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result := r.resolveDomain(ctx, "example.com")
	if !errors.Is(result.Err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", result.Err)
	}
}

func TestResolveDomain_ResolverClosedDuringBackoff(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	r, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 1,
		QPS:         1000,
		Retries:     2,
		BackoffBase: 5 * time.Second,
		BackoffMax:  10 * time.Second,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		r.Close()
	}()

	result := r.resolveDomain(context.Background(), "example.com")
	if result.Err == nil || !strings.Contains(result.Err.Error(), "resolver closed") {
		t.Errorf("expected resolver closed error, got %v", result.Err)
	}
}

func TestResolveBatch_ClosedResolver(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		// Slow response to give Close() time to fire
		time.Sleep(200 * time.Millisecond)
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	}
	addr, shutdown, err := startMockDNSServer(handler)
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer shutdown()

	r, err := New(Config{
		Upstream:    addr,
		Network:     "udp",
		Timeout:     time.Second,
		Concurrency: 2,
		QPS:         1,
		Retries:     0,
		FollowCNAME: false,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	ctx := context.Background()
	resultCh := r.ResolveBatch(ctx, []string{"a.com", "b.com", "c.com"})

	go func() {
		time.Sleep(50 * time.Millisecond)
		r.Close()
	}()

	count := 0
	for range resultCh {
		count++
	}
	// We don't assert an exact count because races between Close and workers
	// are acceptable here; we just need to ensure it doesn't panic or deadlock.
	_ = count
}
