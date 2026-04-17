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

## Iteration 6 — Observability + UI + hardening (1 day) ✅ COMPLETE

* ✅ Prometheus metrics on every step (5 metrics: dns_resolve_*, pipeline_*).
* ✅ `/healthz` + `/readyz` (already existed from Iteration 0).
* ✅ Minimal static UI under `web/` (HTMX, 17.2KB embedded).
* ⚠️ Race detector in CI (incompatible with CGO_ENABLED=0, documented).
* ⚠️ Soak test (deferred to Iteration 8).
* ✅ goleak tests (orchestrator + resolver).
* ✅ Integration tests in netns (nftables + iproute2, build tag `routing_integration`).
* ✅ Docker dev workflow (Dockerfile.dev, cached deps, <5s builds).
* ✅ GitHub Actions CI (5 jobs: test, goleak, lint, build, integration).

**Done:** All core observability + testing complete. Dashboards (Grafana) deferred to Iteration 8.

---

## Iteration 7 — Release Preparation (1 day)

**Goal:** Prepare for v0.1.0 release with multi-arch support and production deployment guides.

### Deliverables

1. **Multi-arch Docker builds** (amd64 + arm64)
   - Update `deploy/Dockerfile` with buildx
   - GitHub Actions matrix build
   - Push to Docker Hub / GHCR
   - Test on ARM (Raspberry Pi or AWS Graviton)

2. **Versioning**
   - Semantic versioning (v0.1.0)
   - Git tags
   - CHANGELOG.md with release notes
   - Version embedded in binary (`-ldflags "-X main.Version=..."`)

3. **Production deployment guides**
   - `deploy/systemd/d2ip.service` — systemd unit file
   - `deploy/docker-compose.yml` — docker-compose example with volumes
   - `deploy/nginx/d2ip.conf` — reverse proxy config (HTTPS + BasicAuth)
   - `docs/DEPLOYMENT.md` — production deployment guide

4. **Fix config tests** ❗
   - Debug 3 failing tests in `internal/config/load_test.go`
   - Fix precedence logic or test expectations
   - **CRITICAL** before v0.1.0

5. **Troubleshooting guide**
   - `docs/TROUBLESHOOTING.md` with common issues
   - DNS resolution failures
   - Routing not working
   - Pipeline stuck
   - Performance tuning

6. **Example configs**
   - `deploy/nftables.example.nft` — sample nftables rules
   - `deploy/config/production.yaml` — production config template
   - `deploy/config/development.yaml` — dev config template

7. **Release automation**
   - GitHub Actions workflow for releases
   - Build binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
   - Create GitHub Release with binaries and checksums

8. **Security documentation**
   - Document API auth requirements (reverse proxy)
   - HTTPS setup guide
   - Rate limiting recommendations
   - Network isolation best practices

**Done when:** 
- `make docker` produces multi-arch images
- v0.1.0 tagged and pushed
- GitHub Release with binaries published
- All config tests pass
- Production deployment guide complete

---

## Iteration 8 — Observability Dashboards + Hardening (1 day)

**Goal:** Complete observability stack and improve production readiness.

### Deliverables

1. **Grafana dashboards**
   - Pre-built dashboard JSON for d2ip metrics
   - Panels: pipeline runs, DNS resolution, routing changes, error rates
   - Alert rules for critical failures
   - `deploy/grafana/d2ip-dashboard.json`

2. **Prometheus alerts**
   - `deploy/prometheus/alerts.yml`
   - Alerts: pipeline failures, high DNS error rate, stale data

3. **Soak testing**
   - 24-hour continuous pipeline runs
   - Monitor: goroutine leaks, memory growth, connection exhaustion
   - `tests/soak/docker-compose.yml` with synthetic workload
   - Document results in `docs/SOAK_TEST_RESULTS.md`

4. **Backup/restore**
   - CLI commands: `d2ip backup` and `d2ip restore`
   - Export: SQLite + routing state + config to tar.gz
   - Restore: unpack and apply
   - Test migration between hosts

5. **Web UI config editing**
   - Form for editing kv_settings (resolver.upstream, scheduler intervals, etc.)
   - Validation + live preview
   - Save to kv_settings table (no restart required)
   - Simple auth (password in ENV var)

6. **Logging improvements**
   - Structured JSON logging option
   - Log levels: debug, info, warn, error
   - Log rotation guidance (systemd journal or logrotate)

**Done when:**
- Grafana dashboard shows all key metrics
- Soak test runs 24h with no leaks
- Config editing works via web UI
- Backup/restore tested successfully

---

## Iteration 9 — Performance + Polish (Optional)

**Goal:** Performance optimizations and quality-of-life improvements.

### Deliverables

1. **nft JSON mode** (replaces plain-text parsing)
   - Rewrite `parseNftSet()` to use `nft --json`
   - Fallback to plain text if JSON unavailable
   - More robust and maintainable

2. **Incremental resolver updates**
   - Only re-resolve domains that changed or are stale
   - Skip unchanged domains
   - Faster pipeline runs (especially for large domain lists)

3. **Property-based testing** (CIDR aggregator)
   - Use `pgregory.net/rapid` for randomized tests
   - Find edge cases in aggregation logic

4. **Fuzzing** (parser + aggregator)
   - `go test -fuzz` on dlc.proto parser
   - Fuzz CIDR aggregator with malformed inputs
   - Run in CI (nightly)

5. **iproute2 Iface validation**
   - Add config validation for `routing.iface` when backend is iproute2
   - Clear error message if missing

6. **API examples**
   - Curl examples for all endpoints
   - Go client library example
   - Python client example (requests)
   - `docs/API_EXAMPLES.md`

**Done when:**
- nft JSON parsing implemented and tested
- Incremental updates reduce pipeline time by 50%+
- Fuzz tests run in CI without crashes

---

## Iteration 10+ — Future Enhancements

**Goal:** Advanced features for power users.

### Possible Features

1. **Plugin system** for custom exporters
2. **gRPC API** (in addition to HTTP)
3. **DNS TTL respect** (honor DNS TTL instead of internal TTL only)
4. **TLS support** for API (optional HTTPS without reverse proxy)
5. **API key authentication** (simpler than OAuth for internal use)
6. **Webhook notifications** (on pipeline success/failure)
7. **Export to cloud storage** (S3, GCS for ipv4.txt/ipv6.txt)
8. **Multi-region deployment** (sync state across regions)

**Priority:** User-driven based on feedback after v0.1.0 release

---

## Test matrix (current status)

| layer        | tooling                                 | status            | gate              |
|--------------|-----------------------------------------|-------------------|-------------------|
| unit         | `go test ./...`                         | ✅ IMPLEMENTED    | every PR (CI)     |
| race         | `go test -race ./...`                   | ❌ INCOMPATIBLE   | n/a (CGO=0)       |
| leaks        | `go.uber.org/goleak` in resolver/orch   | ✅ IMPLEMENTED    | every PR (CI)     |
| property     | `pgregory.net/rapid` in `pkg/cidr`      | 📋 PLANNED        | Iteration 9       |
| integration  | tag `routing_integration` in netns      | ✅ IMPLEMENTED    | manual (sudo)     |
| soak         | docker-compose + synthetic resolver     | 📋 PLANNED        | Iteration 8       |
| fuzz         | `go test -fuzz` on parser & aggregator  | 📋 PLANNED        | Iteration 9       |

**Notes:**
- **race detector**: Requires CGO_ENABLED=1, incompatible with static builds (modernc.org/sqlite)
- **integration tests**: Require CAP_NET_ADMIN, run manually or in CI on main branch
- **goleak tests**: Cover goroutine leak detection (partial race condition coverage)
