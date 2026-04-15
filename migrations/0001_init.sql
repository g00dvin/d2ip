-- d2ip initial schema

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS domains (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS records (
    id         INTEGER PRIMARY KEY,
    domain_id  INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    ip         TEXT    NOT NULL,
    type       TEXT    NOT NULL CHECK (type IN ('A','AAAA')),
    updated_at INTEGER NOT NULL,
    status     TEXT    NOT NULL CHECK (status IN ('valid','failed')),
    UNIQUE (domain_id, ip, type)
);

CREATE INDEX IF NOT EXISTS idx_records_domain_id  ON records(domain_id);
CREATE INDEX IF NOT EXISTS idx_records_ip         ON records(ip);
CREATE INDEX IF NOT EXISTS idx_records_updated_at ON records(updated_at);
CREATE INDEX IF NOT EXISTS idx_records_type_valid ON records(type, status);

CREATE TABLE IF NOT EXISTS categories (
    code  TEXT PRIMARY KEY,
    attrs TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS kv_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS runs (
    id         INTEGER PRIMARY KEY,
    started_at INTEGER NOT NULL,
    ended_at   INTEGER,
    status     TEXT NOT NULL,
    domains    INTEGER NOT NULL DEFAULT 0,
    resolved   INTEGER NOT NULL DEFAULT 0,
    failed     INTEGER NOT NULL DEFAULT 0,
    ipv4_out   INTEGER NOT NULL DEFAULT 0,
    ipv6_out   INTEGER NOT NULL DEFAULT 0,
    error      TEXT
);

INSERT INTO schema_version(version, applied_at)
VALUES (1, strftime('%s','now'))
ON CONFLICT(version) DO NOTHING;
