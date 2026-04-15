# Agent 08 — Config Agent

**Package:** `internal/config`
**Owns:** load, validate, expose, persist Web overrides.

## Tasks

1. **Schema** in `Config struct` (mirrors `docs/CONFIG.md`). Every field has a
   default in code; YAML/ENV are overrides only.
2. **Load order** (highest wins): `ENV (D2IP_*) > kv_settings table > defaults`.
   The optional `config.yaml` is *seed* for first run only.
3. **Validation**: dedicated `Validate()` returning `[]error`; the binary
   refuses to start on validation failure.
4. **Hot reload** for non-listener fields:
   * `PATCH /settings` writes to `kv_settings`, recomputes effective config,
     publishes via `Subscribe() <-chan Snapshot`.
   * Resolver/Aggregator/Router pick up changes between pipeline runs (never
     mid-run).
5. **Redaction**: any field tagged `secret:"true"` (currently none, future-proof)
   is masked in `GET /settings`.
6. **Tests**: round-trip ENV → struct, kv override priority, validation cases,
   subscribe receives updates exactly once per change.

## Acceptance

* `D2IP_RESOLVER_QPS=500 d2ip` produces `cfg.Resolver.QPS == 500` regardless of
  YAML or kv contents.
* Setting an invalid value via API returns `400` and does not mutate state.
