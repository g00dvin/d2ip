# Agent 04 — Cache Agent (SQLite)

**Package:** `internal/cache`
**Owns:** SQLite schema, batched upserts, TTL-based staleness, snapshot reads.

## Contract

```go
type Cache interface {
    NeedsRefresh(ctx context.Context, domains []string, ttl, failedTTL time.Duration) (stale []string, err error)
    UpsertBatch(ctx context.Context, results []resolver.ResolveResult) error
    Snapshot(ctx context.Context) (ipv4 []netip.Addr, ipv6 []netip.Addr, err error)
    Stats(ctx context.Context) (Stats, error)
    Vacuum(ctx context.Context, olderThan time.Duration) (deleted int, err error)
    Close() error
}
```

## Tasks

1. **Open** `modernc.org/sqlite` (CGO-free) with the PRAGMAs from `SCHEMA.md`.
2. **Migrations**: embed `migrations/*.sql` via `embed.FS`; apply on open with a
   `schema_version` table.
3. **TTL semantics** (DNS TTL ignored):
   * For `status='valid'`: stale if `now - max(updated_at) > ttl`.
   * For `status='failed'`: stale if `now - max(updated_at) > failed_ttl`.
   * Domains absent from `domains`/`records` are always stale.
4. **Batch upsert**:
   * Begin tx, insert missing domain names, fetch ids in one `IN (...)` query.
   * Upsert records using the idempotent `ON CONFLICT(domain_id, ip, type)`.
   * Soft cap of 1000 rows per tx; commit and start a new one beyond that.
5. **Snapshot**: `SELECT DISTINCT ip FROM records WHERE type=? AND status='valid'`,
   parse with `netip.ParseAddr`, drop unparseable entries with a metric increment.
6. **Vacuum**: delete records older than `olderThan`, then `VACUUM` if reclaimable
   pages > 10 % (cheap heuristic via `PRAGMA freelist_count`).
7. **Concurrency**: a single `*sql.DB` is fine (SQLite WAL allows concurrent
   readers + serialized writer); cap `MaxOpenConns=1` for writes via a separate
   sql.DB handle to avoid `database is locked`.
8. **Tests**: in-memory `:memory:` DB. Cases: idempotent re-insert, TTL
   expiration boundary, mixed valid/failed, snapshot ordering stability, vacuum.

## Acceptance

* Re-running the pipeline with no DNS changes performs **zero** record-row writes.
* Snapshot returns canonical IP strings (`netip.Addr.String()`).
* Stats reports counts in O(few queries), not O(rows).
