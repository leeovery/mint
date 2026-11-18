TASK: One notes-failure cause classification for the ✗ phrase and the next moves (token-aware-diff-sizing-5-14, tick-17f1a0)

ACCEPTANCE CRITERIA:
- Each of the six causes renders today's ✗ phrase, single-pass and chunked, with today's location suffix or none: the existing CauseText tests (internal/notes/causetext_test.go, halvebound_test.go, halveparts_test.go, waveend_test.go, savenotes_test.go) pass unchanged.
- An unattended release or `regenerate <version>` stopped at the floor prints today's next moves for each cause, byte for byte: the existing tests in internal/engine/notes_floor_causemoves_test.go, notes_floor_sizemoves_test.go, notes_floor_nextmoves_test.go and regenerate_floor_nextmoves_test.go pass unchanged.
- A size failure carrying the call's own generation failure keeps the size cause: ErrTooLargeForModel over an ai.GenerationError reads `too large for the model, even in smaller parts` and takes the three size moves, and a merge too large on a repo's own ai_command takes only the Always moves, never the lean-command pointer an OTHER failure gets.
- An error none of the six sentinels match (e.g. notes.ErrDiffTooLarge) is not a notes failure: CauseText reports ("", false), the run prints no next moves, and an --all skip reads `notes generation failed` with no output beneath it.
- The sentinel precedence lives in one switch: `rg -n 'errors\.Is\(\w+, (notes|ai)\.Err' internal/engine -g '!*_test.go'` matches nothing.

STATUS: complete

SPEC CONTEXT: Spec §12.4 fixes the ✗ phrase per cause (timeout, missing binary, SIZE, OTHER with/without output, no room, merge too large), the location suffix rules (none for no-room and merge-too-large), and the cause-fitted next moves (Size: three moves; no room: shorten prompt/context or raise cap; timeout; missing binary; OTHER: lean-command pointer only when ai_command is not the shipped default; merge too large: Always moves only). §9.3 anticipates a later additive RATE_LIMIT code, which is the divergence risk this refactor removes.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/resolve.go:124-134 — closed `Cause` type, zero value none, six constants
  - internal/notes/resolve.go:140-156 — `ClassifyFailure`, the single ordered switch (NoRoom, MergeTooLarge, TooLargeForModel, ai.ErrTimeout, ai.ErrCommandMissing, ai.ErrGenerationFailed), (0, false) otherwise
  - internal/notes/resolve.go:164-189 — `CauseText` classifies, then maps the Cause; no-room and merge-too-large return without location, the other four take the callError suffix, generation failure worded by generationFailurePhrase
  - internal/engine/notesfloor.go:32-38 — `reachesNotesFloor` keeps context.Canceled first, then the classifier's known bit
  - internal/engine/notesfloor.go:86-112 — `causeMoves` switches on notes.Cause; merge-too-large returns nil; generation-failure arm keeps the `cfg.AICommandFor(config.VerbRelease) != config.DefaultAICommand` condition
  - internal/engine/regenerate_batch.go:386-392 — `notesSkipCause` takes its known/unknown branch from the classifier, phrase from CauseText
  - internal/engine/release.go:2050-2053 (`failureMessage`) and internal/notes/resolve.go:203-208 (`causeText`) stay on CauseText as directed
- Notes: Read side by side against the task commit's pre-image, every path is behaviour-equivalent. CauseText is known exactly when ClassifyFailure is known; the switch arms and early returns are preserved one for one; causeMoves' generation-failure arm on the shipped default falls through to `return nil` as the old compound case did; notesSkipCause's inverted branch returns the same pairs. The engine-side sentinel search (`errors.Is(<ident>, (notes|ai).Err` in non-test files under internal/engine) finds no matches, and the `ai` import was dropped from notesfloor.go. Comments on the changed code (resolve.go:124, :136-139, :158-163, :199-202; notesfloor.go:28-31, :81-85; release.go:2033-2038; internal/ai/transport.go:76-78) still hold against the code.

TESTS:
- Status: Adequate
- Coverage: New internal/notes/classifyfailure_test.go pins each sentinel's Cause through a %w wrap, size precedence over the call's own ai.GenerationError (the order the task calls load-bearing — swapping GenerationFailed above TooLargeForModel would fail it), and the not-known results for a plain error, ErrDiffTooLarge and context.Canceled. Byte-identical rendering is held by the existing engine and notes tests, which the task commit left untouched: notes_floor_sizemoves_test.go:34 (halving runs out → phrase plus the three size moves), notes_floor_causemoves_test.go:189 (merge too large on `llm -m gpt-4o` → Always moves only), notes_floor_causemoves_test.go:108/:144 (OTHER with shipped vs own command), causetext_test.go:50 (unmapped → ("", false)), notes_failure_no_line_ceiling_test.go:90 (ErrDiffTooLarge --all skip reads `notes generation failed`).
- Notes: The task is a pure refactor, so the existing tests are the main check and the new tests stay focused on the classifier. Nothing is redundant enough to count as over-testing.

CODE QUALITY:
- Project conventions: Followed (external test package, t.Parallel, table-driven, errors.Is through the %w chain, sentinel display unchanged)
- SOLID principles: Good — precedence has a single owner in notes; engine readers consume the classification
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- "the existing CauseText tests (internal/notes/causetext_test.go, halvebound_test.go, halveparts_test.go, waveend_test.go, savenotes_test.go) pass unchanged" — reading settled "unchanged" (the task commit touched none of them) and behaviour equivalence. Whether they pass needs a run: `go test -race ./internal/notes/...`
- "the existing tests in internal/engine/notes_floor_causemoves_test.go, notes_floor_sizemoves_test.go, notes_floor_nextmoves_test.go and regenerate_floor_nextmoves_test.go pass unchanged" — reading settled "unchanged" and behaviour equivalence. Whether they pass needs a run: `go test -race ./internal/engine/...`
