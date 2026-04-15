# Agent 02 — Domain Agent

**Package:** `internal/domainlist` (+ generated `internal/domainlist/dlcpb`)
**Owns:** dlc.dat parsing, category selection, attribute filtering, normalization.

## Background

`dlc.dat` is a protobuf-encoded `GeoSiteList` (see `proto/dlc.proto`). Each
`GeoSite` has `country_code` (e.g. `"ru"`, addressed as `geosite:ru`) and a list
of `Domain` rules. `Domain.Type ∈ {Plain, Regex, RootDomain, Full}`. Attributes
are `(key, bool|int)` pairs prefixed by `@` in the source files.

## Contract

```go
type RuleType uint8
const (
    RuleFull       RuleType = iota // Domain.Type=Full
    RuleRootDomain                 // Domain.Type=RootDomain (suffix)
    RulePlain                      // Domain.Type=Plain    (keyword) — UNRESOLVABLE
    RuleRegex                      // Domain.Type=Regex    (regex)   — UNRESOLVABLE
)

type Rule struct {
    Type  RuleType
    Value string                    // already lowercase + punycode for Full/RootDomain
    Attrs map[string]any
    Cat   string                    // origin category (for diagnostics)
}

type CategorySelector struct {
    Code  string                    // "geosite:ru" — case-insensitive
    Attrs []string                  // optional @attr filter (AND)
}

type ListProvider interface {
    Load(ctx context.Context, dlcPath string) error
    Select(sel []CategorySelector) ([]Rule, error)
    Categories() []string           // discovered codes (for UI)
}
```

## Tasks

1. **Generate Go from `proto/dlc.proto`** (`buf` or `protoc-gen-go`); commit
   generated code under `dlcpb/`. CI verifies it's up to date.
2. **Parse**: `proto.Unmarshal` straight from disk via `os.ReadFile` (file is small
   — single-digit MB). Build an in-memory `map[string]*GeoSite` keyed by lowercase
   `country_code`.
3. **Selection**:
   * `Code` matches `geosite:<code>` or bare `<code>` (case-insensitive).
   * `Attrs` AND-filter: a domain passes if **all** listed attrs are present and
     truthy (`bool=true` or `int!=0`).
   * Empty `Attrs` ⇒ pass all domains.
4. **Normalization** (only for `Full`/`RootDomain`):
   * `strings.ToLower`
   * trim trailing `.`
   * IDN → punycode via `golang.org/x/net/idna` (`Lookup` profile);
     drop entries that fail IDNA with a counter.
5. **Dedup**: stable order, dedup on `(Type, Value)` after normalization.
6. **`include:` mechanics**: dlc.dat is *flat* (the v2fly tooling already
   resolves includes at build time), so the parser does not re-resolve includes —
   document this and assert no surprise references in tests.
7. **Tests**: golden-file test against a checked-in tiny dlc.dat fixture with
   `cn`, `google`, attributed `@ads` entries, IDN domains.

## Acceptance

* Selecting an unknown category returns an empty slice + named error.
* Attribute filter `[@cn]` on `geosite:google` yields only the CN-tagged subset.
* IDN like `пример.рф` returns punycode `xn--e1afmkfd.xn--p1ai`.
* `Plain`/`Regex` rules surface in `Select` but are flagged so the resolver can skip them.
