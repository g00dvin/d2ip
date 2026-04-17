# Technical Debt & Future Improvements

**Last Updated:** 2026-04-17  
**Status:** Post-Iteration 6

This document tracks known technical debt, missing features, and improvement opportunities.

---

## Critical Issues (Should Fix Before v1.0)

### 1. Config Tests Failing ❌

**Location:** `internal/config/load_test.go`

**Issue:** 3 tests fail consistently:
- `TestLoad_CategoriesFromEnvJSON`
- `TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults`
- `TestWatcher_MultipleSubscribersConcurrent`

**Impact:** Config precedence and hot-reload not fully tested

**Root Cause:** Unknown (pre-existing before Iteration 6)

**Priority:** HIGH (affects config reliability)

**Effort:** 2-4 hours investigation + fix

**Action:** Debug failures, fix precedence logic or test expectations

---

### 2. Race Detector Incompatible ⚠️

**Location:** Project-wide

**Issue:** `go test -race` fails with:
```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

**Impact:** Cannot detect race conditions in concurrent code (orchestrator, resolver)

**Root Cause:** Project uses `modernc.org/sqlite` (pure Go) and builds with `CGO_ENABLED=0` for static binaries

**Conflict:** Race detector requires CGO, but static builds forbid it

**Workaround:** goleak tests detect goroutine leaks, CI runs tests without -race

**Priority:** MEDIUM (goleak covers leak detection, race is nice-to-have)

**Effort:** 8+ hours (would require CGO-enabled build variant)

**Options:**
1. Accept limitation (current approach)
2. Add separate CI job with CGO_ENABLED=1 for race detection only (not for production builds)
3. Migrate to `github.com/mattn/go-sqlite3` (requires CGO always)

**Recommendation:** Option 1 or 2 (keep static builds)

---

### 3. nftables Plain-Text Parsing Brittle 🔧

**Location:** `internal/routing/nftables.go` → `parseNftSet()`

**Issue:** Parses `nft list set` output as plain text:
```go
// Format: elements = { 192.0.2.0/24, ... }
if strings.HasPrefix(line, "elements = {") { ... }
```

**Problems:**
- Fragile to nft output format changes
- Fails on edge cases (comments, formatting variations)
- Harder to maintain

**Better Approach:** Use `nft --json` output mode with structured parsing

**Impact:** Low (works for current use case, but could break on nft updates)

**Priority:** MEDIUM

**Effort:** 4-6 hours (rewrite parseNftSet, add JSON unmarshaling)

**Action:** Add `nft --json` support, fallback to plain text if JSON unavailable

---

### 4. iproute2 Backend Missing Iface Config Field 📝

**Location:** `internal/routing/iproute2.go`

**Issue:** `Iface` is hardcoded requirement but not validated in config

**Current:**
```go
cfg := config.RoutingConfig{
    Backend: "iproute2",
    // Iface: ???  // Field exists but not documented/validated
}
```

**Impact:** Confusing UX (Iface required but not obvious from config struct)

**Priority:** LOW (documented in code, works when set)

**Effort:** 1 hour (add validation + update docs)

**Action:** Add validation in `internal/config/validate.go`:
```go
if cfg.Routing.Backend == "iproute2" && cfg.Routing.Iface == "" {
    return errors.New("routing.iface required for iproute2 backend")
}
```

---

### 5. DNS TTL Ignored ⏱️

**Location:** `internal/resolver/resolver.go`

**Issue:** DNS TTL from resolver responses is ignored, only internal cache TTL used

**Current Behavior:**
- Resolve google.com (DNS TTL: 300s)
- Cache with internal TTL: 24h (config)
- Re-resolve only after 24h, not 300s

**Impact:** Cache may be stale for domains with short TTLs

**Design Decision:** By design for simplicity (internal TTL only)

**Priority:** LOW (documented behavior, acceptable for use case)

**Effort:** 6-8 hours (respect DNS TTL, min/max clamping)

**Action:** Document clearly in CONFIG.md, consider for v2.0

---

## Known Limitations (By Design)

### 1. No DNS Server Mode

**What:** d2ip is not a DNS server, only resolves and caches

**Why:** Out of scope, use dnsmasq/unbound for DNS serving

**Workaround:** Export ipv4.txt/ipv6.txt and import into your DNS server

---

### 2. No Real-Time Routing Updates

**What:** Routing only updates on pipeline run (scheduled or manual)

**Why:** Batch processing more efficient than per-domain updates

**Workaround:** Schedule frequent pipeline runs (e.g., every 6 hours)

---

### 3. Single Config File

**What:** All config in one YAML file, no config.d/ directory support

**Why:** Simplicity, precedence (ENV > kv > YAML) sufficient

**Workaround:** Use ENV vars for overrides, kv_settings for runtime changes

---

## Missing Features (Future Enhancements)

### 1. Soak Testing 🧪

**What:** 24-hour continuous pipeline runs for stability testing

**Why:** Validate long-term stability (goroutine leaks, memory leaks, connection exhaustion)

**Priority:** MEDIUM

**Effort:** 4 hours (docker-compose + test script)

**Iteration:** 7 or 8

---

### 2. Property-Based Testing (CIDR Aggregator) 🔍

**What:** Use `pgregory.net/rapid` for randomized CIDR aggregation tests

**Why:** Catch edge cases (overlapping prefixes, degenerate inputs)

**Priority:** LOW

**Effort:** 8 hours (learn rapid, write generators)

**Iteration:** 8+

---

### 3. Fuzzing (Parser + Aggregator) 💥

**What:** `go test -fuzz` on dlc.proto parser and CIDR aggregator

**Why:** Find crashes, panics, malformed input handling

**Priority:** LOW

**Effort:** 6 hours (corpus generation, fuzz targets)

**Iteration:** 8+

---

### 4. Multi-Arch Docker (amd64 + arm64) 🐳

**What:** Build Docker images for multiple architectures

**Why:** Support ARM servers (e.g., AWS Graviton, Raspberry Pi)

**Priority:** HIGH (planned for Iteration 7)

**Effort:** 4 hours (buildx, GitHub Actions matrix)

**Iteration:** 7

---

### 5. Web UI Config Editing 🎨

**What:** Edit config via web UI (currently read-only status)

**Why:** Better UX for runtime config changes (kv_settings backend ready)

**Priority:** MEDIUM

**Effort:** 12 hours (form validation, kv_settings integration, auth)

**Iteration:** 8

---

### 6. Incremental Resolver Updates 🔄

**What:** Only re-resolve domains that changed or are stale (not full batch)

**Why:** Faster pipeline runs, less DNS load

**Priority:** LOW

**Effort:** 16 hours (change detection, partial resolution)

**Iteration:** 9+

---

### 7. Backup/Restore State 💾

**What:** Export/import pipeline state (SQLite + routing state)

**Why:** Disaster recovery, migration between hosts

**Priority:** LOW

**Effort:** 4 hours (CLI commands, tar.gz creation)

**Iteration:** 8

---

### 8. Observability Dashboards 📊

**What:** Pre-built Grafana dashboards for metrics

**Why:** Easier onboarding, visualize pipeline health

**Priority:** MEDIUM

**Effort:** 6 hours (dashboard JSON, documentation)

**Iteration:** 7 or 8

---

### 9. Plugin System for Custom Exporters 🔌

**What:** Export to custom formats (JSON, CSV, custom scripts)

**Why:** Flexibility for non-standard routing systems

**Priority:** LOW

**Effort:** 20+ hours (plugin interface, dynamic loading)

**Iteration:** 10+

---

## Performance Optimizations (Nice-to-Have)

### 1. Parallel DNS Resolution (Already Implemented ✅)

**Status:** DONE (worker pool with configurable concurrency)

---

### 2. CIDR Aggregation Performance

**Current:** Radix tree, works well for <100k prefixes

**Future:** Could optimize for millions of prefixes (benchmark first)

**Priority:** LOW (no perf issues reported)

---

### 3. SQLite WAL Checkpointing

**Current:** WAL mode enabled, auto-checkpoint on close

**Future:** Manual checkpointing for better control

**Priority:** LOW (current approach works)

---

## Documentation Gaps

### 1. Deployment Guide (Partial)

**What:** README has quickstart, but no production deployment guide

**Missing:** systemd unit, log rotation, monitoring setup

**Priority:** MEDIUM

**Effort:** 4 hours (write guide, test systemd unit)

**Iteration:** 7

---

### 2. Troubleshooting Guide

**What:** Common issues and solutions

**Examples:**
- "DNS resolution fails" → check resolver.upstream
- "Routing not working" → check routing.enabled=true, CAP_NET_ADMIN
- "Pipeline stuck" → check orchestrator.running=true

**Priority:** MEDIUM

**Effort:** 3 hours (document common issues)

**Iteration:** 7

---

### 3. API Examples (Curl + Go)

**What:** More API usage examples beyond README

**Priority:** LOW

**Effort:** 2 hours (write examples)

**Iteration:** 8

---

## Security Considerations

### 1. No Authentication on API ⚠️

**What:** HTTP API has no auth (anyone on network can trigger pipeline)

**Risk:** MEDIUM (should run on trusted network or behind reverse proxy)

**Mitigation:** Document "run behind nginx with BasicAuth"

**Future:** Add optional API key auth (v2.0)

---

### 2. No HTTPS Support

**What:** API only listens on HTTP

**Risk:** LOW (intended for localhost or private network)

**Mitigation:** Use reverse proxy (nginx, Caddy) for HTTPS

**Future:** Add TLS config (v2.0)

---

### 3. No Rate Limiting on API

**What:** API endpoints have no rate limiting

**Risk:** LOW (DOS possible but unlikely in private network)

**Mitigation:** Use reverse proxy rate limiting

**Future:** Add middleware rate limiter (v2.0)

---

## Summary

**Critical Issues:** 2 (config tests, race detector limitation)

**High Priority Debt:** 2 (nft JSON parsing, Iface validation)

**Missing Features:** 9 (multi-arch Docker most important)

**Documentation Gaps:** 3 (deployment guide, troubleshooting, API examples)

**Security Notes:** 3 (all mitigated by network isolation or reverse proxy)

**Recommendation:** Fix config tests before v1.0, tackle others in v1.x releases
