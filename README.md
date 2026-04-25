# d2ip

Resolve curated domain and IP lists from multiple sources into deduplicated, CIDR-aggregated
IPv4/IPv6 sets and (optionally) install them into the host routing table or
nftables sets — on a schedule, with an HTTP API.

Supports **5 source types**: v2fly geosite (domains), v2fly geoip (IP prefixes), IPverse (country blocks),
MaxMind MMDB (GeoIP2), and plaintext files (domains or IPs). Categories are namespaced by source prefix
(e.g. `geosite:ru`, `ipverse:us`, `mmdb:de`).

> Intended for self-hosted policy routing (e.g. "send `geosite:google` over my
> VPN"). Not a DNS server, not a proxy.

## What it does

```
Sources (v2fly geosite, geoip, IPverse, MMDB, plaintext)
  ↓
Registry loads/parses each source → categories with prefixes
  ↓
Pipeline resolves domains (DNS) + collects IP prefixes directly
  ↓
SQLite cache (resolved IPs, internal TTL — DNS TTL ignored)
  ↓
CIDR aggregate (radix tree) per policy
  ↓
atomic ipv4.txt / ipv6.txt per policy
  ↓
(optional) nftables set or `ip route` table per policy
```

## Quick start

### From GHCR (recommended)

```bash
# Run without routing — just produce ipv4.txt/ipv6.txt
docker run --rm -p 9099:9099 \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_RESOLVER_UPSTREAM=1.1.1.1:53 \
    ghcr.io/g00dvin/d2ip:latest

# Run with iproute2 routing (host routing table)
# NOTE: --network=host and --cap-add=NET_ADMIN are REQUIRED for routing
docker run --rm -d --name d2ip \
    --network=host \
    --cap-add=NET_ADMIN \
    --cap-add=NET_RAW \
    -v d2ip-data:/var/lib/d2ip \
    -v d2ip-data/config.yaml:/app/config.yaml \
    -e D2IP_ROUTING_ENABLED=true \
    -e D2IP_ROUTING_BACKEND=iproute2 \
    ghcr.io/g00dvin/d2ip:latest

# Run with nftables routing (preferred, namespaced set)
docker run --rm -d --name d2ip \
    --network=host \
    --cap-add=NET_ADMIN \
    --cap-add=NET_RAW \
    -v d2ip-data:/var/lib/d2ip \
    -e D2IP_ROUTING_ENABLED=true \
    -e D2IP_ROUTING_BACKEND=nftables \
    ghcr.io/g00dvin/d2ip:latest
```

> **Important:** If `routing.enabled: true` in your config, you MUST use `--network=host`
> so the container can see the host's network interfaces (e.g. `enp2s0`). Without this,
> routing will fail with "Cannot find device" and the pipeline will return HTTP 500.
> `--cap-add=NET_ADMIN` is required for modifying routing tables / nftables sets.

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
  - **Sources** — manage multi-source registry (add/remove/refresh sources)
  - **Categories** — browse categories grouped by source prefix, search/filter domains
  - **Cache** — statistics, SQLite vacuum
  - **Policies** — multi-policy routing configuration
  - **Routing** — dry-run, rollback, snapshot view
- **Tech:** Vue 3 + Tailwind CSS SPA, embedded in binary (~480KB gzipped)

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

## Troubleshooting

### Pipeline returns HTTP 500

**Symptom:** `POST /pipeline/run` returns 500 with "routing apply v4: Cannot find device"

**Cause:** The container cannot see the host network interface (e.g. `enp2s0`).

**Fix:** Run with `--network=host --cap-add=NET_ADMIN`:
```bash
docker run -d --name d2ip \
    --network=host --cap-add=NET_ADMIN \
    -v d2ip-data:/var/lib/d2ip \
    -v d2ip-data/config.yaml:/app/config.yaml \
    ghcr.io/g00dvin/d2ip:latest
```

### Category browse returns 404

**Symptom:** `GET /api/categories/geosite:google/domains` returns 404

**Cause:** Using an older image before the domain status migration was added.

**Fix:** Pull the latest image and delete the old cache.db:
```bash
docker pull ghcr.io/g00dvin/d2ip:latest
sudo rm -f d2ip-data/cache.db
```

### Config save fails with negative duration

**Symptom:** "config reload failed: cache.ttl: must be >= 1m, got -315919h..."

**Cause:** A bug in older versions where the web UI sent raw nanoseconds.

**Fix:** Update to v0.1.13+ where durations are formatted as human-readable strings.

### No routes in routing table

**Symptom:** `ip route show table 101` is empty after pipeline run

**Cause:** Container lacks network access to host interfaces.

**Fix:** Use `--network=host` so the container shares the host's network namespace.

## Status

Production-ready. All pipeline stages implemented with single-flight enforcement,
hot-reload config, and nftables/iproute2 routing backends.

**Note on Race Detector:** This project builds with `CGO_ENABLED=0` for static
binaries (using `modernc.org/sqlite`). The Go race detector requires CGO, so
`go test -race` is incompatible. We use
[goleak](https://github.com/uber-go/goleak) tests instead to detect goroutine
leaks in concurrent code.
