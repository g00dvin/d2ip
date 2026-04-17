# Agent 10 — Config Tests Fix

**Model:** Sonnet  
**Priority:** 🔴 CRITICAL (blocks v0.1.0)  
**Effort:** 2-4 hours  
**Iteration:** 7a

## Goal

Fix 3 failing tests in `internal/config/load_test.go` to ensure config precedence (ENV > kv_settings > YAML > defaults) works correctly.

## Background

Config loading supports multiple sources with precedence order:
1. **ENV vars** (highest): `D2IP_*` variables
2. **kv_settings** (SQLite table): Runtime overrides via web UI
3. **YAML file**: `config.example.yaml`
4. **Defaults**: Hardcoded in `config.go`

Hot-reload via Watcher broadcasts changes to subscribers.

## Problem

3 tests fail consistently:
1. `TestLoad_CategoriesFromEnvJSON` — ENV var parsing for JSON categories
2. `TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults` — Precedence order validation
3. `TestWatcher_MultipleSubscribersConcurrent` — Concurrent watcher subscriptions

**Impact:** Config reliability not fully tested, precedence logic may be incorrect.

## Files Involved

- **Test file:** `internal/config/load_test.go` (failing tests)
- **Implementation:** `internal/config/load.go` (Load function)
- **Validation:** `internal/config/validate.go` (config validation)
- **Watcher:** `internal/config/watcher.go` (hot-reload)
- **Config struct:** `internal/config/config.go` (Config type)

## Requirements

### 1. Debug Failing Tests

Run tests to see exact failures:
```bash
make test
# Or directly:
go test ./internal/config -v -run TestLoad_CategoriesFromEnvJSON
go test ./internal/config -v -run TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults
go test ./internal/config -v -run TestWatcher_MultipleSubscribersConcurrent
```

Analyze:
- What's the expected behavior vs actual behavior?
- Is the test expectation wrong, or is the implementation buggy?
- Are ENV vars being parsed correctly (JSON vs string)?
- Is precedence order correct (ENV > kv > YAML > defaults)?

### 2. Fix Root Cause

**Option A:** Implementation bug
- Fix `Load()` logic in `load.go`
- Ensure ENV parsing handles JSON arrays correctly
- Validate precedence order in viper merge logic
- Test kv_settings override behavior

**Option B:** Test expectation bug
- Update test assertions to match correct behavior
- Document why the expectation was wrong
- Ensure test setup is correct (clearEnv, mock YAML, etc.)

**Option C:** Both
- Fix implementation AND update tests

### 3. Validate Fix

After fixing:
```bash
# All config tests must pass
go test ./internal/config -v

# Full test suite must still pass
make test

# No new failures introduced
go test ./... -v
```

## Acceptance Criteria

- [ ] All 3 failing tests now pass
- [ ] No new test failures introduced
- [ ] Precedence order validated: ENV > kv > YAML > defaults
- [ ] JSON ENV var parsing works (categories array)
- [ ] Concurrent watcher subscriptions safe (no race, no panic)
- [ ] Documentation updated if behavior changed

## Non-Goals

- Performance optimization (existing implementation is fast enough)
- New config features (just fix existing tests)
- Refactoring (minimal changes to fix tests)

## Testing Strategy

1. **Run failing tests individually** to isolate issues
2. **Add debug logging** to understand precedence merge
3. **Test ENV var parsing** for JSON arrays vs primitives
4. **Verify kv_settings** mock behavior in tests
5. **Check viper merge order** (SetDefault, ReadInFile, BindEnv, Set)
6. **Validate watcher** concurrency safety (use `-race` if CGO available, or goleak)

## Expected Outcome

```bash
$ go test ./internal/config -v
=== RUN   TestLoad_Defaults
--- PASS: TestLoad_Defaults (0.00s)
=== RUN   TestLoad_CategoriesFromEnvJSON
--- PASS: TestLoad_CategoriesFromEnvJSON (0.01s)  # ✅ NOW PASSES
=== RUN   TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults
--- PASS: TestLoad_EnvBeatsYAMLBeatsKVBeatsDefaults (0.01s)  # ✅ NOW PASSES
=== RUN   TestWatcher_MultipleSubscribersConcurrent
--- PASS: TestWatcher_MultipleSubscribersConcurrent (0.05s)  # ✅ NOW PASSES
PASS
ok      github.com/yourusername/d2ip/internal/config    0.123s
```

## Hints

- **ENV JSON parsing:** Viper may not auto-unmarshal JSON strings. Check if manual `json.Unmarshal` needed.
- **Precedence:** Viper merges in order: defaults → file → env. Ensure `BindEnv` called for all keys.
- **Watcher concurrency:** Use mutex or channels for subscriber list. Check for race conditions.
- **clearEnv helper:** Ensures tests don't pollute each other. Verify it works correctly.

## Deliverables

1. **Fixed code** in `internal/config/*.go` (if implementation bug)
2. **Fixed tests** in `internal/config/load_test.go` (if test bug)
3. **Test output** showing all tests pass
4. **Brief explanation** of what was wrong and how you fixed it

## Success Metrics

- ✅ 3/3 failing tests now pass
- ✅ 0 new test failures
- ✅ Config precedence validated and documented
- ✅ Ready for v0.1.0 release
