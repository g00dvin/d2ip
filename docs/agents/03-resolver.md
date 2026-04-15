# Agent 03 — Resolver Agent

**Package:** `internal/resolver`, helpers in `pkg/dnsx`
**Owns:** DNS A/AAAA + CNAME chain, worker pool, retry, rate limiting.

## Contract

```go
type Status uint8
const (
    StatusValid    Status = iota
    StatusFailed
    StatusNXDomain
)

type ResolveResult struct {
    Domain     string
    IPv4       []netip.Addr
    IPv6       []netip.Addr
    Status     Status
    ResolvedAt time.Time
    Err        error
}

type Resolver interface {
    ResolveBatch(ctx context.Context, domains []string) <-chan ResolveResult
    Close() error
}
```

## Tasks

1. **Implementation** with `github.com/miekg/dns`. One reusable `*dns.Client` per
   worker, configurable `network` (`udp`/`tcp`/`tcp-tls`).
2. **Worker pool**: `N = config.resolver.concurrency`, fed by `chan string`,
   single output channel. Buffered output channel of size `N` to absorb bursts.
3. **CNAME chain**: when `follow_cname=true`, follow up to 8 hops; always issue
   both A and AAAA queries against the *terminal* name.
4. **Rate limiting**: shared `*rate.Limiter` from `golang.org/x/time/rate`,
   `qps` tokens/sec, burst = `qps`.
5. **Retry / backoff**: on `SERVFAIL`, network timeout, or `i/o timeout`,
   retry up to `retries` times with `backoff_base * 2^attempt + jitter`,
   capped at `backoff_max`. NXDOMAIN does **not** retry.
6. **Per-query timeout** = `resolver.timeout`. Whole batch respects parent ctx.
7. **NXDOMAIN/SERVFAIL handling**: distinct `Status` so cache layer can apply
   `failed_ttl` instead of `ttl`.
8. **Backpressure**: workers select on ctx.Done so a slow consumer pauses
   issuance — never silently drop.
9. **Metrics**: `d2ip_resolve_total{status}`, `d2ip_resolve_latency_seconds`,
   `d2ip_resolve_retries_total`.
10. **Tests**: stub upstream via `dns.Server` listening on `127.0.0.1:0`.
    Cases: A only, AAAA only, both, NXDOMAIN, SERVFAIL→retry→ok, timeout,
    CNAME chain, CNAME loop (must terminate).

## Acceptance

* Resolving 10k mixed domains stays under configured `qps`.
* No goroutine leaks under cancel (verified with `goleak`).
* CNAME loop terminates with `Failed` after 8 hops.
