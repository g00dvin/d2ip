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
docker run --rm -p 8080:8080 \
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
curl -X POST http://localhost:8080/pipeline/run

# Dry-run routing changes
curl -X POST http://localhost:8080/pipeline/dry-run | jq
```

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
| [docs/agents/](docs/agents/)                     | One task spec per agent (01–09)                             |

## Per-agent specs

1. [Source Agent](docs/agents/01-source.md) — dlc.dat fetch + integrity
2. [Domain Agent](docs/agents/02-domainlist.md) — protobuf parse, filter, IDN
3. [Resolver Agent](docs/agents/03-resolver.md) — DNS worker pool
4. [Cache Agent](docs/agents/04-cache.md) — SQLite, internal TTL only
5. [Aggregation Agent](docs/agents/05-aggregator.md) — CIDR radix-tree merge
6. [Export Agent](docs/agents/06-exporter.md) — atomic ipv4/ipv6 files
7. [Routing Agent](docs/agents/07-routing.md) — nft set / table 100, dry-run, rollback
8. [Config Agent](docs/agents/08-config.md) — ENV > Web > defaults
9. [Orchestrator](docs/agents/09-orchestrator.md) — pipeline & single-flight

## Status

Design complete; implementation tracked in [docs/PLAN.md](docs/PLAN.md).
