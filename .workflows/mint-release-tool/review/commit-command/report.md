# Implementation Review: Commit Command (`mint commit`)

**Plan**: mint-release-tool
**QA Verdict**: Approve

## Summary

`mint commit` is implemented cleanly and faithfully to the specification across all 31 plan tasks (7 phases: walking skeleton → staging model → `$EDITOR` fallback → interactive gate actions → auto-push → two analysis-driven refactor cycles). Every task verifier returned **Complete** with **zero blocking issues**. The load-bearing spec invariants are all present and tested at the seam level: *mutate-nothing-until-accept* (staging deferred to the accept tail), *never-unwind-after-accept* (warn-don't-unwind on push failure), the single `$EDITOR` degradation path shared by `--no-ai` / AI-failure / oversized-diff, the AI-path-only `Continue?` gate with `e`/`r` actions, and flag-only `-p` with no config default. The three-layer engine split (L1 git-aware context → L2 content-agnostic message engine → L3 commit glue) is consumed as designed, and the two analysis cycles successfully single-sourced the emptiness verdict, the per-mode git-source selection, the accept tail (`commitAccept`), and the lock-resilient `Mutator` sink. Remaining notes are minor, non-blocking doc/test/dedup refinements.

## QA Verification

### Specification Compliance

Implementation aligns with the specification. Verified directly against `specification.md`:

- **Three-layer engine** — L1 staged-diff source (`git diff --cached`), `diff_exclude` → `:(exclude)` pathspecs, `max_diff_lines` guard short-circuiting L2 *before* any transport call; L2 consumed as content-agnostic transport with one retry; L3 supplies the Conventional Commits prompt and the two-knob override (`[commit].context` / `[commit].prompt`, precedence override > context > default, fail-loud on unreadable override).
- **Staging model** — two faithful flags (`-a` = `git commit -a` tracked-only; `-A` = `git add -A` incl. untracked), mutual-exclusion fail-loud before any work, would-be-staged diff computed read-only per mode, staging applied only on accept.
- **Empty-staging matrix** — keyed on *actual* tree state (`git status --porcelain`), distinguishing clean-tree vs changes-exist-but-none-staged, with mode-specific guidance.
- **`$EDITOR` fallback** — git's editor resolution order (`git var GIT_EDITOR`), save-as-accept (no separate gate), TTY/`-y` fail-loud when no message source, oversized path is a generate-*skip* not a *failure*, regeneration failure routed to the same fallback.
- **Review gate** — AI-path-only, `e` loops back to the gate (verbatim, no AI reprocessing) with empty-save preserving the prior message, `r` regenerates with a one-time non-persisted context line; `e` graceful-degrade vs fallback fail-loud correctly distinguished.
- **Auto-push** — flag-only `-p`, no config key anywhere, single shared `pushAfterCommit` step after both accept paths, one generic warn + verbatim git stderr pass-through, no cause classification, no pre-push/remote-sync gate, no unwind.
- **Config** — verb-namespaced `[commit]` table with only `context`/`prompt`; no push/scope keys; `commit_prefix` correctly stays release-only.

No deviations from settled decisions were found. Forward progress noted by verifiers (e.g. Phase-1 code already handling `-a`/`-A`) is authorised by later tasks, not drift.

### Plan Completion

- [x] Phase 1–7 acceptance criteria met (all 31 tasks verified Complete)
- [x] All tasks completed (`completed_tasks` == `reviewed_tasks`, 31/31)
- [x] No scope creep (no unplanned features; analysis-cycle tasks 6-x/7-1 are plan-authored refactors)

### Code Quality

No issues found. Code follows the project's Go conventions: sentinel-error seams (`ErrNoEditor`, `ErrDiffTooLarge`, `errPushFailed`), shared predicates instead of duplicated branches (`editorUnavailable()`, `isAITransportFailure`), single-sourced descriptors (`sourcesForMode`/`sourceArgs`, `commitAccept`, `pushAfterCommit`), and a renamed `Mutator` seam mirroring `engine.ReleaseDeps.Mutator`. Complexity is low and code paths are clear.

### Test Quality

Tests adequately verify requirements. Coverage is behaviour-focused and balanced across all phases, including genuine lock-retry proofs that staging/commit/push flow through the `git_safe` Mutator (not the raw runner), exact-argv assertions on the read-only probes, boundary tests on `max_diff_lines`, and end-to-end `-Apy` unattended runs. The non-blocking notes below identify a few small coverage gaps and minor redundancy — none material.

### Required Changes (if any)

None. No blocking issues.

## Recommendations

### Do now

1. `internal/commit/preflight.go` — doc-comment / argv drift (Report 6-1)
   - `:89-92` — `wouldStageNothing` doc-comment renders the untracked probe without the `-z` flag that `untrackedBaseArgs()` actually carries; add `-z` so the documented argv matches the executed one.
   - `:136-153` — add a one-line note that `stagedProbeArgs`/`trackedProbeArgs`/`untrackedProbeArgs` are test-facing builders (production routes through `probeArgs` after the 7-1 restructure), so a future reader does not mistake them for live preflight callers.
2. `internal/commit/run.go:347` — comment says the fallback commit "exits 0", but `commitAccept` can return `errPushFailed` (non-zero) on a `-p` push failure after a successful commit; tighten to "exits per the accept tail (0 on commit success, non-zero only on a failed `-p` push)" (Report 3-2).

### Quick-fixes

3. `internal/config/config_test.go` — add a `[commit]` unknown-key test (e.g. `[commit]\npush = true`) mirroring the existing `[release]`/top-level unknown-key tests, to pin the deliberately-excluded-keys guarantee against regression (Report 1-1).
4. `internal/commit/prompt_test.go` — tighten default-prompt tests (Report 1-2)
   - `:96-108` — `TestDefaultPrompt_CarriesEveryRule` duplicates substrings already asserted by the four targeted tests; fold the targeted tests into the table (or drop the overlapping rows) to remove double-coverage.
   - `:50-91` — add one assertion that `ComposePrompt(a,b) == a+"\n\n"+b` to pin the exact blank-line separator the doc comment promises (currently only indirectly covered).
5. `internal/commit/generate.go:286-292` — move `excludePathspecs` into `source.go` beside `sourceArgs` (its only caller), co-locating the full per-mode argv-assembly surface in the declared single source of truth (Report 1-3).
6. `internal/commit/surface.go:15-37` — `surface` and `surfaceOutput` are near-identical; have `surface` delegate to `surfaceOutput("")` to drop the duplicated `StageFailed`/return pair (Report 1-4).
7. `cmd/mint/commit_flags_test.go:131-153` — if the `-a`/`-A` conflict guard ever moves out of the pure parse layer into `commit.Run`, add an integration assertion that zero git invocations occur on a `-aA` run, to lock the "before any git/AI" criterion at the integration layer (Report 2-1).
8. `internal/commit/preflight.go:192-197` — `gitOutputEmpty` uses `strings.TrimSpace`, which does not strip a trailing NUL from the `-z` untracked probe; behaviour is correct today, but harden the shared helper (treat NUL as whitespace / check length after trimming NUL) before it is reused where a lone NUL must count as empty (Report 2-4).
9. `internal/commit/run_editor_push_test.go` — add an oversized-diff (3-4) editor-drop test with `-p` armed asserting commit-then-push on a non-empty save (the named AC is covered only structurally via the shared `commitAccept` tail; the AI-failure variant has the explicit test) (Report 5-3).
10. `internal/commit/run_push_fail_test.go:108-361` — add an assertion (the `hasKind` helper already exists) that `RunFinished` still fires and exactly one push warn is emitted on a failed-push path, locking "the warn does not suppress close-out" on the path where it matters (Report 5-4).
11. `internal/commit/run_failloud_test.go:126` — `failLoudDeps` keeps the boolean-triple positional signature the 6-4 Solution advised replacing; since it now delegates to `editorDeps`, pass `editorDepsOptions` at its 13 call sites and delete `failLoudDeps`, removing the unreadable positional bool runs (Report 6-4).

### Ideas

12. `internal/config/config_test.go:1416` — `TestResolveCommitPrompt_UnreadablePromptFile` silently passes as root (0o000 ignored); decide whether to guard with a skip when `os.Geteuid() == 0` to keep the assertion meaningful in root/CI containers (Report 1-1).
13. `internal/commit/generate_test.go:273,394` — `SurfacesDiffTooLargeDistinctFromGenerationFailure` overlaps `MaxDiffLinesGuardAppliedBeforeTransport`; decide whether the explicit distinctness witness earns its keep or should fold into the guard test (Report 1-3).
14. `cmd/mint/main.go:350-352` — no process-boundary test drives `runCommit` through a gate-decline / non-TTY path to assert `exitCode() == 1`; decide whether the shared `exitCode()` coverage suffices or a cmd-level commit-abort → exit-1 test should lock the wiring (Report 1-5).
15. `internal/commit/editor_open.go:79-88` — the `(ok=false, nil)` path treats any non-not-found `RunInteractive` error as a benign quit/abort, silently swallowing a genuinely broken editor invocation; matches the spec's save-as-accept intent, but flag as a decision point should distinguishing "user quit" from "editor crashed" ever matter (Report 3-2).
16. `internal/commit/preflight.go` — per-mode source-builder decisions (Reports 2-4, 7-1)
    - `:179-182` — the `AddAll` arm of `emptyStagingError` is documented-unreachable and defensively returns the clean-tree message; decide whether to keep the defensive branch, collapse it into the default path, or signal the invariant with a guard-panic vs comment.
    - `:143-153` — `stagedProbeArgs`/`trackedProbeArgs`/`untrackedProbeArgs` are now consumed only by `source_test.go` (production runs through `probeArgs`); decide whether to have `wouldStageNothing` call the named builders, or add an assertion that `probeArgs(spec, exclude)` equals the matching named builder, to close the small test-vs-production argv gap.
