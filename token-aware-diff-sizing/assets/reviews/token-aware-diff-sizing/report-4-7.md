TASK: Halving runs out (token-aware-diff-sizing-4-7)

ACCEPTANCE CRITERIA:
- Given a single-pass call of S bytes classified SIZE, then one of its halved parts classified SIZE, then one of that part's halved parts classified SIZE, the third round's calls are each no bigger than an eighth of S. Given one of those is classified SIZE too, no further re-cut and no merge call is made, and the notes fail (§10.2, §11.1)
- Given a chunked run of 6 parts whose part 3 is classified SIZE, where each re-cut yields 2 parts and the first of them is classified SIZE again, the slice is halved three times and the fourth SIZE fails it. No merge call is made, and the ✗ line reads `too large for the model, even in smaller parts (part 3 of 9)` with that call's captured output verbatim beneath it (§10.2, §11.1, §12.4, §16.1)
- Given a chunk call of 200,000 bytes whose fixed part is 120,000 bytes, classified SIZE, its halved target of 100,000 bytes leaves no room for diff. No call is sent for that round, and the ✗ line reads `too large for the model, even in smaller parts` with that part's location and its call's captured output verbatim beneath it (§8.7, §10.1, §12.4)
- Given a single-pass call of 200,000 bytes whose instructions, Change Map and closing reminder take 120,000 bytes, classified SIZE, its first halving round has no room. No chunk call is sent, and the ✗ line reads `too large for the model, even in smaller parts` with no location and the single-pass call's captured output verbatim beneath it (§8.7, §10.1, §12.4)
- Given halving runs out on an unattended release under `on_notes_failure = abort`, the run stops before the tag. Under `fallback`, the commit-subject list ships (§12.1)
- Given halving runs out on an unattended `mint release regenerate <version>` from a fresh source, it stops with the same ✗ line and writes nothing. Under `regenerate --all`, that version is skipped and the batch carries on (§12.3)

STATUS: complete

SPEC CONTEXT: §10.2 bounds halving at three rounds per slice, counted from the slice's first SIZE answer (so a slice reaches an eighth of the size that first failed); a fourth SIZE fails the slice and so the generation, with no merge (§11.1), and it is a final failure that ends the wave (§11.2). §8.7/§10.1: a halving round aimed below its calls' fixed part has no room and sends no call. §12.4 (and its 2026-10-19 corrigendum): both render `too large for the model, even in smaller parts`; the suffix names the call the failure belongs to — the part whose SIZE answer it was — and a single-pass call's first halving round with no room takes no location. The failure routes like any notes failure: `on_notes_failure` on release (§12.1), the floor/stop on single-version regenerate and skip-and-continue on `--all` (§12.3).

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunk.go:110-112 — `maxHalvings = 3`
  - internal/notes/chunk.go:37-48 — `generateHalved`: the single-pass SIZE counts as the first halving (wave built with `halvings` 1, line 47); a first round with no room returns `ErrTooLargeForModel` wrapping the single-pass call's own failure, no `callError`, so no location (line 44)
  - internal/notes/chunk.go:139-145, 147-155, 253-257 — `waveSlot.halvings`, seeded per wave and carried as `slot.halvings + 1` into every part a halved slice becomes
  - internal/notes/chunk.go:209-241 — `send`: halving only while `slot.halvings < maxHalvings` (line 217); an exhausted bound or a re-cut with no room (`recutErr != nil`) both fall to line 236-237, setting the wave's failure to `partCallError(...)` over `ErrTooLargeForModel` wrapping the call's own error, which ends the wave (no merge, `writeParts` line 58-65 skips the merge on error)
  - internal/notes/generate.go:237-240 — the single-pass SIZE path hands its failure into `generateHalved`
  - internal/notes/size.go:16-19 — the `ErrTooLargeForModel` sentinel carrying the exact phrase
  - internal/notes/resolve.go:146-147, 175-176, 184-187 — `ClassifyFailure` puts the sentinel ahead of `ai.ErrGenerationFailed`; `CauseText` appends `callError.location` only when one is in the chain
  - internal/notes/resolve.go:141-147 — `ErrNoRoomForDiff` is checked first, but the halving no-room paths never put it in the chain (both wrap the call's failure, not the cut error), so they read as the size phrase, not `no room for the diff under max_prompt_bytes`
  - internal/engine/notesfloor.go:96-101 — the cause gets the size moves (lower cap, lean command, diff_exclude); engine routing/rendering (`failureMessage`, `notesFailureOutput` in internal/engine/release.go) is generic over the cause
- Notes: Reading the bound arithmetic: each round's target is `len(prompt)/2` of the call that failed, and `cutParts`/`cutPieces` size room against the bound-numbered fixed part, so each call is at most its target. Three rounds from S therefore give calls of at most S/8, as AC1 requires. A failure landing after the wave has ended is neither classified nor halved (lines 212, 232-233), as §11.2 requires. No drift from the plan.

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/notes/halvebound_test.go:38 — counts 1 single-pass, 3 overflowed chunk calls and 4 classify calls; checks that every third-round call is at most failedSize/8; checks that the fourth SIZE yields no re-cut and no merge, that the output is verbatim, and that the phrase carries the location
  - AC2: internal/notes/halvebound_test.go:75 — part 3 of a 6-part run overflows through three re-cuts. Each overflow is pinned at part 3 of a growing total, with no total past 12, no merge, verbatim output and the phrase `(part 3 of 12)`
  - AC3: internal/notes/halvebound_test.go:120 — a fixture guard checks that the halved target is below the call's fixed part; also checks that exactly 3 calls are sent with no re-cut and no merge, that the phrase is `(part 2 of 3)`, and that the output is verbatim
  - AC4: internal/notes/halvebound_test.go:145 — a fixture guard on the fixed part; checks 1 single-pass call, 1 classify call, 0 chunk calls and 0 merges, the bare phrase and verbatim output
  - Sentinel precedence over the wrapped generation failure: internal/notes/halvebound_test.go:169
  - AC5: internal/engine/notes_halvebound_test.go:120 (abort: non-zero, no mutation, StageFailure message and Output verbatim) and :153 (fallback ships the subjects as the tag body). Each runs two cases: three halvings run out, and the first halving round has no room. `newDeps` sets neither Yes nor StdinInteractive, so both runs are unattended
  - AC6: internal/engine/notes_halvebound_test.go:182 (single-version fresh regenerate: same ✗ line and output, no provider write) and :218 (`--all`: v1.1.0 skipped with phrase and output, v1/v3 written)
- Notes: AC2's fixture yields 3-part re-cuts, so the test pins `(part 3 of 12)` rather than the criterion's `(part 3 of 9)`. A 2-part re-cut is not reachable with a non-zero fixed part on a single-file slice, and the test checks the same behaviour: three halvings, the fourth SIZE fails, and the location is numbered within the grown total. No coverage is lost. The tests are focused, with no redundancy worth reporting.

CODE QUALITY:
- Project conventions: Followed. Sentinels are matched with `errors.Is`/`errors.As` and wrapped with `%w`, and messages are lowercase. AI output travels verbatim through `GenerationError`. Tests are external packages with `t.Parallel()` and fake transports/runners, and pin exact rendered messages.
- SOLID principles: Good
- Complexity: Acceptable (`send`'s outcome switch is ordered and each arm is single-purpose)
- Modern idioms: Yes (`slices.Replace`/`Index`/`Clone`, multi-`%w`)
- Readability: Good. The WHY-comments on `generateHalved`, `wave.run`, `waveSlot.halvings`, `maxHalvings`, `ErrTooLargeForModel` and `CauseText` hold true against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
