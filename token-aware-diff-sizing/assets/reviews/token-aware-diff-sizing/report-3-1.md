TASK: Release with a supplied notes body (token-aware-diff-sizing-3-1, tick-3cf081)

ACCEPTANCE CRITERIA:
- With a prior tag, a non-empty diff and a notes file outside the repo, `mint release -y --notes-file <path>` makes no AI call of any kind and ships the file's contents byte-identical, as an AI body ships, in the tag message, the changelog entry and the provider release (§13.3)
- With `--notes-file`, the supplied body ships with no AI call in each of these cases: a first release (instead of `Initial release.`), an empty post-exclusion diff (instead of the maintenance-release body), `on_notes_failure = "fallback"`, and a live dry-run cache entry, where no `Cached notes found for this diff — use them?` question is asked (§7.1, §7.2, §13.3)
- On an attended run the body shows at the review gate `Release with these notes?` with accept, abort and edit and no `r`. Edit opens the editor on the supplied body, the saved text ships, and the notes file on disk is unchanged. Abort ends the run before any tag (§13.2, §13.3)
- Under `-y` the review gate auto-accepts inside the presenter with its as-built `notes: accepted (-y)` echo, and the supplied body ships (§13.3)
- Run from `<repo>/sub` with `--notes-file notes.md`, where committed `<repo>/sub/notes.md` and `<repo>/notes.md` both exist, the body is `<repo>/sub/notes.md` (§13.1)
- `--notes-file -` with a file named `-` in the run directory reads that file, and stdin is never read for the body (§13.1)
- With a notes file outside the repo that a pre_tag hook rewrites mid-run, the body shipped is what mint read at startup (§13.2)
- `mint release --help` lists `--notes-file P` with `release with the finished notes in file P (no AI)` (§13.1, §18.3)
- The README's `release` section documents `--notes-file`: a flag-table row, that it releases a finished body with no AI, and that an attended run shows the body at the review gate first (§18.2)

STATUS: complete

SPEC CONTEXT: §7.1 puts `--notes-file` first in the release's body routing, ahead of first-release, empty-diff, `--no-ai` and the AI path. §7.2's exclusion check and cache reuse belong to the AI path, so a supplied body never reaches them. §13.1: file path only, no stdin form, and the path resolves against the run directory. §13.2: the file is read once at startup, before any stash, check or gate. §13.3: the supplied body wins over everything and skips the exclusion check, the cap, chunking, every AI call and `on_notes_failure`. The review gate offers y/n/e with no `r`, `-y` auto-accepts inside the presenter, and the body reaches the tag (`--cleanup=verbatim`), the changelog and the provider byte-identical. §18.2 and §18.3 cover the README and the usage line.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - cmd/mint/flags.go:53-57 (fields), :63 (threaded into engine.ReleaseOptions), :88 (flag registration), :102 (`flagGiven`), :155-163 (`flagGiven` helper)
  - cmd/mint/usage.go:41 (curated usage line, exact spec wording)
  - internal/engine/release.go:272-285 (ReleaseOptions.NotesFile / NotesFileSet / WorkDir), :354-358 (file read as Release's first step, before root resolution, config, the version gate, autostash and preflight), :551 (`selectState` threads `supplied`), :903-910 (`selectState`), :912-917 (`notesFilePath`), :964-991 (`ReadNotesFile` / `readNotesFile`: `os.ReadFile(path)` treats `-` as a filename and never reads stdin), :1842-1847 (`gateForKind`: every Kind except KindNormalAI, including KindSupplied, gets `FirstReleaseReviewGate()` (y/n/e))
  - internal/notes/select.go:31-32 (KindSupplied), :45-48 (SelectState.Supplied), :184-186 (supplied branch returns before the first-release check, diff assembly, the degenerate check, `--no-ai`, the exclusion check, the reuse hook, generation and `on_notes_failure`, with non-cacheable CacheInputs)
  - internal/engine/editor.go:137 (edit works on an `os.CreateTemp` copy, so the notes file is never written)
  - README.md:100 (flag-table row), :107 (prose: finished body, no AI, attended review gate first with y/n/e and no r, `-y` accepts unattended)
- Notes: The implementation matches every criterion. The body is captured once at startup into a `*string`, so a later rewrite (the pre_tag hook case) cannot reach the run. Because the supplied branch returns zero CacheInputs, a dry run with `--notes-file` writes no cache entry, which matches the README's statement that the dry-run cache never applies. `WorkDir` exists only for tests. Production leaves it empty, so `os.ReadFile` resolves a relative path against the process cwd, which is the directory mint was run from, as specified.

TESTS:
- Status: Adequate
- Coverage:
  - internal/engine/release_notesfile_test.go:87 — out-of-repo file, prior tag: no `claude` call, no `git diff`, and the body is byte-identical in the tag, the changelog and `gh release create` stdin (`assertSuppliedBodyShipped`, :64). Seeding no diff call means any diff assembly would derail the scripted runner.
  - :104 — Markdown that git's default cleanup would strip reaches the tag verbatim through `--cleanup=verbatim`.
  - :140 — table covering first release, prior tag with no diff assembled (the empty-diff precedence), `on_notes_failure = "fallback"`, and a live seeded cache entry with `reuseConfirmPrompted` asserted false. Each case asserts no AI work and the body in all three sinks.
  - :194 — gate question `Release with these notes?` with keys exactly [y n e], and the supplied body shown at the gate.
  - :224 — edit: the editor opens on the supplied body, the edited body ships to all three sinks, and the file on disk is unchanged.
  - :257 — abort at the review gate: non-zero exit, no StageFailed, no tag, push, release or commit.
  - :287 — real plain presenter with yes=true renders the `notes: accepted (-y)` line, with no AI work and the body shipped.
  - :328 — run from `<root>/sub` with both notes.md files present: ships the sub file's contents.
  - :348 — `-` names a file. The stdin pipe is left unread and asserted intact.
  - :406 — a pre_tag hook (`rewritingHookRunner`) rewrites the out-of-repo file mid-run, and the startup body ships.
  - internal/notes/select_test.go:304 — at the selector level, the supplied body wins over every path, never consults the reuse hook, is non-cacheable and runs zero git calls.
  - cmd/mint/flags_test.go:164 — the path (including `-`) threads through `parseReleaseFlags` into `ReleaseOptions()`. cmd/mint/usage_test.go:179 checks flag coverage and :244 pins the usage line word for word.
- Notes: Each test would fail if its behaviour broke, for example if the supplied branch moved below the first-release or diff-assembly step, `r` appeared on the gate, the editor wrote back to the file, or `-` were read as stdin. One overlap: the verbatim-tag test (:104) partly repeats the byte-identity check in :87. It also pins the cleanup flag and the tag→push ordering. This does not affect correctness.

CODE QUALITY:
- Project conventions: Followed. The flag uses stdlib `flag` with a curated usage line and a coverage test. The gate is answered only through `Prompt`, with the `-y` echo rendered inside the presenter. No `os/exec`, no stdout writes from the engine, and errors are lowercase and wrapped with `%w`. Tests are external packages using FakeRunner and RecordingPresenter, assert exact argv, and use `t.Parallel()` except where `t.Chdir` forbids it (marked by comments).
- SOLID principles: Good. Routing stays in the single `Selector` decision point. The read and refusal logic is one exported helper shared with regenerate's cmd path.
- Complexity: Low
- Modern idioms: Yes (`flag.FlagSet.Visit` for presence, `filepath.IsLocal`, `t.Chdir`, `t.Context()`)
- Readability: Good. The comments on ReleaseOptions.NotesFile, SelectState.Supplied and the SelectBody precedence list are accurate against the shipped code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
