# d2ip — Implementation plan (iterations)

Each iteration is a working slice that can be merged on its own.

## Iteration 0 — bootstrap (½ day)

* `go.mod`, lint config, golangci-lint, Makefile, GitHub Actions skeleton.
* `cmd/d2ip/main.go` prints version and exits.
* `Dockerfile` builds and runs the empty binary.

**Done when:** `make build && make test && make docker` all green.

## Iteration 1 — Source + Domain (1 day)

* `internal/source` with HTTP fetch + sha + atomic rename (Agent 01).
* `proto/dlc.proto` → generated `dlcpb`.
* `internal/domainlist` parser + selector + IDN normalization (Agent 02).
* CLI command `d2ip dump --category geosite:ru` writes resolved rule list to stdout.

**Done when:** end-to-end `dlc.dat` fetch + selection works against the live URL.

## Iteration 2 — Cache + Resolver (1.5 days)

* `internal/cache` schema + migrations + idempotent upsert (Agent 04).
* `internal/resolver` worker pool + retry + rate limit (Agent 03).
* CLI `d2ip resolve --category geosite:ru` populates SQLite.
* Unit tests with hermetic DNS server.

**Done when:** ≥10 k domains resolve under configured QPS, no goroutine leaks.

## Iteration 3 — Aggregator + Exporter (1 day)

* `pkg/cidr` radix-tree + `internal/aggregator` (Agent 05).
* `internal/exporter` atomic write + sha sidecar (Agent 06).
* CLI `d2ip export` reads SQLite snapshot → produces ipv4.txt/ipv6.txt.

**Done when:** export is byte-stable across reruns; `Unchanged=true` on no-op.

## Iteration 4 — Orchestrator + API + Scheduler (1.5 days)

* `internal/orchestrator` wiring + single-flight + run history (Agent 09).
* `internal/api` chi handlers (read-only first, then mutators) (Agent 08+API).
* `internal/scheduler` cron-like loops.
* `internal/config` viper + ENV + kv overrides + hot reload.

**Done when:** `POST /pipeline/run` performs full pipeline minus routing.

## Iteration 5 — Routing (2 days)

* `internal/routing` nft backend + state + dry-run + rollback (Agent 07).
* iproute2 fallback.
* Capability self-check + safe defaults (`enabled=false`).
* Integration test in a netns (build tag `routing_integration`).

**Done when:** dry-run shows correct diff; apply is no-op on second call;
rollback restores pre-apply state.

## Iteration 6 — Observability + UI + hardening (1 day)

* Prometheus metrics on every step.
* `/healthz` + `/readyz`.
* Minimal static UI under `web/` (HTMX or a single React build).
* Race detector in CI (`go test -race ./...`).
* Soak test (24h loop on a stable category).

**Done when:** dashboards show end-to-end latency, error rates per agent.

## Iteration 7 — Release (½ day)

* Versioning, multi-arch Docker (amd64/arm64).
* `deploy/nftables.example.nft` and `README` quickstart.
* Tag `v0.1.0`.

---

## Test matrix (continuous)

| layer        | tooling                                 | gate              |
|--------------|-----------------------------------------|-------------------|
| unit         | `go test ./...`                         | every PR          |
| race         | `go test -race ./...`                   | every PR          |
| leaks        | `go.uber.org/goleak` in resolver/orch   | every PR          |
| property     | `pgregory.net/rapid` in `pkg/cidr`      | every PR          |
| integration  | tag `routing_integration` in netns      | nightly           |
| soak         | docker-compose + synthetic resolver     | nightly           |
| fuzz         | `go test -fuzz` on parser & aggregator  | nightly           |
