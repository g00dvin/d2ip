# d2ip — Multi-Policy Routing Design

**Date:** 2026-04-24  
**Status:** Approved  
**Scope:** Multi-policy routing, new export formats, additional data sources

---

## 1. Problem Statement

d2ip currently supports only a single routing policy: all resolved IPs go into one nftables set or one iproute2 table. This is insufficient for policy-based routing (PBR) use cases like:
- Route streaming traffic (Netflix, YouTube) through WAN2
- Route corporate traffic (Google Workspace, Microsoft 365) through a VPN tunnel
- Block malicious/phishing domains in a dedicated nftables set

**Goal:** Allow multiple independent routing policies, each mapping specific geosite categories to its own routing backend, table/set, and lifecycle.

---

## 2. Architecture

### 2.1 High-level flow

The pipeline stays unchanged through resolution and caching. It forks at aggregation/export:

```
fetch(dlc.dat) → parse → normalize → resolve → cache → snapshot
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
              ┌─────────┐    ┌─────────┐    ┌─────────┐
              │Policy A │    │Policy B │    │Policy C │
              │(stream) │    │(corp)   │    │(block)  │
              └────┬────┘    └────┬────┘    └────┬────┘
                   │              │              │
              aggregate      aggregate      aggregate
              export         export         export
              route          route          route
                   │              │              │
              ip table 200   ip table 300   nft set block
```

### 2.2 Key invariants

- A category can belong to **multiple policies** (one domain → many tables/sets).
- A policy with zero categories is a no-op, not an error.
- Overlapping prefixes across policies are fine; nftables/iproute2 handle it.
- Each policy has its own state file, export directory, and lifecycle.
- Failed policy application does not block other policies.

---

## 3. Config Schema

### 3.1 New `routing.policies` array

Replace the flat `routing` section with a `policies` array. **No legacy migration** — this is a breaking config change.

```yaml
routing:
  enabled: true
  state_dir: /var/lib/d2ip/state

  policies:
    - name: streaming
      enabled: true
      categories: ["geosite:netflix", "geosite:youtube", "geosite:disney"]
      backend: iproute2
      table_id: 200
      iface: eth1
      dry_run: false
      export_format: plain   # plain | ipset | json | nft | iptables | bgp | yaml
      # Optional: per-policy aggregation overrides
      aggregation:
        level: balanced
        v4_max_prefix: 16
        v6_max_prefix: 32

    - name: corporate
      enabled: true
      categories: ["geosite:google", "geosite:microsoft"]
      backend: iproute2
      table_id: 300
      iface: wg0
      dry_run: false
      export_format: plain

    - name: blocklist
      enabled: true
      categories: ["geosite:malware", "geosite:phishing"]
      backend: nftables
      nft_table: inet d2ip
      nft_set_v4: block_v4
      nft_set_v6: block_v6
      dry_run: false
      export_format: nft
```

### 3.2 Validation rules

| Field | Rule |
|-------|------|
| `name` | Unique, non-empty, `[a-z0-9_-]+` |
| `categories` | At least one code per enabled policy; each must contain `:` |
| `backend` | `none`, `nftables`, or `iproute2` |
| `table_id` | Unique across all iproute2 policies; range [1, 252] |
| `iface` | Required when `backend=iproute2` |
| `nft_set_v4/v6` | Unique across nftables policies in the same table |
| `export_format` | `plain` (default), `ipset`, `json`, `nft`, `iptables`, `bgp`, `yaml` |

### 3.3 No automatic `ip rule` creation

**d2ip manages routes only.** It does NOT create `ip rule` entries. The operator is responsible for creating their own rules with custom 5-tuple criteria (fwmark, source, destination, protocol, port) that direct traffic to the policy's table.

Example operator-managed rule:
```bash
# Mark packets from VLAN 10 and route via table 200
ip rule add from 192.168.10.0/24 lookup 200
# Or mark-based
iptables -t mangle -A PREROUTING -p tcp --dport 443 -j MARK --set-mark 0x100
ip rule add fwmark 0x100 lookup 200
```

---

## 4. Pipeline Changes

### 4.1 Per-policy aggregation and export

After `Cache.Snapshot()`, group domains by policy, then aggregate and export per policy:

```go
for _, policy := range cfg.Routing.Policies {
    if !policy.Enabled {
        continue
    }
    policyDomains := filterDomainsByCategories(allDomains, policy.Categories)
    policyIPv4 := extractIPv4(policyDomains)
    policyIPv6 := extractIPv6(policyDomains)

    ipv4Out := aggregator.AggregateV4(policyIPv4, policy.AggLevel, policy.V4MaxPrefix)
    ipv6Out := aggregator.AggregateV6(policyIPv6, policy.AggLevel, policy.V6MaxPrefix)

    report := exporter.WritePolicy(policy, ipv4Out, ipv6Out)
    if !policy.DryRun {
        router.ApplyPolicy(ctx, policy, ipv4Out, ipv6Out)
    }
}
```

### 4.2 Export directory structure

```
/var/lib/d2ip/out/
├── streaming/
│   ├── ipv4.txt
│   └── ipv6.txt
├── corporate/
│   ├── ipv4.txt
│   └── ipv6.txt
└── blocklist/
    ├── ipv4.txt
    └── ipv6.txt
```

### 4.3 Pipeline report

```json
{
  "run_id": 42,
  "total_duration_ms": 45000,
  "policies": [
    {
      "name": "streaming",
      "domains": 1500,
      "resolved": 1480,
      "failed": 20,
      "ipv4_out": 2847,
      "ipv6_out": 1203,
      "duration_ms": 12000
    },
    {
      "name": "corporate",
      "domains": 800,
      "resolved": 790,
      "failed": 10,
      "ipv4_out": 942,
      "ipv6_out": 560,
      "duration_ms": 8000
    }
  ]
}
```

---

## 5. Backend Abstraction

### 5.1 New Router interface

```go
type PolicyRouter interface {
    ApplyPolicy(ctx context.Context, policy config.Policy, v4, v6 []netip.Prefix) error
    DryRunPolicy(ctx context.Context, policy config.Policy, v4, v6 []netip.Prefix) (Plan, string, error)
    RollbackPolicy(ctx context.Context, policyName string) error
    SnapshotPolicy(policyName string) RouterState
    Caps(policy config.Policy) error
}
```

### 5.2 iproute2 backend

- Creates routes in the policy's `table_id` via `ip route replace` (atomic updates).
- Does NOT create `ip rule` entries — the operator manages rules.
- State persisted to `/var/lib/d2ip/state/{policy_name}.json`.
- Rollback removes all routes in the policy's table that were previously applied.

### 5.3 nftables backend

- Creates/flushes named sets per policy in the specified table.
- d2ip only manages the set contents (flush + add element).
- The operator writes their own `nft` rules referencing these sets.

**Example operator nftables rules:**
```nft
# /etc/nftables.conf
 table inet d2ip {
    set block_v4 {
        type ipv4_addr
        flags interval
    }
    set block_v6 {
        type ipv6_addr
        flags interval
    }
    set streaming_v4 {
        type ipv4_addr
        flags interval
    }
    set streaming_v6 {
        type ipv6_addr
        flags interval
    }

    # Block policy: drop traffic to malicious IPs
    chain input {
        type filter hook input priority 0; policy accept;
        ip saddr @block_v4 counter drop
        ip6 saddr @block_v6 counter drop
    }

    # Streaming policy: mark for PBR
    chain prerouting {
        type filter hook prerouting priority mangle; policy accept;
        ip daddr @streaming_v4 meta mark set 0x100
        ip6 daddr @streaming_v6 meta mark set 0x100
    }
}
```

---

## 6. Export Formats

Each policy can specify its own export format:

| Format | File extension | Example output |
|--------|---------------|----------------|
| `plain` | `.txt` | One CIDR per line (`1.2.3.0/24`) |
| `ipset` | `.ipset` | `add {policy}_v4 1.2.3.0/24` |
| `json` | `.json` | `{"v4":["..."],"v6":["..."],"policy":"streaming"}` |
| `nft` | `.nft` | `add element inet d2ip streaming_v4 { 1.2.3.0/24 }` |
| `iptables` | `.iptables` | `-A OUTPUT -d 1.2.3.0/24 -j DROP` |
| `bgp` | `.bgp` | See Section 8.1 |
| `yaml` | `.yaml` | See Section 8.2 |

### 6.1 BGP feed format

MRT/BGP style text feed (one prefix per line with origin AS if known):

```
1.2.3.0/24	 streaming
2001:db8::/32	 streaming
```

Tab-separated: prefix, policy name. Optional third column for ASN.

### 6.2 YAML format

```yaml
policy: streaming
backend: iproute2
generated_at: "2026-04-24T14:32:01Z"
count:
  v4: 2847
  v6: 1203
prefixes:
  v4:
    - 1.2.3.0/24
    - 5.6.7.0/24
  v6:
    - 2001:db8::/32
```

---

## 7. Data Sources

### 7.1 ASN source (`asn:12345`)

- Download ASN-to-prefix mappings from RIPE RIS or Team Cymru.
- Cache ASN data locally with configurable TTL.
- Resolve all announced prefixes for the ASN.
- Output goes through the same aggregation pipeline.

```yaml
categories:
  - code: "asn:15169"    # Google ASN
  - code: "asn:8075"     # Microsoft ASN
```

**Data download:**
- Source: `https://stat.ripe.net/data/announced-prefixes/data.json?resource=AS{asn}`
- Cache path: `/var/lib/d2ip/asn_cache/`
- Refresh interval: 24h (configurable)

### 7.2 Custom list source (`custom:mylist`)

- Upload custom domain lists via API or config file reference.
- Format: one domain per line (plain text).
- Stored in `/var/lib/d2ip/custom_lists/{name}.txt`.
- Treated identically to geosite categories in the pipeline.

```yaml
categories:
  - code: "custom:corporate-vpn"
  - code: "custom:ad-block"
```

### 7.3 GeoIP source (`geo:US`)

- Support MaxMind GeoLite2/GeoIP2 MMDB format.
- Download MMDB database on startup (or use system-provided file).
- Given a country code, resolve all IP ranges for that country.
- Requires geoip lookup in the aggregation stage.

```yaml
categories:
  - code: "geo:RU"   # All Russian IP ranges
  - code: "geo:CN"   # All Chinese IP ranges
```

**Data download:**
- Source: MaxMind GeoLite2-Country (free, requires license key)
- Config: `source.geoip.license_key` + `source.geoip.edition` (GeoLite2-Country, GeoIP2-Country)
- Cache path: `/var/lib/d2ip/geoip/GeoLite2-Country.mmdb`
- Refresh interval: weekly

**Note:** GeoIP categories bypass DNS resolution — they go straight from MMDB to aggregation, since they already are IP ranges.

---

## 8. API Changes

### 8.1 New endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/policies` | List all policies with status |
| GET | `/api/policies/{name}` | Get single policy details |
| POST | `/api/policies` | Create new policy |
| PUT | `/api/policies/{name}` | Update policy |
| DELETE | `/api/policies/{name}` | Delete policy |
| POST | `/api/policies/{name}/run` | Trigger pipeline for single policy |
| POST | `/api/policies/{name}/dry-run` | Dry run single policy |
| POST | `/api/policies/{name}/rollback` | Rollback single policy |
| GET | `/api/policies/{name}/snapshot` | Get policy routing snapshot |

### 8.2 Updated endpoints

| Method | Path | Change |
|--------|------|--------|
| POST | `/pipeline/run` | Now accepts `policy` field to run single policy; omit for all |
| GET | `/pipeline/status` | Returns per-policy status |
| GET | `/api/pipeline/history` | Includes per-policy breakdown |

### 8.3 Response examples

**GET /api/policies**
```json
{
  "policies": [
    {
      "name": "streaming",
      "enabled": true,
      "categories": ["geosite:netflix", "geosite:youtube"],
      "backend": "iproute2",
      "table_id": 200,
      "iface": "eth1",
      "dry_run": false,
      "export_format": "plain",
      "ipv4_count": 2847,
      "ipv6_count": 1203,
      "last_applied": "2026-04-24T14:32:01Z"
    }
  ]
}
```

---

## 9. UI Changes

### 9.1 New "Policies" page

Replaces the current "Routing" page as the primary traffic control interface.

**Policy List Table:**
- Columns: Name, Status (enabled/disabled), Categories count, Backend, Table/Set, IPv4 count, IPv6 count, Last applied, Actions
- Actions per row: Edit, Run, Dry Run, Rollback, Disable/Enable, Delete
- Bulk action: Run all, Disable all

**Add/Edit Policy Drawer:**
- Name input (validates `[a-z0-9_-]+`)
- Category multi-select with search
- Backend picker (nftables / iproute2)
- Dynamic fields based on backend:
  - iproute2: table_id, iface
  - nftables: nft_table, nft_set_v4, nft_set_v6
- Export format dropdown
- Aggregation overrides (optional, inherits global defaults)
- Dry run toggle
- Enabled toggle

**Policy Detail Panel:**
- Shows current prefixes (top 100 with "show more")
- Diff view (planned changes from last dry run)
- Export preview (raw output)

### 9.2 Dashboard updates

- Policy summary cards (top N policies by prefix count)
- Quick enable/disable toggles
- Policy-specific run buttons

### 9.3 Categories page update

- Show badge indicating which policies each category belongs to
- "Add to policy" dropdown on category rows
- Filter categories by policy membership

---

## 10. State & Storage

### 10.1 Policy state files

```
/var/lib/d2ip/state/
├── streaming.json
├── corporate.json
└── blocklist.json
```

Each file:
```json
{
  "policy_name": "streaming",
  "backend": "iproute2",
  "table_id": 200,
  "applied_at": "2026-04-24T14:32:01Z",
  "v4": ["1.2.3.0/24", "5.6.7.0/24"],
  "v6": ["2001:db8::/32"]
}
```

### 10.2 SQLite schema additions

**New table: `policies`**
```sql
CREATE TABLE policies (
    name TEXT PRIMARY KEY,
    config_json TEXT NOT NULL,  -- serialized Policy config
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
```

**New table: `policy_runs`** (per-policy run history)
```sql
CREATE TABLE policy_runs (
    id INTEGER PRIMARY KEY,
    policy_name TEXT NOT NULL,
    run_id INTEGER NOT NULL,
    started_at INTEGER NOT NULL,
    ended_at INTEGER,
    status TEXT NOT NULL,
    domains INTEGER DEFAULT 0,
    resolved INTEGER DEFAULT 0,
    failed INTEGER DEFAULT 0,
    ipv4_out INTEGER DEFAULT 0,
    ipv6_out INTEGER DEFAULT 0,
    error TEXT,
    FOREIGN KEY (policy_name) REFERENCES policies(name)
);
```

---

## 11. Metrics

New Prometheus metrics:

```
d2ip_policy_domains_total{policy}           # Domains matched per policy
d2ip_policy_prefixes{policy,family}         # Prefixes per policy per family
d2ip_policy_apply_duration_seconds{policy}  # Time to apply policy
d2ip_policy_apply_total{policy,op}          # Apply operations count (add/remove)
d2ip_policy_enabled{policy}                 # 1 if enabled, 0 if disabled
```

---

## 12. Failure Model

- **Per-policy isolation:** A failure in one policy (e.g., invalid table_id) does not affect others.
- **Partial success:** Pipeline report indicates which policies succeeded and which failed.
- **Rollback:** Per-policy rollback restores only that policy's previous state.
- **State recovery:** On startup, d2ip reads all policy state files and reconciles with actual kernel state.

---

## 13. Out of Scope (for this iteration)

- Operator-managed `ip rule` creation (documented as manual step)
- Real-time policy updates without full pipeline run
- Policy priority/ordering (nftables/iproute2 handle overlap)
- IPv4/IPv6 policy split (a policy always handles both families)
- Live policy preview before applying (dry-run covers this)
- Dynamic category-to-policy auto-assignment

---

## 14. Migration Path

**This is a breaking config change.** There is no automatic legacy migration.

Users upgrading must:
1. Convert their existing `routing:` section into a single policy under `routing.policies:`
2. Optionally split into multiple policies
3. Update any external scripts that read `/var/lib/d2ip/out/ipv4.txt` to use `/var/lib/d2ip/out/{policy_name}/ipv4.txt`

---

## 15. Implementation Order

1. **Config & validation** — new Policy struct, validation rules, loader
2. **Backend abstraction** — PolicyRouter interface, per-policy state files
3. **Pipeline changes** — per-policy aggregation, export, routing
4. **API endpoints** — CRUD for policies, per-policy run/rollback
5. **Export formats** — ipset, json, nft, iptables, bgp, yaml
6. **Data sources** — ASN, custom lists, GeoIP
7. **UI** — Policies page, dashboard updates, category badges
8. **Tests & docs** — integration tests, nftables examples, config migration guide
