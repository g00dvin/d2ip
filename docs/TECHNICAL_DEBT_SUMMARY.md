# Technical Debt Resolution — Summary for Review

**Date:** 2026-04-17  
**Prepared by:** Claude Code  
**Status:** ⏸️ **AWAITING USER CONFIRMATION**

---

## 📋 What Was Created

### **5 Agent Specifications**

| Agent | Task | Model | Effort | Priority | Spec File |
|-------|------|-------|--------|----------|-----------|
| 10 | Config Tests Fix | sonnet | 2-4h | 🔴 CRITICAL | [10-config-tests-fix.md](agents/10-config-tests-fix.md) |
| 11 | nftables JSON Parsing | sonnet | 4-6h | 🟠 MEDIUM | [11-nftables-json.md](agents/11-nftables-json.md) |
| 12 | Web UI Config Editing | opus | 12h | 🟠 MEDIUM | [12-web-ui-config-editing.md](agents/12-web-ui-config-editing.md) |
| 13 | Incremental Resolver | opus | 16h | 🟡 LOW | [13-incremental-resolver.md](agents/13-incremental-resolver.md) |
| 14 | Property-Based Testing | opus | 8h | 🟡 LOW | [14-property-based-testing.md](agents/14-property-based-testing.md) |

**Total agent effort:** 42-46 hours  
**Estimated cost:** ~$0.33 (57% savings vs all-opus)

### **3 Manual Tasks**

1. **iproute2 Iface Validation** (1 hour, trivial)
2. **Verify Parallel DNS Resolution** (1 hour, code review)
3. **Race Detector Analysis** (1-2 hours, documentation)

### **Documentation Updates**

1. **[docs/PLAN.md](PLAN.md)** — Added Iterations 7a, 7b (split), updated Iterations 8-9
2. **[docs/TECHNICAL_DEBT_EXECUTION_PLAN.md](TECHNICAL_DEBT_EXECUTION_PLAN.md)** — Comprehensive execution guide (3 phases, 8 tasks)
3. **[README.md](README.md)** — Added agents 10-14, TECHNICAL_DEBT_EXECUTION_PLAN link

---

## 🎯 Execution Phases

### **Phase 1: CRITICAL (Iteration 7a)** — 0.5 day
*Must complete before v0.1.0*

- ✅ **Task 1:** Config Tests Failing (agent 10, sonnet)
- ✅ **Task 2:** iproute2 Iface Validation (manual)
- ✅ **Task 3:** Verify Parallel DNS (manual)

### **Phase 2: QUALITY (Iteration 7b)** — 0.5 day
*Improve robustness*

- ✅ **Task 4:** nftables JSON Parsing (agent 11, sonnet)
- ✅ **Task 5:** Race Detector Analysis (manual)

### **Phase 3: ENHANCEMENT (Iterations 8-9)** — 3 days
*Advanced features*

- ✅ **Task 6:** Web UI Config Editing (agent 12, opus)
- ✅ **Task 7:** Incremental Resolver (agent 13, opus)
- ✅ **Task 8:** Property-Based Testing (agent 14, opus)

---

## 🚀 Recommended Execution Strategy

### **Option A: Sequential (Safest)**
Run agents one by one, validate each before proceeding.  
**Duration:** 4 days

### **Option B: Parallel (Faster, User Preference)** ⭐
Based on Iteration 6 success with parallel agents:

**Phase 1 (Start Now):**
- Launch **Agent 10** (config tests) + **Agent 11** (nftables JSON) **in parallel**
- Both are sonnet, independent files, no conflicts
- Manual: Tasks 2-3 while agents run

**Phase 2 (After Phase 1):**
- Manual: Task 5 (race detector docs)

**Phase 3 (Post-Release):**
- Launch **Agent 12** + **Agent 13** + **Agent 14** **in parallel** (all opus, independent)

**Duration:** ~2 days (with 2-3 way parallelism)

---

## ✅ What Each Agent Will Do

### **Agent 10: Config Tests Fix** (sonnet, 2-4h)
**Files:** `internal/config/load_test.go`, `load.go`, `watcher.go`  
**Goal:** Fix 3 failing tests (ENV JSON parsing, precedence, concurrency)  
**Blocker:** v0.1.0 cannot release without this

### **Agent 11: nftables JSON Parsing** (sonnet, 4-6h)
**Files:** `internal/routing/nftables.go`, `nftables_test.go`  
**Goal:** Replace brittle plain-text parsing with `nft --json`  
**Benefit:** More robust, maintainable, fallback to plain text

### **Agent 12: Web UI Config Editing** (opus, 12h)
**Files:** New: `auth.go`, `login.html`, `config_edit.html`, `kv_backend.go`  
**Goal:** Runtime config editing with password auth  
**Security-sensitive:** Auth + kv_settings mutation

### **Agent 13: Incremental Resolver** (opus, 16h)
**Files:** New: `diff.go`, `003_incremental.sql`, Modified: `orchestrator.go`, `cache.go`  
**Goal:** Only re-resolve changed/stale domains  
**Benefit:** 50%+ faster pipeline runs for incremental changes

### **Agent 14: Property-Based Testing** (opus, 8h)
**Files:** New: `pkg/cidr/aggregate_rapid_test.go`  
**Goal:** Find edge cases in CIDR aggregator with randomized testing  
**Benefit:** Higher confidence in correctness

---

## 📊 Task Breakdown by Type

| Type | Count | Effort | Cost |
|------|-------|--------|------|
| Agent (sonnet) | 2 | 6-10h | ~$0.03 |
| Agent (opus) | 3 | 36h | ~$0.30 |
| Manual | 3 | 3-5h | $0 |
| **Total** | **8** | **45-51h** | **~$0.33** |

---

## 🎯 Critical Path to v0.1.0

```
Phase 1 (CRITICAL) → Phase 2 (QUALITY) → v0.1.0 Release → Phase 3 (ENHANCEMENTS)
     ↓                      ↓                   ↓                    ↓
  Tasks 1-3            Tasks 4-5         Multi-arch Docker      Tasks 6-8
  (1 day)              (1 day)           + Deploy guides        (3 days)
                                         + CHANGELOG
```

**v0.1.0 requirements:**
- ✅ All config tests pass (Task 1)
- ✅ iproute2 validation (Task 2)
- ✅ Parallel DNS verified (Task 3)
- ✅ (Optional) nftables JSON (Task 4)

---

## 🛡️ Risk Assessment

### **Low Risk**
- Tasks 1-5: Well-specified, clear acceptance criteria
- Agent 10 & 11 (sonnet): Proven reliable for these task types
- Manual tasks: Trivial, <50 lines each

### **Medium Risk**
- Agent 12 (opus): Security-sensitive, requires auth implementation
  - **Mitigation:** Detailed spec with security checklist
- Agent 13 (opus): Complex concurrency, state management
  - **Mitigation:** Opus model, extensive testing required

### **Low-Medium Risk**
- Agent 14 (opus): Learning new library (rapid)
  - **Mitigation:** Examples in spec, property testing is well-documented

### **Parallel Execution Risk**
- **Low:** Tasks 1-5 touch different files (no conflicts)
- **Low:** Tasks 6-8 touch different packages (no conflicts)
- **Mitigation:** Iteration 6 parallel execution was successful (2/3 agents)

---

## 📝 Files Created/Modified

### **New Files (7)**
1. `docs/agents/10-config-tests-fix.md`
2. `docs/agents/11-nftables-json.md`
3. `docs/agents/12-web-ui-config-editing.md`
4. `docs/agents/13-incremental-resolver.md`
5. `docs/agents/14-property-based-testing.md`
6. `docs/TECHNICAL_DEBT_EXECUTION_PLAN.md`
7. `docs/TECHNICAL_DEBT_SUMMARY.md` (this file)

### **Modified Files (2)**
1. `docs/PLAN.md` — Added Iterations 7a, 7b, updated 8-9
2. `README.md` — Added agents 10-14, execution plan link

---

## ❓ Questions for User

### **1. Execution Mode**
Do you want to:
- **A)** Run agents sequentially (safer, slower)
- **B)** Run agents in parallel (faster, your preference from Iter 6)

**Recommendation:** Option B (parallel Phase 1: Agents 10 + 11)

### **2. Phase Scope**
Which phases to execute now:
- **A)** Phase 1 only (CRITICAL, blocks v0.1.0)
- **B)** Phase 1 + 2 (CRITICAL + QUALITY, ready for release)
- **C)** All phases (CRITICAL + QUALITY + ENHANCEMENTS)

**Recommendation:** Option B (Phases 1-2, then release v0.1.0)

### **3. Manual Tasks**
Who handles manual tasks (2, 3, 5)?
- **A)** You handle manually
- **B)** I handle manually (between agent runs)

**Recommendation:** Option B (I handle during agent execution)

---

## ✅ Ready to Start?

**If you confirm:**

1. I will launch **Agent 10** (config tests) + **Agent 11** (nftables JSON) **in parallel**
2. I will complete **manual tasks 2-3** while agents run
3. I will validate results and proceed to Phase 2 or Phase 3 based on your preference

**Next steps after confirmation:**
- Update CLAUDE.md with agent execution history
- Launch agents with detailed prompts
- Track progress with TodoWrite
- Report results and any issues found

---

## 📚 Reference Documents

- **Analysis:** [TECHNICAL_DEBT.md](TECHNICAL_DEBT.md)
- **Execution Plan:** [TECHNICAL_DEBT_EXECUTION_PLAN.md](TECHNICAL_DEBT_EXECUTION_PLAN.md)
- **Agent Specs:** [docs/agents/10-14](agents/)
- **Iteration Plan:** [PLAN.md](PLAN.md)
- **Project Guide:** [CLAUDE.md](CLAUDE.md)

---

**Status:** ⏸️ **AWAITING YOUR CONFIRMATION TO PROCEED**
