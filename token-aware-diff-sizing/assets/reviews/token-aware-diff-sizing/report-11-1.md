TASK: Drop the single-version regenerate request's unread `Yes` field (token-aware-diff-sizing-11-1)

ACCEPTANCE CRITERIA:
- `mint release regenerate <version>` under -y at a terminal, notes generation failing: prints the ✗ line, the AI's verbatim output and regenerate's cause-fitted next moves, asks nothing, writes nothing, exits 1; -y reaches the floor through `ReleaseDeps.Yes` alone (§12.1, §12.3, §12.4)
- The same failure at a terminal without -y is offered the `c` / `e` / `a` choice (§12.2, §12.3)
- The same failure with stdin not a terminal and no -y stops with the next moves and asks nothing (§12.3)
- For `1.4.0 --notes-file notes.md --target both -y`, `regenerateRunFlags` gives NotesFile `notes.md` and target both; without `--notes-file`, NotesFile is empty (§13.1)
- `RegenerateRunRequest` has no `Yes` field, and its doc comment no longer lists "the -y flag" among its inputs. `BatchRegenerateRequest.Yes`, its read in `gatePerVersion`, and the cmd layer's `regenerateRequest.Yes` stay as they are
- Behaviour is unchanged; existing tests stay green apart from the forced edits; all five CLAUDE.md gates pass

STATUS: issues_found

SPEC CONTEXT: §12.1/§12.3 make attended versus unattended an engine decision for regenerate's notes floor: an attended single-version regenerate gets the c/e/a choice, and an unattended one (-y or non-TTY) stops with the cause-fitted next moves from §12.4 and is never prompted. In the code that decision is `ReleaseDeps.attended()` (`!d.Yes && d.StdinInteractive`, internal/engine/notesfloor.go:18-20), and the floor branch reads it at internal/engine/regenerate_interactive.go:238. A request-level `Yes` that nothing read was a trap: it looked like the live sibling of `BatchRegenerateRequest.Yes`.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/regenerate_interactive.go:100-158: `RegenerateRunRequest` no longer has a `Yes` field (removed in f223ea7)
  - cmd/mint/regenerate_run.go:30-37: `regenerateRunFlags` sets only Source, Target and NotesFile
  - Untouched as required: `BatchRegenerateRequest` (internal/engine/regenerate_batch.go:41), its `req.Yes` read in `gatePerVersion` (internal/engine/regenerate_batch.go:464), `ReleaseDeps.Yes: req.Yes` (cmd/mint/main.go:194), the -y/--target check (cmd/mint/regenerate_validate.go:33), and the batch request's `Yes: req.Yes` (cmd/mint/regenerate_all.go:59)
  - The engine's only remaining non-test reads of `Yes` are regenerate_batch.go:464 (batch request) and notesfloor.go:19 (deps). Nothing reads -y from the single-version request.
- Notes: The task commit (f223ea7) removed "the -y flag" from the struct's doc comment as planned. The later phase 11 comment-corrections commit (49f7b86) then deleted that doc comment entirely, so the exported type now has no doc comment. The AC's doc-comment clause is met vacuously. See FINDINGS.

TESTS:
- Status: Adequate
- Coverage:
  - AC1 (-y at a terminal, through deps alone): internal/engine/regenerate_floor_nextmoves_test.go:26-33 defines the "-y at a terminal" mode with `yes: true, stdinInteractive: true`. `newUnattendedRegenFloor` (:36-42) sets only `run.deps.Yes` / `run.deps.StdinInteractive`. `regenerateUnattended` (:44-54) builds the request through `runReq` with no Yes. `assertRegenStopsWithNextMoves` (:58-85) checks exit 1, the ✗ line before a single notes NextMoves event, nothing after it, no prompts, and no writes. -y reaches the floor only through `ReleaseDeps.Yes`, so the test proves the criterion.
  - AC2 (attended choice): internal/engine/regenerate_floor_test.go:26-43 (`newAttendedRegenFloor` sets `StdinInteractive = true`, leaves Yes false).
  - AC3 (non-TTY, no -y): the second `unattendedModes` entry (regenerate_floor_nextmoves_test.go:32).
  - AC4: cmd/mint/regenerate_run_test.go:123-145 keeps both table rows (with and without --notes-file, `-y` in args), the NotesFile check and the Target check. The `run.Yes` assertion is removed.
  - Forced edits match the Do list: `runReq` lost its `yes` parameter (internal/engine/regenerate_interactive_test.go:51-61). `rg '\brunReq\('` now finds 38 lines: the 37 the task measured plus internal/engine/notes_reporoot_test.go:113, which task 12-1 added later with the two-argument form. None passes a third argument. `req.Yes = true` is gone from regenerate_notesfile_test.go (that test's -y comes from `presenter.NewForStartup(true, true, ...)` at :335). `run.deps.Yes = yes` in the next-moves helper is kept. `batchReq`'s `yes` and the batch `req.Yes = mode.yes` (regenerate_batch_skipcause_test.go:130) are untouched.
  - Former `yes=true` callers are still unattended through their deps: regenerate_chunknotes_test.go:68 and notes_waveend_test.go:181 leave `StdinInteractive` false. `TestRegenerateRun_Yes_AlwaysCallsPrompt` now sets `deps.Yes`/`deps.StdinInteractive` (regenerate_interactive_test.go:236-238, from task 11-3).
- Notes: No new tests were needed, since this is a deletion with no behaviour change, and none were added. Not over-tested.

CODE QUALITY:
- Project conventions: Followed (with one exception, listed in FINDINGS)
- SOLID principles: Good. A single-version regenerate now learns -y only from `ReleaseDeps.Yes`.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: The exported `RegenerateRunRequest` has no doc comment (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/regenerate_interactive.go:100 — The phase 11 comment-corrections commit (49f7b86) deleted the type's whole doc comment instead of only dropping "the -y flag" as the task's Do said. `type RegenerateRunRequest struct` now has no doc. Fix: restore a short doc comment that is true of the current struct. For example: "// RegenerateRunRequest carries one version's inputs to RegenerateRun, the single-version counterpart of BatchRegenerateRequest. It carries no -y: the notes floor reads -y from ReleaseDeps.Yes, and the presenter answers gates under it." — FAILS: This is the only one of the 25 top-level exported types in internal/engine's non-test files without a doc comment. `go doc` and editor hover show nothing for the request type at the centre of this fix. Nothing on the type now tells a maintainer that it is the single-version request or that -y does not belong on it. That is the confusion this task removed, and it would make re-adding a field that does nothing an easy mistake. Comment-only remedy; never blocking.

UNSETTLED:
- "Behaviour is unchanged. Every existing test stays green with its assertions as they stand, apart from the edits the deletion forces, which are listed under Do. All five gates in CLAUDE.md pass" — settling this needs `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` run on the current tree. Reading confirms that no code reads the removed field and that every `runReq` call site has the two-argument form.
