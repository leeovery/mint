TASK: Refuse a `--notes-file` that cannot be used (token-aware-diff-sizing-3-5, tick-9b80b4)

ACCEPTANCE CRITERIA:
- Given `mint release --notes-file notes.md --no-ai`, mint exits 2 with a usage error (§13.4)
- Given `mint release regenerate --all --notes-file notes.md`, or `mint release regenerate <version> --notes-file notes.md --source <value>` for any value, `fresh` included, mint exits 2 with a usage error (§13.1, §13.4)
- Given a notes file that is empty or holds only whitespace, `mint release --notes-file P` and `mint release regenerate <version> --notes-file P` each fail with `--notes-file P is empty`, P being the path as given, and exit 1 (§13.4)
- Given a path that does not exist, or a file mint cannot read, each verb fails with `reading --notes-file P: <cause>`, `<cause>` being the read error, and exits 1 (§13.4)
- Given `--notes-file ""` on either verb, the run fails as a missing file does, with the `reading --notes-file P: <cause>` refusal and exit 1. It never runs as if the flag were left off, so no AI call is made (§13.4)
- Given any of these refusals, the run stops before any step: no stash, no gate, no AI call and no git mutation. On release not even the version confirmation shows, and on regenerate no source or target question is asked (§13.2, §13.4)

STATUS: complete

SPEC CONTEXT: §13.4 lists four refusals: `--no-ai` with `--notes-file` (and, on regenerate, `--all` or `--source`) is a usage error, exit 2. An empty or whitespace-only file fails with `--notes-file P is empty`, exit 1. A missing or unreadable file fails with `reading --notes-file P: <cause>`, exit 1. The 2026-10-19 corrigendum adds that an empty path names no file, fails as a missing file does, and never counts as the flag left off. §13.1: one file answers one version. §13.2: the file is read once at startup, before any stash, check or gate. §12.2 gives the reason for the blank-file refusal: git accepts an empty tag message.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/flags.go:102-105: `flagGiven` + `can't combine --notes-file with --no-ai` usage error. flags.go:153-163: `flagGiven` uses `fs.Visit`, so `--notes-file ""` / `--notes-file=` count as given.
  - cmd/mint/regenerate_flags.go:131-134 and 151-162: `checkNotesFileScope` rejects `--notes-file` with `--all` (`cannot combine --notes-file with --all`) or with any `--source` (presence-based through `flagGiven`, so `--source fresh` and `--source ""` are both refused).
  - internal/engine/release.go:978-991: `readNotesFile`. Returns nil only when the path is empty AND the flag was not given. A given empty path falls through to `os.ReadFile("")` and fails as missing. Read errors wrap as `reading --notes-file %s: %w`. A `strings.TrimSpace`-blank body is refused as `--notes-file %s is empty`.
  - internal/engine/release.go:964-973: `ReadNotesFile` shows the refusal as a "notes" StageFailed through `surface`, which returns an `*AbortError` with exit 1 (engine.go:37, 68-70).
  - internal/engine/release.go:354-358: the read is the first statement of `Release`, ahead of root resolution, RunStarted, the version confirmation, autostash and preflight. `notesFilePath` (release.go:912-917) leaves an empty path empty, so it never resolves to the WorkDir directory.
  - cmd/mint/main.go:147-153: `regenerate` reads the file first, ahead of root, config, validation, branch, version and publisher resolution. The refusal maps through `exitCode` to 1, so it does not use the usage-code-2 paths the task context warned about.
- Notes: Nothing has drifted from the task. Usage errors print as `mint: <message>` (exit 2). File refusals go through the presenter as the notes stage's ✗ line (exit 1), which matches CLAUDE.md seam 3. In production P is the path as given: `WorkDir` is a test-only seam that nothing outside tests sets.

TESTS:
- Status: Adequate
- Coverage:
  - Usage errors, release: cmd/mint/notesfile_refusal_test.go:8-33 checks the exact message in both flag orders and with an empty path, plus the exit code through `runRelease`.
  - Usage errors, regenerate: :60-102 covers `--all`, `--all` with an empty path, `--source fresh|tag|release|""`, and the exit code through `runRegenerate`.
  - Empty value counts as given: :35-58 and :104-123 cover the separate-token and `=` forms on both verbs, and that `ReleaseOptions()` carries the flag through.
  - Engine refusals: internal/engine/notesfile_refusal_test.go:31-117 covers empty, whitespace-only, missing, unreadable (a directory) and empty path, through both `Release` and `ReadNotesFile`. Each case asserts the exact message, exit 1, exactly one presenter event and zero runner invocations. `AutoStash` is on, so a stash would show up as a command, and a version-confirmation gate would show up as a second event.
  - Regenerate refusals: cmd/mint/regenerate_notesfile_test.go:64-172 drives the cmd-layer `regenerate` with the empty, whitespace, missing and empty-path cases. It asserts exit 1, zero commands and exactly one StageFailed, so no source or target prompt appears. The cases are pinned even with an unresolvable provider and an untagged version, which stops a regression to exit 2 or a stray warning.
  - Negative case: :119-135 pins that an absent flag reads nothing and shows nothing.
- Notes: Each test would fail if its behaviour broke. Swapping `flagGiven` for `notesFile != ""` breaks the parse tests, and dropping the `!set` guard breaks the empty-path engine cases. The unreadable case is not repeated at the regenerate cmd layer, but both verbs share the same `ReadNotesFile`. Some cases overlap between `Release` and `ReadNotesFile`, but each layer guards a different property (ordering inside Release, and the shared function regenerate calls), so I don't count this as over-testing.

CODE QUALITY:
- Project conventions: Followed. Usage errors stay in the flag parsers with the house wording for each verb ("can't combine" on release, "cannot combine" on regenerate). The read logic lives in the engine. User output goes through the presenter. Errors are lowercase and wrapped with `%w`.
- SOLID principles: Good. One shared read/refuse function serves both verbs, and the cmd layer stays thin.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The comments in the changed code are accurate against it (flags.go:53-57, regenerate_flags.go:85-90 and 151-152, main.go:147-148, release.go:272-282 and 964-977).
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
