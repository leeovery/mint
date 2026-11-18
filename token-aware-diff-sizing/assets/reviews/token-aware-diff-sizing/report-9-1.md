TASK: Give `mint release`'s attended-run and chunk-notes wiring a tested seam (token-aware-diff-sizing-9-1)

ACCEPTANCE CRITERIA:
- A `-y` release through the seam with stdin a terminal, whose exclusion check suggests a glob, presents no exclusion-check pause, writes nothing to `.mint.toml`, and makes no answers commit (§6.7)
- A release without `-y` whose stdin is not a terminal, and whose notes call fails under the default `on_notes_failure`, is offered no floor choice; it stops with the next moves, the `on_notes_failure = "fallback"` move among them (§12.1, §12.4)
- The same failure with stdin a terminal and no `-y` is offered the floor's `c` / `e` / `a` choice instead of the next moves (§12.1, §12.2)
- A release whose chunked notes run fails after at least one part finished saves the finished parts' notes in a file under the chunk-notes store the seam was given, and the notes failure names that file's path (§11.4)
- `mint release` behaves as it does today: `--help` prints the release usage to stdout and exits 0, a flag error exits 2 before the seam runs, and a real run is wired with the same presenter, exec runner, user-cache note cache and user-cache chunk-notes store as now; the existing suite stays green

STATUS: issues_found

SPEC CONTEXT: §6.7 — an unattended run (`-y`, non-TTY) gets no exclusion pause and nothing is written; suggestions print with their `diff_exclude` line instead. §12.1/§12.2 — under `abort`, an attended run gets the one-keypress `c`/`e`/`a` choice (default `a`); an unattended run gets the cause-fitted next moves and never a prompt; the `on_notes_failure = "fallback"` move shows only unattended (§12.4). §11.4 — a failed chunked wave with at least one finished part saves `{base}/{repo path, separators as dashes}/{tag}.md` and the failure names the full path. The engine decides attendedness from `ReleaseDeps.Yes`/`StdinInteractive` (internal/engine/notesfloor.go:18-26) and saves through `ReleaseDeps.ChunkNotes` (internal/engine/chunknotes.go:27-31), so the cmd wiring of those three fields is what this task pins.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/main.go:284-306 — `runRelease` is now the shell: flag parse with unchanged `--help` (stdout, exit 0) and usage-error (stderr, exit 2) branches ahead of the seam, `presenter.DetectStartupSignals`, `presenter.NewForStartup`, then `release(ctx, p, runner.NewExecRunner(), opts, signals.StdinInteractive, newNoteCache(), notescache.NewUserChunkNotes(os.UserCacheDir))`
  - cmd/mint/main.go:310-335 — the seam `release(...)` builds every `ReleaseDeps` field the old literal set: Mutator via `git.NewMutator(r)`, Releaser via `releasing.NewReleaser(mut)`, Editor via `engine.NewEditorLauncher(p, r)`, NoteCache/ChunkNotes from the parameters, `Yes: opts.Yes`, `StdinInteractive: stdinInteractive`; runs `engine.Release(ctx, deps, opts.ReleaseOptions())` and maps through `exitCode`
  - cmd/mint/main.go:30 — `releasing "mint/internal/release"` alias; no other file in cmd/mint imports `mint/internal/release`, so the package-block `release` func does not collide
  - cmd/mint/main.go:193 — `regenerate(...)` keeps its hard-wired `NewUserChunkNotes(os.UserCacheDir)`, as the task required
- Notes: Behaviour-preserving. The seam's parameter types (`engine.NoteCache`, `engine.ChunkNotesSaver`) are the field types, so `newNoteCache()`'s deliberate true-nil return still reaches the engine's `NoteCache == nil` guard unchanged, and the `*notescache.ChunkNotes` → interface conversion happens exactly as it did in the field assignment. The task commit (acf3174) touched only cmd/mint/main.go and cmd/mint/release_seam_test.go; the engine is untouched. Existing tests still drive the shell's pre-seam branches: cmd/mint/usage_test.go:53 (`--help`) and cmd/mint/notesfile_refusal_test.go:30 (flag error → `usageExitCode`).

TESTS:
- Status: Adequate
- Coverage (cmd/mint/release_seam_test.go, `package main`, FakeRunner + RecordingPresenter, repo/note cache/chunk-notes store each in `t.TempDir()`):
  - :165 `TestRelease_YesAtATerminal_RaisesNoExclusionPauseAndRecordsNoAnswers` — `-y`, stdin true, check suggests `docs/design/`; asserts the suggestion printed (proves the check ran, so the rest is non-vacuous), no gate carrying `ChoiceKeep`, `.mint.toml` absent from the root, and no git call naming `.mint.toml`. Criterion 1.
  - :208 `TestRelease_NotesFailureWithStdinNotATerminal_StopsWithTheNextMoves` — no `-y`, stdin false, notes call and retry fail with empty streams (no classify call); asserts exit 1, no floor gate, one NextMoves carrying the exact fallback move string (matches internal/engine/notesfloor.go:50). Criterion 2.
  - :227 `TestRelease_NotesFailureAtATerminalWithoutYes_OffersTheFloorChoice` — same failure, stdin true; asserts exactly one floor gate with keys `c`/`e`/`a` in order and no NextMoves. Criterion 3.
  - :260 `TestRelease_ChunkedRunFails_SavesTheFinishedPartsNotesInTheGivenStore` — `max_prompt_bytes = 20000`, two ~10 KB files (mirrors the engine's seedOverCapRelease fixture), first-arriving part succeeds, the other fails twice; asserts the file at `{base}/{repoScope(root)}/v1.2.4.md` holds the finished part's notes and the one StageFailure's `SavedNotes` is that path. Criterion 4. The scripted sequence is deterministic under the concurrent wave: whichever part call arrives first takes the success, and only the failing call retries.
  - Mutation check of the Outcome: dropping `Yes` fails :165 (pause raised, answers written); dropping `StdinInteractive` or crossing it with `Yes` (either direction) fails :227; forcing `StdinInteractive` true fails :208; dropping `ChunkNotes` fails :260. Every drop/cross named in the task is caught.
- Notes: Not over-tested — one test per criterion, each assertion load-bearing. The read-gate git sequence matches the engine's own pattern (internal/engine/release_answersautostash_test.go:94-106).

CODE QUALITY:
- Project conventions: Followed — every subprocess through `runner.CommandRunner`, mutations through `git.Mutator`, output through the presenter; `NewForStartup` stays the single production construction site; tests use FakeRunner/RecordingPresenter, `t.Parallel()`, `t.TempDir()`.
- SOLID principles: Good — the shell/seam split mirrors `runRegenerate`/`regenerate`, and the stores are injected for the reason the task gives (no test writes the real user cache).
- Complexity: Low
- Modern idioms: Yes (`t.Context()`, `slices.Contains`/`slices.Equal`)
- Readability: Good
- Issues: One inaccurate test comment (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] cmd/mint/release_seam_test.go:154 — The `touchesConfig` doc says the git calls naming `.mint.toml` are "the answers commit's ignore probe, staging or commit". The commit step does not name it: `record.CommitFile` runs `git -C dir commit -m subject` (internal/record/commit.go:111), so only the `check-ignore -q -- .mint.toml` probe (internal/record/commit.go:121) and `add .mint.toml` (internal/record/commit.go:108) match. Change the comment's second clause to "the answers commit's ignore probe or its staging". Remedy is comment text only, so this is non-blocking. — FAILS: the comment says the guard covers more than it does. A maintainer who changes the answers commit to stage some other way (for example `commit -a`, or dropping the probe) would believe the `commit` call still trips `touchesConfig`. It does not, and the -y test's "no answers commit" assertion would then pass unnoticed.

UNSETTLED:
- "the existing suite stays green" — run `go test -race ./...`, together with the other gates in CLAUDE.md. This also confirms that the four new seam tests' scripted git/claude sequences match the engine's real call order. Reading the code shows they match the engine fixtures, but only a run can confirm they pass.
