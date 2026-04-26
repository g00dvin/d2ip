package resolver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/goleak"
)

// startMockDNSServer starts a local UDP DNS server with the given handler.
// Returns the server address, a shutdown function, and any error.
func startMockDNSServer(handler dns.HandlerFunc) (string, func(), error) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	addr := pc.LocalAddr().String()
	server := &dns.Server{PacketConn: pc, Handler: handler}
	go func() {
		_ = server.ActivateAndServe()
	}()
	// Give server time to start
	time.Sleep(20 * time.Millisecond)
	shutdown := func() {
		_ = server.Shutdown()
		pc.Close()
		// Allow goroutine to exit before goleak checks
		time.Sleep(30 * time.Millisecond)
	}
	return addr, shutdown, nil
}

func TestQueryType_SuccessA(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("192.0.2.1"),
		})
		_ = w.WriteMsg(m)
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

	addrs, err := res.queryType(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 1 {
		t.Fatalf("expected 1 address, got %d", len(addrs))
	}
	if addrs[0].String() != "192.0.2.1" {
		t.Errorf("expected 192.0.2.1, got %s", addrs[0].String())
	}
}

func TestQueryType_SuccessAAAA(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
			AAAA: net.ParseIP("2001:db8::1"),
		})
		_ = _ = w.WriteMsg(m)
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

	addrs, err := res.queryType(context.Background(), "example.com", dns.TypeAAAA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addrs) != 1 {
		t.Fatalf("expected 1 address, got %d", len(addrs))
	}
	if addrs[0].String() != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", addrs[0].String())
	}
}

func TestQueryType_NXDOMAIN(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeNameError
		_ = _ = w.WriteMsg(m)
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

	_, err = res.queryType(context.Background(), "example.com", dns.TypeA)
	if err == nil {
		t.Fatal("expected error for NXDOMAIN")
	}
	if !isNXDomain(err) {
		t.Errorf("expected NXDOMAIN error, got %v", err)
	}
}

func TestQueryType_SERVFAIL(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		_ = w.WriteMsg(m)
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

	_, err = res.queryType(context.Background(), "example.com", dns.TypeA)
	if err == nil {
		t.Fatal("expected error for SERVFAIL")
	}
	if !isRetryable(err) {
		t.Errorf("expected retryable error, got %v", err)
	}
}

func TestQueryType_GenericRcode(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeRefused
		_ = _ = w.WriteMsg(m)
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

	_, err = res.queryType(context.Background(), "example.com", dns.TypeA)
	if err == nil {
		t.Fatal("expected error for REFUSED")
	}
	dnsErr, ok := err.(*DNSError)
	if !ok || dnsErr.Rcode != dns.RcodeRefused {
		t.Errorf("expected DNSError with REFUSED, got %v", err)
	}
}

func TestQueryType_EmptyAnswer(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		_ = w.WriteMsg(m)
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

	addrs, err := res.queryType(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addrs != nil {
		t.Errorf("expected nil addresses for empty answer, got %v", addrs)
	}
}

func TestQueryType_ContextCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		time.Sleep(5 * time.Second)
		m := new(dns.Msg)
		m.SetReply(r)
		_ = _ = w.WriteMsg(m)
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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = res.queryType(ctx, "example.com", dns.TypeA)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestQueryCNAME_Success(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer, &dns.CNAME{
			Hdr:    dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: "target.example.com.",
		})
		_ = w.WriteMsg(m)
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
		FollowCNAME: true,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	cname, err := res.queryCNAME(context.Background(), "alias.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cname != "target.example.com" {
		t.Errorf("expected target.example.com, got %s", cname)
	}
}

func TestQueryCNAME_NoRecord(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		_ = w.WriteMsg(m)
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
		FollowCNAME: true,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	cname, err := res.queryCNAME(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cname != "" {
		t.Errorf("expected empty cname, got %s", cname)
	}
}

func TestQueryCNAME_ErrorRcode(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		_ = w.WriteMsg(m)
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
		FollowCNAME: true,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	cname, err := res.queryCNAME(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cname != "" {
		t.Errorf("expected empty cname for error rcode, got %s", cname)
	}
}

func TestFollowCNAME_Disabled(t *testing.T) {
	defer goleak.VerifyNone(t)

	res, err := New(Config{
		Upstream:      "127.0.0.1:53",
		Network:       "udp",
		Timeout:       time.Second,
		Concurrency:   1,
		QPS:           1000,
		FollowCNAME:   false,
		MaxCNAMEChain: 8,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	result, err := res.followCNAME(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "example.com" {
		t.Errorf("expected example.com, got %s", result)
	}
}

func TestFollowCNAME_NormalChain(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if r.Question[0].Name == "alias.example.com." {
			m.Answer = append(m.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: "alias.example.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
				Target: "target.example.com.",
			})
		}
		_ = w.WriteMsg(m)
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

	result, err := res.followCNAME(context.Background(), "alias.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "target.example.com" {
		t.Errorf("expected target.example.com, got %s", result)
	}
}

func TestFollowCNAME_Loop(t *testing.T) {
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
		_ = w.WriteMsg(m)
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

	_, err = res.followCNAME(context.Background(), "a.example.com")
	if err == nil {
		t.Fatal("expected error for CNAME loop")
	}
	if !strings.Contains(err.Error(), "CNAME loop") {
		t.Errorf("expected CNAME loop error, got %v", err)
	}
}

func TestFollowCNAME_TooLong(t *testing.T) {
	defer goleak.VerifyNone(t)

	handler := func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		// Always return a new CNAME target to extend the chain
		m.Answer = append(m.Answer, &dns.CNAME{
			Hdr:    dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 300},
			Target: fmt.Sprintf("hop-%d.example.com.", time.Now().UnixNano()),
		})
		_ = w.WriteMsg(m)
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
		MaxCNAMEChain: 2,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	_, err = res.followCNAME(context.Background(), "start.example.com")
	if err == nil {
		t.Fatal("expected error for CNAME chain too long")
	}
	if !errors.Is(err, fmt.Errorf("CNAME chain too long (>2 hops)")) && err.Error() != "CNAME chain too long (>2 hops)" {
		t.Errorf("expected chain too long error, got %v", err)
	}
}

func TestFollowCNAME_NetworkError(t *testing.T) {
	defer goleak.VerifyNone(t)

	res, err := New(Config{
		Upstream:      "127.0.0.1:1", // No server listening
		Network:       "udp",
		Timeout:       100 * time.Millisecond,
		Concurrency:   1,
		QPS:           1000,
		FollowCNAME:   true,
		MaxCNAMEChain: 8,
	})
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}
	defer res.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := res.followCNAME(ctx, "example.com")
	if err != nil {
		t.Fatalf("expected no error on network failure, got %v", err)
	}
	if result != "example.com" {
		t.Errorf("expected example.com, got %s", result)
	}
}

func TestIsRetryable_AdditionalCases(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"i/o timeout is retryable", errors.New("read udp: i/o timeout"), true},
		{"connection refused is retryable", errors.New("connection refused"), true},
		{"timeout string is retryable", errors.New("network timeout"), true},
		{"random error not retryable", errors.New("something went wrong"), false},
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
