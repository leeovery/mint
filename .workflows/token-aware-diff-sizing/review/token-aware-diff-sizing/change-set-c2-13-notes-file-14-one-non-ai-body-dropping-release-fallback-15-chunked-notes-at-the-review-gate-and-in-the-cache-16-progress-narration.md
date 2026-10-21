SECTION: 13. `--notes-file` + 14. One Non-AI Body: Dropping `[release].fallback` + 15. Chunked Notes at the Review Gate and in the Cache + 16. Progress Narration

SCOPE: One repo (`.`), base 6aa345e4f121d09d08cc16527aa594240f7bb737 (from `engine.cjs code log`). There are 105 task commits, b0877e56 through 608ccc0c, and the change-set is confined to the 277 files `code log` lists. `engine.cjs code diff` was captured in full to the scratchpad. Per-file `git diff 6aa345e4 HEAD -- <file>` was read for internal/preflight/preflight.go, internal/notescache/cache.go, internal/presenter/spinner.go, internal/engine/editor.go and internal/engine/regenerate_tagsource.go.

These files were read in their final state:
- cmd/mint: flags.go, regenerate_flags.go, regenerate_run.go, main.go (regenerate/runRegenerateSingle), usage.go, regenerate_all.go (newBatchRegeneratorProducer)
- internal/engine: release.go (Release spine, notes-file read and exclusion, resolveBody/cache reuse, reviewGate, regenerateBody, gateForKind, perRunRegenerator), chunknotes.go, notesstage.go, unwind.go, autostash.go, regenerate_interactive.go, regenerate_write.go, regenerate_changelog.go, regenerate_batch.go, regenerate_batch_changelog.go, regenerate_fresh.go, regenerate.go:94
- internal/notes: select.go, generate.go, chunk.go, remerge.go, chunknotes.go, noai.go, resolve.go
- internal/record/commit.go, internal/release/release.go, internal/git/exclusion.go
- internal/config: config.go:540-560 and metadata.go:134
- internal/presenter: presenter.go, pretty.go:376-546, plain.go:235-265, gate.go:157-202
- internal/bytesize/bytesize.go
- README.md:95-175
- internal/config/fallback_removed_test.go, and the notes-file test names across cmd/mint and internal/engine

Run (per CLAUDE.md gates; no whole-suite run):
- `go build ./...`
- `gofmt -l .`
- `go vet ./...`
- `golangci-lint run`
- `go test -race -count=1 ./internal/notes/...` (single package, for criterion 5-16)

The tree was left as found. No artefacts were created: `go build ./...` over multiple packages writes no binary.

MEASURED:
- "the `NoAIBody` and `ResolveFailure` tests in internal/notes/noai_test.go (:14-193) pass unchanged" / "the `VersionNoAIBody` tests (noai_test.go:242-319) pass unchanged" [5-16] — measured: `go test -race -count=1 ./internal/notes/...` → `ok mint/internal/notes 2.100s`. Also `go test -race -count=1 -run 'NoAIBody|ResolveFailure|VersionNoAIBody' -v ./internal/notes/` → every NoAIBody, ResolveFailure and VersionNoAIBody test PASS, including the four TestVersionNoAIBody_ListsTheVersionsSubjectsAsItsForwardReleaseDid subtests — holds
- "Rule: the existing tests pass with their semantics untouched. ... All five CLAUDE.md gates pass" [6-11] — the four non-test gates only: `go build ./...` → ok; `gofmt -l .` → no output; `go vet ./...` → ok; `golangci-lint run` → `0 issues.` — holds (the test half is under NOT MEASURED)
- "Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass" [6-12] — the four non-test gates only, the same four commands with the same clean results — holds (the test half is under NOT MEASURED)

NOT MEASURED:
- "Afterwards `notes.md` is still untracked with its contents unchanged, and no commit the release made contains it" [3-2] — Settling this means running real git: `git status --porcelain`, `git add -A` and `git commit` under `:(top,literal,exclude)` against a scratch repo. CLAUDE.md's test idioms forbid spawning real git, and the project documents no way to run mint or git against a disposable repo. Read: internal/git/exclusion.go:14-19, internal/record/commit.go:33-52 and :155-178, internal/engine/release.go:473, :524, :657.
- "afterwards the file carries the same edits, staged if they were staged and unstaged if not" [3-2] — Settling this means observing real git's `commit --pathspec-from-file=- --pathspec-file-nul` (`--only`) semantics on a partly staged file, which the project gives no way to run. Read: internal/record/commit.go:155-178 takes the pathspec route only when the kept path has staged edits.
- "The notes file stays in the work tree through the run with its contents and staged state unchanged. Given the notes file is the only uncommitted change, nothing is stashed and no `git stash pop` runs" [3-2] — Settling this means real `git stash push --include-untracked -- :(top,literal,exclude)<path>` and `git stash pop`, which the project gives no way to run. Read: internal/engine/autostash.go:53-60 owes no pop on "No local changes to save".
- "the file that path names is the one left out of the clean-tree check, the stash and the commits" [3-2] — Settling this means running `mint release --notes-file <rel>` from a subdirectory of a real repo, which the project gives no way to do. One case is unverified by reading alone: a notes file inside an otherwise untracked directory, where it is unknown whether `git status --porcelain` collapses the directory despite the exclude.
- "afterwards the file is as it was: still untracked, or carrying the same edits, staged if they were staged and unstaged if not" [3-3] — Settling this means three real `mint release regenerate <v> --notes-file notes.md --target changelog -y` runs against a scratch repo with a bare origin. The project describes no such procedure.
- "In each case HEAD is back at the starting commit, and the rest of the work tree is as the as-built reset leaves it: the changelog write, any version-file projection and the hook's artifacts are gone (§13.2)" [3-4] — Settling this means running the two-step `git reset --soft S` + `git restore --source=S --staged --worktree -- ':(top,literal,exclude)…'` in a real repo, from the root and from a subdirectory. The project gives no way to do this. Read: internal/engine/unwind.go:291-311.
- "the file carries the same edits it had, staged if they were staged and unstaged if not (§13.2)" [3-4] — This needs `git ls-files -s` and work-tree content observed before and after a real two-step reset. Same reason.
- "Given that file, `--autostash` with other uncommitted changes, and a failure before the push, the other changes are restored after the unwind as built. The notes file carries the same edits, with the same staged state (§13.2)" [3-4] — This needs a real stash push with the exclude pathspec, mint's commits, the two-step reset and `git stash pop`. Same reason.
- "the tag body, the changelog section and the provider release body are the same bytes (§13.3)" [3-8] — This needs real `git tag -a --cleanup=verbatim vX -F -` (with and without `tag.gpgSign`) plus `git cat-file tag` and `for-each-ref %(contents:body)` in a scratch repo. Same reason. Read: internal/release/release.go:108 and :138-145 pipe subject, blank line and verbatim body under `--cleanup=verbatim`, adding one "\n" only when the body lacks one.
- "the existing tests pass with their semantics untouched" (the `go test -race ./...` half of "All five CLAUDE.md gates pass") [6-11] — The whole suite runs once, later, over the corrected tree. A verifier runs a single package only to confirm a suspected defect, and none was suspected here.
- "the existing tests pass with their semantics untouched" (the `go test -race ./...` half) [6-12] — Same reason.
- "The existing tests pass with no test file edited — … — and all five gates pass" [1-6] — §17 / §9 (section 17-18 and section 9-12). The four non-test gates are clean tree-wide, as above.
- "`diff.noprefix = true`, custom `diff.srcPrefix`/`diff.dstPrefix` and `diff.mnemonicPrefix` all yield `a/P b/P` headers once `--src-prefix=a/ --dst-prefix=b/` sit ahead of the range" [4-15] — §11.4 (section 9-12).
- "the existing CauseText tests (internal/notes/causetext_test.go, halvebound_test.go, halveparts_test.go, waveend_test.go, savenotes_test.go) pass unchanged" [5-14] — §9/§12 (section 9-12). Incidentally, `go test -race -count=1 ./internal/notes/...` → ok covers the internal/notes files.
- "the existing tests in internal/engine/notes_floor_causemoves_test.go, notes_floor_sizemoves_test.go, notes_floor_nextmoves_test.go and regenerate_floor_nextmoves_test.go pass unchanged" [5-14] — §12 (section 9-12).
- "Given a Ctrl-C at a pause, the run is cancelled before any notes call, as a Ctrl-C during the notes stage is (§6.6)" [7-7] — §6 (section 1-8).
- "The five gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues)" [7-14] — §5/§6 (section 1-8). The four non-test gates are clean tree-wide.
- "Rule: no Go file changes except `internal/config/readme_exclusion_check_test.go`, and the five gates pass" [7-15] — §6/§18 (section 1-8 and section 17-18).
- "Given either warning, carrying out the steps it names, in the order it names them, leaves the branch at the starting commit with the answers committed on top (§6.6)" [8-6] — §6.6 (section 1-8).
- "A tracked `.mint.toml`, one a `.gitignore` rule matches included, and an untracked or absent `.mint.toml` that no rule matches, take today's route" [8-9] — §6.6 (section 1-8).
- "the existing suite stays green" [9-1] — §6.7/§11.4/§12 (section 1-8 and section 9-12).
- "The five gates in CLAUDE.md pass with no test added, removed or changed." [9-4] — §18 (section 17-18).
- "every gate passes" [10-2] — §18 (section 17-18).
- "Behaviour is unchanged. Every existing test stays green with its assertions as they stand, apart from the edits the deletion forces, which are listed under Do. All five gates in CLAUDE.md pass" [11-1] — §12.1/§12.3 (section 9-12). The four non-test gates are clean tree-wide.
- "all five gates in CLAUDE.md pass" [11-2] — §18 (section 17-18).
- "With a trial engine-side skip keyed on deps.Yes … TestRegenerateRun_Yes_AlwaysCallsPrompt fails; with the trial reverted it passes" [11-3] — §12.3 (section 9-12).
- "The existing suite stays green … and the five CLAUDE.md gates pass" [11-3] — §12.3/§18 (section 9-12 and section 17-18).
- "Run against the current engine, TestRegenerateRun_Yes_AlwaysCallsPrompt … and the test passes" [11-3] — §12.3 (section 9-12).
- "All five gates in CLAUDE.md pass" [12-1] — §4/§8, notes diff at the repo root (section 1-8).
- "Every other existing test stays green, and all five gates in CLAUDE.md pass" [12-2] — §6.6, unwind warnings (section 1-8).
- "Given the as-built transport, which takes a fresh per-attempt deadline for every call, the test passes" [13-1] — §8/§4.4 (section 1-8).
- "With a trial change that gives the calls one deadline spanning the wave and the merge, in place of the per-attempt one, the test fails on a deadline ordering. With the trial reverted it passes" [13-1] — §8/§4.4 (section 1-8).
- "With a trial change that gives the six chunk calls one shared deadline while the merge keeps its own per-attempt deadline, the test fails on the second-round ordering. With the trial reverted it passes" [13-1] — §8/§4.4 (section 1-8).
- "Run under the contention that reproduced the flake (a race-built internal/notes test binary run as twice as many concurrent processes as cores, -test.count=15), the test fails in none of the runs" [13-1] — §8/§4.4 (section 1-8).
- "Rule: no production file changes, and the five CLAUDE.md gates pass" [13-1] — §8/§4.4 (section 1-8). The four non-test gates are clean tree-wide.

FINDINGS:
- None

COVERAGE:
- The notes file is read once, before any stash, check or gate, on release — internal/engine/release.go:356-360 runs ahead of ResolveRoot at :363, the autostash at :452-455 and preflight at :473 — read
- The notes file is read first on regenerate — cmd/mint/main.go:150-153 runs ahead of ResolveRoot, config.Load and validateRegenerateRequest — read
- A relative path resolves against the run directory (WorkDir is a test seam only; production leaves it empty) — internal/engine/release.go:914-919 and :943-957 (filepath.Abs, symlink-resolved directory and root) — read; `grep -rn WorkDir` → only release.go and release_notesfile_git_test.go
- An empty or whitespace-only file is refused with `--notes-file P is empty`; a missing or unreadable file with `reading --notes-file P: <cause>`; an empty path, given, fails as a missing file and is never read as the flag left off; all exit 1 through surface() — internal/engine/release.go:969-993 — read
- `--notes-file` with `--no-ai` is a usage error, exit 2 — cmd/mint/flags.go:102-105 — read
- `--notes-file` with `--all` or `--source` is a usage error — cmd/mint/regenerate_flags.go:131-134 and :153-160 — read
- `--notes-file=` (empty) counts as given — cmd/mint/flags.go:102 and regenerate_flags.go:131 (flagGiven) — read
- The usage lines match §13.1 — cmd/mint/usage.go:41 and :58 — read
- A supplied body wins over first-release, degenerate, --no-ai and the AI path. No exclusion check, cache, AI call or on_notes_failure runs — internal/notes/select.go:184-186 returns before FirstRelease, AssembleDiff and checkExclusions — read
- The supplied body's gate is y/n/e with no r on release — internal/engine/release.go:1844-1849 (gateForKind: only KindNormalAI gets ReleaseReviewGate) — read
- The supplied body's gate is regenerate's own y/n/e review gate, not the reuse confirm — internal/engine/regenerate_write.go:239-240 and regenerate_interactive.go:166-168 — read
- The supplied body skips ProduceBody, the oldest version included — internal/engine/regenerate_interactive.go:272-277 — read
- The tag annotation is subject, blank line and body verbatim under `--cleanup=verbatim`, with one "\n" added only when absent — internal/release/release.go:108 and :138-145 — read
- The notes file is left out of the clean-tree check — internal/engine/release.go:473 (forward) and internal/engine/regenerate.go:94 (regenerate, changelog targets only, KeepOut from regenerate_interactive.go:199-207) — read
- The notes file is left out of the autostash; when it is the only change no pop is owed — internal/engine/autostash.go:53-60 — read
- The notes file is left out of the pre-tag `add -A` and commit, the bookkeeping commit and regenerate's changelog commit, with staged edits kept staged via the `--pathspec-from-file` route — internal/record/commit.go:33-52 and :155-178; internal/engine/release.go:1582 and :657; internal/engine/regenerate_write.go:347-358 — read
- An ignored notes file gets no exclusion; a tracked one always does (ls-files --others lists only untracked paths) — internal/engine/release.go:924-937 — read
- The answers commit (a whole-index `git commit -m`, internal/record/commit.go:112-119) never runs beside a notes file, because the supplied body returns before the exclusion check — internal/notes/select.go:184 — read
- The unwind and regenerate's reset leave the notes file's content and staged state: soft reset, then restore excluding the path — internal/engine/unwind.go:291-311; StartState.KeepOut at release.go:505; resetTarget at regenerate_write.go:304 — read
- Every review-gate abort on a release unwinds: decline, failed r, launched editor that fails, and closed stdin at the keypress. A missing editor returns to the gate — internal/engine/release.go:611-616 (any reviewGate error → Unwind) and :1802-1815 (ErrEditorReturnToGate → continue) — read
- `[release].fallback` fails at load with the spec's removed-key message — internal/config/config.go:553-555; message pinned in internal/config/fallback_removed_test.go — read
- The Release.Fallback field is gone — `grep -rn '\.Fallback\b'` outside tests → no output — measured
- The on_notes_failure metadata row and README row say `fallback` ships the commit-subject list — internal/config/metadata.go:134; README.md:256 — read
- --no-ai, on_notes_failure=fallback and the release floor's c share one builder over `{lastTag}..HEAD`, which drops answers commits, with an empty list giving StubBody — internal/notes/noai.go, internal/notes/resolve.go:65-71 and :99-105, internal/engine/notesfloor.go:189 — read
- Regenerate's floor c drops the bookkeeping commit the tag sits on by its ` Release {tag}` suffix, under any prefix — internal/notes/resolve.go:78-95; internal/engine/notesfloor.go:198 — read; measured via `go test -race -count=1 -run 'VersionNoAIBody' ./internal/notes/` → PASS
- r on chunked notes makes one Remerge call over the held part notes plus the nudge. Merge inputs keep the nudge-free instructions — internal/engine/chunknotes.go:115-125; internal/notes/remerge.go:51-57; internal/notes/generate.go:224-238 — read
- r on single-pass notes re-runs from the size check with the nudge in every chunk, piece, halving and merge call, and never re-runs the exclusion check. Answered globs persist on the shared Assembler — internal/notes/generate.go:213-240; internal/notes/select.go:249-258 — read
- A failed r saves part notes: a merge-only r through keepRegen, and a ladder-made chunked r through the generator's own keepChunkNotes. The saved file is named with the verb's command, and closed input at the nudge prompt also saves — internal/engine/chunknotes.go:66-92; internal/engine/release.go:1870-1888; internal/engine/notesfloor.go:65-76 — read
- A Ctrl-C during r saves nothing — internal/notes/chunknotes.go:69 (errors.Is(failure, context.Canceled)); internal/notes/chunk.go run() returns no parts on ctx.Err() — read
- On regenerate, single-version and --all, r uses the same rule. Under --all a failed r ends the batch rather than skipping — internal/engine/regenerate_write.go:250-257; internal/engine/regenerate_batch.go:337-340; cmd/mint/regenerate_all.go:131-137 — read
- A chunked body is cached with its merge inputs on dry and real runs. A reused chunked entry restores them so r re-merges. Prune runs after write — internal/engine/release.go:1035-1057; internal/notescache/cache.go:53 and :110 — read
- The split, halving, single-pass halving and merging lines in pretty and plain mode match §16.1 wording — internal/presenter/pretty.go:520-546, plain.go:243-265, presenter.go:535-540 — read
- The split line's size is the composed single-pass prompt. Parts done count against a total that grows with halving. A halved single-pass run moves to the split line as parts finish — internal/notes/chunk.go:33, :46, :231, :259 — read
- Sizes are decimal: one place below 10, whole from 10, bytes under 1 KB, rounding up into the next unit — internal/bytesize/bytesize.go:10-26 — read
- The ✓ line reads `Notes written (n parts, merged)` with elapsed appended for the blocking stage — internal/engine/notesstage.go:283-295; internal/engine/engine.go:161-171 — read
- An r that becomes chunked opens its own timed notes line and closes it with the merged completion. A one-call r (including merge-only Remerge, which emits no progress) narrates as built — internal/engine/notesstage.go:185-215; internal/notes/remerge.go:51-57 — read
- Follow-ups print after the notes: the lower-cap hint and then the -y suggestions; on an unattended floor stop, the suggestions print with the next moves — internal/engine/release.go:592-595 and :571-577 — read
- The answers line shows inside a spinner suspend before the notes line resumes — internal/engine/release.go:1123-1127 — read
- The failed-check warning is not a follow-up: it prints inside a suspend when the check fails — internal/engine/notesstage.go:51-59 — read
- Spinner text swaps safely from wave goroutines, under the library lock — internal/presenter/spinner.go (briandownsSpinner.SetText) — read
- Tree-wide gates: `go build ./...` → ok; `gofmt -l .` → no output; `go vet ./...` → ok; `golangci-lint run` → `0 issues.` — measured
