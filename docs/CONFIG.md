# d2ip — Configuration model

Resolution order (highest wins): **ENV** > **Web UI overrides (`kv_settings`)** > **defaults**.

`viper` is loaded with prefix `D2IP_`, dot→underscore. e.g. `resolver.qps` → `D2IP_RESOLVER_QPS`.

```yaml
# config.yaml (defaults; every key is overridable)
listen: ":8080"

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
  failed_ttl: 30m                # short retry for failed lookups
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
  backend: nftables              # nftables|iproute2|none
  table_id: 100                  # iproute2 only
  nft_table: inet d2ip
  nft_set_v4: d2ip_v4
  nft_set_v6: d2ip_v6
  state_path: /var/lib/d2ip/state.json
  dry_run: false

scheduler:
  dlc_refresh: 24h
  resolve_cycle: 1h

logging:
  level: info                    # debug|info|warn|error
  format: json

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

* `resolver.concurrency` ∈ [1, 4096]
* `resolver.qps`         ∈ [1, 100000]
* `cache.ttl`            ≥ 1m
* `aggregation.v4_max_prefix` ∈ [8, 32]; v6 ∈ [16, 128]
* `routing.enabled=true` requires `routing.backend != none` and one of
  `cap NET_ADMIN` / `--network=host`. The Routing Agent self‑checks at startup
  and refuses to apply if capabilities are missing.
