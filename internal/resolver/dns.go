package resolver

import (
	"context"
	"fmt"
	"math/rand"
	"net/netip"
	"strings"
	"time"

	"github.com/goodvin/d2ip/internal/metrics"
	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// queryType performs a DNS query for the specified record type (A or AAAA).
func (r *DNSResolver) queryType(ctx context.Context, domain string, qtype uint16) ([]netip.Addr, error) {
	start := time.Now()
	defer func() {
		metrics.DNSResolveDuration.Observe(time.Since(start).Seconds())
	}()

	// Ensure domain ends with dot for absolute queries
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	// Create DNS message
	msg := new(dns.Msg)
	msg.SetQuestion(domain, qtype)
	msg.RecursionDesired = true

	// Exchange with timeout from context
	respCh := make(chan *dns.Msg, 1)
	errCh := make(chan error, 1)

	go func() {
		resp, _, err := r.client.Exchange(msg, r.cfg.Upstream)
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	// Wait for response or context cancellation
	var resp *dns.Msg
	select {
	case <-ctx.Done():
		metrics.DNSResolveTotal.WithLabelValues("failed").Inc()
		return nil, ctx.Err()
	case err := <-errCh:
		metrics.DNSResolveTotal.WithLabelValues("failed").Inc()
		return nil, err
	case resp = <-respCh:
	}

	// Check response code
	if resp.Rcode == dns.RcodeNameError {
		metrics.DNSResolveTotal.WithLabelValues("nxdomain").Inc()
		return nil, &DNSError{Rcode: resp.Rcode, Message: "NXDOMAIN"}
	}
	if resp.Rcode == dns.RcodeServerFailure {
		metrics.DNSResolveTotal.WithLabelValues("failed").Inc()
		return nil, &DNSError{Rcode: resp.Rcode, Message: "SERVFAIL"}
	}
	if resp.Rcode != dns.RcodeSuccess {
		metrics.DNSResolveTotal.WithLabelValues("failed").Inc()
		return nil, &DNSError{Rcode: resp.Rcode, Message: dns.RcodeToString[resp.Rcode]}
	}

	// Parse IP addresses from answer section
	var addrs []netip.Addr
	for _, rr := range resp.Answer {
		switch qtype {
		case dns.TypeA:
			if a, ok := rr.(*dns.A); ok {
				if addr, err := netip.ParseAddr(a.A.String()); err == nil {
					addrs = append(addrs, addr)
				}
			}
		case dns.TypeAAAA:
			if aaaa, ok := rr.(*dns.AAAA); ok {
				if addr, err := netip.ParseAddr(aaaa.AAAA.String()); err == nil {
					addrs = append(addrs, addr)
				}
			}
		}
	}

	if len(addrs) == 0 {
		// No records found (but not NXDOMAIN) - still count as success
		metrics.DNSResolveTotal.WithLabelValues("success").Inc()
		return nil, nil
	}

	metrics.DNSResolveTotal.WithLabelValues("success").Inc()
	return addrs, nil
}

// followCNAME follows CNAME chains up to MaxCNAMEChain hops.
// Returns the final canonical name or an error if the chain is too long or loops.
func (r *DNSResolver) followCNAME(ctx context.Context, domain string) (string, error) {
	if !r.cfg.FollowCNAME {
		return domain, nil
	}

	visited := make(map[string]bool)
	current := domain

	for i := 0; i < r.cfg.MaxCNAMEChain; i++ {
		// Detect loops
		if visited[current] {
			return "", fmt.Errorf("CNAME loop detected at %s", current)
		}
		visited[current] = true

		// Query CNAME record
		cname, err := r.queryCNAME(ctx, current)
		if err != nil {
			// No CNAME or error - return current domain
			return current, nil
		}

		if cname == "" || cname == current {
			// No more CNAMEs
			return current, nil
		}

		log.Debug().
			Str("from", current).
			Str("to", cname).
			Msg("resolver: following CNAME")

		current = cname
	}

	// Too many hops
	return "", fmt.Errorf("CNAME chain too long (>%d hops)", r.cfg.MaxCNAMEChain)
}

// queryCNAME queries the CNAME record for a domain.
// Returns empty string if no CNAME exists.
func (r *DNSResolver) queryCNAME(ctx context.Context, domain string) (string, error) {
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	msg := new(dns.Msg)
	msg.SetQuestion(domain, dns.TypeCNAME)
	msg.RecursionDesired = true

	resp, _, err := r.client.Exchange(msg, r.cfg.Upstream)
	if err != nil {
		return "", err
	}

	if resp.Rcode != dns.RcodeSuccess {
		return "", nil
	}

	// Extract CNAME from answer
	for _, rr := range resp.Answer {
		if cname, ok := rr.(*dns.CNAME); ok {
			target := strings.TrimSuffix(cname.Target, ".")
			return target, nil
		}
	}

	return "", nil
}

// DNSError represents a DNS-specific error with response code.
type DNSError struct {
	Rcode   int
	Message string
}

func (e *DNSError) Error() string {
	return fmt.Sprintf("DNS error: %s (rcode=%d)", e.Message, e.Rcode)
}

// isNXDomain checks if an error represents NXDOMAIN.
func isNXDomain(err error) bool {
	if err == nil {
		return false
	}
	dnsErr, ok := err.(*DNSError)
	return ok && dnsErr.Rcode == dns.RcodeNameError
}

// isRetryable checks if an error should trigger a retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Retry on SERVFAIL
	if dnsErr, ok := err.(*DNSError); ok {
		return dnsErr.Rcode == dns.RcodeServerFailure
	}

	// Retry on network timeouts and I/O errors
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o") ||
		strings.Contains(errStr, "connection refused")
}

// randomFloat returns a random float in [0, 1).
func randomFloat() float64 {
	return rand.Float64()
}
