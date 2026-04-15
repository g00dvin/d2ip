# d2ip — Go package layout

```
d2ip/
├── cmd/
│   └── d2ip/
│       └── main.go                  # entrypoint, wiring, signals
├── internal/
│   ├── config/                      # Config Agent
│   │   ├── config.go                # struct + defaults
│   │   ├── load.go                  # viper, ENV>kv>defaults
│   │   ├── store.go                 # kv_settings persistence
│   │   └── validate.go
│   ├── source/                      # Source Agent
│   │   ├── store.go                 # DLCStore impl
│   │   ├── http.go                  # ETag + sha256
│   │   └── atomic.go
│   ├── domainlist/                  # Domain Agent
│   │   ├── dlcpb/                   # generated protobuf (dlc.proto)
│   │   ├── parser.go                # Unmarshal GeoSiteList
│   │   ├── selector.go              # category + @attrs filter
│   │   ├── normalize.go             # lowercase + idna punycode
│   │   └── rules.go                 # Rule, RuleType
│   ├── resolver/                    # Resolver Agent
│   │   ├── resolver.go              # Resolver interface
│   │   ├── pool.go                  # worker pool
│   │   ├── miekg.go                 # miekg/dns implementation
│   │   ├── retry.go                 # exponential backoff
│   │   └── ratelimit.go
│   ├── cache/                       # Cache Agent (SQLite)
│   │   ├── sqlite.go                # open + PRAGMA + migrations
│   │   ├── domains.go
│   │   ├── records.go
│   │   ├── snapshot.go
│   │   └── runs.go
│   ├── aggregator/                  # Aggregation Agent
│   │   ├── aggregator.go
│   │   └── radix.go                 # backed by pkg/cidr
│   ├── exporter/                    # Export Agent
│   │   ├── exporter.go
│   │   └── atomic.go
│   ├── routing/                     # Routing Agent
│   │   ├── router.go                # Router interface, Plan
│   │   ├── nft.go                   # nftables backend
│   │   ├── iproute2.go              # ip route backend (table 100)
│   │   ├── state.go                 # state.json snapshot
│   │   └── caps.go                  # capability self-check
│   ├── orchestrator/
│   │   ├── orchestrator.go          # pipeline
│   │   └── singleflight.go
│   ├── scheduler/
│   │   └── scheduler.go             # dlc refresh + resolve cycles
│   ├── api/
│   │   ├── router.go                # chi mux
│   │   ├── handlers_pipeline.go
│   │   ├── handlers_categories.go
│   │   ├── handlers_settings.go
│   │   ├── handlers_routing.go
│   │   └── middleware.go
│   ├── metrics/
│   │   └── prom.go
│   └── logging/
│       └── log.go                   # zerolog setup
├── pkg/
│   ├── cidr/                        # reusable CIDR aggregation
│   │   ├── radix.go
│   │   ├── merge.go
│   │   └── aggregate.go
│   └── dnsx/                        # reusable DNS helpers (CNAME chain)
│       └── chain.go
├── proto/
│   └── dlc.proto                    # source of dlcpb generation
├── migrations/
│   └── 0001_init.sql                # CREATE TABLE / indexes
├── deploy/
│   ├── Dockerfile
│   ├── entrypoint.sh
│   └── nftables.example.nft
├── web/                             # optional minimal UI (static)
│   └── index.html
├── docs/
│   ├── ARCHITECTURE.md
│   ├── SCHEMA.md
│   ├── CONFIG.md
│   ├── PACKAGES.md
│   ├── API.md
│   ├── PIPELINE.md
│   ├── PLAN.md
│   ├── RISKS.md
│   └── agents/
│       ├── 01-source.md
│       ├── 02-domainlist.md
│       ├── 03-resolver.md
│       ├── 04-cache.md
│       ├── 05-aggregator.md
│       ├── 06-exporter.md
│       ├── 07-routing.md
│       ├── 08-config.md
│       └── 09-orchestrator.md
├── go.mod
└── README.md
```

## Dependency rules

* `cmd/d2ip` → `internal/*` (wiring only).
* `internal/orchestrator` depends only on **interfaces** from sibling packages.
* `internal/api` depends on the orchestrator and on read‑only views of cache/config.
* `pkg/*` is import‑safe from anywhere; never imports `internal/*`.
* No package imports another package's concrete struct that would create a cycle —
  contracts live next to consumers (orchestrator owns the interfaces it calls).
