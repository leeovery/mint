# Consolidation Findings: token-aware-diff-sizing (Phase 4)

## Findings

### F1: The saved notes' missing-files marker shows a doubled path when the operator's git drops or changes diff prefixes
- **Class**: behaviour
- **Failure**: An operator with `diff.noprefix = true` (or custom `diff.srcPrefix`/`diff.dstPrefix`) in their git config whose chunked run fails gets a saved chunk-notes file whose marker reads ``**Notes missing:** no notes were written for `api/handler.go api/handler.go` — add them before releasing.``. That path does not exist. The marker is meant to be read later, by the operator through the floor's `e` or by an agent releasing with `--notes-file` after the terminal output is gone, and it tells them to fill in files under a name that matches no file in the repo. They see it when they open the saved file. The same doubled name already reached the AI in piece framing before this phase. This phase adds it to the artifact the operator has to act on.
- **Evidence**:
  - `internal/notes/chunknotes.go:116-127`: `chunkPart.paths()` (added in 4-11) names each file in the marker. For a whole-file part it calls `headerPath` (line 124); for a piece it uses `p.file` (lines 118-119), which `cutParts` also gets from `headerPath` (`internal/notes/chunk.go:389`).
  - `internal/notes/chunknotes.go:83-114`: `notesMissingLine` writes those paths into the marker verbatim.
  - `internal/notes/chunk.go:287-312`: `headerPath` handles only the `a/P b/P` form (lines 305-310) and quoted paths. Any other header comes back whole (line 311), and the doc comment says so on purpose (lines 289-290: "A header in any other form, such as diff.noprefix's, keys on itself whole"). That was harmless while `headerPath` only fed the sort key. It is wrong now that its result is shown to the operator.
  - `internal/notes/assemble.go:178`: `git diff <range> -- . <excludes>` does not pin prefixes, so the user's diff prefix config decides the header form.
  - Pre-existing, same cause: piece framing names `p.file` (`internal/notes/chunk.go:546-550`).
- **Proposed shape**: Pin the prefixes in `AssembleRange`'s argv (`git diff --src-prefix=a/ --dst-prefix=b/ <range> -- . …`). Every header then takes the `a/P b/P` form `headerPath` already reads, whatever the user's prefix config. This fixes the marker, the piece framing and the sort key in one place. Then delete the no-prefix clause from `headerPath`'s doc. The tests that pin the exact diff argv change with it (`internal/notes/assemble_test.go`, `range_test.go`, `generate_test.go`, `chunk_test.go`, `internal/engine/notes_halve*_test.go`, `regenerate_fresh_test.go`, `release_priortag_test.go`, `cmd/mint/regenerate_all_test.go`). Narrower alternative: teach `headerPath` the unprefixed `P P` form (odd length, a space at the midpoint, two equal halves). This stays inside `chunk.go` and leaves argv alone, but covers only `diff.noprefix`, not custom src/dst prefixes.
- **Bank**: headerPath keys an unprefixed diff header on the whole string, so the marker and piece framing show a doubled path (reviewer, 4-11)

## Comment Corrections

- internal/engine/release.go:294-298 — the `DryRun` doc says a dry run has one intentional side effect. Since this phase, a dry run whose chunked notes fail also writes the chunk-notes file (the keeper at release.go:518 is wired whatever `DryRun` holds). The sentence also names the user-level cache "gitignored/temp" and carries task ids.
  OLD: // flag is wired in production (cmd/mint) and sets this field. The ONE intentional
	// side effect (task 4-7): after the notes PREVIEW is
	// generated, the generated AI note is WRITTEN to the gitignored/temp dry-run cache
	// (keyed by the post-diff_exclude diff + version + prompt/context) so the
	// subsequent real run can REUSE it (reuse itself is task 4-8).
  NEW: // flag is wired in production (cmd/mint) and sets this field. Outside the repo, a
	// dry run still writes the AI note it previews to the user-level dry-run cache, so
	// the following real run can reuse it, and a failed chunked run's finished chunk
	// notes to their file, as a real run does.
