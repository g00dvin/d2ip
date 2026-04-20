# d2ip — HTTP API

Base: `http://<host>:9099`. JSON in/out. Errors: `{"error":"...","code":"..."}`.

## Pipeline

### POST /pipeline/run

Trigger a full pipeline run (single-flight).

**Request:**
```json
{}
```

**Response (200):**
```json
{ "run_id": 1, "domains": 100, "resolved": 95, "failed": 5, "ipv4_out": 1847, "ipv6_out": 3201, "duration": 12400000000 }
```

**Error (409):**
```json
{ "error": "pipeline already running" }
```

### POST /pipeline/dry-run

Run pipeline up to `route` step, no apply.

**Request:**
```json
{}
```

### GET /pipeline/status

Returns current run status and last 10 runs.

### GET /api/pipeline/history

Returns last 10 pipeline runs.

**Response:**
```json
{ "history": [{ "run_id": 1, "domains": 100, "resolved": 95, "failed": 5, "ipv4_out": 1847, "ipv6_out": 3201, "duration": 12400000000 }] }
```

### POST /pipeline/cancel

Cancel the currently running pipeline.

**Response:**
```json
{ "status": "cancelled" }
```

**Error (409):**
```json
{ "error": "no pipeline running" }
```

## Categories

### GET /api/categories

List all available geosite categories with domain counts.

**Response:**
```json
{ "categories": [{ "code": "geosite:cn", "domain_count": 14203, "attrs": [] }] }
```

### GET /api/categories/{code}/domains

Get paginated domains for a category. Query params: `page` (default 1), `per_page` (default 100, max 500).

**Response:**
```json
{ "code": "geosite:cn", "domains": ["example.com", "test.com"], "page": 1, "per_page": 100, "total": 14203, "has_more": true }
```

### POST /api/categories

Add a new category.

**Request:**
```json
{ "code": "geosite:example", "attrs": ["@cn"] }
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

Remove a category.

**Response:**
```json
{ "status": "ok" }
```

### GET /categories

List all categories (legacy).

### PUT /categories

Replace all categories (legacy).

**Request:**
```json
[{"code":"geosite:ru","attrs":["@cn"]}]
```

### DELETE /categories/{code}

Remove a category (legacy).

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

### GET /source/info

Return local path, sha256, fetched_at, version (legacy).

### POST /source/refresh

Force re-download dlc.dat (legacy).

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
  "oldest_updated": "2026-04-19T20:27:09Z",
  "newest_updated": "2026-04-20T14:32:01Z"
}
```

### POST /api/cache/purge

Purge cache entries.

**Request:**
```json
{ "pattern": "*.example.com", "older": "24h", "failed": true }
```

**Response:**
```json
{ "status": "ok", "message": "purge requires cache.DeleteByPattern — not yet implemented" }
```

### POST /api/cache/vacuum

Run SQLite VACUUM.

**Response:**
```json
{ "status": "ok" }
```

### GET /api/cache/entries?domain=example.com

Search cached entries by domain.

**Response:**
```json
{ "domain": "example.com", "ipv4_count": 1847, "ipv6_count": 3201, "note": "domain-level lookup requires cache.GetByDomain — showing totals" }
```

### GET /cache/stats

Return counts by type/status (legacy).

### POST /cache/purge

Wipe all records, keeps domains (legacy).

### POST /cache/vacuum

Drop entries older than threshold (legacy).

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

### GET /settings

Return current settings (legacy).

### PATCH /settings

Update settings (legacy).

**Request:**
```json
{"cache.ttl":"4h"}
```

## Routing

### GET /routing/state

Return last applied state.json.

### POST /routing/dry-run

Preview diff vs current desired.

**Request:**
```json
{ "ipv4_prefixes": ["1.0.0.0/24"], "ipv6_prefixes": ["2001:db8::/32"] }
```

**Response:**
```json
{
  "v4_plan": { "add": [...], "remove": [...] },
  "v6_plan": { "add": [...], "remove": [...] },
  "v4_diff": {},
  "v6_diff": {}
}
```

### POST /routing/apply

Apply pending plan.

### POST /routing/rollback

Remove only entries we own.

**Response:**
```json
{ "status": "ok" }
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

* `GET /healthz` → `200 OK` if process up
* `GET /readyz`  → `200 OK` if DB open and last run < 2× resolve_cycle
* `GET /metrics` → Prometheus exposition
