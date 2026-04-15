# Agent 01 — Source Agent

**Package:** `internal/source`
**Owns:** local copy of `dlc.dat`, integrity, refresh policy.

## Contract

```go
type Version struct {
    SHA256    string
    Size      int64
    FetchedAt time.Time
    ETag      string
}

type DLCStore interface {
    Get(ctx context.Context, maxAge time.Duration) (path string, v Version, err error)
    ForceRefresh(ctx context.Context)             (path string, v Version, err error)
    Info() Version
}
```

## Tasks

1. **HTTP fetch** with `If-None-Match` (ETag) and `If-Modified-Since`. Configurable
   `source.url`, `source.http_timeout`. On `304 Not Modified` keep current file.
2. **Integrity**: stream into a temp file in the same dir as `cache_path`, compute
   sha256 inline, then `os.Rename` to final path (atomic).
3. **Fallback**: on network error, if a local file exists return it with the cached
   `Version` and a `WARN` log. Never delete a known-good file.
4. **Concurrency**: serialize refreshes with an internal mutex; `Get` is read-only.
5. **Tests**: hermetic via `httptest.Server`. Cases: 200 fresh, 304, 500, partial
   body, sha mismatch (must reject), missing local + offline (must error).

## Acceptance

* `Get` with `maxAge=0` always returns cached path without network.
* `ForceRefresh` writes new file or fails atomically.
* `Info()` reflects on-disk truth after restart (load metadata from sidecar
  `dlc.dat.meta.json`).
