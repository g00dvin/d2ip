# Technical Debt Execution Plan

**Date:** 2026-04-17  
**Status:** Ready for execution  
**Based on:** [TECHNICAL_DEBT.md](TECHNICAL_DEBT.md), [CLAUDE.md](CLAUDE.md)

This document provides a comprehensive plan to resolve 8 critical technical debt items using automated agents and manual fixes.

---

## 📋 Executive Summary

**Total tasks:** 8  
**Agent tasks:** 5 (sonnet: 2, opus: 3)  
**Manual tasks:** 3  
**Total effort:** ~46 hours  
**Phases:** 3 (Critical → Quality → Enhancement)

**Agent cost estimate (at current rates):**
- 2 sonnet agents (6-10 hours) = ~30k tokens = $0.03
- 3 opus agents (36 hours) = ~150k tokens = $0.30
- **Total: ~$0.33** (57% savings vs all-opus: $0.77)

---

## 🎯 Task Breakdown

### **Phase 1: CRITICAL (Iteration 7a) — Release Blockers**
*Must complete before v0.1.0*

| # | Task | Type | Agent | Effort | Priority |
|---|------|------|-------|--------|----------|
| 1 | Config Tests Failing | Agent | sonnet | 2-4h | 🔴 CRITICAL |
| 2 | iproute2 Iface Validation | Manual | n/a | 1h | 🟡 LOW |
| 3 | Verify Parallel DNS | Manual | n/a | 1h | 🟢 LOW |

**Total Phase 1:** 4-6 hours

---

### **Phase 2: QUALITY (Iteration 7b) — Robustness**
*Improve maintainability and reliability*

| # | Task | Type | Agent | Effort | Priority |
|---|------|------|-------|--------|----------|
| 4 | nftables JSON Parsing | Agent | sonnet | 4-6h | 🟠 MEDIUM |
| 5 | Race Detector Analysis | Manual | n/a | 1-2h | 🟡 MEDIUM |

**Total Phase 2:** 5-8 hours

---

### **Phase 3: ENHANCEMENT (Iteration 8-9) — Advanced Features**
*Long-term quality and performance*

| # | Task | Type | Agent | Effort | Priority |
|---|------|------|-------|--------|----------|
| 6 | Web UI Config Editing | Agent | opus | 12h | 🟠 MEDIUM |
| 7 | Incremental Resolver | Agent | opus | 16h | 🟡 LOW |
| 8 | Property-Based Testing | Agent | opus | 8h | 🟡 LOW |

**Total Phase 3:** 36 hours

---

## 📝 Detailed Task Specifications

### **Task 1: Config Tests Failing** 🔴

**Spec:** [docs/agents/10-config-tests-fix.md](agents/10-config-tests-fix.md)  
**Agent:** Sonnet (debugging, well-specified)  
**Effort:** 2-4 hours  
**Priority:** CRITICAL (blocks v0.1.0)

**Description:**
Fix 3 failing tests in `internal/config/load_test.go`:
- `TestLoad_CategoriesFromEnvJSON` — ENV var JSON parsing
- `TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults` — Precedence order
- `TestWatcher_MultipleSubscribersConcurrent` — Concurrency safety

**Files:**
- `internal/config/load_test.go` (tests)
- `internal/config/load.go` (implementation)
- `internal/config/watcher.go` (concurrency)

**Acceptance:**
- ✅ All 3 tests pass
- ✅ No new failures
- ✅ Precedence validated (ENV > kv > YAML > defaults)

---

### **Task 2: iproute2 Iface Validation** 🟡

**Type:** Manual (trivial, <50 lines)  
**Effort:** 1 hour  
**Priority:** LOW (quick win)

**Description:**
Add config validation for `routing.iface` when backend is `iproute2`.

**Files:**
- `internal/config/validate.go`

**Code:**
```go
if cfg.Routing.Backend == "iproute2" && cfg.Routing.Iface == "" {
    return errors.New("routing.iface required for iproute2 backend")
}
```

**Acceptance:**
- ✅ Clear error message when Iface missing
- ✅ Config validation tests updated

---

### **Task 3: Verify Parallel DNS Resolution** 🟢

**Type:** Manual (code review)  
**Effort:** 1 hour  
**Priority:** LOW (verification only)

**Description:**
Review existing parallel DNS resolver implementation in `internal/resolver/dns.go`:
- Worker pool (goroutines + channels)
- Rate limiting (`golang.org/x/time/rate`)
- Concurrency safety
- Graceful shutdown

**Files:**
- `internal/resolver/dns.go`
- `internal/resolver/resolver_test.go` (goleak tests)

**Deliverable:**
- Documentation of current implementation
- List of any issues found (race conditions, leaks, etc.)
- Confirmation that implementation is correct

---

### **Task 4: nftables JSON Parsing** 🟠

**Spec:** [docs/agents/11-nftables-json.md](agents/11-nftables-json.md)  
**Agent:** Sonnet (well-specified parser rewrite)  
**Effort:** 4-6 hours  
**Priority:** MEDIUM

**Description:**
Replace brittle plain-text parsing of `nft list set` with robust JSON parsing using `nft --json`, with fallback to plain text for older nftables.

**Files:**
- `internal/routing/nftables.go` (add JSON structs, parseNftSetJSON)
- `internal/routing/nftables_test.go` (add JSON parser tests)

**Acceptance:**
- ✅ JSON parsing implemented with correct structs
- ✅ Fallback to plain text if JSON unavailable
- ✅ All 31 tests pass (18 unit + 13 integration)
- ✅ No behavior change (output identical)

---

### **Task 5: Race Detector Analysis** ⚠️

**Type:** Manual (documentation)  
**Effort:** 1-2 hours  
**Priority:** MEDIUM

**Description:**
Document the CGO_ENABLED=0 vs race detector tradeoff:
- Why race detector is incompatible (requires CGO, but project uses CGO_ENABLED=0 for static builds)
- Goleak tests as mitigation (cover goroutine leaks)
- Consider CI job variant with CGO_ENABLED=1 (race only, not production build)

**Files:**
- `docs/TECHNICAL_DEBT.md` (update)
- `README.md` (mention limitation)
- `.github/workflows/test.yml` (optional: add race job)

**Deliverable:**
- Updated documentation explaining limitation
- Validation that goleak coverage is sufficient
- (Optional) CI job for race detection only

---

### **Task 6: Web UI Config Editing** 🎨

**Spec:** [docs/agents/12-web-ui-config-editing.md](agents/12-web-ui-config-editing.md)  
**Agent:** Opus (security-sensitive: auth + kv_settings mutation)  
**Effort:** 12 hours  
**Priority:** MEDIUM

**Description:**
Add config editing interface to web UI with password authentication. Users can edit config at runtime via `kv_settings` table without server restart.

**Files (new):**
- `internal/api/auth.go` (password auth middleware)
- `internal/api/web/login.html` (login form)
- `internal/api/web/config_edit.html` (editing form)
- `internal/config/kv_backend.go` (kv_settings CRUD)

**Files (modified):**
- `internal/api/api.go` (routes: /config/edit, /config/save, /config/login)
- `go.mod` (add session library)

**Acceptance:**
- ✅ Password auth works (`D2IP_WEB_PASSWORD` ENV var)
- ✅ Session management secure (httpOnly, SameSite, 1h timeout)
- ✅ Config editing saves to kv_settings (not YAML)
- ✅ No security vulnerabilities (XSS, CSRF, SQL injection)
- ✅ Config watcher reloads after save (no restart)

---

### **Task 7: Incremental Resolver Updates** 🔄

**Spec:** [docs/agents/13-incremental-resolver.md](agents/13-incremental-resolver.md)  
**Agent:** Opus (complex concurrency + state management)  
**Effort:** 16 hours  
**Priority:** LOW

**Description:**
Only re-resolve domains that changed or are stale (cache expired), skipping unchanged domains. Reduces pipeline run time by 50%+ for incremental changes.

**Files (new):**
- `internal/orchestrator/diff.go` (domain diff logic)
- `internal/cache/migrations/003_incremental.sql` (metadata table)

**Files (modified):**
- `internal/orchestrator/orchestrator.go` (use incremental resolution)
- `internal/cache/cache.go` (CheckStaleDomains, GetMetadata, SetMetadata)

**Acceptance:**
- ✅ Change detection works (SHA256 fingerprint)
- ✅ Unchanged domains skipped if TTL valid
- ✅ Stale domains re-resolved
- ✅ 50%+ faster pipeline runs for incremental changes
- ✅ Metrics track incremental stats

---

### **Task 8: Property-Based Testing (CIDR)** 🔍

**Spec:** [docs/agents/14-property-based-testing.md](agents/14-property-based-testing.md)  
**Agent:** Opus (complex algorithm testing)  
**Effort:** 8 hours  
**Priority:** LOW

**Description:**
Add property-based testing to CIDR aggregator using `pgregory.net/rapid` to find edge cases that unit tests miss.

**Files (new):**
- `pkg/cidr/aggregate_rapid_test.go` (property tests)

**Files (modified):**
- `go.mod` (add `pgregory.net/rapid`)
- `.github/workflows/test.yml` (run property tests in CI)

**Acceptance:**
- ✅ 5+ properties tested (lossless, no-overlap, idempotent, etc.)
- ✅ 1000+ random inputs per property
- ✅ All properties pass (or bugs found and fixed)
- ✅ CI runs property tests on every PR

---

## 🚀 Execution Strategy

### **Recommended Order**

**Phase 1: Critical (Do First)**
1. Task 1: Config Tests Failing (agent 10, sonnet) ← **START HERE**
2. Task 2: iproute2 Iface Validation (manual)
3. Task 3: Verify Parallel DNS (manual)

**Phase 2: Quality (Before Release)**
4. Task 4: nftables JSON Parsing (agent 11, sonnet) ← Can run in parallel with Task 1
5. Task 5: Race Detector Analysis (manual)

**Phase 3: Enhancements (Post-Release)**
6. Task 6: Web UI Config Editing (agent 12, opus)
7. Task 7: Incremental Resolver (agent 13, opus)
8. Task 8: Property-Based Testing (agent 14, opus)

### **Parallel Execution Options**

**Option A: Sequential (safest)**
- Run agents one by one
- Validate each before proceeding
- Total time: 46 hours (if sequential)

**Option B: Parallel (faster, user preference)**
- Phase 1: Launch Task 1 (agent 10) + Task 4 (agent 11) in parallel (both sonnet, independent)
- Phase 1 manual: Do Tasks 2 + 3 while agents run
- Phase 3: Launch Tasks 6, 7, 8 in parallel (all opus, independent)
- Total time: ~20 hours (with 3-way parallelism)

**User preference:** Run agents in parallel when possible (per Iteration 6 experience).

---

## 📊 Success Metrics

### **Phase 1 Complete When:**
- ✅ All config tests pass (3/3)
- ✅ iproute2 Iface validation added
- ✅ Parallel DNS implementation verified
- ✅ Ready for v0.1.0 release

### **Phase 2 Complete When:**
- ✅ nftables uses JSON parsing (robust + fallback)
- ✅ Race detector limitation documented
- ✅ v0.1.0 released with quality improvements

### **Phase 3 Complete When:**
- ✅ Web UI config editing works with auth
- ✅ Incremental updates reduce pipeline time by 50%+
- ✅ Property-based tests find no bugs (or bugs fixed)
- ✅ v0.2.0 ready with advanced features

---

## 🛡️ Risk Mitigation

### **Agent Risks**

1. **False-positive malware warnings** (seen in Iteration 6)
   - **Mitigation:** If agent blocked, complete manually
   - **Context:** Routing code is legitimate kernel manipulation

2. **Agent gets stuck or produces bugs**
   - **Mitigation:** Agent specs are detailed, acceptance criteria clear
   - **Fallback:** Switch to manual or different model (sonnet → opus)

3. **Parallel agents conflict** (edit same files)
   - **Mitigation:** Tasks 1-5 are independent (different files)
   - **Phase 3:** Tasks 6-8 also independent

### **Technical Risks**

1. **Config tests reveal deeper bugs** (precedence logic broken)
   - **Impact:** May require more than 2-4 hours
   - **Mitigation:** Agent 10 spec includes debugging steps, fallback to opus if needed

2. **nftables JSON format varies** (across versions)
   - **Mitigation:** Fallback to plain text, integration tests validate real kernel output

3. **Incremental resolver introduces bugs** (state management complex)
   - **Mitigation:** Agent 13 (opus) handles concurrency, extensive testing required

---

## 📦 Deliverables

### **Code Artifacts**
- 5 agent specifications (agents/10-14)
- Updated [docs/PLAN.md](PLAN.md) (Iterations 7a, 7b, 8, 9)
- This execution plan

### **After Execution**
- Fixed code (passing tests)
- Updated documentation (TECHNICAL_DEBT.md, README.md)
- New features (web UI editing, incremental resolver, property tests)
- Release readiness (v0.1.0 after Phase 1+2)

---

## 🔄 Iteration Mapping

| Phase | Iteration | Tasks | Duration |
|-------|-----------|-------|----------|
| 1 | 7a | 1, 2, 3 | 0.5 day |
| 2 | 7b | 4, 5 | 0.5 day |
| 3a | 8 | 6 | 1 day |
| 3b | 9 | 7, 8 | 2 days |

**Total:** 4 days (if sequential), 2 days (if parallel)

---

## ✅ Ready for Confirmation

**User:** Review this plan and confirm to start execution.

**Recommended start:**
1. Launch **Agent 10** (Config Tests Fix) — CRITICAL blocker
2. Launch **Agent 11** (nftables JSON) — in parallel (independent files)
3. Manual: iproute2 validation + DNS verification (while agents run)

**After confirmation, I will:**
1. Update agent history in CLAUDE.md
2. Launch agents as specified
3. Complete manual tasks
4. Track progress with TodoWrite
5. Update TECHNICAL_DEBT.md as items completed

---

## 📚 References

- **Technical Debt:** [docs/TECHNICAL_DEBT.md](TECHNICAL_DEBT.md)
- **Agent Specs:** [docs/agents/10-14](agents/)
- **Project Guide:** [CLAUDE.md](CLAUDE.md)
- **Implementation Plan:** [docs/PLAN.md](PLAN.md)
- **Agent Lessons:** [docs/AGENT_LESSONS.md](AGENT_LESSONS.md)
