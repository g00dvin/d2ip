# Agent 09 — Orchestrator

**Package:** `internal/orchestrator`
**Owns:** pipeline composition, lifecycle, single-flight, run accounting.

## Contract

```go
type PipelineRequest struct {
    DryRun       bool
    ForceResolve bool   // ignore cache TTL, re-resolve everything
    SkipRouting  bool
}

type PipelineReport struct {
    RunID       int64
    Domains     int
    Stale       int
    Resolved    int
    Failed      int
    IPv4Out     int
    IPv6Out     int
    Export      exporter.ExportReport
    RoutingPlan *routing.Plan
    Duration    time.Duration
}

type Orchestrator interface {
    Run(ctx context.Context, req PipelineRequest) (PipelineReport, error)
    Status() RunStatus
    Cancel()
}
```

## Tasks

1. **Wiring**: take all agent interfaces in the constructor; never instantiate
   them itself. `cmd/d2ip` does the wiring.
2. **Single-flight**: `sync.Mutex` + atomic "running" flag. Concurrent `Run`
   returns a sentinel error `ErrBusy` with the in-flight `RunID`.
3. **Step execution**: each step in a named function; emit a structured log
   line and a metric per step boundary.
4. **Cancellation**: derive a `ctx, cancel` from the parent; `Cancel()` calls
   it. Persist the run row with `status='canceled'`.
5. **Run history**: insert a `runs` row at start, update on completion.
6. **Backpressure between Resolver and Cache**: the orchestrator owns the
   buffered channel and the drain goroutine; closes the in-channel on resolver
   exhaustion, then waits for the drain to commit.
7. **Tests**: full table-driven test with mocked agent interfaces. Cases:
   happy path, resolver error mid-stream, cache failure (must not leak
   goroutines), cancel mid-resolve.

## Acceptance

* Two simultaneous `Run` calls: first proceeds, second returns `ErrBusy`.
* Cancel during resolve aborts within `<resolver.timeout + 100ms>`.
* `runs` table always has a terminal status row per `RunID`.
