# d2ip — Configuration model

Resolution order (highest wins): **ENV** > **Web UI overrides (`kv_settings`)** > **defaults**.

`viper` is loaded with prefix `D2IP_`, dot→underscore. e.g. `resolver.qps` → `D2IP_RESOLVER_QPS`.

```yaml
# config.yaml (defaults; every key is overridable)
listen: ":9099"

sources:
  # Multi-source registry. Each source has a unique prefix that namespaces
  # its categories. Categories are referenced as "prefix:name" in policies.
  - id: v2fly-geosite
    provider: v2flygeosite
    prefix: geosite
    enabled: true
    config:
      url: https://github.com/v2fly/domain-list-community/releases/latest/download/dlc.dat
      cache_path: /var/lib/d2ip/dlc.dat
      refresh_interval: 24h
      http_timeout: 30s

  - id: v2fly-geoip
    provider: v2flygeoip
    prefix: geoip
    enabled: true
    config:
      url: https://github.com/v2fly/geoip/releases/latest/download/geoip.dat
      cache_path: /var/lib/d2ip/geoip.dat
      refresh_interval: 24h
      http_timeout: 30s

  - id: ipverse-ru
    provider: ipverse
    prefix: ipverse
    enabled: true
    config:
      base_url: https://ipverse.net/ipblocks/data/countries/{country}.zone
      countries: ["ru", "us", "de"]

  - id: mmdb-local
    provider: mmdb
    prefix: mmdb
    enabled: true
    config:
      file: /var/lib/d2ip/GeoLite2-Country.mmdb
      countries: ["ru", "us", "de"]

categories:
  # Global categories to resolve and cache.
  # The pipeline resolves ALL of these into the SQLite cache, making them
  # available to any routing policy below. This is the "what domains do we
  # know about?" list — independent of how (or whether) they are routed.
  # Categories use "prefix:name" format from the source registry.
  - code: geosite:ru
  - code: geosite:google
    attrs: []                    # optional @attribute filter (AND)
  - code: ipverse:ru            # prefix sources (no DNS resolution needed)
  - code: geoip:us

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
  state_dir: /var/lib/d2ip      # directory for per-policy state files

  policies:
    # Each policy selects a SUBSET of the global categories above and routes
    # their resolved IPs through its own backend/table. Multiple policies can
    # share categories. This is the "which domains go through this table?"
    # list — it filters from the already-resolved global cache.
    - name: streaming
      enabled: true
      categories: ["geosite:netflix", "geosite:youtube"]
      backend: iproute2
      table_id: 200
      iface: eth1
      dry_run: false
      export_format: plain       # plain | ipset | json | nft | iptables | bgp | yaml

    - name: blocklist
      enabled: true
      categories: ["geosite:malware"]
      backend: nftables
      nft_table: inet d2ip
      nft_set_v4: block_v4
      nft_set_v6: block_v6
      dry_run: false
      export_format: nft

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
* `sources[*].id` — non-empty, unique
* `sources[*].provider` — one of `v2flygeosite`, `v2flygeoip`, `ipverse`, `mmdb`, `plaintext`
* `sources[*].prefix` — non-empty, unique across all sources, lowercase alphanumeric + hyphens
* `sources[*].enabled` — boolean
* `v2flygeosite.url` — non-empty, must start with `http://` or `https://`
* `v2flygeosite.cache_path` — non-empty
* `v2flygeosite.refresh_interval` ≥ 1m
* `v2flygeosite.http_timeout` ≥ 1s
* `ipverse.countries` — required, array of country codes
* `mmdb.file` or `mmdb.url` — at least one required
* `categories[*].code` — non-empty, must contain `:` (prefix:name format), must be unique
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
* `routing.enabled` — boolean
* `routing.state_dir` — non-empty when `routing.enabled=true`
* `routing.policies[*].name` — unique, non-empty, must match `[a-z0-9_-]+`
* `routing.policies[*].categories` — at least one code per enabled policy, must contain `:`
* `routing.policies[*].backend` — `none`, `nftables`, or `iproute2` (cannot be `none` for enabled policies)
* `routing.policies[*].table_id` ∈ [1, 252] for iproute2, must be unique across policies
* `routing.policies[*].iface` — required when `backend=iproute2`
* `routing.policies[*].nft_table`, `routing.policies[*].nft_set_v4`, `routing.policies[*].nft_set_v6` — required when `backend=nftables`, sets must be unique per table
* `routing.policies[*].export_format` — `plain` (default), `ipset`, `json`, `nft`, `iptables`, `bgp`, or `yaml`
* `scheduler.dlc_refresh` ≥ 1m
* `scheduler.resolve_cycle` ≥ 1m or 0 (disabled)
* `logging.level` — `debug|info|warn|error|fatal|panic`
* `logging.format` — `json|console|text`
* `metrics.path` — must start with `/` when enabled

## Backend Prerequisites

Before enabling routing policies, ensure the required backends are prepared:

- **nftables backend**: The `nft` binary must be installed and available in PATH.
  The process must have `CAP_NET_ADMIN` capability (or run as root).
  The configured nftables table (`inet d2ip` by default) will be created automatically
  on the first pipeline run if it doesn't exist.

- **iproute2 backend**: The `ip` binary must be installed and available in PATH.
  The process must have `CAP_NET_ADMIN` capability (or run as root).
  The configured routing table and interface must exist.

d2ip performs validation at startup and before each pipeline run. If a backend
is unavailable, the pipeline will log a warning and skip routing for affected
policies while continuing with the rest of the pipeline.
