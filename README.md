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

### From GHCR (recommended)

```bash
# Run without routing — just produce ipv4.txt/ipv6.txt
docker run --rm -p 9099:9099 \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_RESOLVER_UPSTREAM=1.1.1.1:53 \
    ghcr.io/g00dvin/d2ip:latest

# Run with nftables routing (preferred, namespaced set)
docker run --rm -d --name d2ip \
    --cap-add=NET_ADMIN --network=host \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_ROUTING_ENABLED=true \
    -e D2IP_ROUTING_BACKEND=nftables \
    ghcr.io/g00dvin/d2ip:latest
```

### Build from source

```bash
docker build -t d2ip -f deploy/Dockerfile .
```

### Usage

```bash
# Trigger a run
curl -X POST http://localhost:9099/pipeline/run

# Check pipeline status
curl http://localhost:9099/pipeline/status | jq

# Dry-run routing changes
curl -X POST http://localhost:9099/routing/dry-run | jq
```

## Web UI

d2ip includes a terminal-inspired web interface with sidebar navigation:

- **Access:** http://localhost:9099/
- **Sections:**
  - **Dashboard** — health status, quick actions, last run summary, routing state
  - **Pipeline** — trigger, cancel, run history table
  - **Config** — view/edit all config fields with hot-reload
  - **Categories** — browse geosite categories, search/filter domains
  - **Cache** — statistics, SQLite vacuum
  - **Source** — dlc.dat metadata (SHA256, size, ETag)
  - **Routing** — dry-run, rollback, snapshot view
- **Tech:** Vue 3 + Tailwind CSS SPA, embedded in binary (~174KB)

## Configuration

Resolution order (highest wins): ENV (`D2IP_*`) > kv_settings overrides > YAML config > defaults.

See [docs/CONFIG.md](docs/CONFIG.md) for the full reference.

## Documentation

| Doc | What's in it |
|-----|-------------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Components, contracts, concurrency, isolation |
| [docs/PIPELINE.md](docs/PIPELINE.md) | End-to-end pipeline diagram & guarantees |
| [docs/SCHEMA.md](docs/SCHEMA.md) | SQLite schema, indexes, upsert patterns |
| [docs/CONFIG.md](docs/CONFIG.md) | Full configuration reference + precedence |
| [docs/API.md](docs/API.md) | HTTP API surface |
| [docs/WEB_UI.md](docs/WEB_UI.md) | Web UI documentation |

## Status

Production-ready. All pipeline stages implemented with single-flight enforcement,
hot-reload config, and nftables/iproute2 routing backends.

**Note on Race Detector:** This project builds with `CGO_ENABLED=0` for static
binaries (using `modernc.org/sqlite`). The Go race detector requires CGO, so
`go test -race` is incompatible. We use
[goleak](https://github.com/uber-go/goleak) tests instead to detect goroutine
leaks in concurrent code.
