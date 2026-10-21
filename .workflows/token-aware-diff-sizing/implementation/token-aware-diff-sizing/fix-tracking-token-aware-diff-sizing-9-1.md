## Attempt 1

ISSUES:
- cmd/mint/release_seam_test.go:154-181 (`TestRelease_YesAtATerminal_RaisesNoExclusionPauseAndRecordsNoAnswers`): nothing asserts that the exclusion check actually produced a suggestion. All three guards are absence checks, so each passes whenever no suggestion survives: no `ChoiceKeep` gate at :171, no `.mint.toml`, `touchesConfig` at :178. This test is the only guard on `Yes` in the -y direction, the regression the task's Problem names first. I swapped the check answer at :161 for `gitOut("none")` and forced `Yes: false` at main.go:337, and all four seam tests passed. So if a future change to the check's answer parsing or glob matching makes this scripted answer stop yielding a suggestion, the engine tests get updated, this test keeps passing, and a dropped `Yes` ships again.
  FIX: After the exit-code check, collect the `presentertest.KindExclusionSuggestions` events. Assert there is exactly one, carrying one suggestion with `Glob == "docs/design/"`, and optionally `Line == "diff_exclude = [\"docs/design/\"]"`; that is the payload the run records today. This proves the premise, and it also pins the positive half of §6.7: an unattended run prints the suggestion after the notes instead of pausing. Collect the events with the same loop shape as `nextMoves`/`stageFailures` in the same file.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- cmd/mint/main.go:317-319 — the second sentence is a claim about tests, which the comment rules exclude. Removing the parameter would break the tests' compile anyway, so the sentence guards nothing.
  OLD: // release runs a parsed `mint release` invocation over the given seams, returning
// the process exit code. The note cache and chunk-notes store are parameters so a
// test roots them in a temp dir rather than the user cache dir.
  NEW: // release runs a parsed `mint release` invocation over the given seams, returning
// the process exit code.
- cmd/mint/main.go:282-284 — the doc lists the body's steps and the seams it builds, so it goes stale when the next seam is added. The NEW text keeps only the rule the code can't show: os globals stay in the shell.
  OLD: // runRelease is the thin shell of `mint release`: it parses the flags, detects the
// startup signals, and builds the production presenter, exec runner, note cache and
// chunk-notes store from the os globals, handing them all to release.
  NEW: // runRelease is the thin shell of `mint release`: it builds the production seams
// from the os globals and hands them to release.

NOTES:
- Unscripted transport retry. `seedFailedNotes` (release_seam_test.go:188) and the chunked test (:252) seed one `aiFailed()` per failing call. The ai transport retries a non-zero exit once, and that retry hits FakeRunner's `fakerunner: command "claude" was not seeded` error. I confirmed the counts: 3 claude invocations against 2 seeded in the floor tests, and 4 against 3 in the chunked test. The cause class comes out the same (OTHER, no output), so every assertion still holds and fails loudly if it drifts. Still, the engine's final failure is the fake's missing-setup error, not `aiFailed()`. Seeding `aiFailed()` twice, or a `run.f.Seed("claude", …)` fallback as regenerate_floor_test.go does, would make the script exact. The executor's report ("then one failure") leaves out the retry.
- The chunked test checks the saved file with `strings.Contains`. Its contents depend on which part's call arrives first, so the substring check is a fair choice; the file format is pinned at engine level.
- `regenerate(...)` keeps its hard-wired store at main.go:193, as the task directs. No cmd-level regenerate test reaches `deps.ChunkNotes` on a failed wave today: the `--all` chunked test goes through `newBatchBodyProducer` with empty sinks. So that hard-wired store writes nothing into the developer's cache during the suite.
- `release(...)` takes 7 parameters, including a bool, as the task's Do lays out. Regenerate carries `StdinInteractive` on its request struct instead. Since the task set this shape, no change is asked.
- `gitNonZero` and `aiFailed` (release_seam_test.go:94, :99) build the same ScriptedCall under two names that state intent. No change asked.
- The dispatch carried only the enumerated inputs.
