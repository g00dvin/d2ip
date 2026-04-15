# d2ip — Pipeline contract

```
                  ┌──────────────┐
trigger (API/cron)│ Orchestrator │
─────────────────►│  Run(ctx)    │──run_id──┐
                  └──────┬───────┘          │
                         ▼                  ▼
                   ┌────────────┐    ┌──────────┐
                   │ singleflight│   │  runs DB │
                   └─────┬──────┘    │  insert  │
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
              │ 3. normalize+dedup   │  → []string domains (Full+RootDomain only*)
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 4. Cache.NeedsRefresh│  → []string stale
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 5. Resolver.Resolve  │  fan‑out N workers → chan ResolveResult
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 6. Cache.UpsertBatch │  drained in 1k‑row tx
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 7. Cache.Snapshot    │  → []netip.Addr per family
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 8. Aggregator        │  → []netip.Prefix per family
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │ 9. Exporter.Write    │  ipv4.txt + ipv6.txt (atomic)
              └──────────┬───────────┘
                         ▼
              ┌──────────────────────┐
              │10. Router.Plan/Apply │  nft set diff or table 100
              └──────────┬───────────┘
                         ▼
                    runs.update(ok)
```

\* `Plain` (keyword) and `Regex` rule types do **not** map to a single domain you can
resolve — they are recorded in cache metadata but skipped at the resolve stage with
a `pipeline_skipped_total{reason="unresolvable_rule"}` counter increment. The
operator decides whether to expose them via a separate "wildcard" backend later.

## Pipeline guarantees

1. **Single-flight**: only one pipeline runs at a time. Concurrent triggers get
   `409 Busy` with the in-flight `run_id`.
2. **Cancellable**: parent `ctx` cancel propagates to every step.
3. **Idempotent**: re-running with no changes produces zero writes (cache upserts
   no-op, exporter detects unchanged digest, router computes empty diff).
4. **Crash-safe**: state file written *after* router apply succeeds; if process
   dies mid-apply, next start computes diff from on-disk reality vs desired.
