TASK: Suggest a lower cap after halving (token-aware-diff-sizing-4-6, tick-54f589)

ACCEPTANCE CRITERIA:
- Given an attended release whose single-pass call is classified SIZE and whose halved parts all succeed, the largest of them 612,345 bytes, mint shows the hint after the notes and before the review gate. It says the cap is set higher than this model accepts and shows `max_prompt_bytes = 612345` (§10.3, §16.3)
- Given a chunked run whose parts of 1,400,000 and 1,260,000 bytes succeed, and whose third part is classified SIZE and re-cut into parts that succeed, the hint shows `max_prompt_bytes = 1400000`: the largest diff-carrying call that succeeded, halved or not (§10.3)
- Given a run whose merge call is bigger than every chunk and piece call, the hint names the largest chunk or piece call. Neither the merge call nor a classify call ever sets the value (§10.3)
- The value is the call's exact byte count with no separators, rounding or units: a call of 1,260,001 bytes is written `max_prompt_bytes = 1260001`, never `1.3 MB` or `1260000` (§10.3)
- Given a run whose notes were written with no halving, no hint shows (§10.3)
- Given a run that halved but whose notes then failed, as when halving runs out or another part fails for good, no hint shows (§10.3)
- Given `mint release regenerate <version>` from a fresh source whose notes were written after halving, the hint shows after its notes and before regenerate's review gate (§1.3, §10.3)

STATUS: complete

SPEC CONTEXT: §10.3 (plus its 2026-10-19 corrigenda): when the notes were written after halving, mint says the cap is set higher than the model accepts and suggests the exact byte count of the largest succeeded diff-carrying call (single-pass, chunk, piece; never merge, classify or exclusion-check) as `max_prompt_bytes = N`, never rounded. Halving that ran out gets no hint (the run goes to the floor). §16.3: the hint is a follow-up printed after the notes, ahead of the exclusion suggestions. §1.3: regenerate's fresh source gets halving and therefore the hint; `--all` defers it to the batch end (task 4-14, not this task).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/generate.go:39-41 — `Progress.CallSucceeded(promptBytes)`, documented as single-pass/chunk/piece only
  - internal/notes/generate.go:234 — single-pass success reports `len(prompt)`
  - internal/notes/chunk.go:230 — wave (chunk and piece) success reports `len(prompt)`, under the wave mutex
  - internal/notes/chunk.go:78 — `Merged` only after the merge returns the body; merge (chunk.go:82-91), classify (classify.go:53-70) and exclusion check (exclusioncheck.go, which takes only a CheckNarration) never report CallSucceeded
  - internal/engine/notesstage.go:65-80 — `Halved` sets `halved`, `Merged` sets `parts`, `CallSucceeded` keeps the max
  - internal/engine/notesstage.go:143-152 — `lowerCap`/`suggestedCap`: non-zero only when halved AND a merge wrote the notes
  - internal/engine/notesstage.go:272-280 — `hintLowerCap`: `strconv.Itoa`, one presenter Warn, no-op on 0
  - internal/engine/release.go:591-593 — ShowNotes, then hint, then exclusion suggestions, then (dry-run boundary / review gate)
  - internal/engine/regenerate_interactive.go:264 + internal/engine/regenerate_write.go:229-230 — single-version regenerate passes `stage.lowerCap()` and `gateRegenerate` prints it right after ShowNotes, before the gate; the batch's `gateRegenerate` call (regenerate_batch.go:467) leaves LowerCap 0, consistent with 4-14
- Notes: The "notes were written" gate is `mergedParts != 0`, which holds because a halved run always ends in a merge and `Merged` fires only on a successful merge; every failure route (halving exhausted, a part failing for good, a failed merge, fallback, attended floor body) leaves `parts` 0, so the floor/fallback body paths at release.go:591 and RegenerateWriteRequest.LowerCap both resolve to no hint. Nothing in resolveBody can fail after a successful generation, so no written-then-lost case reaches the hint. All output goes through the presenter (seam 3). No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - C1: internal/engine/notes_lowercap_test.go:73 — single-pass SIZE halved; `assertLowerCapHint` (lines 40-56) pins exact message, exactly one hint, ShowNotes immediately before and the ReleaseReviewGate prompt immediately after
  - C2: notes_lowercap_test.go:92 — chunk halved, with a fixture check that the unhalved part is the largest; plus internal/notes/succeededcalls_test.go:69 and :90 (chunk and piece calls reported exactly)
  - C3: notes_lowercap_test.go:153 — merge bigger than every part (fixture-checked), hint names the largest part; classify exclusion is proved by internal/notes/succeededcalls_test.go:55, which asserts the reported sizes equal exactly the halved chunk calls (any classify, merge or failed-call report would break it)
  - C4: internal/engine/notes_lowercap_internal_test.go:10 — 1,260,001 bytes rendered `max_prompt_bytes = 1260001`
  - C5: notes_lowercap_test.go:177 — single-pass and chunked runs with no halving, classify count 0 fixture-checked
  - C6: notes_lowercap_test.go:239 — halving runs out and failed merge under fallback, with a fixture check that a diff-carrying call did succeed (so the test would catch a gate on `halved` alone); notes_lowercap_internal_test.go:47 covers halved-without-merge directly
  - C7: notes_lowercap_test.go:280 — fresh single-version regenerate, hint between notes and NotesReviewGate
- Notes: The release tests run with zero-valued deps (non-TTY, so not "attended" by `attended()`), while C1 names an attended release. The hint code has no attended branch and nothing prints between the hint and the gate on an attended run (suggestions are collected only when unattended), so this is not a gap that would hide a defect. No redundant or implementation-detail tests.

CODE QUALITY:
- Project conventions: Followed — presenter-only output, external test packages plus a focused internal test, t.Parallel throughout, exact rendered-payload assertions
- SOLID principles: Good — notes package reports facts through Progress; the engine decides and renders
- Complexity: Low
- Modern idioms: Yes (builtin `max`, `strconv.Itoa`)
- Readability: Good; comments at notesstage.go:24, :141-142, :270-271 and regenerate_write.go:141 hold against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
