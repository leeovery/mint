## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/README.md:141. The new paragraph gives "such as the part notes a failed chunked run saved" as an example of P. As built, a failed chunked run saves no file. That is Phase 4's §11.4 work (`phase-4-tasks.md`, "saved chunk notes"); no production code writes chunk notes today. A reader would look for a saved file after a failed chunked regenerate and find none, and the ✗ line names none. CLAUDE.md and spec §18 require the README to match what is built. No README criterion asked for this clause.
  FIX: Drop the clause, so the sentence reads "`--notes-file P` is a fourth notes source beside `fresh`, `tag` and `release`: the finished body in file P, such as notes you wrote by hand for an old version." Phase 4's README work can add the saved-file example when the file exists.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/engine/regenerate_interactive.go:133-135 (used at :164). `RegenerateRunRequest.WorkDir` is set by nothing: not `runRegenerateSingle`, and not any test. The relative-path test uses `t.Chdir` instead. The field is a dead exported input added in this task's own scope. Someone extending regenerate could set it believing it is wired, while the only path that exercises its effect is release's.
  FIX: Remove the `WorkDir` field and its doc, and use `req.NotesFile` directly at :164 (`notesFilePath` with an empty workDir returns the path unchanged). Keep the `t.Chdir` test, because it exercises the production path (empty WorkDir, process cwd).
  ALTERNATIVE: Keep the field and have the relative-path test use it, so the test can run `t.Parallel()` like release's `inRepoNotesFileOptions`. But then nothing tests regenerate's production empty-WorkDir path, so removal is the better choice.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/engine/regenerate_write.go:207-208 — "`-y` skips both" is now wrong: there are three gate kinds (fresh review, supplied review without `r`, reuse confirm). The line also runs past the comment's wrap width.
  OLD: // without `r`; reuse runs a simple two-choice confirm (no review gate). A decline aborts non-zero. `-y` skips both — the
// presenter returns the gate default (ChoiceYes), so the gate accepts immediately.
  NEW: // without `r`; reuse runs a simple two-choice confirm (no review gate). A decline
// aborts non-zero. Under `-y` the presenter returns each gate's default (ChoiceYes),
// so the gate accepts immediately.
- /Users/leeovery/Code/mint/internal/engine/regenerate.go:30-33 — the type doc says it "carries only the two facts", but the diff added a third field, KeepOut, which does not come from the target.
  OLD: // RegenerateGateSet is the RESOLVED preflight selection for one regenerate run,
// derived from the request's target surface. It carries only the two facts that
// drive the subset, so the selector stays a pure mapping with no knowledge of the
// cmd-layer source/target enums:
  NEW: // RegenerateGateSet is the RESOLVED preflight selection for one regenerate run,
// derived from the request's target surface, plus the path the clean-tree gate
// leaves out. Two facts drive the subset, so the selector stays a pure mapping with
// no knowledge of the cmd-layer source/target enums:
- /Users/leeovery/Code/mint/internal/engine/regenerate_write.go:76-77 — "set only by RegenerateRun" is a cardinality claim, which the comment rules forbid. Phase 5's floor bodies take this same gate (§12.3) and may set it elsewhere. The invariant worth keeping is that no producer handles the value.
  OLD: 	// runs the review gate without `r`, since no AI wrote it. It is set only by
	// RegenerateRun, which holds the body, so no body producer ever sees it.
  NEW: 	// runs the review gate without `r`, since no AI wrote it. The run already holds
	// its body, so no body producer handles it.

NOTES:
- The cmd wiring `NotesFile: req.NotesFile` (/Users/leeovery/Code/mint/cmd/mint/main.go:238) has no test. If it were dropped, the flag would be ignored and the run would silently take the AI path, which §13.4 warns about. This matches the existing state of `runRegenerateSingle`'s struct literal, where no field is tested; only the extracted helpers are. Release has a testable `ReleaseOptions()` mapping, and 3-5's cmd-level refusals would be a natural place to add regenerate's equivalent.
- Some older comments the diff did not touch now leave out the supplied source. They are incomplete, not false: the `RegenerateSource` type doc (regenerate_write.go:56-59), `RegenerateWriteRequest.Source` (:113), the inline comment in `RegenerateWrite` (:152-153), and `RegenerateRun`'s delegation comment (regenerate_interactive.go:242-245, "fresh → notes-review, reuse → simple confirm").
- Until 3-5 lands, `--all --notes-file` ignores the file and runs the AI batch, and `--source X --notes-file` lets the file win silently. 3-5 owns both. Until 3-4 lands, the README's "leaves it as it was" does not hold after a failed changelog push, as the executor reported. The plan accepts both gaps.
- A supplied body is narrated by the notes stage as "generating release notes…" / "Generated release notes", as every non-AI body and release's supplied body (3-1) already are.
- Gate choice is keyed on the source enum. Phase 5's floor sends `c`/`e` bodies from a fresh-source run through this no-`r` gate, so it will need either to switch the source to `regenerateSourceSupplied` or to carry the gate choice separately from the source.
- An edge with no realistic use: `--notes-file CHANGELOG.md` on a changelog target would overwrite the notes file and then fail with `ErrNothingStaged`. Release's bookkeeping commit has the same edge.

## Attempt 2

ISSUES:
- /Users/leeovery/Code/mint/cmd/mint/main.go:238: the `NotesFile: req.NotesFile` line that carries the parsed flag into `engine.RegenerateRunRequest` has no test. I deleted that line in a scratch copy and `go test ./cmd/mint/ ./internal/engine/` still passed. Without it, `mint release regenerate 1.4.0 --notes-file notes.md --target both -y` parses fine, then the source gate answers `source: fresh (-y)` and the AI notes overwrite the changelog and the provider release. That is the outcome §13.4 is written to prevent: the run going down the AI path the caller meant to skip. Release guards its own hop in this phase (/Users/leeovery/Code/mint/cmd/mint/flags_test.go:178 asserts `opts.ReleaseOptions().NotesFile`). Regenerate already pulls out and tests its source mapping (`regenerateRunAxes`, /Users/leeovery/Code/mint/cmd/mint/regenerate_run_test.go:18). The new source input skips that tested path.
  FIX: In /Users/leeovery/Code/mint/cmd/mint/regenerate_run.go, extract the flag-derived part of the run request into a pure helper, e.g. `func regenerateRunFlags(req regenerateRequest) engine.RegenerateRunRequest`. It returns `Source`/`Target` (via `regenerateRunAxes`), `NotesFile: req.NotesFile` and `Yes: req.Yes`. `runRegenerateSingle` starts from that value and fills in Tag, VersionKey, Project, ReleaseBranch, ChangelogEnabled and the producers. Then add a test in /Users/leeovery/Code/mint/cmd/mint/regenerate_run_test.go that:
  - feeds `{"1.4.0", "--notes-file", "notes.md", "--target", "both", "-y"}` through `parseRegenerateFlags` and asserts the helper's `NotesFile == "notes.md"`
  - asserts that a parse without the flag gives `""`.
  This mirrors `TestParseReleaseFlags_NotesFile`.
  ALTERNATIVE: A cmd test that drives `runRegenerateSingle` with a FakeRunner and a temp notes file, and asserts that no `claude` is invoked. It covers the real end-to-end path, but it needs the reads behind `version.ResolveRegenerateTarget` and `engine.ResolvePublisher` scripted, so it is heavier. I recommend the pure helper because it matches how release guards its hop.
  CONFIDENCE: medium

NOTES:
- Fix round checked:
  - The README no longer has the "part notes a failed chunked run saved" clause.
  - `RegenerateRunRequest.WorkDir` is gone. `RegenerateRun` passes `req.NotesFile` straight through at regenerate_interactive.go:161 and :201, and the `NotesFile` doc now says the process working directory.
  - All three earlier comment corrections are in the files word for word.
- /Users/leeovery/Code/mint/internal/engine/release.go:316 and :843: `notesFilePath` was changed from `(opts ReleaseOptions)` to `(workDir, notesFile string)` for the regenerate `WorkDir` that has now been removed. Only release calls it. It is harmless; reverting is optional.
- Some older comments the diff did not touch now leave out the supplied source. They are outside this task's scope:
  - regenerate_write.go:56-59, :113, :152-153
  - regenerate_interactive.go:16-17 ("A source resolves from `--source …` or the source prompt"), :224-225, :238-241
  - regenerate_interactive.go:170-172, "a reuse source FORCES release", was already false before this task.
- Cosmetic only: the reflow at regenerate_changelog.go:52-54 ("committed is / true. When the write nets NO change, / nothing is staged…") is ragged, and the edited comment lines at regenerate_flags.go:80 and :89 run past the wrap width.
- Gaps the plan already assigns to sibling tasks:
  - `--all`/`--source` together with `--notes-file`, an empty path and an empty file are 3-5's.
  - Keeping a tracked file's edits through the reset after a failed changelog push is 3-4's. Until 3-4 lands, the README's "leaves it as it was" does not hold on that path.
- Edge case: `--notes-file CHANGELOG.md` on a changelog target would overwrite the notes file and then fail with `record.ErrNothingStaged`. Release's bookkeeping commit has the same edge.
- For context on this round I read the on-disk file /Users/leeovery/Code/mint/.workflows/token-aware-diff-sizing/implementation/token-aware-diff-sizing/fix-tracking-token-aware-diff-sizing-3-3.md. The previous round listed the cmd-hop gap as a note. I raise it as an issue here after confirming it by mutation. Nothing beyond the enumerated inputs came with the dispatch.
