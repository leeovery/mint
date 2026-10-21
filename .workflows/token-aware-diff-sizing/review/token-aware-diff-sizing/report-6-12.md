TASK: One review-gate `r` assembly for both verbs (token-aware-diff-sizing-6-12, tick-fedaa4)

ACCEPTANCE CRITERIA:
- A release `r` written after halving: once the regenerated notes are shown, and before the gate asks again, the lower-cap hint names the largest succeeded call of the run. An `r` that did not halve shows no hint (§10.3, §16.3)
- A single-version regenerate `r` written after halving shows the hint before the gate asks again. Under `regenerate --all`, such an `r` prints no hint of its own and counts toward the one batch-end hint (§10.3)
- An `r` that becomes a chunked run narrates as a chunked run on release, single-version regenerate and `--all` alike, and its notes line closes before the regenerated notes show. An `r` that makes one call narrates as built (§16.1)
- A failed `r` on chunked notes saves the part notes and names the file with the verb's own command: `mint release --notes-file <path>` on release, `mint release regenerate <version> --notes-file <path>` on regenerate. Under `--all` the batch then ends (§15.1, §11.4)
- A failed `r` whose part notes cannot be saved: the failure stays the AI's, and one warning carries the write error verbatim beneath it, on release and regenerate alike (§11.4)
- A direct `RegenerateWrite` on a fresh source that carries a regenerator but no `regenStage` or keeper: `r` consults that regenerator with the one-time context, a failed `r` saves nothing and names no file, and no hint follows the `r` (as built)
- With `deps.Regenerator` wired, `r` on release and on regenerate uses the override's body in place of the per-run regenerator (as built)
- Rule: `rg -n '\.narrating\(' internal -g '!*_test.go'` matches only inside the helper that builds the gate's `r`, and `rg -n '\.reportKept\(' internal -g '!*_test.go'` matches only inside the helper that runs the gate and the report
- Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass

STATUS: complete

SPEC CONTEXT: §15.1 defines `r` at the review gate for release, single-version regenerate and `--all`: a failed `r` on chunked notes (or one the ladder made chunked) saves the per-chunk notes and names the file with the verb's own `--notes-file` command; under `--all` it ends the batch. §11.4 covers the unsaved-file case (failure stays the AI's, one warning with the write error verbatim). §16.1 requires an `r` that becomes a chunked run to narrate as one; §10.3/§16.3 place the lower-cap hint after the notes, once per batch under `--all`. This task is a pure consolidation: no behaviour change except that the `deps.Regenerator` test seam is now narrated on regenerate as it already was on release.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/engine/notesstage.go:177-181 — `(*regenStage).review(regenerator, k)` builds `reviewRegen{regenerator: r.narrating(regenerator), shown: r.shown, keep: k.keepRegen}`; the only non-test `.narrating(` call site (notesstage.go:180)
  - internal/engine/chunknotes.go:84-92 — `(*chunkNotesKeeper).reviewGate(...)` runs the package-level `reviewGate` then `k.reportKept(deps.Presenter, err, command)` on error; the only non-test `.reportKept(` call site (chunknotes.go:89)
  - internal/engine/release.go:607-609 — release builds its `r` through `regens.review(perRunRegenerator(...), chunkNotes)` and runs the gate through `chunkNotes.reviewGate(..., "mint release")`; `perRunRegenerator` (release.go:1340-1350) selects the `deps.Regenerator` override before narration, as before
  - internal/engine/regenerate_write.go:143-147 — `RegenerateWriteRequest` drops `RegenShown`; unexported `regens *regenStage` and `chunkNotes *chunkNotesKeeper` carry the run state; `Regenerator` is now un-narrated
  - internal/engine/regenerate_write.go:250-257 — `reviewFresh` selects the override via `regenerateRegenerator` first, falls back to the as-built gate when either `regens` or `chunkNotes` is nil, otherwise routes through `review` + keeper `reviewGate` with `"mint release regenerate "+req.VersionKey`
  - internal/engine/regenerate_interactive.go:255-267 and internal/engine/regenerate_batch.go:463-479 — `RegenerateRun` and `gatePerVersion` pass the un-narrated regenerator plus their stage and keeper; `--all` builds its stage over `hint.regenWritten` (regenerate_batch.go:336), so an `r` folds into the batch-end hint
- Notes: Measured by reading: `.narrating(` and `.reportKept(` each have exactly one non-test call site, inside the two helpers. The remaining `reviewRegen{` literals are regenerate_write.go:240 (supplied/floor gate, no `r`, unchanged per the plan), regenerate_write.go:253 (the as-built direct-caller path) and notesstage.go:180 (the helper). The release floor path still runs through the helper; its `warnUnsaved` already cleared `saveErr` at release.go:568, so no duplicate warning, same as before. The fallback condition is `regens == nil || chunkNotes == nil` rather than the plan's "neither". Both production callers set both fields, the fields are unexported, and the field comment states the either-nil rule, so the difference changes no reachable behaviour. On regenerate, the `deps.Regenerator` override is now wrapped by `narrating`. A test fake reports no progress, so `r.succeeded` stays nil and nothing extra is emitted. Only the test seam sees this, as the plan intended.

TESTS:
- Status: Adequate
- Coverage:
  - New, internal/engine/regenerate_reviewr_test.go:15 — direct `RegenerateWrite` with only `req.Regenerator` set and a chunk-notes store wired: asserts the one-time context reaches the regenerator, an abort exiting 1 that wraps the failure, no saved file, no next moves, no warnings, no provider write. Its strongest guard is structural: dropping the nil guard in `reviewFresh` would panic inside `narrating`'s closure (`r.began` on a nil stage).
  - New, internal/engine/regenerate_reviewr_test.go:57 — `RegenerateRun` with `deps.Regenerator` wired and a distinct per-run regenerator: the override gets the nudge, the per-run one is never consulted, and the provider receives the override's body.
  - Pre-existing (unchanged by the commit) behaviour proofs for the other criteria: hint after a halved `r` on release/regenerate/`--all` (regen_halve_test.go:399, :415, :431, :460, :492, :536, :575, :590); chunked-`r` narration on all three paths (regen_narration_test.go:151, :352, :387, :400, :412, :423); failed `r` saving and naming with each verb's command, and `--all` ending (release_regenchunkfail_test.go:192-276, release_remergefail_test.go:123-209, regenerate_regenfail_test.go:171-261); unsaved-file warning on both verbs (release_regenchunkfail_test.go:409, release_remergefail_test.go:320, regenerate_regenfail_test.go:317); release override (release_test.go:1857).
- Notes: The commit touched no existing test file, so existing test semantics are untouched by construction. The new tests are focused and add no redundant assertions.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, FakeRunner/RecordingPresenter, WHY-comments true to the as-built code)
- SOLID principles: Good — `review` and the keeper's `reviewGate` each own one half of the `r` assembly
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "Rule: the existing tests pass with their semantics untouched, and all five CLAUDE.md gates pass" — run `go build ./...`, `gofmt -l .`, `go vet ./...`, `go test -race ./...` and `golangci-lint run` at the project root. Reading settled "semantics untouched": the commit (b427ee3) modifies no existing test file.
