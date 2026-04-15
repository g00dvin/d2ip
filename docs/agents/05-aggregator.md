# Agent 05 — Aggregation Agent

**Package:** `internal/aggregator`, algorithm in `pkg/cidr`
**Owns:** CIDR aggregation per family with tunable aggressiveness.

## Contract

```go
type Aggressiveness uint8
const (
    AggOff Aggressiveness = iota
    AggConservative
    AggBalanced
    AggAggressive
)

type Aggregator interface {
    AggregateV4(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix
    AggregateV6(in []netip.Addr, level Aggressiveness, maxPrefix int) []netip.Prefix
}
```

## Tasks

1. **Radix tree** in `pkg/cidr`:
   * Insert each `/32` (or `/128`) addr.
   * Walk bottom-up: if both children of a node are present and it's a *full*
     subtree, replace with the parent prefix (lossless merge).
   * For `Balanced`/`Aggressive`, allow merging when the populated leaf ratio
     in a subtree ≥ threshold (75 % / 50 %). Stop climbing at `maxPrefix`.
2. **Fallback** `sort+merge` path used when input is small (< 64 addrs) or radix
   build is disabled — same external behavior.
3. **Determinism**: output sorted by `prefix.Addr().As16()` ascending; identical
   input ⇒ identical output (important for diff-based routing apply).
4. **Boundary safety**: never emit `0.0.0.0/0`, `::/0`, or anything broader than
   `maxPrefix`. Document this as a hard guarantee in tests.
5. **Tests**: property test (random `/32`s, output covers all inputs at every
   level; aggressive output is a superset of conservative). Snapshot tests on a
   fixed corpus.

## Acceptance

* `AggOff` returns `len(in)` `/32` prefixes verbatim.
* `AggConservative` is **lossless**: every input addr remains covered, no extra
  addrs introduced.
* `AggAggressive` may introduce addrs but never produces a prefix shorter than
  `maxPrefix`.
