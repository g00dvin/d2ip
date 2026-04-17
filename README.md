# d2ip

Resolve curated v2fly `geosite:*` domain lists into deduplicated, CIDR-aggregated
IPv4/IPv6 sets and (optionally) install them into the host routing table or
nftables sets — on a schedule, with an HTTP API.

> Intended for self-hosted policy routing (e.g. "send `geosite:google` over my
> VPN"). Not a DNS server, not a proxy.

## What it does

```
fetch dlc.dat → parse → filter (categories + @attrs) → normalize (idna)
→ DNS resolve (worker pool, custom upstream)
→ SQLite cache (internal TTL only — DNS TTL ignored)
→ CIDR aggregate (radix tree)
→ atomic ipv4.txt / ipv6.txt
→ (optional) nftables set or `ip route` table 100
```

## Quick start

```bash
# Build
docker build -t d2ip -f deploy/Dockerfile .

# Run without routing — just produce ipv4.txt/ipv6.txt
docker run --rm -p 9099:9099 \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_RESOLVER_UPSTREAM=1.1.1.1:53 \
    d2ip

# Run with nftables routing (preferred, namespaced set)
docker run --rm -d --name d2ip \
    --cap-add=NET_ADMIN --network=host \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_ROUTING_ENABLED=true \
    -e D2IP_ROUTING_BACKEND=nftables \
    d2ip

# Trigger a run
curl -X POST http://localhost:9099/pipeline/run

# Dry-run routing changes
curl -X POST http://localhost:9099/pipeline/dry-run | jq
```

## Web UI

d2ip includes a minimal, mobile-friendly web interface built with HTMX:

- **Access:** http://localhost:9099/
- **Features:**
  - Real-time health status indicator
  - Trigger pipeline runs with one click
  - Auto-refreshing status display (5s interval)
  - Routing dry-run and rollback controls
  - Current routing snapshot (IPv4/IPv6 prefix counts)
  - Direct link to Prometheus metrics

**No external dependencies** — UI is embedded in the binary using Go's `embed` package. Total size: 24KB.

## Documentation

| Doc                                              | What's in it                                                |
|--------------------------------------------------|-------------------------------------------------------------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)     | Components, contracts, concurrency, isolation               |
| [docs/PIPELINE.md](docs/PIPELINE.md)             | End-to-end pipeline diagram & guarantees                    |
| [docs/SCHEMA.md](docs/SCHEMA.md)                 | SQLite schema, indexes, upsert patterns                     |
| [docs/CONFIG.md](docs/CONFIG.md)                 | Full configuration reference + precedence                   |
| [docs/API.md](docs/API.md)                       | HTTP API surface                                            |
| [docs/PACKAGES.md](docs/PACKAGES.md)             | Go package layout                                           |
| [docs/PLAN.md](docs/PLAN.md)                     | Iteration-by-iteration build plan                           |
| [docs/RISKS.md](docs/RISKS.md)                   | Risks, bottlenecks, operational guardrails                  |
| [docs/TECHNICAL_DEBT.md](docs/TECHNICAL_DEBT.md) | Known issues, missing features, future improvements         |
| [docs/TECHNICAL_DEBT_EXECUTION_PLAN.md](docs/TECHNICAL_DEBT_EXECUTION_PLAN.md) | Agent execution plan for debt resolution |
| [docs/agents/](docs/agents/)                     | One task spec per agent (01–14)                             |

## Per-agent specs

**Pipeline Agents (Iterations 0-6):**
1. [Source Agent](docs/agents/01-source.md) — dlc.dat fetch + integrity
2. [Domain Agent](docs/agents/02-domainlist.md) — protobuf parse, filter, IDN
3. [Resolver Agent](docs/agents/03-resolver.md) — DNS worker pool
4. [Cache Agent](docs/agents/04-cache.md) — SQLite, internal TTL only
5. [Aggregation Agent](docs/agents/05-aggregator.md) — CIDR radix-tree merge
6. [Export Agent](docs/agents/06-exporter.md) — atomic ipv4/ipv6 files
7. [Routing Agent](docs/agents/07-routing.md) — nft set / table 100, dry-run, rollback
8. [Config Agent](docs/agents/08-config.md) — ENV > Web > defaults
9. [Orchestrator](docs/agents/09-orchestrator.md) — pipeline & single-flight

**Technical Debt Resolution Agents (Iterations 7-9):**
10. [Config Tests Fix](docs/agents/10-config-tests-fix.md) — Fix failing config tests
11. [nftables JSON Parsing](docs/agents/11-nftables-json.md) — Replace plain-text with JSON
12. [Web UI Config Editing](docs/agents/12-web-ui-config-editing.md) — Runtime config editing + auth
13. [Incremental Resolver](docs/agents/13-incremental-resolver.md) — Partial re-resolution
14. [Property-Based Testing](docs/agents/14-property-based-testing.md) — CIDR aggregator PBT

## Status

Design complete; implementation tracked in [docs/PLAN.md](docs/PLAN.md).

**Note on Race Detector:** This project builds with `CGO_ENABLED=0` for static binaries (using `modernc.org/sqlite`). The Go race detector requires CGO, so `go test -race` is incompatible. We use [goleak](https://github.com/uber-go/goleak) tests instead to detect goroutine leaks in concurrent code. See [docs/TECHNICAL_DEBT.md](docs/TECHNICAL_DEBT.md#2-race-detector-incompatible) for details.
