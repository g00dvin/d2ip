# d2ip — Pipeline contract

```
                  ┌──────────────┐
trigger (API/cron)│ Orchestrator │
─────────────────►│  Run(ctx)    │──run_id──┐
                  └──────┬───────┘          │
                         ▼                  ▼
                   ┌────────────┐    ┌──────────┐
                   │ singleflight│   │  history │
                   └─────┬──────┘    │  (mem)   │
                         ▼           └──────────┘
              ┌──────────────────────┐
              │ 1. Source.Get        │  → dlcPath, version
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 2. ListProvider.Load │
              │    .Select(cats)     │  → []Rule
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 3. normalize+dedup   │  → []string domains
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 4. filter resolvable │  → []string (Full+RootDomain only*)
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 5. Cache.NeedsRefresh│  → []string stale
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 6. Resolver.Resolve  │  fan‑out N workers → chan ResolveResult
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 7. Cache.UpsertBatch │  drained in 1k‑row tx
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 8. Cache.Snapshot    │  → []netip.Addr per family
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 9. Aggregator        │  → []netip.Prefix per family
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │10. Exporter.Write    │  ipv4.txt + ipv6.txt (atomic)
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │11. Router            │  cap check → plan v4 → plan v6
              │   (Plan/Apply)       │  → apply v4 → apply v6 (unless dry-run)
              └──────────┬───────────┘
                         ▼
                    history.append
```

\* `Plain` (keyword) and `Regex` rule types do **not** map to a single domain you can
resolve — they are skipped at the resolve stage. The operator decides whether to
expose them via a separate "wildcard" backend later.

## Pipeline guarantees

1. **Single-flight**: only one pipeline runs at a time. Concurrent triggers get
   `409 Busy` with the in-flight `run_id`.
2. **Cancellable**: parent `ctx` cancel propagates to every step.
3. **Idempotent**: re-running with no changes produces zero writes (cache upserts
   no-op, exporter detects unchanged digest, router computes empty diff).
4. **Crash-safe**: state file written *after* router apply succeeds; if process
   dies mid-apply, next start computes diff from on-disk reality vs desired.

## PipelineRequest fields

| Field          | Type | Default | Description                                      |
|----------------|------|---------|--------------------------------------------------|
| `dry_run`      | bool | false   | Stop before export/routing apply                 |
| `force_resolve`| bool | false   | Ignore cache TTL, re-resolve all domains         |
| `skip_routing` | bool | false   | Stop after export, skip routing step entirely    |
