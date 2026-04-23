-- Track per-domain resolution status so that failed/NXDomain results
-- are persisted even when no IP records exist. This enables:
--   1. NeedsRefresh to honour failedTTL for domains with zero IPs
--   2. Observability into why domains produce no records

ALTER TABLE domains ADD COLUMN last_resolved_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE domains ADD COLUMN resolve_status TEXT NOT NULL DEFAULT 'unknown' CHECK (resolve_status IN ('unknown','valid','failed','nxdomain'));

CREATE INDEX IF NOT EXISTS idx_domains_status_refresh ON domains(resolve_status, last_resolved_at);