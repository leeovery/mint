TASK: Write exclusion answers into `.mint.toml`'s `[release]` table (token-aware-diff-sizing-8-1, tick-335d83)

ACCEPTANCE CRITERIA:
- Existing `[release]` with `diff_exclude = ["specs/"]` and `diff_keep = ["docs/api/"]`; attended release answers Exclude on `docs/design/` and Keep on `test/fixtures/` -> after the last pause the file loads with `[release].diff_exclude` = specs/, docs/design/ and `[release].diff_keep` = docs/api/, test/fixtures/ (§6.5, §6.6)
- `[release]` setting only `version_file`/`version_pattern` + Exclude on `docs/design/` -> `diff_exclude` added to that table; still one `[release]` table; no `diff_keep` added (§6.6)
- Shared multi-line `diff_exclude` + `[release.hooks]`, no `[release]` + Exclude on `docs/design/` -> a `[release]` table is added holding it; file loads with shared list and hooks unchanged (§6.6)
- No `.mint.toml` + Keep on `test/fixtures/` -> file created holding only `[release]` with that `diff_keep` (§6.6)
- Header comment, own-line and trailing comments, blank lines, aligned `=`, shared `diff_exclude`, multi-line `[release].diff_exclude` with comments among entries; Exclude on `docs/design/` and `tmp/` -> everything else byte-identical and in place; both globs follow the existing entries; shared list byte-identical (§6.6)
- Inline `release = { tag_prefix = "v" }` or dotted `release.version_file = "VERSION"` + Exclude on `docs/design/` -> lands in that form (inside the braces, or a `release.diff_exclude` dotted key); no `[release]` header added; file loads; every other line byte-identical (§6.6, Corrigendum 2026-10-19)

STATUS: complete

SPEC CONTEXT: §6.6 holds the answers until the last pause, then writes them together into `.mint.toml`'s `[release]` table: append to an existing key, add the key, add the table, or create the file, as needed. An inline or dotted `[release]` takes the answer in its own form, because TOML allows no second definition of the table. Everything else stays as written, and answers never go to the shared list (§5.1), so commit messages still see those files. §6.5: Exclude joins `[release].diff_exclude`, Keep joins `[release].diff_keep`. The §6.7 corrigendum says to append, never overwrite: a second `diff_exclude` line in the table would break the next strict load.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/appendrelease.go:35 `AppendReleaseGlobs`. It reads `{root}/.mint.toml`; an absent file gets mode 0644 and an existing one keeps its mode. It writes through `fsutil.WriteFile` only after the edit plan succeeds, so a file that does not parse is left untouched.
  - internal/config/appendrelease.go:161 `locateRelease` / :195 `locateKeyValue`. A single go-toml `unstable.Parser` pass classifies `[release]` as absent, header, inline or dotted and records each list's closing bracket and last-entry offsets. `[release.hooks]` and other headers move the scope to "other", so root-level and sub-table keys are never touched.
  - internal/config/appendrelease.go:254 `appendToList`. It inserts after the last entry and covers empty, single-line, trailing-comma, multi-line with or without a comma, comment-trailed last entries and CRLF. Comments that follow the last entry stay in place.
  - internal/config/appendrelease.go:314 `addKeys` / :348 `addInlineKeys`. Header form: the key goes after the last key/value line of `[release]` (or after its header line), with that line's indent. Dotted form: `release.`-prefixed after the last dotted key. Inline form: `, key = [...]` inside the braces, or `{ key = [...] }` when the table is empty. Absent: a new `[release]` table at end of file, separated by a blank line.
  - Wiring: internal/notes/exclusioncheck.go:192 calls the record once, after the pause loop, with every answer in the order answered. internal/engine/release.go:1090 (`answersCommit.record`) -> :1127 (`write`) calls `config.AppendReleaseGlobs(a.root, ...)` with the same `root` that `config.Load` reads (internal/engine/release.go:370, :550).
- Notes: I traced each criterion's fixture through the go-toml v2.3.1 `unstable` parser's Raw ranges. A KeyValue's Raw spans key through value end, so `rangeEnd(kv)-1` is the array's `]`. An InlineTable's Raw covers only `{`, and the code accounts for that. With `KeepComments` off, comments are not array children. Edits at distinct offsets never collide, including the case where one list is appended and the other key is added in the same run. Answers reach `[release]` only: a root-level `diff_exclude` (key length 1 in root scope) is never recorded as a target.

TESTS:
- Status: Adequate
- Coverage:
  - Criterion 1: internal/config/appendrelease_test.go:44 checks the exact bytes and the loaded values. internal/engine/release_exclusionrecord_test.go:25 runs the attended release end to end, with an Exclude and a Keep at the gates, and asserts the loaded lists plus an untouched shared list.
  - Criterion 2: :63 checks exact bytes (one `[release]`, no `diff_keep`) and that `DiffKeep` is unset after load.
  - Criterion 3: :95 checks exact bytes, the shared list and `pre_tag` after load.
  - Criterion 4: :123 at config level, plus internal/engine/release_exclusionrecord_test.go:52 through a real attended release.
  - Criterion 5: :136, a byte-exact whole-file comparison over the specified layout, plus the loaded release and shared lists.
  - Criterion 6: :175 covers the inline, empty inline, inline with the key set, dotted, and dotted with the key set cases, each byte-exact and loaded.
  - Edge shapes: :222 (empty, trailing comma, last entry without a comma plus a comment, closed on the entry line, CRLF) and :267 (header-only with a trailing comment, indented keys with no final newline, `[release]` after its own sub-table, no table, empty file).
  - Also tested: escaping round-trips (:316), mode preserved (:327), and an unparseable file returned as an error and left unchanged (:350).
- Notes: Every byte-exact assertion would fail if the writer moved or reformatted anything. The table cases each exercise a distinct branch of `appendToList`/`addKeys`, so the tests are not redundant.

CODE QUALITY:
- Project conventions: Followed. Config logic stays in `internal/config`, errors are wrapped with `%w` in lowercase, tests are external with `t.Parallel()` and `t.TempDir()`, and the write is atomic via `fsutil`.
- SOLID principles: Good. Locating, planning edits and applying them are separate steps, and the writer is pure over bytes (`appendReleaseGlobs`) behind a thin file shell.
- Complexity: Acceptable. The offset arithmetic is dense but each helper has one job, and doc comments state the offset contracts.
- Modern idioms: Yes (`slices`, `strings.Builder`, `errors.Is(fs.ErrNotExist)`).
- Readability: Good. The comments in appendrelease.go match what the code does.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
