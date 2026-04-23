package resolver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// DNSResolver implements Resolver using miekg/dns with a worker pool.
type DNSResolver struct {
	cfg     Config
	limiter *rate.Limiter
	client  *dns.Client

	// Shutdown coordination
	closeOnce sync.Once
	closed    chan struct{}
}

// New creates a new DNS resolver with the given configuration.
func New(cfg Config) (*DNSResolver, error) {
	if cfg.Upstream == "" {
		return nil, fmt.Errorf("resolver: upstream cannot be empty")
	}
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("resolver: concurrency must be >= 1")
	}
	if cfg.QPS < 1 {
		return nil, fmt.Errorf("resolver: qps must be >= 1")
	}
	if cfg.MaxCNAMEChain == 0 {
		cfg.MaxCNAMEChain = 8
	}

	// Create rate limiter: QPS tokens/sec, burst = QPS
	limiter := rate.NewLimiter(rate.Limit(cfg.QPS), cfg.QPS)

	// Create DNS client
	client := &dns.Client{
		Net:     cfg.Network,
		Timeout: cfg.Timeout,
	}

	r := &DNSResolver{
		cfg:     cfg,
		limiter: limiter,
		client:  client,
		closed:  make(chan struct{}),
	}

	log.Info().
		Str("upstream", cfg.Upstream).
		Str("network", cfg.Network).
		Int("concurrency", cfg.Concurrency).
		Int("qps", cfg.QPS).
		Msg("resolver: initialized")

	return r, nil
}

// ResolveBatch resolves domains concurrently using a worker pool.
func (r *DNSResolver) ResolveBatch(ctx context.Context, domains []string) <-chan ResolveResult {
	resultCh := make(chan ResolveResult, r.cfg.Concurrency)
	domainCh := make(chan string, r.cfg.Concurrency)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < r.cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			r.worker(ctx, workerID, domainCh, resultCh)
		}(i)
	}

	// Feed domains to workers
	go func() {
		defer close(domainCh)
		for _, domain := range domains {
			select {
			case domainCh <- domain:
			case <-ctx.Done():
				return
			case <-r.closed:
				return
			}
		}
	}()

	// Close result channel when all workers finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	return resultCh
}

// worker processes domains from the input channel and sends results to output.
func (r *DNSResolver) worker(ctx context.Context, id int, domainCh <-chan string, resultCh chan<- ResolveResult) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.closed:
			return
		case domain, ok := <-domainCh:
			if !ok {
				return
			}

			// Wait for rate limiter
			if err := r.limiter.Wait(ctx); err != nil {
				// Context cancelled during rate limit wait
				return
			}

			// Resolve the domain
			result := r.resolveDomain(ctx, domain)

			// Send result (with backpressure handling)
			select {
			case resultCh <- result:
			case <-ctx.Done():
				return
			case <-r.closed:
				return
			}
		}
	}
}

// resolveDomain performs the actual DNS resolution with retry logic.
func (r *DNSResolver) resolveDomain(ctx context.Context, domain string) ResolveResult {
	var lastErr error

	for attempt := 0; attempt <= r.cfg.Retries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			backoff := r.calculateBackoff(attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ResolveResult{
					Domain:     domain,
					Status:     StatusFailed,
					ResolvedAt: time.Now(),
					Err:        ctx.Err(),
				}
			case <-r.closed:
				return ResolveResult{
					Domain:     domain,
					Status:     StatusFailed,
					ResolvedAt: time.Now(),
					Err:        fmt.Errorf("resolver closed"),
				}
			}
		}

		// Perform DNS query
		result, shouldRetry := r.queryDomain(ctx, domain)
		if !shouldRetry {
			return result
		}

		lastErr = result.Err
		log.Debug().
			Str("domain", domain).
			Int("attempt", attempt+1).
			Err(lastErr).
			Msg("resolver: retrying")
	}

	// All retries exhausted
	return ResolveResult{
		Domain:     domain,
		Status:     StatusFailed,
		ResolvedAt: time.Now(),
		Err:        fmt.Errorf("max retries exceeded: %w", lastErr),
	}
}

// queryDomain performs a single DNS query (both A and AAAA).
// Returns (result, shouldRetry).
func (r *DNSResolver) queryDomain(ctx context.Context, domain string) (ResolveResult, bool) {
	result := ResolveResult{
		Domain:     domain,
		ResolvedAt: time.Now(),
	}

	// Follow CNAME chain if enabled
	targetDomain := domain
	if r.cfg.FollowCNAME {
		finalDomain, err := r.followCNAME(ctx, domain)
		if err != nil {
			// CNAME loop or too many hops
			result.Status = StatusFailed
			result.Err = err
			return result, false // Don't retry CNAME loops
		}
		targetDomain = finalDomain
	}

	// Query A records (IPv4)
	ipv4, errA := r.queryType(ctx, targetDomain, dns.TypeA)

	// Query AAAA records (IPv6)
	ipv6, errAAAA := r.queryType(ctx, targetDomain, dns.TypeAAAA)

	// Determine status based on results
	if len(ipv4) > 0 || len(ipv6) > 0 {
		result.IPv4 = ipv4
		result.IPv6 = ipv6
		result.Status = StatusValid
		return result, false
	}

	// Check for NXDOMAIN
	if isNXDomain(errA) || isNXDomain(errAAAA) {
		result.Status = StatusNXDomain
		result.Err = errA
		return result, false // Don't retry NXDOMAIN
	}

	// Both queries returned no IPs
	if len(ipv4) == 0 && len(ipv6) == 0 {
		// If both queries returned nil error (NOERROR with no records),
		// this is a valid domain that simply has no A/AAAA records (e.g. MX-only).
		if errA == nil && errAAAA == nil {
			result.Status = StatusNXDomain
			return result, false
		}
		// Otherwise one or both queries failed
		result.Status = StatusFailed
		if errA != nil {
			result.Err = errA
		} else {
			result.Err = errAAAA
		}
		shouldRetry := isRetryable(errA) || isRetryable(errAAAA)
		return result, shouldRetry
	}

	// Should not reach here, but handle defensively
	result.Status = StatusFailed
	return result, false
}

// calculateBackoff returns the backoff duration for the given attempt.
func (r *DNSResolver) calculateBackoff(attempt int) time.Duration {
	// Exponential: base * 2^attempt
	backoff := r.cfg.BackoffBase * time.Duration(1<<uint(attempt))

	// Cap at maximum
	if backoff > r.cfg.BackoffMax {
		backoff = r.cfg.BackoffMax
	}

	// Add jitter (±25%)
	jitter := time.Duration(float64(backoff) * 0.25 * (2*randomFloat() - 1))
	backoff += jitter

	if backoff < 0 {
		backoff = r.cfg.BackoffBase
	}

	return backoff
}

// Close releases resources.
func (r *DNSResolver) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
		log.Debug().Msg("resolver: closed")
	})
	return nil
}
