# Retro — v0.1.15 (SSE + Web UI Rewrite)

**Date:** 2026-04-24
**Scope:** Ground-up rewrite of Vue 3 SPA with real-time SSE, Pinia, Naive UI, charts, responsive design
**Commits:** 22 commits since v0.1.14

---

## What Went Well

1. **Architecture decisions were solid.** EventBus for SSE pub/sub, Pinia stores, Naive UI component library — all worked well together.
2. **Option 4 (gzip compression) was the right call.** We hit the <500KB target (468KB) without throwing away 20+ files of UI work.
3. **CI caught real bugs.** Unused imports, unchecked error returns, mismatched function signatures — all caught by GitHub Actions.
4. **Docs stayed in sync.** We audited and updated 6 documentation files to match current code.

## What Went Wrong

1. **Broken validation for resolve_cycle=0.** The validator rejected `0` even though `main.go` already treated it as "disabled." A user reported the bug post-release.
   - **Root cause:** `validateScheduler` had `s.ResolveCycle < time.Minute` with no exception for `0`.
   - **Fix:** Added `s.ResolveCycle != 0 &&` guard.

2. **CI failures on unused imports.** Pushed code that compiled locally (Go 1.19) but failed on CI (Go 1.23) because of stricter unused import detection.
   - **Root cause:** Changed `serveEmbeddedFile` to remove `time` usage but forgot to remove the import.

3. **io.Copy error returns unchecked.** golangci-lint caught unchecked `io.Copy` errors in `serveEmbeddedFile`.
   - **Root cause:** Rushed the gzip compression refactor, didn't run the full linter locally.

4. **Main.go signature mismatch.** `orchestrator.New` was called with 9 args but the function only accepted 8 (missing `eventBus` parameter in the definition).
   - **Root cause:** Partial refactor — added `eventBus` to the struct and `emit()` helper, but forgot to add it to the `New()` constructor signature.

## Lessons Learned

1. **Always run the full CI chain locally before push.** Even if `go build` passes, `golangci-lint` catches things Go compiler doesn't (unchecked errors, unused imports).
2. **Validation rules must match business logic.** If `0` means "disabled" in the scheduler initialization, the validator must accept `0`.
3. **Refactor across all call sites.** When adding a new dependency to a constructor, update the definition, all call sites, and tests simultaneously.
4. **Compressed assets need special serving logic.** Pre-gzipping dist files reduced binary size by 72%, but required handling `Content-Encoding: gzip` and fallback decompression.

## Metrics

| Metric | Value |
|--------|-------|
| Commits | 22 |
| CI failures before green | 3 |
| Docs updated | 6 |
| Embedded asset size | 468KB (was 1.7MB) |
| New files created | ~25 (Go + Vue) |
