# d2ip — Architecture

`d2ip` resolves curated domain lists (v2fly `dlc.dat`) into deduplicated, CIDR‑aggregated
IPv4/IPv6 sets and (optionally) installs them into the host routing/firewall, on a
schedule, with a controllable HTTP API.

## 1. High‑level component diagram

```
                                  ┌───────────────────────────────┐
                                  │           HTTP API            │
                                  │      (chi, internal/api)      │
                                  └───────────────┬───────────────┘
                                                  │ commands / status
                                                  ▼
┌───────────────────┐  schedule  ┌─────────────────────────────────────┐
│    Scheduler      ├───────────►│            Orchestrator             │
│  (cron-like)      │            │  pipeline:                          │
└───────────────────┘            │   fetch→parse→normalize→resolve→    │
                                  │   cache→aggregate→export→route      │
                                  └──┬──────┬────┬──────┬─────┬─────┬──┘
                                     │      │    │      │     │     │
                                     ▼      ▼    ▼      ▼     ▼     ▼
                               ┌──────┐ ┌─────┐┌────┐┌─────┐┌────┐┌─────┐
                               │Source│ │Domn ││Resv││Cache││Aggr││Expt │
                               │Agent │ │List ││lver││ DB  ││ IP ││File │
                               └──┬───┘ └──┬──┘└─┬──┘└──┬──┘└──┬─┘└──┬──┘
                                  │        │     │      │      │     │
                               dlc.dat   GeoSite worker  SQLite radix ipv4/6.txt
                                                 pool            tree    (atomic)
                                                                        │
                                                                        ▼
                                                               ┌─────────────────┐
                                                               │  Routing Agent  │
                                                               │ nft set / iproute2
                                                               │  table 100      │
                                                               └─────────────────┘

                   cross‑cutting:  Config Agent · Logging · Metrics · Health
```

## 2. Module responsibilities

| # | Agent (Go pkg)                       | Responsibility                                                                                       |
|---|--------------------------------------|------------------------------------------------------------------------------------------------------|
| 1 | `internal/source` — Source Agent     | Download/refresh `dlc.dat`, integrity (sha256), atomic replace, ETag caching                          |
| 2 | `internal/domainlist` — Domain Agent | Parse protobuf, expand `include:`, filter by category + attribute, normalize (lowercase + punycode)   |
| 3 | `internal/resolver` — Resolver Agent | DNS A/AAAA + CNAME chain, custom upstream, worker pool, retry/backoff, rate limit                    |
| 4 | `internal/cache` — Cache Agent       | SQLite store, internal TTL only, batch upserts, transactional, idempotent                            |
| 5 | `pkg/cidr` + `internal/aggregator`   | CIDR aggregation (radix tree, fallback sort+merge), aggressiveness levels                            |
| 6 | `internal/exporter` — Export Agent   | Generate `ipv4.txt` / `ipv6.txt`, dedup, sort, atomic temp→rename                                    |
| 7 | `internal/routing` — Routing Agent   | nftables sets (preferred) or `ip route` into table `100`, dry‑run, snapshot, rollback                |
| 8 | `internal/config` — Config Agent     | ENV > Web UI overrides > defaults; live reload; validation                                           |
| 9 | `internal/orchestrator`              | Wires pipeline, owns context, fan‑out/fan‑in, run state, single‑flight per pipeline                  |
|   | `internal/api`                       | chi router; commands + status + UI                                                                   |
|   | `internal/scheduler`                 | dlc refresh + resolve cycles                                                                         |
|   | `internal/metrics`, `internal/logging` | Prometheus, zerolog                                                                                |

## 3. Inter‑agent contracts (Go interfaces)

Most cross‑module communication goes through small interfaces. The orchestrator imports
concrete types for `aggregator` and `exporter` because their APIs are stable and there
is no need for runtime polymorphism.

```go
// internal/source
type DLCStore interface {
    Get(ctx context.Context, maxAge time.Duration) (path string, version Version, err error)
    ForceRefresh(ctx context.Context) (path string, version Version, err error)
    Info() Version
}

// internal/domainlist
type Rule struct {
    Type  RuleType // Full | RootDomain | Plain | Regex
    Value string
    Attrs map[string]any
    Cat   string // origin category (diagnostics)
}
type CategorySelector struct {
    Code  string   // e.g. "ru" or "geosite:ru"
    Attrs []string // optional @attrs filter (AND)
}
type ListProvider interface {
    Load(dlcPath string) error
    Select(sel []CategorySelector) ([]Rule, error)
    Categories() []string
}

// internal/resolver
type ResolveResult struct {
    Domain     string
    IPv4       []netip.Addr
    IPv6       []netip.Addr
    Status     Status // Valid | Failed | NXDomain
    ResolvedAt time.Time
    Err        error
}
type Resolver interface {
    ResolveBatch(ctx context.Context, domains []string) <-chan ResolveResult
    Close() error
}

// internal/cache
type Cache interface {
    NeedsRefresh(ctx context.Context, domains []string, ttl, failedTTL time.Duration) ([]string, error)
    UpsertBatch(ctx context.Context, results []ResolveResult) error
    Snapshot(ctx context.Context) (ipv4 []netip.Addr, ipv6 []netip.Addr, err error)
    Stats(ctx context.Context) (Stats, error)
    Vacuum(ctx context.Context, olderThan time.Duration) (deleted int, err error)
    Close() error
}

// internal/aggregator — concrete type (no interface)
type Aggregator struct{}
func (a *Aggregator) AggregateV4(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix
func (a *Aggregator) AggregateV6(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix

// internal/exporter — concrete type (no interface)
type FileExporter struct{ ... }
func (e *FileExporter) Write(ctx context.Context, ipv4 []netip.Prefix, ipv6 []netip.Prefix) (ExportReport, error)

// internal/routing
type Plan struct {
    Family Family
    Add    []netip.Prefix
    Remove []netip.Prefix
}
type Router interface {
    Caps() error
    Plan(ctx context.Context, desired []netip.Prefix, family Family) (Plan, error)
    Apply(ctx context.Context, p Plan) error
    Snapshot() RouterState
    Rollback(ctx context.Context) error
    DryRun(ctx context.Context, desired []netip.Prefix, f Family) (Plan, string, error)
}
```

The orchestrator's pipeline shape:

```go
func (o *Orchestrator) Run(ctx context.Context, req PipelineRequest) (PipelineReport, error)
```

`PipelineRequest` fields: `DryRun bool`, `ForceResolve bool`, `SkipRouting bool`.

Channels carry `ResolveResult` between Resolver → Cache writer; everything else is
slice‑based for simpler back‑pressure semantics.

## 4. Data model (SQLite)

See [SCHEMA.md](SCHEMA.md). Highlights:

* `domains` (id, name UNIQUE, last_resolved_at, resolve_status) — canonical punycode form.
* `records` (id, domain_id, ip, type, updated_at, status) — 1:N with `domains`.
* Composite uniqueness `(domain_id, ip, type)` to make upserts idempotent.
* PRAGMA: `journal_mode=WAL`, `synchronous=NORMAL`, `foreign_keys=ON`, `busy_timeout=5000`.
* Batch writes inside a single transaction; ~1k upserts per tx as a soft budget.

## 5. Pipeline

```
fetch(dlc.dat) → parse(protobuf) → expand(include) → filter(category, @attrs)
→ normalize(lowercase, punycode, dedup)
→ filter resolvable (Full + RootDomain only; Plain/Regex skipped)
→ resolve(worker pool, A+AAAA+CNAME, retry/backoff, rate‑limited)
→ cache(SQLite upsert, internal TTL only — DNS TTL ignored)
→ snapshot(SELECT all valid IPs, by family)
→ aggregate(radix tree CIDR merge per family + aggressiveness)
→ export(ipv4.txt, ipv6.txt — atomic)
→ route(cap check → plan v4 → plan v6 → apply v4 → apply v6)
```

A run is single‑flight; a second trigger while one is in flight returns `409 Busy`
with the in-flight `run_id`.

## 6. Concurrency model

* Worker pool of `N = config.resolver.concurrency` goroutines, fed by an unbuffered
  `chan string` from the dispatcher.
* Each worker owns a `*dns.Client` reused across requests; outbound rate limited by a
  shared `golang.org/x/time/rate.Limiter` (`config.resolver.qps`).
* Results pushed onto a single `chan ResolveResult`; one writer goroutine drains it
  into batched SQLite transactions (≤1k rows per tx).
* Context cancel propagates everywhere.

## 7. Routing isolation

* **Multi-policy support**: each policy maps categories to an independent routing
  backend (nftables set or iproute2 table). Policies are isolated — a failure in
  one does not affect others.
* **nftables backend**: per-policy named sets (`{policy_name}_v4`, `{policy_name}_v6`,
  type `ipv4_addr` / `ipv6_addr`, flag `interval`). d2ip only manages set contents
  (flush + add element); the operator writes `nft` rules referencing these sets.
* **iproute2 backend**: per-policy routing table (table_id configurable per policy).
  d2ip manages routes only; the operator creates `ip rule` entries with custom
  5-tuple criteria to direct traffic to the policy's table.
* **Marker**: every managed object encodes `d2ip` in its name/comment.
* **State files**: `/var/lib/d2ip/state/{policy_name}.json` — last applied prefixes
  per family per policy.
* **Dry run**: `PipelineRequest.dry_run` and `policy.dry_run` compute the plan and
  diff without applying.
* **Rollback**: per-policy rollback removes only entries previously applied for that
  policy (set difference with per-policy state).

## 8. Configuration

ENV > Web overrides (persisted in SQLite `kv_settings`) > defaults. See
[CONFIG.md](CONFIG.md). Hot‑reload for non‑listener fields; listener changes need a restart.

## 9. Failure model

* DNS failures degrade gracefully — failed entries marked `status='failed'` with a
  short retry TTL, never block the rest of the batch.
* Source download failures fall back to the last good `dlc.dat` on disk.
* SQLite is the source of truth for *what to export*; aggregation/export are pure
  functions over its snapshot.
* Routing is the only step with external side effects — gated behind `routing.enabled`
  and always preceded by a capability check and recorded plan.

## 10. Observability

* `zerolog` JSON logs with `run_id`, `agent`, `domain` fields.
* Prometheus on `/metrics`:
  `d2ip_pipeline_duration_seconds`, `d2ip_resolve_total{status}`,
  `d2ip_cache_hits_total`, `d2ip_aggregate_input/output`,
  `d2ip_route_apply_total{op}`.
* `/healthz` (process), `/readyz` (DB + last successful run age — stubbed).
