SECTION: test surface — the change-set's test files, held against everything the specification asks them to guard

SCOPE: one repo ("."), base 6aa345e4f121d09d08cc16527aa594240f7bb737 (from `engine.cjs code log token-aware-diff-sizing token-aware-diff-sizing`), HEAD 84ec34b. The change-set is 88 task commits touching 278 files, 206 of them `_test.go` files across cmd/mint, internal/{ai,aitransport,bytesize,cmdline,commit,config,engine,git,notes,notescache,preflight,presenter,record,release,runner,setupguide}. I read the specification in full, including every corrigendum, plus CLAUDE.md, the finding floor and the unsettled-criteria file. I read the test surface for the three failure modes the floor admits:
- vacuous guards: every negative argv assertion in the changed tests was enumerated with a script and checked against the production argv; the shared negative helpers were read
- isolation holes: every changed test was scanned for env, cwd, stdio, real-exec, user-cache and temp-path reach
- flakes: the wall-clock and grace-window tests were read and stress-run
Measurements:
- the four non-test CLAUDE.md gates over the tree
- a probe test and a corrected-helper run in a byte-copy of the Go sources under the session temp dir
- concurrent runs of race-built `internal/notes` and `internal/engine` test binaries, to confirm suspected flakes
The tree was never written. The temp copy, test binaries and logs were deleted before return.

MEASURED:
- "the existing CauseText tests (internal/notes/causetext_test.go, halvebound_test.go, halveparts_test.go, waveend_test.go, savenotes_test.go) pass unchanged" [5-14] — `go test -c -race -o <tmp>/notes.test ./internal/notes` built from a copy whose internal/notes is byte-identical to the tree (`/usr/bin/diff -rq` → identical). The binary was then run as 6 concurrent processes with `-test.count=2`: all exit 0. The same tests also passed in every stress round below, where the only failure was the deadline test in FINDINGS — holds
- "the existing tests in internal/engine/notes_floor_causemoves_test.go, notes_floor_sizemoves_test.go, notes_floor_nextmoves_test.go and regenerate_floor_nextmoves_test.go pass unchanged" [5-14] — `go test -race ./internal/engine -count=1 -run 'NotesFloor|DryRunNotesFloor|ReviewGate|Unattended_AtTheFloor|Fallback_AttendedAbortAtTheReviewGate'` → `ok`. Every Test function in those four files matches the filter. The copy differs from the tree only in assertNoTag's argv, which makes that check stricter, so the tree's tests pass as well — holds
- "the `NoAIBody` and `ResolveFailure` tests in internal/notes/noai_test.go (:14-193) pass unchanged" / "the `VersionNoAIBody` tests (noai_test.go:242-319) pass unchanged" [5-16] — the same race-built internal/notes binary runs as for the CauseText criterion above: all pass — holds

NOT MEASURED:
- "The existing tests pass with no test file edited … and all five gates pass" [1-6] — I measured four gates clean: `go build ./...`, `gofmt -l .` (empty), `go vet ./...` (exit 0) and `golangci-lint run` (0 issues). The fifth, `go test -race ./...`, is the whole suite, which runs once later over the corrected tree.
- "Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it" [3-2] — needs real git honouring `:(top,literal,exclude)` on status and `add -A`. The project's tests never spawn real git, and its conventions describe no scratch-repo procedure. Subject is §13.2.
- "afterwards the file carries the same edits, staged if they were staged and unstaged if not" [3-2] — needs real `git commit --pathspec-from-file` semantics observed in a repo. Same reason. §13.2.
- "The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs" [3-2] — needs a real `git stash push` and `git stash pop`. Same reason. §13.2.
- "the file that path names is the one left out of the clean-tree check, the stash and the commits" [3-2] — needs `mint release --notes-file <rel>` run from a subdirectory of a real repo. Same reason. §13.1/§13.2.
- "afterwards the file is as it was: still untracked, or carrying the same edits, staged if they were staged and unstaged if not" [3-3] — needs real regenerate runs against a scratch repo with a bare origin. Same reason. §13.2.
- "In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it …" [3-4] — needs the two-step reset run in a real repo, from the root and from a subdirectory. Same reason. §13.2.
- "the file carries the same edits it had, staged if they were staged and unstaged if not (§13.2)" [3-4] — needs index blobs observed across a real reset. Same reason. §13.2.
- "Given that file, `--autostash` with other uncommitted changes, and a failure before the push, the other changes are restored after the unwind as built …" [3-4] — needs a real stash, reset and pop sequence. Same reason. §13.2.
- "the tag body, the changelog section and the provider release body are the same bytes (§13.3)" [3-8] — needs a real `git tag -a --cleanup=verbatim` and `git cat-file` / `for-each-ref` read-back. Same reason. §13.3.
- "`diff.noprefix = true`, custom `diff.srcPrefix`/`diff.dstPrefix` and `diff.mnemonicPrefix` all yield `a/P b/P` headers once `--src-prefix=a/ --dst-prefix=b/` sit ahead of the range" [4-15] — needs real `git diff` under each config. Same reason. Subject is §11.4 (the saved notes naming real paths), via §8.
- "Rule: the existing tests pass with their semantics untouched. … All five CLAUDE.md gates pass" [6-11] — four gates measured clean (see [1-6]). The suite run is reserved for later.
- "Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass" [6-12] — as [6-11].
- "Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)" [7-7] — needs a real keypress on a real TTY, in pretty and `--plain` modes, which no disposable instance gives. §6.6.
- "The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)" [7-14] — four measured clean. `go test -race ./...` is reserved for the later suite run.
- "Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass" [7-15] — the Go-file half was settled by the task verifier's reading. For the gates, four measured clean and the suite run is reserved.
- "Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)" [8-6] — needs forced failures in a real git repo, then the manual steps run. Same no-real-git reason. §6.6.
- "A tracked `.mint.toml`, one a `.gitignore` rule matches included, and an untracked or absent `.mint.toml` that no rule matches, take today's route" [8-9] — needs real `git check-ignore` exit codes observed. Same reason. §6.6.
- "the existing suite stays green" [9-1] — this is `go test -race ./...`, reserved for the later suite run. The other four gates measured clean.
- "The five gates in CLAUDE.md pass with no test added, removed or changed." [9-4] — four measured clean. The suite run is reserved.
- "every gate passes" [10-2] — four measured clean. The suite run is reserved.
- "Behaviour is unchanged. Every existing test stays green with its assertions as they stand … All five gates in CLAUDE.md pass" [11-1] — four measured clean. The suite run is reserved.
- "all five gates in CLAUDE.md pass" [11-2] — four measured clean. The suite run is reserved.
- "Run against the current engine, TestRegenerateRun_Yes_AlwaysCallsPrompt … and the test passes" [11-3] — I read the guard at internal/engine/regenerate_interactive_test.go:223-256 and it is sound: the subjects must equal source, target, notes, and the AcceptEcho is checked. No defect is suspected, and a test run is reserved for confirming one.
- "With a trial engine-side skip keyed on deps.Yes … TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes" [11-3] — this is a production-code mutation trial, not a project-described command. Reading shows a skip at any of the three gates removes a subject from the `[]string{"source", "target", "notes"}` equality at :246.
- "The existing suite stays green … and the five CLAUDE.md gates pass" [11-3] — four measured clean. The suite run is reserved.
- "All five gates in CLAUDE.md pass" [12-1] — four measured clean. The suite run is reserved.
- "Every other existing test stays green, and all five gates in CLAUDE.md pass" [12-2] — four measured clean. The suite run is reserved.

FINDINGS:
- [in-scope] [contained] internal/engine/notes_floor_test.go:77 — `assertNoTag` counts `git tag -a v1.2.4 -F -`. Since task 3-8, the release creates its tag as `git tag -a --cleanup=verbatim <tag> -F -` (internal/release/release.go:108), and `countCmd` compares the whole command line exactly (internal/engine/release_surgicalunwind_test.go:341-349). The count is therefore always 0 and the helper cannot fail. It was added in 5-1, after 3-8, so it has never checked anything. It has 12 call sites across 7 files:
  - notes_floor_test.go: 5 (:120, :291, :324, :348, :365)
  - notes_floor_edit_test.go: 2 (:292, :326)
  - notes_floor_dryrun_test.go:32
  - notes_floor_nextmoves_test.go:92
  - notes_fallback_warning_test.go:282
  - release_exclusionsuggestions_test.go:347
  - release_reviewgate_unwind_test.go:109, inside assertUnwoundToStart, which has 15 callers
  These are the tests' guards for the specification's "before any tag" promises: every abort at the floor (§12.2), stdin closing at a pause (§6.6), and a failed `r` whose abort "lands before any tag" (§15.1). TestRelease_NotesFloor_AbortEndsBeforeAnyTagAndUnwindsThePreTagCommit is named for exactly that property. Measured in a temp byte-copy: a probe ran a floor run that does create the tag (the production-argv count was 1), then called `assertNoTag(t, f)`, and the probe passed (`go test -race ./internal/engine -run TestProbe_AssertNoTagPassesWhenATagIsCreated` → PASS). Fix: count the production argv, `countCmd(f, "git", "tag", "-a", "--cleanup=verbatim", nextTag, "-F", "-")`. Alternatively, match the `git tag -a` prefix as assertNoMutation does at internal/engine/release_test.go:1311, so a future argv change cannot empty it again. With the corrected argv in the copy, `go test -race ./internal/engine -count=1 -run 'NotesFloor|DryRunNotesFloor|ReviewGate|Unattended_AtTheFloor|Fallback_AttendedAbortAtTheReviewGate'` → `ok`, so the fix lands green and exposes no hidden tag — FAILS: a regression that creates the tag on a floor abort, an unattended stop at the floor, a dry run at the floor or a failed `r` passes all 12 "no tag" assertions. The property survives only where another check happens to pin it: assertNoMutation's `git tag -a` prefix inside assertUnwoundToStart, or an Unwound summary that a tag delete would change. Everywhere else, the guard named for the property is inert, and nobody would notice until a tag appeared on a release that was supposed to stop before one.
- [in-scope] [spreading] internal/notes/chunkwave_test.go:379 — TestGenerator_OverCap_WaveLongerThanTimeout_EachCallOwnDeadline_NotesWritten (:354, added in 2-6) relies on real wall-clock margins:
  - each 200ms call (:358) must finish inside a 500ms per-attempt deadline (:357)
  - each call's deadline must be at least 400ms away when the runner starts (`allotted < timeout-100*time.Millisecond`, a 100ms tolerance)
  Reproduced under CPU contention: `go test -c -race ./internal/notes`, then 20 concurrent processes on 10 cores running the wave subset with `-test.count=15`. That gave 2 failures in 300 runs of this test, with two different symptoms:
  - `chunkwave_test.go:364: Generate returned unexpected error: generating notes for part 1 of 6: ai command timed out: context deadline exceeded`
  - `chunkwave_test.go:380: call 4 was given 353.810875ms by its deadline, want a fresh per-attempt 500ms`
  A repeat at the same load gave 0 failures in 300 runs, and 450 runs at one process per core gave none. Fix: widen the margins so scheduler jitter cannot cross them, while the scenario still tells a per-call deadline from a wave-wide one. One way is to scale callTime and timeout up while keeping three serial steps longer than the timeout, and relax the lower bound to what a shared deadline could never give: a wave-wide deadline leaves the merge at most timeout − 2×callTime. There is more than one defensible shape, and no check can show that a re-tuned timing test is no longer flaky — FAILS: the `go test -race ./...` gate, which runs up to one race-built package binary per core, each with GOMAXPROCS threads, intermittently goes red on a loaded developer machine or CI runner. It fails with a misleading "ai command timed out" from a test and blocks the gate on no defect.

COVERAGE:
- The four non-test gates are clean — repo root — measured: `go build ./...` → exit 0; `gofmt -l .` → no output; `go vet ./...` → exit 0; `golangci-lint run` → `0 issues.`
- `assertNoTag` is vacuous, and the corrected helper passes — internal/engine/notes_floor_test.go:75-79 — measured: the probe and the corrected-argv engine run above (temp copy; `/usr/bin/diff -r` showed it differing from the tree only at :77)
- `assertUnwoundToStart` still guards "no tag" through assertNoMutation's `git tag -a` prefix — internal/engine/release_reviewgate_unwind_test.go:100-112, internal/engine/release_test.go:1306-1323 — read
- Every other negative argv assertion in the changed tests matches a production argv — read, enumerated by a script over the 206 changed test files:
  - `reset --hard` (internal/engine/unwind.go:294)
  - `push origin HEAD` (internal/engine/regenerate_write.go:375)
  - `log --format=%s` (internal/notes/resolve.go:22)
  - `-C root add|commit -m` (internal/record/commit.go:88,108,111,161)
  - unstage `-C root reset -q <sha> -- .mint.toml` (internal/engine/unwind.go:252 vs internal/engine/release_answersfailure_test.go:121-123)
  - the prefixed notes diff (internal/notes/assemble.go:184)
  - `gh release create … --verify-tag` (internal/publish/publish.go:112-115)
- The exclusion-check call detector matches the production prompt — internal/engine/exclusioncheck_helpers_test.go:12-16 vs internal/notes/exclusioncheck.go:347 — read
- The "no saved chunk notes" negatives walk the directory their store actually writes to, with a store wired — internal/engine/notes_savedchunknotes_test.go:38-44,67-79; internal/engine/release_remergefail_test.go:84-86; internal/engine/regenerate_reviewr_test.go:29,44; internal/engine/release_regenchunkfail_test.go:151-153 — read
- No test writes the developer's real user cache dir:
  - the release seam injects stores rooted in `t.TempDir()` (cmd/mint/release_seam_test.go:36-37,49-50)
  - a nil store saves nothing (internal/engine/chunknotes.go:29-33)
  - cmd `regenerate()` keeps its hard-wired `os.UserCacheDir` store (cmd/mint/main.go:193), as plan task 9-1 says, but its cmd tests drive only successes and single-pass failures (cmd/mint/regenerate_floor_test.go:17-114, cmd/mint/regenerate_exclusioncheck_test.go:58-129), so no chunk notes are ever saved there
  - read
- Environment and process-global reach is confined:
  - editor tests resolve the binary from the process env (internal/engine/notes_floor_edit_test.go:56-59)
  - the one `t.Setenv` test is non-parallel (internal/engine/release_reviewgate_unwind_test.go:252-253)
  - `t.Chdir`, the `os.Stdin` swap and the `os.Stdout`/`os.Stderr` swap run only in non-parallel tests (internal/engine/release_notesfile_test.go:327-379; internal/engine/regenerate_notesfile_test.go:455-461; cmd/mint/regenerate_notesfile_test.go:191-195; cmd/mint/usage_test.go:90-150)
  - no changed test adds a real-exec spawn (`git diff` of internal/ai/transport_test.go adds no NewExecRunner or exec)
  - read
- The `runRelease` usage-error test cannot reach real git if the flag check regressed: the notes file is read before any step — cmd/mint/notesfile_refusal_test.go:27-33, internal/engine/release.go:354-355 — read
- FakeRunner is safe for the parallel wave and fails on unseeded commands — internal/runner/fake_runner.go (`record` holds the mutex across append and outcome; `Invocations` returns a clone) — read
- The RecordingPresenter is not mutex-guarded, but every progress event reaches it under the wave's lock — internal/notes/chunk.go:222-232,259 (send takes the lock at :222 and reports at :230-231; halve runs under that lock and reports at :259) — read
- The four-in-flight bound is a real guard: an unbounded wave never sits at exactly 4 in flight, so `waitInFlight(4)` times out, and maxInFlight is checked — internal/notes/chunkwave_test.go:88-101,174-196 — read; measured under load: 10 and 20 concurrent `notes.test -test.run 'Wave|…|InFlight|Deadline'` processes ×15 passed it every run
- The wave-end guards assert both the starts that must not happen and the in-flight ends that must — internal/notes/waveend_test.go:155-198 — read; measured: the same stress rounds, no failures
- The 50ms-window batch skip-cause test is stable under load — internal/engine/regenerate_batch_skipcause_test.go:189-246 — measured: 24 concurrent race-built `engine.test -test.run '…WaveFailsForGoodAtPartThree…|TestGenerator' -test.count=40` → all exit 0
- The piece-cut guards exercise multi-byte cuts at 8 offsets and check byte-for-byte reassembly and the cap per piece — internal/notes/chunkpieces_test.go:42-45,130-192,239-275 — read
- Classify-answer normalisation and the OTHER fall-throughs (sentence, both codes, unknown word, failed classify) are pinned — internal/notes/halve_test.go:80,205-210 — read
- The lower-cap hint's exclusion of the merge self-checks its fixture (the merge is bigger than every part) — internal/engine/notes_lowercap_test.go:153-176 — read
- The lean default is pinned as a literal, not the production constant — internal/engine/lean_command_test.go:17-22; internal/commit/lean_command_test.go:16-21; internal/config/config_test.go:774 — read
- `.mint.toml` still reaches the commit AI diff — internal/commit/generate_test.go:1057-1073 (exact argv plus prompt content) — read
- The unattended suggestion lines append to existing entries per suggestion — internal/engine/release_exclusionsuggestions_test.go:197-235 — read
- Answers written into `.mint.toml` are reloaded through strict `config.Load` — internal/config/appendrelease_test.go:37,316 — read
- The README and setup-guide pins fail on a missing heading or section rather than passing empty — internal/config/readme_ai_command_test.go:13-30; internal/setupguide/setupguide_test.go:359-373 — read
- The regenerate `-y` prompt guard is not vacuous — internal/engine/regenerate_interactive_test.go:223-256 — read
