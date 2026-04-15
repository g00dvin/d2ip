# Agent 06 — Export Agent

**Package:** `internal/exporter`
**Owns:** `ipv4.txt` / `ipv6.txt` generation with atomic write.

## Contract

```go
type ExportReport struct {
    IPv4Path   string
    IPv6Path   string
    IPv4Count  int
    IPv6Count  int
    IPv4Digest string // sha256
    IPv6Digest string
    Unchanged  bool
}

type Exporter interface {
    Write(ctx context.Context, v4 []netip.Prefix, v6 []netip.Prefix) (ExportReport, error)
}
```

## Tasks

1. **Format**: one prefix per line, no comments, LF newlines, trailing newline.
2. **Sort & dedup** before write (Aggregator already sorts; verify with assert).
3. **Atomic write**:
   * `os.CreateTemp(dir, "ipv4-*.tmp")`
   * write + `fsync` + `Close`
   * `os.Rename(tmp, final)` (atomic on same FS)
   * `fsync(parent dir)` for crash safety.
4. **Unchanged detection**: compute sha256 while writing; compare with previous
   file's sha (sidecar `ipv4.txt.sha256`). If equal, skip rename (still report
   `Unchanged=true`).
5. **Permissions**: `0644` files, parent dir created with `0755` if missing.
6. **Tests**: temp-dir based; verify atomic rename, idempotency, large input
   (1M prefixes) memory profile (must stream-write, no full string buffer).

## Acceptance

* Concurrent readers never observe a partial file.
* No-op run produces `Unchanged=true` and zero disk writes besides sha sidecar.
