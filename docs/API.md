# d2ip — HTTP API

Base: `http://<host>:9099`. JSON in/out. Errors: `{"error":"..."}`.

## Pipeline

### POST /pipeline/run

Trigger a full pipeline run (single-flight).

**Request:**
```json
{ "dry_run": false, "force_resolve": false, "skip_routing": false }
```

All fields are optional. `dry_run=true` stops before export/routing apply. `force_resolve=true` ignores cache TTL. `skip_routing=true` stops after export.

**Response (200):**
```json
{ "run_id": 1, "domains": 100, "stale": 50, "resolved": 45, "failed": 5, "ipv4_out": 1847, "ipv6_out": 3201, "duration": 12400000000 }
```

**Error (409):**
```json
{ "error": "orchestrator: pipeline already running: run_id=1234567890" }
```

### GET /pipeline/status

Returns current run status.

**Response:**
```json
{ "running": false, "run_id": 1, "started": "2026-04-20T14:32:01Z", "report": { ... } }
```

### GET /api/pipeline/history

Returns last 10 pipeline runs.

**Response:**
```json
{ "history": [{ "run_id": 1, "domains": 100, "stale": 50, "resolved": 45, "failed": 5, "ipv4_out": 1847, "ipv6_out": 3201, "duration": 12400000000 }] }
```

### POST /pipeline/cancel

Cancel the currently running pipeline.

**Response (200):**
```json
{ "status": "cancelled" }
```

**Response (200, idempotent):**
```json
{ "status": "not running" }
```

## Categories

### GET /api/categories

List configured and available geosite categories.

**Response:**
```json
{
  "configured": [{ "code": "geosite:cn", "attrs": [], "domain_count": 14203 }],
  "available": ["geosite:google", "geosite:facebook"]
}
```

### GET /api/categories/{code}/domains

Get paginated domains for a category. Query params: `page` (default 1), `per_page` (default 100, max 500).

**Response:**
```json
{ "code": "geosite:cn", "domains": ["example.com", "test.com"], "page": 1, "per_page": 100, "total": 14203, "has_more": true }
```

### POST /api/categories

Add a new category. Code is auto-prefixed with `geosite:` if missing.

**Request:**
```json
{ "code": "example", "attrs": ["@cn"] }
```

**Response:**
```json
{ "status": "ok" }
```

**Error (409):**
```json
{ "error": "category already exists: geosite:example" }
```

### DELETE /api/categories/{code}

Remove a category. Code is auto-prefixed with `geosite:` if missing.

**Response:**
```json
{ "status": "ok" }
```

**Error (404):**
```json
{ "error": "category not found: geosite:example" }
```

## Source

### GET /api/source/info

Return dlc.dat metadata (SHA256, size, ETag, fetch time).

**Response:**
```json
{
  "available": true,
  "fetched_at": "2026-04-20T14:32:01Z",
  "size": 1234567,
  "etag": "\"abc123\"",
  "last_modified": "2026-04-20T10:00:00Z",
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}
```

**Response (unavailable):**
```json
{ "available": false }
```

## Cache

### GET /api/cache/stats

Return cache statistics.

**Response:**
```json
{
  "domains": 14203,
  "records_total": 28406,
  "records_v4": 14203,
  "records_v6": 14203,
  "records_valid": 28000,
  "records_failed": 406,
  "records_nxdomain": 200,
  "oldest_updated": 1713562029,
  "newest_updated": 1713623521
}
```

### POST /api/cache/purge

Purge cache entries (placeholder — not yet implemented).

**Request:**
```json
{ "pattern": "*.example.com", "older": "24h", "failed": true }
```

**Response:**
```json
{ "status": "ok", "message": "purge requires cache.DeleteByPattern — not yet implemented" }
```

### POST /api/cache/vacuum

Run SQLite VACUUM and delete stale records.

**Response:**
```json
{ "status": "ok", "deleted": 42 }
```

### GET /api/cache/entries?domain=example.com

Search cached entries by domain (not yet implemented).

**Error (503):**
```json
{ "error": "domain-level lookup not yet implemented" }
```

## Settings

### GET /api/settings

Returns current config, defaults, and KV overrides.

**Response:**
```json
{
  "config": { ... },
  "defaults": { ... },
  "overrides": { "resolver.qps": "100" }
}
```

### PUT /api/settings

Set config overrides. Hot-reloads via Watcher.

**Request:**
```json
{ "resolver.qps": "100", "logging.level": "debug" }
```

**Response:**
```json
{ "status": "ok" }
```

### DELETE /api/settings/{key}

Remove a config override, reverting to default.

**Response:**
```json
{ "status": "ok" }
```

## Routing

### POST /routing/dry-run

Preview diff vs current desired without applying.

**Request:**
```json
{ "ipv4_prefixes": ["1.0.0.0/24"], "ipv6_prefixes": ["2001:db8::/32"] }
```

**Response:**
```json
{
  "v4_plan": { "add": [...], "remove": [...] },
  "v6_plan": { "add": [...], "remove": [...] },
  "v4_diff": "+ 1.0.0.0/24\n",
  "v6_diff": "+ 2001:db8::/32\n"
}
```

**Error (503, routing disabled):**
```json
{ "error": "routing disabled" }
```

### POST /routing/rollback

Remove only entries we own.

**Response:**
```json
{ "status": "ok" }
```

**Error (503, routing disabled):**
```json
{ "error": "routing disabled" }
```

### GET /routing/snapshot

Show current applied routing state.

**Response:**
```json
{
  "backend": "nftables",
  "applied_at": "2026-04-20T14:32:01Z",
  "v4": ["1.0.0.0/24"],
  "v6": ["2001:db8::/32"]
}
```

## Health & metrics

### GET /healthz

Returns `200 OK` if process is alive.

**Response:**
```json
{ "status": "ok" }
```

### GET /readyz

Returns `200 OK` (stub — DB check TODO).

**Response:**
```json
{ "status": "ready" }
```

### GET /metrics

Prometheus exposition format.
