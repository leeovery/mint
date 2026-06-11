# Implementation Review: Mint Release Tool

**Plan**: mint-release-tool
**QA Verdict**: Approve

## Summary

The mint release-tool implementation is **complete and verified across all 92 tasks** spanning twelve phases (walking skeleton → AI notes engine → hooks/projection → robustness → regenerate → config/init → three analysis cycles → review remediation → two further analysis cycles). The prior review pass (80 tasks) flagged **nine Required Changes** — one blocking gate-bypass plus a five-defect external-audit set and three promoted latent bugs. **All nine were remediated in Phase 10 and re-verified against the code this session — every one is now `Complete` with zero blocking issues.** Phases 11–12 added three further analysis-cycle remediations (a regenerate gh-auth gating parity fix, removal of a load-dependent test-timing flake, and a cmd-layer producer-closure consolidation), all likewise verified `Complete`. The seam architecture (`CommandRunner`, `Publisher`, `Presenter`, lock-resilient `Mutator`) remains clean and uniformly applied; the point-of-no-return model and surgical-unwind invariants are now structurally enforced on the regenerate spine as well as the forward spine; and the remediation work added targeted regression tests at exactly the seams the prior pass found untested. No blocking issues remain. The residual findings are a long tail of non-blocking polish — recurring brittleness themes (gh-stderr substring matching, go-toml/v2 error-text coupling), test-tightening opportunities, and a handful of doc-comment nits — carried forward from the Phase 1–9 review (unchanged by remediation), plus seven small items newly surfaced by this remediation review.

## QA Verification

### Specification Compliance

Implementation aligns with the specification with **no remaining deviations**. The one prior deviation — the interactive-regenerate preflight gate bypass (spec lines 547–550) — is closed: `RegenerateRun` now preflights the resolved target after axis resolution, so an interactively-chosen `changelog`/`both` runs clean-tree/branch/remote-sync before committing+pushing and an interactively-chosen `release`/`both` runs gh-auth before the provider write. Verifiers re-confirmed faithful realization of: tag-as-truth version determination and strict 3-part SemVer grammar; the seven-stage spine and PONR asymmetry (pre-push surgical unwind vs post-push warn-only), now extended with SIGINT/SIGTERM cancellation routed through the existing surgical unwind; the layered AI notes engine (context assembly vs content-agnostic transport) including the now-correct production timeout classification (`errors.Is(context.DeadlineExceeded)` holds on a real deadline kill → non-retried, honouring "timeout is not retried"); single-body whole distribution to tag/CHANGELOG/provider with a retry-safe `Mutator` stdin path (no empty tag annotation after a lock retry); the interactive `y/n/e/r` gate with a panic-free whitespace-only `$EDITOR` fall-through; the regenerate two-axis contract with downgrade behaviour now identical across forward and regenerate verbs (warn-and-downgrade, gh-auth skipped, no nil-`Publisher` panic); `--plain` accepted on every verb including regenerate; honest unwind reporting (no false "repo clean" when recovery itself failed); and no blocking-stage spinner leaks across `Warn`. Phases 10–12 are remediation of prior review/analysis findings, not new scope.

### Plan Completion
- [x] Phase 1–12 acceptance criteria met — no remaining deviations
- [x] All 92 completed tasks implemented and verified (80 in the prior pass; 12 remediation tasks this session)
- [x] No scope creep — Phases 10–12 map one-to-one to prior review Required Changes (#1–#9) and analysis-cycle 5/6 findings (11-1, 12-1, 12-2)

### Code Quality

No blocking code-quality issues. Strong throughout: single-responsibility seams, DRY via genuinely shared helpers (now including `engine.ResolvePublisher`, consolidating the two former resolve-and-discard call sites, and the two canonical Resolution-keyed regenerate producers), idiomatic error handling (sentinels + `errors.Is/As`, `%w` wrapping — `translateRun` now wraps `ctx.Err()` so deadline/cancel causes propagate), and intent-rich doc comments. Recurring low-grade fragilities (non-blocking, unchanged by remediation): provider/release classification by gh-stderr substring matching, and bad-type config messages keyed on go-toml/v2's internal error-text format.

### Test Quality

Tests adequately verify requirements. Uniformly behaviour-focused (exact argv, event timelines, on-disk bytes, invocation counts) over `FakeRunner` / `RecordingPresenter` / fake `Publisher` seams; balanced (both under- and over-testing avoided). The remediation closed the prior pass's notable gap — the previously-untested interactive-regenerate preflight seam now has tests asserting the resolved-target gate runs (clean-tree/branch/remote-sync for `changelog`, gh-auth for `release`) — and added regression tests for each fixed defect (mid-spine cancellation unwind, per-attempt full stdin on retry, failed-mid-unwind warn without "repo clean", nil/non-nil publisher gate selection, the real-deadline non-retried-timeout classification). Phase 12 also removed a genuine load-dependent timing flake (the real-deadline test no longer races a subprocess fork against a 300ms SIGKILL, while keeping the timing-robust `ErrTimeout` classification assertion). Smaller test-tightening opportunities remain under Recommendations.

## Remediation Outcome (prior Required Changes #1–#9 + Phases 11–12)

All previously-blocking Required Changes are **resolved and verified** this session (each re-checked against the committed code, not merely marked done):

1. **Interactive-regenerate preflight gate bypass** → fixed in task 10-1. `RegenerateRun` preflights the resolved target after axis resolution; the cmd-layer early empty-gate call is gone; tag-free/version-compute gates remain excluded. Tests assert the `changelog` and `release` interactive choices run their gates. _(Report 10-1: Complete)_
2. **Nil-`Publisher` crash on regenerate** → fixed in task 10-2. Shared `engine.ResolvePublisher` mirrors `engine.Release`'s warn-and-downgrade/abort branching; both cmd call sites use it; nil-guards precede `DispatchRelease` on single and batch paths. Single + batch unresolvable-provider tests present. _(Report 10-2: Complete)_
3. **SIGINT/SIGTERM handling** → fixed in task 10-3. `signal.NotifyContext` built once in `run()` and threaded to every spine entry point; the lone remaining `context.Background()` is the legitimate seed; pre-PONR `ctx.Err()` routes through surgical unwind with `context.WithoutCancel`-protected recovery; post-PONR stays warn-only. _(Report 10-3: Complete)_
4. **`Mutator.Mutate` consumed-reader retry** → fixed in task 10-4. `Mutate`/`invoke` take `stdin []byte` and build a fresh reader per attempt; the only stdin caller (`createAnnotatedTag`) passes bytes; the nil path is unchanged. Regression test asserts both attempts pipe the full payload. _(Report 10-4: Complete)_
5. **False "repo clean" on failed unwind** → fixed in task 10-5. All three recovery `Mutate` sites capture the error and emit a shared manual-cleanup `Warn`; the "; repo clean" tail is gated on full recovery success; per-site failure tests plus a success regression guard present. _(Report 10-5: Complete)_
6. **`--plain` not parsed on regenerate** → fixed in task 10-6. Flag defined in `parseRegenerateFlags`, threaded into `NewForStartup` (hardcoded `false` removed), composes with the other regenerate flags; flag-parse test rows added. _(Report 10-6: Complete)_
7. **Production timeout misclassification** → fixed in task 10-7. `translateRun` inspects `ctx.Err()` and wraps `DeadlineExceeded`/`Canceled` with `%w` ahead of the `*exec.ExitError` branch, so `classifyFatal` sees a non-retried timeout. Real-kill runner + transport tests present. _(Report 10-7: Complete)_
8. **Whitespace-only `$EDITOR` panic** → fixed in task 10-8. `ResolveEditor` trims candidates so a blank-after-trim value falls through to `vi`, making `fields[0]` provably safe; resolution-level and end-to-end no-panic + temp-cleanup tests present. _(Report 10-8: Complete)_
9. **Blocking-stage spinner leaks across `Warn`** → fixed in task 10-9. A single general presenter fix stops the active spinner before any `Warn` during a blocking stage, covering both the batch-skip and cache-reuse/miss/unreadable sites; both required tests plus a no-op safety test present; normal stage lifecycle unchanged. _(Report 10-9: Complete)_

Additional analysis-cycle remediations (Phases 11–12), also verified:

10. **Gate regenerate gh-auth on the resolved publisher** (11-1) → the gate set now selects `CallsProvider = target.writesProvider() && publisherResolved`; both call sites thread `publisher != nil`; `CommitsAndPushes` stays target-only; nil/non-nil/downgrade-flow/changelog-only tests present — regenerate now matches the forward spine's `if publisher != nil` guard. _(Report 11-1: Complete)_
11. **Remove the load-dependent real-deadline timing flake** (12-1) → the test drops all marker plumbing and the invocation-count assertion, keeps the timing-robust `ErrTimeout` / not-`ErrGenerationFailed` checks against a real `sleep 5` vs 300ms kill; not-retried coverage stays in the deterministic FakeRunner sibling. _(Report 12-1: Complete)_
12. **Collapse the four producer closures into two** (12-2) → the reuse-vs-fresh dispatch lives in exactly one function per concern; single-version producers keep their signatures and delegate to the canonical Resolution-keyed producers; batch caller and engine helpers unchanged. _(Report 12-2: Complete)_

## Recommendations

_Open non-blocking items below are **unchanged by the Phase 10–12 remediation**. Items 1–5 (do-now) were applied during the Phase 1–9 review session (commit `review(mint-release-tool): apply do-now fixes`) and are retained for the record under "Previously applied" at the foot of this section. Items 6–41 remain open. Items 42–48 are **newly surfaced by this remediation review** and not yet applied._

### Do now

42. `cmd/mint/regenerate_flags_test.go:5-9` — `TestParseRegenerateFlags`'s doc comment still says "the `--all` / `-y` booleans" and "parse skeleton only", predating the `--plain` addition; append `--plain` to the enumerated surface so the comment matches the asserted columns (Report 10-6)
43. `internal/engine/editor_test.go:22-25` — the comment block states `ResolveEditor` "checks for non-empty" and treats `""` as "unset"; the check is now `strings.TrimSpace(...) != ""`, so reword to "treats empty-or-blank as unset" to match the behaviour the same test now exercises (Report 10-8)

### Quick-fixes

6. `bookkeepingWillCommit` duplicates the commit-or-not rule that `record.bookkeepingPaths` already owns — expose a single shared predicate (e.g. `record.BookkeepingWillCommit`) both the spine and `CommitBookkeeping` consume, so `made.Commits` can't desync.
   - `internal/engine/release.go:918-920` / `internal/record/commit.go:100-109` (Reports 3-7, 3-8)
7. `reuse --all` reads each body-bearing tag's annotation twice (skip-check `ReadTagBody` then `ProduceBody`→`ReadReuseBody`) — thread the pre-read body into the reuse producer to halve git calls on large backfills.
   - `internal/engine/regenerate_batch.go:227` / `cmd/mint/regenerate_all.go:91` (Reports 5-11, 5-12)
8. `internal/engine/regenerate_batch.go:339` — `gatePerVersion` builds the inner `RegenerateWriteRequest` with no `Target` (inert today since the gate ignores it); set `Target: req.Target` explicitly (Report 5-11)
9. Config validation robustness:
   - `internal/config/config.go:344-352` — add a test that fails loudly if `DecodeError` matches none of the four mapped `typeErrorMessages` field-paths, converting a future go-toml/v2 silent-degrade into a hard failure (Reports 6-3, 6-1)
   - `internal/config/config_test.go:700` — assert the targeted `[hooks]`-nest message specifically (contains "not valid at the top level", not the generic "unknown") (Report 6-2)
10. Test-coverage tightening (engine seams):
    - `regenerate_interactive_test.go:51` — drive a `RegenerateRun` fresh `[r]` flow asserting the injected `ProduceRegenerator` is consulted (Report 7-1)
    - `regenerate_batch_test.go` — assert a mid-batch degenerate version yields `StubBody` collected (not skipped) and later versions still run (Report 7-2)
    - `regenerate_stageevents_test.go` — assert the batch per-version notes `StageStarted(Blocking)`/`StageSucceeded` fire and a skip emits no `StageSucceeded` (Report 7-3)
    - `regenerate_write_test.go` — `RegenerateWrite`-level no-op test (no commit, no `git push origin HEAD`, `pushed=false`); and a write-stage (not probe-stage) post-push provider failure exercising warn-only; fix the stale `:188-191` comment name (Reports 8-1, 5-9)
11. Test-coverage tightening (notes / version / preflight / config / record):
    - `version_test.go:171-187` — assert `Args == ["tag","--list"]`; add a `git tag --list` error-path test (Report 1-3)
    - `internal/version/resolve_test.go` — add a single-resolve `git tag --list` failure test (Report 5-3)
    - `preflight_test.go:18` — add a named gitignored-exempt subtest (asserting no `--ignored`) (Report 1-5)
    - `prompt_test.go` — cover `resolveContext` directory-as-context and non-ENOENT stat-error branches (Report 2-5)
    - `changelog.go:99-111` — test the non-ENOENT read-error branch wraps with "reading CHANGELOG.md" (Report 1-9)
    - `versionfile_test.go` — add a QuoteMeta anchoring test with regex-metachar surroundings (Report 3-6)
    - `hooks_test.go` — assert an empty-string element inside a hook array is dropped while preserving order (Report 3-1)
    - `release_test.go:1228-1241` — add a `" commit -m"` arm to `assertNoMutation`; assert `.mint.toml` byte-unchanged after an `r` run (Reports 2-12, 2-14)
12. Atomic-write / dry-run cache test & wiring polish:
    - `atomicwrite_test.go` — cover a Write/Close failure branch; add domain-noun assertions in `versionfile_test.go`/`changelog_test.go` (Report 7-4)
    - `release_dryruncache_test.go:302-319` — export a dir/extension accessor on `Store` instead of trimming `EntryPath` (Report 4-7)
    - `release_dryrun_test.go:385-392` — reference a shared "publish skipped" label constant rather than a string literal (Report 4-7a)
    - `internal/engine/release.go:746` — one-line guard comment that a nil `NoteCache` ⇒ always-generate (Report 4-8)
13. ~~`cmd/mint` — cmd-level test for an empty remote yielding a nil/unresolved publisher~~ → **resolved by Remediation #2** (the discarded resolve error was a crash, fixed and tested in task 10-2). (Report 7-5)
14. `internal/version/resolve.go:151` — `highestBelow` is an O(n) pass; optionally share predecessor-finding with the `--all` sorted path (Report 5-3)
15. `cmd/mint/regenerate_flags.go:142` — `--target` as a trailing token with no value falls through to flag's generic error; add a test (or guard for the curated message) (Report 5-1)
16. `cmd/mint/main.go:45,192` and version paths — add thin end-to-end `run([]string{…})` tests for `runRelease`, `mint version`, and `mint --version` (and replace the tautological byte-identity test) (Reports 1-11, 6-8)
16a. `internal/notes/changemap.go:197` — rename the rendered `"New package: "` label to `"New package/dir: "` (a directory area, not necessarily a package) AND update the two asserting tests (`range_test.go:217`, `regenerate_fresh_test.go:216`). _Touches test logic_ (Report 2-4)
16b. `internal/engine/release_pretaghook_test.go:293-295` — remove the stale `f.SeedSequence("git", ScriptedOut(startingSHA))` (the surgical unwind no longer re-probes HEAD) and fix the accompanying "unwind re-probes HEAD" comment. _Comment fix coupled to seed removal_ (Report 3-3)
44. `internal/engine/regenerate_nilpublisher_test.go` — add a test for `engine.ResolvePublisher`'s abort arm (a non-`ErrProviderUnresolved` resolver error → returns the surfaced error, `StageFailed` emitted); the "aborts with a clear message" branch is currently asserted only on the forward path, not on the shared regenerate entry. Low priority — the helper reuses the forward-covered `resolvePublisher` (Report 10-2)
45. `cmd/mint/regenerate_run_test.go` / `regenerate_all_test.go` — add a focused fresh-body test (single-bound and/or batch-threaded) that drives `RegenerateFreshBody` via a fake/stub AI transport and asserts the fresh route is taken, closing the one branch (fresh body) currently proven only by structure rather than an executed assertion (Report 12-2)

### Ideas

17. Consolidate the duplicated `publish.ResolvePublisher(engine.RemoteURL(…), …)` resolve-and-discard step shared by the two regenerate cmd paths into one `engine.ResolvePublisher(ctx, r, cfg)` helper. → **resolved by Remediation #2** — the shared helper now exists and performs real `ErrProviderUnresolved` handling.
    - `cmd/mint/main.go:166` / `cmd/mint/regenerate_all.go:49` (Reports 4-9, 7-5)
18. gh-output classification brittleness (substring-matching English stderr) — decide whether to harden against gh wording/locale changes (exit code / `gh api` 404 / structured signal):
    - `internal/publish/publish.go:118,148` — `ReleaseExists` "release not found" marker (Reports 1-8, 5-7)
    - `internal/preflight/preflight.go:261` — `CheckGhAuth` probes `gh auth status` with no `--hostname`, so any-host auth passes (Report 1-8)
19. go-toml/v2 error-text coupling — `translateTypeError` and the hand-maintained `typeErrorMessages` map identify fields by substring-matching the library's internal error text; decide whether to derive these structurally (struct-tag reflection / custom unmarshal types) or accept the test-guarded coupling.
    - `internal/config/config.go:330-352` (Reports 6-1, 6-3, 6-4, 1-2)
20. `internal/notes/select.go:155-158,185` — a pre-AI `AssembleDiff` failure (or a reuse-hook error) reports `KindNormalAI`; decide whether a dedicated/neutral Kind is warranted for callers that branch on Kind for reporting (Report 2-10)
21. `internal/ai/transport.go:170-179` — `isValid` is empty/whitespace-only; an exit-0 polite refusal passes. Confirm minimal refusal detection is intended or add a heuristic (Report 2-1)
22. `internal/notes/resolve.go:91` / `degenerate.go:14` — the `--no-ai`/fallback empty-log floor reuses the degenerate `StubBody()` wording; decide whether the floor warrants its own "no commits since last tag" phrase (Report 2-7)
23. `internal/notes/changemap.go:208-211` — the novelty/notable lists are uncapped; on a very large release they could themselves become mush. Decide whether to bound/summarize (Report 2-4)
24. `internal/notes/changemap.go:49,55` & `assemble.go:173` — `append([]string{…}, excludePathspecs()…)` relies on the slice being freshly allocated; make the no-aliasing contract explicit if `excludePathspecs` is ever cached (Report 3-9)
25. `internal/record/versionfile.go:22` — `versionSlot` matches only bare X.Y.Z; an embedded source already holding `1.3.9-rc1` would trip the zero-match abort. Broaden the slot or document the X.Y.Z-only constraint at the config boundary (Report 3-6)
26. `internal/record/rebuild.go:111-121` — `composeChangelog` relies on each preserved block carrying its own trailing newline(s); decide whether to normalise preserved-block trailing whitespace (latent — only record-written sections are preserved today) (Report 5-13)
27. `internal/hooks/hooks.go:113-120` — `normaliseAnySlice` silently skips non-string `[]any` elements; confirm Phase-6 schema validation owns rejecting a non-string array element so the case isn't lost (Report 3-1)
28. `internal/git/mutator.go:166,217` — clarify that `retryBudget` counts total attempts (not clears) so a stale lock cleared on the (N-1)th pass gets one retry; document the empty-`lockPath` fall-through; add a test for the generic no-path "Another git process" signature (Report 4-1)
29. `internal/engine/unwind.go:104,117,146-148` — add a test that a mid-unwind `Mutate` failure is non-fatal (best-effort contract); make the `surgicalSummary` impossible `default` branch explicit. _Partly addressed by Remediation #5 (recovery errors are now captured + warned); the best-effort non-fatal contract test still stands._ (Report 4-2)
30. `internal/engine/release.go:345-349` — `--autostash` is not gated on `!opts.DryRun`, so `--dry-run --autostash` with dirty WIP briefly mutates the tree via real `git stash push`/`pop` (net-unchanged but in tension with "dry run never reaches the Mutator"); decide whether dry-run should skip the stash (Report 4-4)
31. `internal/engine/release_anybranch_test.go` — add an explicit off-branch `--any-branch` + unauthenticated-gh-aborts test so gh-auth coverage matches the clean-tree/tag-free/remote-sync trio (Report 4-5)
32. `internal/version/version.go:79` — `TrimPrefix` strips at most one prefix occurrence (`vv1.2.3` → loud rejection); decide whether a tailored stray-prefix message is worth adding (Report 4-6)
33. `internal/notes/select.go:104` (`ReuseFunc`) — takes no `context.Context`, so the cache `Lookup` file-I/O is uncancellable; decide whether to thread `ctx` for consistency (Report 4-8)
34. `internal/engine/release.go:562-563` — the non-`ErrProviderUnresolved` resolve-error abort branch is presently unreachable (GitHub driver construction does no I/O); keep as forward-compat scaffolding (+ a guard test) or drop (Report 4-10)
35. `internal/engine/init.go:162-165` — `fileExists` treats any non-`IsNotExist` stat error as "exists" (silent skip); confirm skip-silently vs abort-with-diagnostic is the intended direction (Report 6-7)
36. `internal/initgen/initgen.go:42` — the `diff_exclude` example `'*.min.js'` is a top-level-only basename glob; consider a recursive `'**/*.min.js'` so a copied example matches nested files (Report 6-5)
37. `internal/fsutil/atomicwrite.go:43` — the shared helper now always `Chmod`s (notescache previously relied on `CreateTemp`'s 0o600 default); observable mode unchanged — optionally document the deliberate addition (Report 7-4)
38. `Release.Fallback` (`internal/config/config.go:155`) is a used-but-undocumented schema key (consumed via `rel.Fallback`); decide whether to add `fallback` to the spec's schema block or keep it internal (Report 6-1)
39. `internal/runner/exec_runner_test.go` — ExecRunner tests assume a POSIX shell + coreutils; decide whether to guard (build tag / GOOS) or document so `go test ./...` holds on non-POSIX CI; and whether `FakeRunner` should match on args, not just command name (Report 1-1)
40. `internal/fsutil/atomicwrite.go:48` — the atomic rename is never torn but isn't fsync-durable; align the doc-comment's "crash-safe" wording with the intended guarantee (Report 1-9)
41. `internal/engine/release.go:953` — surface `preflight.ErrNoUpstream` with tailored "set an upstream / push -u" guidance rather than the bubbled wrapped text (Report 1-6)
46. `internal/engine/regenerate_write.go:317-324` (`pushChangelogCommit`) — the regenerate spine has the cancellation-resilient recovery (`WithoutCancel` at :342) but **no** explicit pre-push `ctx.Err()` gate mirroring `release.go:610`. A Ctrl-C in the no-subprocess window between the regenerate CHANGELOG commit and its plain `git push origin HEAD` would not be caught until the push subprocess surfaces it (in production `git push` honours cancellation and routes through `resetAndAbort` anyway). Lower-risk than the forward spine; decide whether a symmetric pre-push gate is worth it for parity (Report 10-3)
47. `internal/engine/unwind.go:93` + `release_cancellation_test.go` — the `context.WithoutCancel` resilience in `Unwind` is pinned only indirectly (the autostash pop runs after cancellation) and the `FakeRunner` ignores ctx, so a regression dropping `WithoutCancel` would fail no current test. Decide whether a runner seam that records the ctx passed to `Mutate` (asserting unwind `Mutate` calls receive a non-cancelled ctx) is worth adding to make the invariant regression-proof (Report 10-3)
48. `internal/engine/unwind.go:158-160` — `unwindEvent`/`Unwound` fires only when `(tagDeleted || commitsReset)`; if the only issued op fails (e.g. tag-not-made + a single reset that fails) the user gets the manual-cleanup `Warn` but **no** `Unwound` summary line at all. Decide whether a fully-failed single-op unwind should still emit a summary with the "manual cleanup required" tail, or whether the standalone `Warn` suffices (Report 10-5)

### Previously applied (Phase 1–9 review session)

_Retained for the record — these zero-risk doc/comment fixes were applied in the earlier review (commit `review(mint-release-tool): apply do-now fixes`); a few items turned out to need no change and are noted inline._

1. Comment / wording fixes that touch no logic — **applied**: `internal/runner/exec_runner.go:81` (ExitCode nil-safety note, Report 1-1); `internal/notes/assemble.go:112` (`2+len(globs)` cap rationale, Report 2-2); the reported "single-slash" comments at `internal/notes/degenerate.go:42`, `internal/engine/release_test.go:1244`, `internal/engine/regenerate_write.go:299` were grep artifacts — **no change needed** (Reports 7-2, 1-11, 5-8)
2. Stale / misleading test & file comments — **applied**: `release_pretaghook_test.go:13-18` (Report 3-3); `release_downgrade_test.go:30-33` (Report 4-10); `regenerate_batch_changelog.go:117` (Report 5-13); `release_commitgraph_test.go:22-24` (Report 3-8); `release.go:1386-1387` `ErrNotInteractive` reword (Report 2-14); `regenerate.go:5` header reviewed — **no change needed** (Report 5-4)
3. Doc comments omitting the strategy-aware `version_file` exclude tier — **applied**: `internal/notes/changemap.go:22-26`, `internal/notes/assemble.go:138-144` (Report 5-6)
4. Config / generator doc tidies — **applied**: `internal/config/config.go:344-352` (one-field-per-decode-error invariant, Report 6-1); `config.go:133-136` paragraph order already matched — **no change needed** (Report 6-4); `internal/initgen/initgen.go:62` (`pre_tag` array-vs-string note, Report 6-5)
5. `cmd/mint/version.go:37` — note the `NewForStartup(false, true, …)` first arg is `plainFlag` (TTY still governs plain/pretty) — **applied** (Report 6-8)
