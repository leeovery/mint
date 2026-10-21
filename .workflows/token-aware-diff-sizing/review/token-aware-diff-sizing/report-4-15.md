TASK: Pin the diff prefixes so saved notes and piece framing name real paths (token-aware-diff-sizing-4-15)

ACCEPTANCE CRITERIA:
- A forward release reads its diff with `git diff --src-prefix=a/ --dst-prefix=b/ {lastTag}..HEAD -- . :(exclude)CHANGELOG.md`, followed by the configured `diff_exclude` globs and, in plain mode, the `version_file` exclude, in today's order
- A regenerate of one version, and each version of a `regenerate --all` batch, reads its `{PreviousTag}..{Tag}` diff with the same two prefix flags ahead of the range and the same exclude pathspecs after `-- .`
- The Change Map's `--name-status` and `--numstat` reads keep their current argv, with no prefix flags
- A failed chunked run over prefixed headers (`diff --git a/api/handler.go b/api/handler.go`) that leaves `api/handler.go` without notes saves a file whose marker names `api/handler.go`, or `api/handler.go` (partly) when only some of its pieces failed
- A piece cut from that file is framed as a piece "of the diff for api/handler.go", and the file packs in path order under `api/handler.go`

STATUS: complete

SPEC CONTEXT: The spec has no section on diff prefixes. The task comes from planning. It closes a gap in the saved chunk-notes file (task 4-11's "Notes missing" marker), the piece framing and the chunk sort. All three name files through `headerPath`, which reads only git's `a/P b/P` header form. An operator's `diff.noprefix`, `diff.mnemonicPrefix` or `diff.srcPrefix`/`diff.dstPrefix` would otherwise give doubled or wrong path names. The plan chose to pin the prefixes in the argv rather than teach `headerPath` more header forms, because flags given on the command line override git's prefix config.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/assemble.go:184 — `AssembleRange` argv is `diff --src-prefix=a/ --dst-prefix=b/ <range> -- . <excludePathspecs>`. `AssembleDiff` delegates to it (internal/notes/assemble.go:165), so the forward release, a single-version regenerate and `regenerate --all` all use this one argv.
  - internal/notes/assemble.go:143-147 and :171-172 — the doc comments state the pinned prefixes and why they are there.
  - internal/notes/chunk.go:296-299 — the no-prefix clause is gone from `headerPath`'s doc. It now reads "A header in any other form than "a/P b/P" keys on itself whole", which matches the code at :300-319.
  - internal/notes/changemap.go:50,56 — the Change Map's `--name-status` and `--numstat` argv are unchanged and carry no prefix flags.
- Notes: `rg -n '"diff"' --type go --glob '!*_test.go'` finds AssembleRange as the only range-patch read. The other hits are the two Change Map reads, `mint commit`'s sources (internal/commit/source.go:62,70, internal/commit/generate.go:306) and the record commit's `--cached --name-only` probes (internal/record/commit.go:167,185). None of these changed, as the task required. The exclusion check also names files through `headerPath` (internal/notes/exclusioncheck.go:276), so it now gets real repo paths too. Under git's default prefix config the flags change nothing in the diff text, so notes-cache keys and downstream bytes are unchanged. The release diff's built-in excludes later gained `:(exclude).mint.toml` (internal/notes/assemble.go:29-32); that came from a different task and does not affect this one.

TESTS:
- Status: Adequate
- Coverage:
  - Exact argv for the forward release: internal/notes/assemble_test.go:15 (`wantArgs`, used by every AssembleDiff test), internal/notes/generate_test.go:248 with globs, internal/engine/release_priortag_test.go:128 (built-ins), :204 (globs), :245 (plain-mode version_file), internal/engine/release_builtin_exclude_test.go:56, internal/engine/release_diffexclude_test.go:106.
  - Exact argv for regenerate: internal/notes/range_test.go:25 (`wantRangeArgs`) and internal/engine/regenerate_fresh_test.go:88,112,135,160,187,327,427. For `regenerate --all`: cmd/mint/regenerate_all_test.go:275,422,480. Halving: internal/engine/notes_halve_test.go:127 and internal/engine/notes_halveparts_test.go:162.
  - The three negative assertions now use the prefixed form, so they actually test something: internal/engine/regenerate_fresh_test.go:93,163 and internal/engine/release_priortag_test.go:298. `invokedWith` compares the whole command line exactly (internal/engine/release_test.go:400-408).
  - The Change Map argv is pinned with no prefix flags: internal/notes/changemap_test.go:17,23, internal/notes/range_test.go:166-167, internal/notes/chunk_test.go:600, internal/engine/release_priortag_test.go:207,210,248,251.
  - Marker, framing and order over prefixed headers: the new test internal/notes/notesmissing_test.go:236. Its fixture lists zeta/z.go before api/handler.go, so the sort really runs. It checks the "(partly)" marker, the "of the diff for api/handler.go, a file" piece framing, and that the api/handler.go pieces pack before zeta/z.go. The whole-file marker over prefixed headers was already covered at internal/notes/notesmissing_test.go:21-56 (`api/handler.go`, `api/routes.go`).
  - A regression that drops the flags would fail every exact-argv assertion above.
- Notes: The one new test is focused and adds framing and order checks that the existing partly-marker test (internal/notes/notesmissing_test.go:110) does not have. No redundant tests were added. The stale "preserved BYTE-IDENTICAL ... `git diff {lastTag}..HEAD`" comment in internal/notes/range_test.go was removed. Leaving it would have falsely described the argv.

CODE QUALITY:
- Project conventions: Followed. The diff still goes through `runner.CommandRunner.RunInDir` at the repo root, the argv is asserted exactly, and the tests use external packages and `t.Parallel()`.
- SOLID principles: Good. The fix sits at the one place the diff is built (`AssembleRange`), so the forward release, regenerate and `--all` all get it.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good. The doc comments say why the prefixes are pinned and name the config keys they override.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "`diff.noprefix = true`, custom `diff.srcPrefix`/`diff.dstPrefix` and `diff.mnemonicPrefix` all yield `a/P b/P` headers once `--src-prefix=a/ --dst-prefix=b/` sit ahead of the range" (the plan's row for 4-15; also the task's Outcome, "Under any diff prefix config ... name each file by its repo path") — this needs a run of real `git diff --src-prefix=a/ --dst-prefix=b/ <range> -- .` in a scratch repo under each of the four configs (`diff.noprefix=true`, `diff.mnemonicPrefix=true`, `diff.srcPrefix=x/` + `diff.dstPrefix=y/`, and the default), confirming every `diff --git` header reads `a/P b/P`. The project's tests never run real git, so reading the code cannot settle this.
