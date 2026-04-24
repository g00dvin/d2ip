# d2ip — Configuration model

Resolution order (highest wins): **ENV** > **Web UI overrides (`kv_settings`)** > **defaults**.

`viper` is loaded with prefix `D2IP_`, dot→underscore. e.g. `resolver.qps` → `D2IP_RESOLVER_QPS`.

```yaml
# config.yaml (defaults; every key is overridable)
listen: ":9099"

source:
  url: https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat
  cache_path: /var/lib/d2ip/dlc.dat
  refresh_interval: 24h
  http_timeout: 30s

categories:
  - code: geosite:ru
  - code: geosite:google
    attrs: []                    # optional @attribute filter (AND)

resolver:
  upstream: 1.1.1.1:53
  network: udp                   # udp|tcp|tcp-tls
  concurrency: 64                # worker pool size
  qps: 200                       # global rate limit
  timeout: 3s
  retries: 3
  backoff_base: 200ms
  backoff_max: 5s
  follow_cname: true
  enable_v4: true
  enable_v6: true

cache:
  db_path: /var/lib/d2ip/cache.db
  ttl: 6h                        # internal TTL — DNS TTL is IGNORED
  failed_ttl: 30m                # short retry for failures
  vacuum_after: 720h             # 30d

aggregation:
  enabled: true
  level: balanced                # off|conservative|balanced|aggressive
  v4_max_prefix: 16              # never aggregate broader than this
  v6_max_prefix: 32

export:
  dir: /var/lib/d2ip/out
  ipv4_file: ipv4.txt
  ipv6_file: ipv6.txt

routing:
  enabled: false                 # SAFE default
  backend: nftables              # none|nftables|iproute2
  table_id: 100                  # iproute2 only
  iface: ""                      # REQUIRED for iproute2 (e.g. "eth0")
  nft_table: inet d2ip
  nft_set_v4: d2ip_v4
  nft_set_v6: d2ip_v6
  state_path: /var/lib/d2ip/state.json
  dry_run: false

scheduler:
  dlc_refresh: 24h
  resolve_cycle: 1h

logging:
  level: info                    # debug|info|warn|error|fatal|panic
  format: json                   # json|console|text

metrics:
  enabled: true
  path: /metrics
```

## Aggressiveness levels

| level         | semantics                                                                |
|---------------|--------------------------------------------------------------------------|
| off           | no aggregation, /32 + /128 only                                          |
| conservative  | merge only fully covered adjacent prefixes (lossless)                    |
| balanced      | conservative + collapse runs where ≥75 % of /24 (or /48) is present       |
| aggressive    | balanced + climb up to `v4_max_prefix` / `v6_max_prefix` if ≥50 % covered |

## Validation rules

* `listen` — non-empty, valid `host:port`
* `source.url` — non-empty, must start with `http://` or `https://`
* `source.cache_path` — non-empty
* `source.refresh_interval` ≥ 1m
* `source.http_timeout` ≥ 1s
* `categories[*].code` — non-empty, must contain `:`, must be unique
* `resolver.upstream` — valid `host:port`, port in [1,65535]
* `resolver.network` — `udp`, `tcp`, or `tcp-tls`
* `resolver.concurrency` ∈ [1, 4096]
* `resolver.qps` ∈ [1, 100000]
* `resolver.timeout` ≥ 100ms
* `resolver.retries` ∈ [0, 10]
* `resolver.backoff_base` > 0
* `resolver.backoff_max` ≥ `backoff_base`
* At least one of `resolver.enable_v4` or `resolver.enable_v6` must be `true`
* `cache.db_path` — non-empty
* `cache.ttl` ≥ 1m
* `cache.failed_ttl` ≥ 1s
* `cache.vacuum_after` ≥ 1h
* `aggregation.v4_max_prefix` ∈ [8, 32]
* `aggregation.v6_max_prefix` ∈ [16, 128]
* `export.dir`, `export.ipv4_file`, `export.ipv6_file` — non-empty
* `export.ipv4_file` and `export.ipv6_file` must differ
* `routing.backend` — `none`, `nftables`, or `iproute2`
* `routing.enabled=true` requires `routing.backend != none`
* `routing.table_id` ∈ [1, 252] for iproute2
* `routing.iface` — required when `backend=iproute2`
* `routing.nft_table`, `routing.nft_set_v4`, `routing.nft_set_v6` — required when `backend=nftables`
* `routing.state_path` — required when `routing.enabled=true`
* `scheduler.dlc_refresh` ≥ 1m
* `scheduler.resolve_cycle` ≥ 1m or 0 (disabled)
* `logging.level` — `debug|info|warn|error|fatal|panic`
* `logging.format` — `json|console|text`
* `metrics.path` — must start with `/` when enabled
