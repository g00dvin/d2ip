-- Migration: source registry tables
CREATE TABLE IF NOT EXISTS sources (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    prefix TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL DEFAULT (unixepoch()),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX IF NOT EXISTS idx_sources_prefix ON sources(prefix);
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);
