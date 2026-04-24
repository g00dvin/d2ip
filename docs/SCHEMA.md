# d2ip — SQLite Schema

The cache agent uses `modernc.org/sqlite` with these PRAGMAs set at open time:

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=5000;
```

Migrations live in `migrations/*.sql` and are applied at runtime via an embedded `embed.FS`, guarded by the `schema_version` table.

## Tables

### `schema_version`

Tracks which migrations have been applied.

| Column     | Type    | Notes          |
|------------|---------|----------------|
| version    | INTEGER | PK             |
| applied_at | INTEGER | Unix timestamp |

### `domains`

Canonical domain names (punycode, lowercase).

| Column           | Type    | Notes                        |
|------------------|---------|------------------------------|
| id               | INTEGER | PK                           |
| name             | TEXT    | UNIQUE, NOT NULL             |
| last_resolved_at | INTEGER | NOT NULL DEFAULT 0           |
| resolve_status   | TEXT    | NOT NULL DEFAULT 'unknown'   |

**Constraints:** `resolve_status IN ('unknown','valid','failed','nxdomain')`

**Index:** `idx_domains_status_refresh ON (resolve_status, last_resolved_at)`

### `records`

IP records per domain (1:N).

| Column     | Type    | Notes                              |
|------------|---------|------------------------------------|
| id         | INTEGER | PK                                 |
| domain_id  | INTEGER | NOT NULL, FK → domains(id) ON DELETE CASCADE |
| ip         | TEXT    | NOT NULL                           |
| type       | TEXT    | NOT NULL, CHECK IN ('A','AAAA')    |
| updated_at | INTEGER | NOT NULL (Unix timestamp)          |
| status     | TEXT    | NOT NULL, CHECK IN ('valid','failed') |

**Constraints:** `UNIQUE (domain_id, ip, type)`

**Indexes:**
- `idx_records_domain_id ON (domain_id)`
- `idx_records_ip ON (ip)`
- `idx_records_updated_at ON (updated_at)`
- `idx_records_type_valid ON (type, status)`

### `categories`

Configured geosite categories (mirrors in-memory config; used by cache for domain lists).

| Column | Type | Notes          |
|--------|------|----------------|
| code   | TEXT | PK             |
| attrs  | TEXT | NOT NULL DEFAULT '' |

### `kv_settings`

Runtime config overrides persisted by the Web UI.

| Column | Type | Notes          |
|--------|------|----------------|
| key    | TEXT | PK             |
| value  | TEXT | NOT NULL       |

### `runs`

Pipeline run history (schema exists but currently unused — history is kept in-memory by the orchestrator).

| Column     | Type    | Notes              |
|------------|---------|--------------------|
| id         | INTEGER | PK                 |
| started_at | INTEGER | NOT NULL           |
| ended_at   | INTEGER |                    |
| status     | TEXT    | NOT NULL           |
| domains    | INTEGER | NOT NULL DEFAULT 0 |
| resolved   | INTEGER | NOT NULL DEFAULT 0 |
| failed     | INTEGER | NOT NULL DEFAULT 0 |
| ipv4_out   | INTEGER | NOT NULL DEFAULT 0 |
| ipv6_out   | INTEGER | NOT NULL DEFAULT 0 |
| error      | TEXT    |                    |

## Write patterns

- **Batch upserts:** The cache writer drains `chan ResolveResult` into transactions capped at ~1,000 rows (`maxRowsPerTx`).
- **Parameter ceiling:** Individual prepared statements stay under 900 placeholders (`maxParamsPerStmt`) to remain compatible with SQLite's default `SQLITE_MAX_VARIABLE_NUMBER`.
- **Vacuum:** `Vacuum()` deletes stale records then runs `PRAGMA incremental_vacuum` or `VACUUM` if the freelist is large.
