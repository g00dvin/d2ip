# d2ip — Risk & bottleneck analysis

| # | Risk                                                                 | Likelihood | Impact | Mitigation                                                                                                                            |
|---|----------------------------------------------------------------------|------------|--------|---------------------------------------------------------------------------------------------------------------------------------------|
| 1 | Routing changes brick host network                                    | Low        | Critical | Default `routing.enabled=false`; isolated nft set / table 100; never touch `main`; mandatory dry-run; rollback scoped via state file. |
| 2 | DNS upstream rate-limits or blocks us                                 | Medium     | High   | Per-process rate limit, exponential backoff, configurable upstream, `failed_ttl` short retry.                                        |
| 3 | dlc.dat schema changes upstream                                       | Low        | Medium | Pinned `.proto` mirror; CI fetches latest dlc.dat nightly, decodes, alerts on parse errors.                                          |
| 4 | SQLite write contention under high concurrency                        | Medium     | Medium | WAL mode + `busy_timeout`; single writer connection; batched 1k-row tx; readers unaffected.                                          |
| 5 | Memory blow-up on huge category sets (millions of IPs)                | Low        | Medium | Streaming exporter; radix tree is O(N·bits) memory; chunked snapshot iterator if N > 1M.                                             |
| 6 | Aggressive aggregation overshoots into unrelated networks             | Medium     | High   | Hard `maxPrefix` floor; default level `balanced`; surface aggregation report in API; "off" trivially safe.                           |
| 7 | Resolver goroutine leak on cancel                                     | Medium     | Medium | `errgroup`-scoped pool; close-once channels; `goleak` test in CI.                                                                    |
| 8 | nftables / iproute2 binary missing in container                       | Low        | High   | Caps self-check at startup; refuse `Apply` with clear error; runtime image includes both.                                            |
| 9 | Concurrent `POST /pipeline/run` triggers race                         | High       | Medium | Single-flight in orchestrator; second caller gets `409 Busy + run_id`.                                                               |
|10 | Atomic file write fails on cross-FS rename (bind-mount weirdness)     | Low        | Medium | Temp file in same dir as final; fallback to copy+fsync+rename if `EXDEV` observed; explicit error otherwise.                         |
|11 | IDN/punycode parsing differences vs upstream consumers                | Low        | Low    | Use `idna.Lookup` profile, document, golden-file tests.                                                                              |
|12 | Web override silently shadows ENV after restart                       | Medium     | Medium | Documented precedence ENV>kv>defaults; `GET /settings` shows source per field.                                                       |
|13 | Container cap `NET_ADMIN` not granted ⇒ silent route failures         | Medium     | Medium | Caps self-check; `/readyz` reports degraded; metrics counter `route_apply_total{result="denied"}`.                                   |
|14 | Long CNAME chain or loop wastes resolver budget                       | Medium     | Low    | Hard cap 8 hops; loop detection; metric `resolve_cname_loops_total`.                                                                 |
|15 | `dlc.dat` upstream temporarily 5xx                                    | Medium     | Low    | Source falls back to last known-good local copy; metric `source_fetch_total{result}`.                                                |

## Bottleneck analysis

* **Resolver throughput** is the natural ceiling. With `qps=200, timeout=3s,
  retries=3`, expected ~200 dom/s sustained. Push higher only if upstream is
  ours.
* **SQLite writes** scale to ~50k inserts/sec in WAL with 1k-row tx — well above
  resolver output. Not a bottleneck unless concurrency is misconfigured.
* **Aggregation** is O(N log N) sort + O(N·bits) radix walk; for N=10M,
  ~few seconds and ~1.5 GB RAM. Mitigated by the rare need for aggressive on
  such large inputs.
* **nft `add element` batch** is sub-second up to ~100k elements; for larger
  sets prefer recreate-and-swap pattern (`nft replace set`).

## Operational guardrails

* Refuse to start with `routing.enabled=true` and missing `CAP_NET_ADMIN`.
* Refuse aggregation `aggressive` with `v4_max_prefix < 16` (would emit /8s).
* `/readyz` flips unhealthy if last successful run age > `2 × resolve_cycle`.
