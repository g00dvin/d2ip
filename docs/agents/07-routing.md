# Agent 07 — Routing Agent

**Package:** `internal/routing`
**Owns:** host route/firewall mutation, isolation, dry-run, rollback.

> **Risk class: HIGH.** Only this agent mutates external state on the host.
> Every change must be planned, owned, and reversible.

## Contract

```go
type Family uint8
const (
    FamilyV4 Family = iota
    FamilyV6
)

type Plan struct {
    Family Family
    Add    []netip.Prefix
    Remove []netip.Prefix
}

type RouterState struct {
    Backend   string
    AppliedAt time.Time
    V4        []netip.Prefix
    V6        []netip.Prefix
}

type Router interface {
    Caps() error                     // self-check capabilities; nil = OK
    Plan(ctx context.Context, desired []netip.Prefix, f Family) (Plan, error)
    Apply(ctx context.Context, p Plan) error
    Snapshot() RouterState
    Rollback(ctx context.Context) error
    DryRun(ctx context.Context, desired []netip.Prefix, f Family) (Plan, string /*human diff*/, error)
}
```

## Backends

### nftables (preferred)

* Bootstrap (idempotent, on first apply):
  ```nft
  table inet d2ip {
      set d2ip_v4 { type ipv4_addr; flags interval; }
      set d2ip_v6 { type ipv6_addr; flags interval; }
  }
  ```
* Apply diff via `nft -f -` script: `delete element` for `Plan.Remove`,
  `add element` for `Plan.Add`. Single transaction (nft is atomic per script).
* The user wires their own routing/firewall rule referencing the set, e.g.
  `ip rule add fwmark 0x1 lookup 100`, or a verdict `goto vpn` on set match.
  We never touch `main` table or `nat` chain.

### iproute2 (fallback)

* `ip -4 route show table 100` to enumerate; diff against desired.
* `ip -4 route add <prefix> dev <iface> table 100` / `del`.
* `iface` (or `via`) is configurable; refuses to apply if neither present.

## Tasks

1. **Caps self-check**: at startup, `cap_get_proc()` for `CAP_NET_ADMIN`; verify
   `nft` (or `ip`) binary present. Refuse `Apply` with a clear error if missing.
2. **Plan**: read current state from kernel, compute set difference vs `desired`.
   Plan must be deterministic and minimal.
3. **State file**: `/var/lib/d2ip/state.json` — written *after* successful apply,
   includes backend, timestamp, and applied prefixes. Used to scope rollback.
4. **Rollback**: removes only entries listed in state file. Never enumerates the
   set/table and removes everything (would clobber user-added entries).
5. **Dry-run**: returns plan + a human-readable unified diff (`+ 1.2.3.0/24`,
   `- 5.6.7.0/24`). Used by `POST /pipeline/dry-run` and `POST /routing/dry-run`.
6. **Naming convention**: every owned object has `d2ip` in its name. Comments on
   iproute2 routes via `proto static` + a custom `realm` if available.
7. **Concurrency**: a process-wide mutex around Apply/Rollback. The orchestrator
   already serializes pipeline runs, but external API endpoints could overlap.
8. **Tests**: split into unit (plan logic over fake state) and integration
   (build tag `routing_integration`, requires NET_ADMIN; CI runs in a netns).

## Acceptance

* `Apply` after `Apply` with the same desired set is a no-op (zero `nft`
  commands issued).
* `Rollback` after a failed `Apply` returns the kernel to pre-apply state.
* Disabling routing (`routing.enabled=false`) means `Plan/Apply/Rollback` all
  return immediately with no syscalls.
