# Implementation Review: Mint Release Tool

**Plan**: mint-release-tool
**QA Verdict**: Request Changes

## Summary

The mint release-tool implementation is, across 80 verified tasks spanning nine phases (walking skeleton → AI notes engine → hooks/projection → robustness → regenerate → config/init → three analysis cycles), of consistently high quality: the seam architecture (`CommandRunner`, `Publisher`, `Presenter`, lock-resilient `Mutator`) is clean and uniformly applied, the spec's point-of-no-return model and surgical-unwind invariants are faithfully and structurally enforced, and test coverage is thorough, behaviour-focused, and rarely over- or under-built. 79 of 80 tasks are Complete with no blocking issues. **One blocking issue** was found: the interactive `mint release regenerate <ver>` path (no `-y`, no `--target`) runs an empty preflight gate-set because the write target is resolved *after* preflight, so an interactively-chosen changelog/both commits+pushes without the clean-tree/branch/remote-sync gates and release/both writes the provider without the gh-auth gate — a deviation from spec §"Preflight subset per verb" (lines 547-550). It is a localized integration-wiring fix. Beyond that, the findings are a long tail of non-blocking polish: three latent bugs (a production timeout-misclassification, a whitespace-only `$EDITOR` panic, a pretty-mode spinner leak), a handful of recurring brittleness themes (gh-stderr substring matching, go-toml/v2 error-text coupling), and test-tightening / doc-comment nits.

## QA Verification

### Specification Compliance

Implementation aligns with the specification with one deviation (the blocking item below). Verifiers confirmed faithful realization of: tag-as-truth version determination and strict 3-part SemVer grammar; the seven-stage spine and PONR asymmetry (pre-push surgical unwind vs post-push warn-only); the layered AI notes engine (context assembly vs content-agnostic transport), Change Map salience preamble, notes-path precedence (first-release → degenerate → `--no-ai` → normal AI), and `on_notes_failure` governing only the normal path; single-body whole distribution to tag/CHANGELOG/provider; the interactive `y/n/e/r` gate with `r` omitted on no-AI paths; hooks (`preflight`/`pre_tag`/`post_release`) with the commit-interplay rule; strategy-aware diff exclusion; lock-resilient git; `--autostash`/`--any-branch`/`--set-version`/`--dry-run` (incl. dry-run note caching with deterministic reuse); the regenerate two-axis contract (source × target) with create-or-update probe and batch skip-and-continue/whole-file rebuild; and the verb-namespaced TOML schema with fail-loud key/type validation plus the provider-VALUE warn-and-downgrade carve-out. Documented as-built evolution (later phases extending earlier files) was reviewed and judged legitimate, not scope creep.

### Plan Completion
- [x] Phase 1–9 acceptance criteria met (one deviation: interactive-regenerate preflight, below)
- [x] All 80 completed tasks implemented and verified
- [x] No scope creep — later-phase extensions of earlier files are deliberate and tested

### Code Quality

No blocking code-quality issues. Strong throughout: single-responsibility seams, DRY via genuinely shared helpers (`emitBlockingStageStarted`, `stageAndCommitChangelog`, `pushChangelogCommit`, `record.BookkeepingSubject`, `fsutil.WriteFile`, `excludePathspecs`), idiomatic error handling (sentinels + `errors.Is/As`, `%w` wrapping), and intent-rich doc comments. Recurring low-grade fragilities (non-blocking): provider/release classification by gh-stderr substring matching, and bad-type config messages keyed on go-toml/v2's internal error-text format.

### Test Quality

Tests adequately verify requirements. Uniformly behaviour-focused (exact argv, event timelines, on-disk bytes, invocation counts) over `FakeRunner` / `RecordingPresenter` / fake `Publisher` seams; balanced (both under- and over-testing avoided). The notable gap is the untested interactive-regenerate preflight seam (the blocking issue is invisible to the engine unit tests because preflight lives one layer up in the cmd wiring). Smaller test-tightening opportunities are listed under Recommendations.

### Required Changes

1. **Close the interactive-regenerate preflight gate bypass.** For a bare `mint release regenerate <ver>` (no `-y`, no `--target`, no `--reuse`/`--fresh`), `validateRegenerateRequest` leaves `Target = targetUnset` (`cmd/mint/regenerate_validate.go:30-52`); preflight then runs at `cmd/mint/main.go:129` as `regenerateGateSet(targetUnset)` → `{CallsProvider:false, CommitsAndPushes:false}` (`cmd/mint/regenerate_preflight.go:16-21`), i.e. **zero gates**. The real target is resolved *after* preflight inside `RegenerateRun` → `resolveTarget` (`internal/engine/regenerate_interactive.go:245-259`), and neither `RegenerateRun` nor `RegenerateWrite` (`internal/engine/regenerate_write.go:137-179`) re-runs any gate. Result, against spec lines 547-550: an interactively-chosen `changelog`/`both` commits+pushes the CHANGELOG with **no** clean-tree/branch/remote-sync gate, and an interactively-chosen `release`/`both` writes the provider with **no** gh-auth gate. Fix: after `ResolveRegenerateAxes` resolves the target, run `RegeneratePreflight(regenerateGateSet(resolvedTarget))` before the write (or move axis resolution ahead of preflight). Add a test asserting an interactive `changelog` choice runs clean-tree/branch/remote-sync and an interactive provider choice runs gh-auth. (Reports 5-10, 5-4)

## Recommendations

### Do now

1. Comment / wording fixes that touch no logic:
   - `internal/runner/exec_runner.go:81` — note `cmd.ProcessState.ExitCode()` is nil-safe (-1 for unstarted/not-found) so the unconditional call doesn't read as a latent nil-deref (Report 1-1)
   - `internal/notes/assemble.go:112` — state the `2+len(globs)` cap is CHANGELOG + worst-case version_file headroom so it isn't "tightened" to `1+len(...)` (Report 2-2)
   - `internal/notes/changemap.go:197` — `"New package: "` label vs a directory area; use `"New package/dir: "` or align the doc (Report 2-4)
   - `internal/notes/degenerate.go:42` — `/ StubBody` single-slash comment → `//` (Report 7-2)
   - `internal/engine/release_test.go:1244` — `/ (here…)` single-slash comment → `//` (Report 1-11)
   - `internal/engine/regenerate_write.go:299` — `/ is narrated…` single-slash comment → `//` (Report 5-8)
2. Stale / misleading test & file comments:
   - `internal/engine/release_pretaghook_test.go:293-294` and `:13-18` — fix the "unwind re-probes HEAD" comment (surgical unwind drives off MadeState) and note the duplicated `pretagArtifactSubject` mirrors production (Report 3-3)
   - `internal/engine/release_downgrade_test.go:30-33` — reword the garbled `seedDowngradeGit` doc (Report 4-10)
   - `internal/engine/regenerate.go:5` — file-header self-reference disagrees with filename `regenerate.go` (Report 5-4)
   - `internal/engine/regenerate_batch_changelog.go:117` — add a call-site note that the preserved path needs no `dates` entry (Report 5-13)
   - `internal/engine/release_commitgraph_test.go:22-24` — cross-reference `record.BookkeepingSubject`/`pretagArtifactSubject` as the subject source of truth (Report 3-8)
   - `internal/engine/release.go:1386-1387` — reword the `ErrNotInteractive` "unreachable… defended anyway" comment to match the generic `abort` handling (Report 2-14)
3. Doc comments that omit the strategy-aware `version_file` exclude tier:
   - `internal/notes/changemap.go:22-26` and `internal/notes/assemble.go:138-144` (Report 5-6)
4. Config / generator doc tidies:
   - `internal/config/config.go:344-352` — note the "one offending field per decode error" invariant so map-iteration order isn't assumed load-bearing (Report 6-1)
   - `internal/config/config.go:133-136` — align the `Release` struct doc paragraph order with field order (`Fallback`) (Report 6-4)
   - `internal/initgen/initgen.go:62` — note the `pre_tag` array-form replaces the string form (only one may be set) (Report 6-5)
5. `cmd/mint/version.go:37` — note the `NewForStartup(false, true, …)` first arg is `plainFlag` (TTY still governs plain/pretty), not "forces plain" (Report 6-8)

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
13. `cmd/mint` — when the publisher resolve-and-discard duplication (Recommendation 17) is lifted, add a cmd-level test asserting an empty remote yields a nil/unresolved publisher through the shared path (Report 7-5)
14. `internal/version/resolve.go:151` — `highestBelow` is an O(n) pass; optionally share predecessor-finding with the `--all` sorted path (Report 5-3)
15. `cmd/mint/regenerate_flags.go:142` — `--target` as a trailing token with no value falls through to flag's generic error; add a test (or guard for the curated message) (Report 5-1)
16. `cmd/mint/main.go:45,192` and version paths — add thin end-to-end `run([]string{…})` tests for `runRelease`, `mint version`, and `mint --version` (and replace the tautological byte-identity test) (Reports 1-11, 6-8)

### Ideas

17. Consolidate the duplicated `publish.ResolvePublisher(engine.RemoteURL(…), …)` resolve-and-discard step (+ comment) shared by the two regenerate cmd paths into one `engine.ResolvePublisher(ctx, r, cfg)` helper.
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
29. `internal/engine/unwind.go:104,117,146-148` — add a test that a mid-unwind `Mutate` failure is non-fatal (best-effort contract); make the `surgicalSummary` impossible `default` branch explicit (Report 4-2)
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

### Bugs

42. `internal/runner/exec_runner.go:97-100` — **production timeout misclassification.** On a context-deadline kill, `exec.CommandContext` returns an `*exec.ExitError`, so `translateRun` wraps the ExitError, not `context.DeadlineExceeded`. The AI transport's `classifyFatal` (`internal/ai/transport.go:160`) detects timeouts solely via `errors.Is(err, context.DeadlineExceeded)`, which is therefore false in production — a real ~60s timeout is misclassified as bad content and **retried**, defeating the spec's "timeout is not retried" guarantee (worst-case latency becomes two timeouts). The transport's own tests inject a `DeadlineExceeded`-wrapping error so they pass; the gap is the runner not surfacing the deadline. Fix: in `translateRun`, when `ctx.Err()` is `DeadlineExceeded`/`Canceled`, wrap that cause so `errors.Is(err, context.DeadlineExceeded)` holds. (Report 2-1)
43. `internal/engine/editor.go:99` — **whitespace-only `$EDITOR` panic.** `args := append(fields[1:], tmpPath)` then `fields[0]` panics when the resolved editor value is whitespace-only (e.g. `EDITOR=" "`), since `strings.Fields` returns an empty slice and `ResolveEditor` only guards against the empty string. Latent, low-likelihood (operator misconfiguration). Fix: guard `len(fields)==0` after splitting and treat as "no launchable editor" (Warn + `ErrEditorReturnToGate`), or have `ResolveEditor` fall through to `vi` when blank after trimming. (Report 2-13)
44. `internal/engine/regenerate_batch.go:244-248` — **pretty-mode spinner leak on notes-failure skip.** On the `--all` notes-production-failure skip path the blocking notes spinner is started but `notesDone()` is never called and `reportSkip`→`Warn` doesn't stop it (`internal/presenter/pretty.go:520`/`:905`), so for the last/only version hitting diff-too-large the spinner animates over the `⚠ skipped` and end-summary lines until exit. Cosmetic, pretty-mode only; invisible to `RecordingPresenter` tests. Fix: stop/close the stage before warning on the notes-failure skip (e.g. emit `StageFailed` for "notes" before `reportSkip`, or have the skip path stop any active spinner). The reuse skip path is unaffected. (Report 5-12)
