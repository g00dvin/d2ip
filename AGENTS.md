# AGENTS.md

Quick-reference for agents working in this repo. See `CLAUDE.md` for full details.

## Build & Test Commands

```bash
make docker-dev           # Required first: builds dev image with cached deps. Re-run when go.mod/go.sum change.
make build                # → bin/d2ip (auto-detects local Go 1.22+ or uses docker-dev)
make test                 # All tests (auto-detects local Go or uses docker-dev)
make proto                # Regenerate protobuf (runs inside docker-dev)
make lint                 # golangci-lint (falls back to go vet)
make docker                # Production image
```

**Local Go is 1.19** — `go test`/`go build` directly will fail. Always use `make build` and `make test`, which auto-fallback to docker-dev.

### Targeted test commands (require local Go 1.22+)

```bash
go test ./internal/routing -v
go test ./pkg/cidr -run TestConservative
sudo -E go test -tags=routing_integration ./internal/routing   # needs CAP_NET_ADMIN
```

## Architecture

**Entry point:** `cmd/d2ip/main.go` → orchestrator in `internal/orchestrator/orchestrator.go`

**9-step sequential pipeline:** fetch dlc.dat → parse protobuf → normalize → check cache → DNS resolve → upsert cache → CIDR aggregate → export files → apply routing

**Package layout:**
- `internal/source` — fetch & verify dlc.dat
- `internal/domainlist` — protobuf parse, IDN normalize
- `internal/resolver` — DNS A/AAAA worker pool with rate limiting
- `internal/cache` — SQLite (WAL mode), embedded migrations in `migrations/`
- `pkg/cidr` + `internal/aggregator` — radix tree CIDR aggregation
- `internal/exporter` — atomic file writes (temp→rename)
- `internal/routing` — nftables or iproute2 backends (disabled by default)
- `internal/config` — ENV > kv_settings > YAML > defaults
- `internal/api` — chi HTTP server on :9099
- `internal/scheduler` — cron-based pipeline triggers

**Interface isolation:** Agents communicate via Go interfaces, never import each other's concrete types.

**Single-flight:** Orchestrator uses `atomic.Bool` — concurrent pipeline triggers get `ErrBusy`.

## Critical Gotchas

### IPv4 byte offset in `netip.Addr.As16()`

IPv4 addresses are in bytes 12-15, NOT 0-3:

```go
// WRONG
addr.As16()[0:4]

// CORRECT
byteOffset := 0
if addr.Is4() {
    byteOffset = 12
}
addr.As16()[byteOffset:byteOffset+4]
```

### Race detector incompatible

This project uses `modernc.org/sqlite` (pure Go) and builds with `CGO_ENABLED=0`. The `-race` flag conflicts. Use goleak for leak detection instead.

### Orchestrator `New()` signature

Adding an agent requires updating all parameters. Inject agents, then config getter last:

```go
func New(src, dl, res, cch, agg, exp, rtr ..., cfgGetter func() config.Config) *Orchestrator
```

### Routing agent safety

- `routing.enabled=false` by default — never auto-enable in tests
- Always test routing changes in isolated netns (`ip netns`)
- Integration tests use build tag `routing_integration` and require `sudo`

### Context cancellation

Check `ctx.Done()` between every pipeline step in orchestrator.

## Verified Commands for Verification

After any code change, run:

```bash
make build && make test
```

These use docker-dev automatically. Do NOT run bare `go build` or `go test` on this system (Go 1.19 < 1.22).

## Adding a New Config Field

1. Struct in `internal/config/config.go`
2. Validation in `internal/config/validate.go`
3. `config.example.yaml`
4. Handle in consuming agent

## Adding a Pipeline Step

Edit `internal/orchestrator/orchestrator.go` `Run()` — insert step, add ctx cancellation check, update `PipelineReport` struct, update `New()` signature.

## Integration Tests

Located in `internal/routing/*_integration_test.go`. Require `sudo` and build tag `routing_integration`. Only run on `main` branch in CI. To run locally:

```bash
sudo -E go test -tags=routing_integration ./internal/routing
```