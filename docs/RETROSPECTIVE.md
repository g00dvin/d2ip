# Retrospective — Iterations 0-5

**Date:** 2026-04-15  
**Scope:** Full project implementation (bootstrap → production-ready routing)

## What Went Well ✅

### Architecture Decisions

1. **Interface-Based Isolation** — Огромный успех
   - Агенты общаются через Go interfaces
   - Никогда не импортируют конкретные типы друг друга
   - Orchestrator легко тестируется с моками
   - **Урок:** Держать границы строгими с самого начала

2. **Single-Flight Pattern** — Простое и надежное
   - `atomic.Bool` в orchestrator
   - Предотвращает race conditions без сложных мьютексов
   - Concurrent triggers получают `ErrBusy` с runID
   - **Урок:** Простейшее решение часто лучшее

3. **Config Precedence (ENV > kv > YAML > defaults)** — Работает идеально
   - ENV для Docker/K8s
   - kv_settings для runtime (готово для Web UI)
   - YAML для dev
   - **Урок:** Система precedence должна быть с Iteration 0

4. **Atomic Writes (temp → rename)** — Ни одной потери данных
   - Все файлы: exporter, source, state
   - fsync родительской директории
   - Unchanged detection через SHA256
   - **Урок:** Атомарность везде, где state касается файлов

### Agent Strategy (Cost Optimization)

**Итоговая экономия: 58% ($0.35 из $0.60)**

| Strategy | When | Result |
|----------|------|--------|
| **Sonnet first** | Boilerplate, API, config | 100% success, 60% cheaper |
| **Opus for HIGH RISK** | Routing, concurrency, algorithms | 100% success, worth the cost |
| **Manual** | Trivial (<50 lines) | Fastest для простых задач |

**Ключевые инсайты:**
- Iteration 4: 3 sonnet agents — perfect results, 89k tokens
- Iteration 5: 1 opus (routing) + 1 sonnet (API) — balanced, 73k tokens
- Sonnet недооценен для well-specified tasks
- Opus критичен для kernel manipulation

### Technical Wins

1. **CIDR Radix Tree** — Работает отлично после фикса
   - Lossless aggregation
   - IPv4 byte offset bug (bytes 12-15, не 0-3) — урок усвоен
   - 10/10 tests pass

2. **Routing Safety** — Все safety features работают
   - Disabled by default
   - Idempotent apply
   - State-scoped rollback (не трогает user entries)
   - Dry-run показывает точный diff

3. **DNS Resolver** — Worker pool + rate limiting
   - No memory leaks (пока)
   - CNAME following (max 8 hops)
   - Exponential backoff + jitter

4. **SQLite Cache** — WAL mode, batch upserts
   - ~1000 rows per transaction
   - Composite uniqueness для idempotent upserts
   - Migrations embedded

## What Could Be Better ⚠️

### Missing Features

1. **Integration Tests in netns** (build tag `routing_integration`)
   - Unit tests 18/18 pass, но нет real kernel testing
   - Routing может brick network — нужна изоляция
   - **Action:** Iteration 6 — netns integration tests

2. **Prometheus Metrics Incomplete**
   - Resolver missing: `dns_resolve_total`, `dns_resolve_duration`
   - **Action:** Iteration 6 — complete metrics

3. **No goleak Tests**
   - Orchestrator и resolver используют goroutines
   - **Action:** Add `go.uber.org/goleak`

4. **Web UI Not Implemented**
   - Planned для Iteration 6
   - kv_settings backend готов, frontend нет
   - **Action:** Minimal HTMX UI

### Technical Debt

1. **iproute2 Backend** — Needs `Iface` config field
   - Сейчас hardcoded requirement
   - **Action:** Add `RoutingConfig.Iface string`

2. **nft Plain-Text Parsing** — Brittle
   - `parseNftSet` парсит `elements = { ... }` текст
   - Better: `nft --json` mode
   - **Action:** Consider JSON parser

3. **DNS TTL Ignored** — Только internal cache TTL
   - По дизайну, но может быть неожиданным
   - **Action:** Document clearly

### Process Learnings

1. **Go Version Mismatch** — Local 1.19 vs required 1.22
   - Workaround: Docker для всех команд
   - **Урок:** Проверять версии в CI, не полагаться на local

2. **Compact Context Loss** — После compact нужна хорошая документация
   - PROGRESS.md и AGENT_LESSONS.md спасли контекст
   - **Урок:** Писать docs ПЕРЕД compact

## Agent-Specific Lessons

### Iteration 3: CIDR Bug Discovery

**Проблема:** Radix tree bug — IPv4 addresses not covered

**Root cause:** `netip.Addr.As16()` stores IPv4 in bytes 12-15, code used 0-3

**Solution:** Opus agent added `byteOffset = 12` в insert() и collectPrefixes()

**Урок:** Complex algorithms → opus from start, не экономить

### Iteration 4: Sonnet Success

**3 sonnet agents:**
- Orchestrator (362 lines) — perfect
- Scheduler (167 lines) — perfect  
- API expansion (6 lines) — perfect

**Урок:** Well-specified tasks с detailed specs → sonnet отлично справляется

### Iteration 5: Balanced Approach

**HIGH RISK код (routing) → opus**
- 1,063 lines, 18/18 tests
- Kernel manipulation — zero tolerance для ошибок

**Boilerplate (API) → sonnet**
- 3 endpoints, perfect integration

**Урок:** Risk-based model selection работает

## Key Gotchas Discovered

### 1. IPv4 in netip.Addr.As16()
```go
// ❌ WRONG
addr.As16()[0:4]  // IPv4 not here!

// ✅ CORRECT
byteOffset := 0
if addr.Is4() {
    byteOffset = 12  // IPv4 in bytes 12-15
}
addr.As16()[byteOffset:byteOffset+4]
```

### 2. Orchestrator New() Signature
- Каждый новый agent → update New() parameters
- **Pattern:** All agents injected, config getter последним

### 3. Routing Idempotence
- Second Apply с тем же input должен быть no-op
- **Check:** Plan.Add и Plan.Remove оба empty

### 4. Context Cancellation
- Проверять `ctx.Done()` между шагами pipeline
- **Pattern:** 
```go
select {
case <-ctx.Done():
    return ctx.Err()
default:
}
```

### 5. Docker Go Version Workaround
```bash
# Local go build fails (Go 1.19)
docker run --rm -v $(PWD):/work -w /work golang:1.22-alpine go build ./...
```

## Metrics Summary

**Code:**
- ~8,300 lines total (6,500 prod + 1,800 tests)
- 56 tests (100% pass)
- 9 packages in internal/
- 1 package in pkg/

**Agents:**
- 8 spawned (3 in Iter3, 3 in Iter4, 2 in Iter5)
- 252k tokens total
- $0.25 cost (58% savings)
- 100% success rate (1 bug fixed)

**Time:**
- 2 days (2026-04-14 to 2026-04-15)
- Iterations 0-2: Manual
- Iterations 3-5: Agent-assisted

## Recommendations for Future Work

### Iteration 6 Priority

1. **Integration tests in netns** — CRITICAL для routing safety
2. **Complete Prometheus metrics** — Observability gap
3. **Minimal Web UI** — HTMX + kv_settings
4. **goleak tests** — Prevent goroutine leaks

### Long-Term

1. **nft JSON mode** — Более надежный parsing
2. **Property-based testing** — CIDR aggregator (`pgregory.net/rapid`)
3. **Fuzzing** — Parser и aggregator
4. **Multi-arch Docker** — amd64 + arm64

## Conclusion

**Status:** PRODUCTION READY для non-routing use case

**Routing:** SAFE (unit tested, disabled by default), но needs netns integration testing

**Next:** Iteration 6 — Observability, UI, hardening

**Главный урок:** Risk-based agent selection + strong interfaces + atomic operations = reliable system
