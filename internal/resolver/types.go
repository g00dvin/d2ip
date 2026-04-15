// Package resolver implements DNS resolution with worker pool, rate limiting,
// retry logic, and CNAME chain following.
package resolver

import (
	"context"
	"net/netip"
	"time"
)

// Status represents the outcome of a DNS resolution attempt.
type Status uint8

const (
	StatusValid    Status = iota // Successfully resolved to at least one IP
	StatusFailed                  // Temporary failure (SERVFAIL, timeout, network error)
	StatusNXDomain                // Domain does not exist (NXDOMAIN)
)

func (s Status) String() string {
	switch s {
	case StatusValid:
		return "valid"
	case StatusFailed:
		return "failed"
	case StatusNXDomain:
		return "nxdomain"
	default:
		return "unknown"
	}
}

// ResolveResult contains the outcome of resolving a single domain.
type ResolveResult struct {
	Domain     string        // Original domain name
	IPv4       []netip.Addr  // Resolved IPv4 addresses (A records)
	IPv6       []netip.Addr  // Resolved IPv6 addresses (AAAA records)
	Status     Status        // Resolution status
	ResolvedAt time.Time     // Timestamp of resolution
	Err        error         // Error details (if Status != StatusValid)
}

// Resolver performs DNS resolution with configurable concurrency and rate limiting.
type Resolver interface {
	// ResolveBatch resolves multiple domains concurrently.
	// Results are sent to the returned channel as they complete.
	// The channel is closed when all domains are resolved or ctx is cancelled.
	ResolveBatch(ctx context.Context, domains []string) <-chan ResolveResult

	// Close releases resources. Safe to call multiple times.
	Close() error
}

// Config holds resolver configuration.
type Config struct {
	// Upstream DNS server (e.g., "8.8.8.8:53", "1.1.1.1:53")
	Upstream string

	// Network protocol: "udp", "tcp", or "tcp-tls"
	Network string

	// Concurrency: number of worker goroutines
	Concurrency int

	// QPS: queries per second limit (shared across all workers)
	QPS int

	// Timeout: per-query timeout
	Timeout time.Duration

	// Retries: number of retry attempts for failed queries
	Retries int

	// BackoffBase: base backoff duration for retries
	BackoffBase time.Duration

	// BackoffMax: maximum backoff duration
	BackoffMax time.Duration

	// FollowCNAME: whether to follow CNAME chains
	FollowCNAME bool

	// MaxCNAMEChain: maximum CNAME hops (default 8)
	MaxCNAMEChain int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Upstream:      "8.8.8.8:53",
		Network:       "udp",
		Concurrency:   64,
		QPS:           1000,
		Timeout:       5 * time.Second,
		Retries:       2,
		BackoffBase:   100 * time.Millisecond,
		BackoffMax:    5 * time.Second,
		FollowCNAME:   true,
		MaxCNAMEChain: 8,
	}
}
