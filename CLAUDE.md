# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**d2ip** resolves curated v2fly `geosite:*` domain lists into CIDR-aggregated IPv4/IPv6 sets and optionally installs them into the host routing table or nftables sets. It is **not** a DNS server or proxy — it's designed for self-hosted policy routing (e.g., "route `geosite:google` through VPN").

**Pipeline:** fetch dlc.dat → parse protobuf → normalize IDN → DNS resolve (worker pool) → SQLite cache → CIDR aggregate → export ipv4.txt/ipv6.txt → (optional) apply to nftables/iproute2

**Status:** Production ready. All 6 iterations (0-5) complete. See [docs/PROGRESS.md](docs/PROGRESS.md) for detailed implementation status.

## Build Environment

**Go version:** 1.22+ required (local system may have 1.19, use Docker for builds)

**Build commands:**
```bash
# First-time setup: build development image (caches Go modules)
make docker-dev                         # → d2ip-dev:latest (run once, or when go.mod changes)

# Compile binary (auto-detects local Go 1.22+ or uses docker-dev)
make build                              # → bin/d2ip

# Generate protobuf code (if proto/dlc.proto changes)
make proto                              # Uses docker-dev

# Run tests (auto-detects local Go 1.22+ or uses docker-dev)
make test                               # All tests with race detector
go test ./internal/routing -v          # Single package (if local Go available)
go test ./pkg/cidr -run TestConservative  # Single test (if local Go available)

# Build production Docker image
make docker                             # → d2ip:latest

# Lint (requires golangci-lint)
make lint
```

**Docker workflow:**
- `Dockerfile.dev` pre-installs Go modules and protoc-gen-go for fast rebuilds
- `make docker-dev` builds the dev image once (or when dependencies change)
- `make build` and `make test` automatically use docker-dev if local Go < 1.22
- No more repeated downloads with `docker run --rm` — dependencies are cached in the image

## Architecture

### 9-Agent Pipeline (Sequential Execution)

The orchestrator ([internal/orchestrator/orchestrator.go](internal/orchestrator/orchestrator.go)) wires 7 agents into a single-flight pipeline:

1. **Source Agent** (`internal/source`) — Fetch dlc.dat with ETag caching, SHA256 verification, atomic writes
2. **Domain Agent** (`internal/domainlist`) — Parse protobuf, filter categories/attrs, normalize to punycode
3. **Resolver Agent** (`internal/resolver`) — DNS A/AAAA with worker pool, rate limiting, CNAME following (max 8 hops)
4. **Cache Agent** (`internal/cache`) — SQLite with internal TTL (DNS TTL ignored), batch upserts, WAL mode
5. **Aggregator** (`pkg/cidr` + `internal/aggregator`) — Radix tree CIDR aggregation with aggressiveness levels
6. **Exporter** (`internal/exporter`) — Atomic file writes (temp → rename), SHA256 digest tracking, unchanged detection
7. **Routing Agent** (`internal/routing`) — nftables sets (preferred) or iproute2 table 100, dry-run, rollback

**Cross-cutting:** Config (`internal/config`), API (`internal/api`), Scheduler (`internal/scheduler`), Logging, Metrics

### Key Patterns

**Interface-Based Isolation:** Agents communicate via Go interfaces (never import each other's concrete types). See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) §3 for full contract definitions.

**Single-Flight Enforcement:** Orchestrator uses `atomic.Bool` to ensure only one pipeline run at a time. Concurrent triggers receive `ErrBusy`.

**Config Precedence:** ENV vars > kv_settings (SQLite) > YAML file > hardcoded defaults. Hot-reload via Watcher.

**Atomic Writes:** All file operations use temp→rename pattern. Routing uses transactions (`nft -f -` or `ip -batch -`).

**State-Scoped Rollback:** Routing only removes entries it owns (tracked in `/var/lib/d2ip/state.json`), never flushes entire sets.

## Critical Implementation Notes

### Routing Agent (HIGH RISK)

**Location:** `internal/routing/`

**⚠️ Kernel Manipulation Code** — Can brick network connectivity. Always test in isolated netns first.

**Two backends:**
- **nftables** (preferred): Creates `table inet d2ip` with sets `d2ip_v4`, `d2ip_v6`. Uses `nft -f -` for atomic transactions.
- **iproute2** (fallback): Uses custom routing table (default: table 100). Requires `Iface` config.

**Safety guarantees:**
- Disabled by default (`routing.enabled=false`)
- Idempotent: second Apply with same input is no-op
- Process-wide mutex prevents concurrent mutations
- Capability self-check validates nft/ip binary availability
- All objects prefixed with `d2ip` for ownership tracking

**Testing:** Unit tests pass (18/18), but integration tests in netns with CAP_NET_ADMIN are TODO (build tag `routing_integration`).

### CIDR Aggregation Bug (FIXED)

**Location:** `pkg/cidr/tree.go`

**Past issue:** IPv4 byte offset calculation was wrong. `netip.Addr.As16()` stores IPv4 in bytes **12-15**, not 0-3. Fixed in both `insert()` and `collectPrefixes()` with `byteOffset = 12` for IPv4.

**Test coverage:** All 10 tests in `pkg/cidr/aggregate_test.go` validate lossless aggregation.

### DNS Resolver Concurrency

**Location:** `internal/resolver/resolver.go`

**Pattern:** Worker pool (N goroutines + channels) with:
- Rate limiting via `golang.org/x/time/rate.Limiter`
- Retry with exponential backoff + jitter
- CNAME chain following (max 8 hops, loop detection)
- Graceful shutdown via context cancellation

**No goleak tests yet** — this is a known TODO.

### SQLite Cache

**Location:** `internal/cache/`

**Pragmas:** `journal_mode=WAL`, `synchronous=NORMAL`, `foreign_keys=ON`, `busy_timeout=5000`

**Batch size:** Soft cap of 1000 rows per transaction.

**Schema:** Two tables (`domains`, `records`) with composite uniqueness `(domain_id, ip, type)` for idempotent upserts.

**Migrations:** Embedded in `migrations/embed.go` using `//go:embed`. Applied on `cache.Open()`.

## Critical Gotchas

### 1. IPv4 in netip.Addr.As16()

**Problem:** IPv4 addresses stored in last 4 bytes, not first 4

```go
// ❌ WRONG
addr.As16()[0:4]  // IPv4 not here!

// ✅ CORRECT
byteOffset := 0
if addr.Is4() {
    byteOffset = 12  // IPv4 in bytes 12-15
}
addr.As16()[byteOffset:byteOffset+4]
```

### 2. Orchestrator New() Signature Changes

Every new agent requires updating `New()` parameters. Pattern: all agents injected, config getter last.

```go
func New(
    src source.DLCStore,
    dl domainlist.ListProvider,
    res resolver.Resolver,
    cch cache.Cache,
    agg *aggregator.Aggregator,
    exp *exporter.FileExporter,
    rtr routing.Router,      // ← new agent added
    cfgGetter func() config.Config,  // ← always last
) *Orchestrator
```

### 3. Routing Idempotence Check

Second Apply with same input must be no-op. Verify: both `Plan.Add` and `Plan.Remove` are empty.

### 4. Context Cancellation Between Pipeline Steps

Always check `ctx.Done()` between orchestrator steps:

```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
}
```

### 5. Docker Development Workflow

Local go commands fail (Go 1.19 < required 1.22). Use development image:

```bash
# Build dev image with cached dependencies (run once)
make docker-dev

# Now builds are fast (no repeated downloads)
make build  # Uses docker-dev automatically if local Go < 1.22
make test   # Uses docker-dev automatically if local Go < 1.22

# Rebuild dev image only when go.mod/go.sum changes
make docker-dev
```

## CLI Commands

**Binary:** `bin/d2ip` or `./bin/d2ip`

```bash
# Server mode
d2ip serve --config config.example.yaml

# CLI utilities (bypass server)
d2ip dump --category cn --list-categories   # Parse dlc.dat, show categories
d2ip resolve --category google --db ./d2ip.db --qps 100
d2ip export --db ./d2ip.db --output ./out --agg conservative

# Version
d2ip version
```

## API Endpoints

**Base:** `http://localhost:8080`

```bash
# Pipeline control
POST /pipeline/run       # Trigger full pipeline (JSON body optional)
GET  /pipeline/status    # Last run status

# Routing (routing.enabled=true required)
POST /routing/dry-run    # Preview changes (JSON: {ipv4_prefixes: [...], ipv6_prefixes: [...]})
POST /routing/rollback   # Restore previous state
GET  /routing/snapshot   # Show current applied state

# Health & metrics
GET  /healthz            # Liveness probe
GET  /readyz             # Readiness probe
GET  /metrics            # Prometheus metrics
```

## Configuration

**File:** `config.example.yaml` (all sections with comments)

**Override via ENV:** Prefix with `D2IP_`, use double underscore for nesting:
```bash
export D2IP_RESOLVER_UPSTREAM=1.1.1.1:53
export D2IP_ROUTING_ENABLED=true
export D2IP_ROUTING_BACKEND=nftables
```

**Key configs:**
- `routing.enabled`: **false by default** (safe)
- `routing.backend`: `"none"`, `"nftables"`, `"iproute2"`
- `aggregation.level`: `"off"`, `"conservative"`, `"balanced"`, `"aggressive"`
- `scheduler.resolve_cycle`: Duration (e.g., `24h`), `0` = disabled

## Testing Strategy

**Unit tests:** 56 tests across packages (all pass). Run with `make test`.

**Critical tests:**
- `pkg/cidr/aggregate_test.go` — CIDR radix tree (10 tests)
- `internal/routing/*_test.go` — Plan computation, nft script builder (18 tests)
- `internal/exporter/exporter_test.go` — Atomic writes (10 tests)

**Integration tests (TODO):**
- End-to-end pipeline run (requires network)
- Routing in netns (build tag `routing_integration`, requires CAP_NET_ADMIN)

## Common Tasks

### Adding a New Config Field

1. Update struct in `internal/config/config.go`
2. Add validation in `internal/config/validate.go`
3. Update `config.example.yaml`
4. Handle in consuming agent

### Modifying the Pipeline

**Entry point:** `internal/orchestrator/orchestrator.go` → `Run()` method

**Current steps:** 9 steps (fetch → parse → check cache → resolve → upsert → aggregate → export → route)

**Adding a step:** Insert between existing steps, add context cancellation check, update `PipelineReport` struct.

### Debugging a Routing Issue

1. Enable dry-run mode: `routing.dry_run=true` in config
2. Check nft/ip binary availability: `Router.Caps()` error
3. Inspect state file: `cat /var/lib/d2ip/state.json`
4. View plan diff: `POST /routing/dry-run` endpoint
5. Test in isolated netns:
   ```bash
   sudo ip netns add d2ip-test
   sudo ip netns exec d2ip-test bash
   # ... run d2ip with routing.enabled=true
   nft list table inet d2ip
   exit
   sudo ip netns del d2ip-test
   ```

## Documentation Map

| File | Purpose |
|------|---------|
| [README.md](README.md) | Quick start, Docker usage |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Component diagram, interfaces, contracts |
| [docs/PROGRESS.md](docs/PROGRESS.md) | Implementation status, iterations 0-5 |
| [docs/AGENT_LESSONS.md](docs/AGENT_LESSONS.md) | Agent performance metrics, cost analysis |
| [docs/RETROSPECTIVE.md](docs/RETROSPECTIVE.md) | What went well, gotchas, recommendations |
| [docs/agents/](docs/agents/) | Per-agent specifications (01-09) |
| [docs/SCHEMA.md](docs/SCHEMA.md) | SQLite schema, indexes |
| [docs/CONFIG.md](docs/CONFIG.md) | Full config reference |
| [docs/API.md](docs/API.md) | HTTP API spec |

## Known Limitations

- Prometheus metrics incomplete (resolver missing `dns_resolve_total`, `dns_resolve_duration`)
- No integration tests in netns yet (build tag `routing_integration` TODO)
- iproute2 backend needs `Iface` config field addition
- nft plain-text parsing is brittle (JSON mode via `nft --json` would be better)
- No goleak tests for orchestrator/resolver
- DNS TTL is ignored (internal cache TTL only)

## Agent Usage History (for Cost Optimization Context)

**Total project:** 8 agents spawned across iterations 3-5, 252k tokens, ~$0.25 cost (58% savings vs all-opus)

**Strategy validated:**
- Use **sonnet** for well-specified implementation tasks (API handlers, config, scheduler)
- Use **opus** for HIGH RISK code (routing/kernel manipulation, complex algorithms, concurrency)
- Use **manual** for trivial tasks (<50 lines, config files)

**Example:** Iteration 5 used 1 opus (routing logic) + 1 sonnet (API endpoints) = perfect results, 73k tokens.

**When sonnet struggles:**
- Complex state tracking across recursion
- Bit manipulation (byte offsets, masks)
- Concurrency edge cases
- Algorithm design (not implementation)

**When to use opus:**
- Kernel/system integration (nftables, iproute2, netns)
- Critical concurrency logic (worker pools, rate limiters)
- Complex algorithms (radix tree, CIDR aggregation)
- After 1-2 sonnet attempts with bugs
