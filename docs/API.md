# d2ip — HTTP API

Base: `http://<host>:8080`. JSON in/out. Errors: `{"error":"...","code":"..."}`.

## Pipeline

| Method | Path                  | Body                              | Description                         |
|--------|-----------------------|-----------------------------------|-------------------------------------|
| POST   | `/pipeline/run`       | `{}`                              | Trigger a full run (single‑flight)  |
| POST   | `/pipeline/dry-run`   | `{}`                              | Run pipeline up to `route`, no apply|
| GET    | `/pipeline/status`    | —                                 | Current run + last 10 runs          |
| POST   | `/pipeline/cancel`    | —                                 | Cancel current run                  |

## Categories

| Method | Path                  | Body                                       |
|--------|-----------------------|--------------------------------------------|
| GET    | `/categories`         | —                                          |
| PUT    | `/categories`         | `[{"code":"geosite:ru","attrs":["@cn"]}]`  |
| DELETE | `/categories/{code}`  | —                                          |

## Source

| Method | Path                  | Description                              |
|--------|-----------------------|------------------------------------------|
| GET    | `/source/info`        | local path, sha256, fetched_at, version  |
| POST   | `/source/refresh`     | Force re-download dlc.dat                |

## Cache

| Method | Path                  | Description                       |
|--------|-----------------------|-----------------------------------|
| GET    | `/cache/stats`        | counts by type/status             |
| POST   | `/cache/purge`        | wipe all records (keeps domains)  |
| POST   | `/cache/vacuum`       | drop entries older than threshold |

## Settings (Web overrides → kv_settings)

| Method | Path                  | Body                              |
|--------|-----------------------|-----------------------------------|
| GET    | `/settings`           | —                                 |
| PATCH  | `/settings`           | `{"cache.ttl":"4h", ...}`         |

## Routing

| Method | Path                    | Description                          |
|--------|-------------------------|--------------------------------------|
| GET    | `/routing/state`        | last applied state.json              |
| POST   | `/routing/dry-run`      | preview diff vs current desired      |
| POST   | `/routing/apply`        | apply pending plan                   |
| POST   | `/routing/rollback`     | remove only entries we own           |

## Health & metrics

* `GET /healthz` → `200 OK` if process up
* `GET /readyz`  → `200 OK` if DB open and last run < 2× resolve_cycle
* `GET /metrics` → Prometheus exposition
