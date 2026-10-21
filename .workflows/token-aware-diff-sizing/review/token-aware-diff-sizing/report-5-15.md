TASK: Printed `--notes-file` commands quote the saved path, built in one place (token-aware-diff-sizing-5-15, tick-266a9d)

ACCEPTANCE CRITERIA:
- An unattended release from a repo whose path holds a space, its chunked notes failing with part notes saved, stops with the next move `tidy the saved notes at <path>, filling in any files its marker lists, and release with mint release --notes-file '<path>'`; an unattended `mint release regenerate <version>` failing the same way ends its move with `mint release regenerate <version> --notes-file '<path>'`.
- Under `on_notes_failure = "fallback"` from that repo, the fallback warning names `mint release regenerate <version> --notes-file '<path>'` and `mint release --notes-file '<path>'`, and a dry run's warning names `mint release --notes-file '<path>'`.
- A `regenerate --all` version skipped with saved part notes from that repo names `mint release regenerate <version> --notes-file '<path>'` in its skip warning and in the end summary.
- A saved path shown rather than passed prints raw, a space included: the ✗ line's `part notes saved to <path>`, the fallback warning's and the `--all` skip's `part notes saved to <path>`, and the next move's `tidy the saved notes at <path>`.
- `cmdline.Split` of `cmdline.Quote(p)` gives back `[p]` for a path with a space, a `'`, or any of `$`, `` ` ``, `*`, `;`, `&`, `|`, `~`.
- A saved path made only of `[A-Za-z0-9_@%+=:,./-]` bytes prints unquoted, byte-identical to today: the existing literals at notes_floor_nextmoves_test.go:22, regenerate_floor_nextmoves_test.go:22, notes_fallback_warning_test.go:32-33, notes_fallback_dryrun_test.go:18 and regenerate_chunknotes_test.go:184 pass unchanged.
- With no chunk notes saved, the move still reads `write the notes to a file and release with <command> --notes-file <file>`, the placeholder unquoted.

STATUS: complete

SPEC CONTEXT: §12.4 makes the `--notes-file` route an Always next move: with a saved chunk-notes file, tidy it and release with `mint release --notes-file <path>` (version not out) or `mint release regenerate <version> --notes-file <path>` (version out). §12.1 has the fallback warning name both commands (dry run: only the release one); §12.3 has each `--all` skip name its own regenerate command in the warning and end summary. The printed command is the one the reader is told to run, so it must survive a shell paste; the saved path keeps any space from the repo root because repoScope (internal/notescache/cache.go:190-192) turns only separators into dashes.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/cmdline/quote.go:10-26 — `Quote` returns the word unchanged when every rune is in `[A-Za-z0-9_@%+=:,./-]` (non-ASCII and invalid UTF-8 runes fall outside the set, so this matches the byte rule), else wraps it in single quotes with each `'` written `'\''`; empty word is `''`.
  - internal/cmdline/split.go:5-6 — package doc names Quote as Split's inverse.
  - internal/engine/notesfloor.go:75-77 — `notesFileCommand(base, path)` renders `base + " --notes-file " + cmdline.Quote(path)`.
  - Callers: notesfloor.go:69 (`notesFileMove`, serving `notesFloorMoves` :46 and `regenerateNotesFloorMoves` :59), notesstage.go:117-118 (`warnFallback`), notesstage.go:126 (`warnDryRunFallback`), regenerate_batch.go:256 (`savedNotesPointer`).
  - Stay raw, as planned: the `<file>` placeholder (notesfloor.go:67), `tidy the saved notes at` (notesfloor.go:69), `part notes saved to` (notesstage.go:136, regenerate_batch.go:256, presenter.go:604 `savedNotesText`).
- Notes: The task's grep `rg -n --glob '!*_test.go' -- '--notes-file " \+' internal cmd` now matches only notesfloor.go:76, the helper's own line, which is what the task's Do section asks for. No other non-test site prints a `--notes-file` command with a path (checked across internal/presenter, internal/notes, internal/notescache and cmd). The commit (ec50fdf) leaves the five AC6 literal helpers untouched.

TESTS:
- Status: Adequate
- Coverage:
  - internal/cmdline/quote_test.go:10-46 — round-trip through `Split` for space, `'`, only-quotes, `$`, backtick, `*`, `;`, `&`, `|`, `~`, double quote + backslash, tab + newline and empty (AC5). :48-75 pins the exact output: safe bytes unchanged, a space wrapped, `'` written `'\''`, empty `''`.
  - internal/engine/notesfile_quoting_test.go — five behaviour tests from a repo root holding a space (`spacedRoot`, :17-24): release floor next move with SavedNotes raw (:35-50), regenerate floor next move (:52-68), release fallback warning naming both commands (:70-92), dry-run fallback warning (:94-112), and the `--all` skip warning plus end summary (:114-134). The expected strings hard-code `'` + path + `'` and do not call `Quote`, so they check the output independently. `assertSpaced` (:28-33) stops a test passing vacuously if the root change did not take effect. Each test also asserts that the prose `part notes saved to` and `tidy the saved notes at` text and `StageFailure.SavedNotes` stay raw (AC1-AC4).
  - AC6 / AC7: the existing unquoted literals (notes_floor_nextmoves_test.go:16 and :22, regenerate_floor_nextmoves_test.go:18 and :22, notes_fallback_warning_test.go:32-33, notes_fallback_dryrun_test.go:18, regenerate_chunknotes_test.go:184) are unchanged in the commit. Their callers' subtest names produce t.TempDir paths built only from safe bytes, so `Quote` returns those paths unchanged.
- Notes: The tests are focused, with one test per printed surface and no redundant cases.

CODE QUALITY:
- Project conventions: Followed (external test packages, t.Parallel throughout, table-driven where the shape fits, exact rendered strings asserted; WHY-comments accurate to the code)
- SOLID principles: Good — one renderer for the command; quoting sits in cmdline beside its inverse
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
