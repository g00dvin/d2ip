# d2ip — SQLite schema

PRAGMA on open:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous  = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA temp_store   = MEMORY;
```

## Tables

```sql
CREATE TABLE IF NOT EXISTS domains (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE         -- punycode, lowercase
);

CREATE TABLE IF NOT EXISTS records (
    id         INTEGER PRIMARY KEY,
    domain_id  INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    ip         TEXT    NOT NULL,      -- canonical netip.Addr.String()
    type       TEXT    NOT NULL CHECK (type IN ('A','AAAA')),
    updated_at INTEGER NOT NULL,      -- unix seconds (UTC)
    status     TEXT    NOT NULL CHECK (status IN ('valid','failed')),
    UNIQUE (domain_id, ip, type)      -- idempotent upserts
);

CREATE INDEX IF NOT EXISTS idx_records_domain_id  ON records(domain_id);
CREATE INDEX IF NOT EXISTS idx_records_ip         ON records(ip);
CREATE INDEX IF NOT EXISTS idx_records_updated_at ON records(updated_at);
CREATE INDEX IF NOT EXISTS idx_records_type_valid ON records(type, status);

-- Categories the user has selected (persisted Web UI state).
CREATE TABLE IF NOT EXISTS categories (
    code  TEXT PRIMARY KEY,           -- e.g. "geosite:ru"
    attrs TEXT NOT NULL DEFAULT ''    -- CSV @attrs filter
);

-- Free-form key/value for runtime settings overridable in UI.
CREATE TABLE IF NOT EXISTS kv_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Pipeline run history (last N kept).
CREATE TABLE IF NOT EXISTS runs (
    id         INTEGER PRIMARY KEY,
    started_at INTEGER NOT NULL,
    ended_at   INTEGER,
    status     TEXT NOT NULL,         -- running|ok|error
    domains    INTEGER NOT NULL DEFAULT 0,
    resolved   INTEGER NOT NULL DEFAULT 0,
    failed     INTEGER NOT NULL DEFAULT 0,
    ipv4_out   INTEGER NOT NULL DEFAULT 0,
    ipv6_out   INTEGER NOT NULL DEFAULT 0,
    error      TEXT
);
```

## Upsert pattern

```sql
INSERT INTO domains(name) VALUES (?)
ON CONFLICT(name) DO NOTHING;

INSERT INTO records(domain_id, ip, type, updated_at, status)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(domain_id, ip, type) DO UPDATE SET
    updated_at = excluded.updated_at,
    status     = excluded.status;
```

## Snapshot for export

```sql
SELECT DISTINCT r.ip
FROM   records r
WHERE  r.type = ?           -- 'A' or 'AAAA'
  AND  r.status = 'valid';
```

## TTL eviction (resolver decides what's stale)

```sql
SELECT d.name
FROM   domains d
LEFT JOIN records r
       ON r.domain_id = d.id AND r.status = 'valid'
WHERE  d.name IN (...batch...)
GROUP BY d.name
HAVING MAX(COALESCE(r.updated_at, 0)) < ?;   -- now - ttl
```
