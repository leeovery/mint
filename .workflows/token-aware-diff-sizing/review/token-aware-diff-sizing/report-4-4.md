TASK: A SIZE answer on a chunk halves its slice (token-aware-diff-sizing-4-4)

ACCEPTANCE CRITERIA:
- Given a chunked run whose part 3 call, of C bytes, fails with output and is classified SIZE, part 3's slice is re-sent as chunk calls each no bigger than half of C, each carrying the release's instructions, the same whole-release Change Map, its framing, its slice and the closing reminder. Between them they carry every file's diff that part 3 carried, exactly once (§8.2, §10.1)
- Given part 3 classified SIZE while other parts are in flight or not yet started, the wave carries on: the other parts are sent and finish as before (§11.2)
- Given a wave that includes halved parts, never more than four chunk calls are in flight at once, the halved parts included (§8.5, §10.1)
- Given a piece call classified SIZE, its piece is re-cut into smaller pieces by the piece rules, each carrying the file's diff header (§8.4, §10.1)
- Given one of part 3's halved parts also classified SIZE, it is halved in turn, to half its own composed size, in a further round sent after that answer (§10.1, §10.2)
- Given the halved parts and every other part succeed, exactly one merge call follows. It carries every part's notes in chunk order, with part 3's halved parts in part 3's place, and its output ships byte-identical (§8.3, §8.6, §10.1)
- Given every part succeeds and the merge call fails with output and is classified SIZE, no further call is made and the merge is not halved. The ✗ line reads `part notes too large to merge`, with no location, and the merge call's captured output verbatim beneath it (§9.6, §10.1, §11.3, §12.4)
- Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose chunk call is classified SIZE, that version's slice halves the same way (§1.3)

STATUS: complete

SPEC CONTEXT: §10.1 halves only diff-carrying calls (single-pass, chunk, piece). The new target is half the failed call's composed size, the slice is re-cut by the first wave's packing and piece rules, and each call gets whatever room its fixed part leaves. A SIZE answer on the merge call is never halved; it is a failed merge (§11.3) that reads `part notes too large to merge` with no location (§12.4 and its corrigendum), with the merge's captured output verbatim beneath it (§9.6). §11.2: a SIZE answer with halving left does not end the wave. §8.5: at most four chunk calls in flight. §10.2: halving rounds are serial; the three-halving bound and the no-room round belong to task 4-7. Corrigendum on §16.1: parts are numbered by their place in chunk order, and a halved part's new parts take its place. §1.3: regenerate's fresh source and `--all` share the ladder.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/chunk.go:209-241 — `wave.send`: a failed call is classified (`readFailure`, :213). On SIZE with halving left (:217), the slice is re-cut at `len(prompt)/2`, half the call's whole composed prompt (:219). The halve case (:234-235) does not set `w.failure`, so the wave carries on.
  - internal/notes/chunk.go:253-275 — `wave.halve`: replaces the slot with its halves in place in chunk order (`slices.Replace`, :260) and appends them to the queue behind the parts not yet sent (:261), so a further round goes only after that answer. It renumbers the pieces of a re-cut piece's file by place (:263-274).
  - internal/notes/chunk.go:195-197 — `canSend` gates every send, halved parts included, on `inFlight < maxChunkCallsInFlight` (4). `inFlight` is released only after the call's classify answer has been recorded (:225).
  - internal/notes/chunk.go:408-415 — `cutter.recut`: a whole-files chunk is re-cut through `cutParts` (packing plus piece rules). A piece is re-cut through `cutPieces` under the file's header, so every new piece carries the header (:460).
  - internal/notes/chunk.go:367-371 — `fixedPart` uses framing numbered at `numberBound` (the diff's byte length), so a re-cut call stays within the target wherever its place in the grown run lands.
  - internal/notes/chunk.go:199-202 — the prompt is composed at send time from the run's instructions, the same Change Map, framing numbered by current place, the slice and `OutputReminder`.
  - internal/notes/chunk.go:57-66, :187-192 — notes come back in slot order, halved parts in their slice's place, and go into exactly one `merge` call whose body is returned untouched.
  - internal/notes/chunk.go:82-91 — `sendMerge`: on a SIZE classification it returns `ErrMergeTooLarge` wrapping the call's own error. There is no `callError`, so no location, and no halving. internal/notes/resolve.go:173-174 renders the bare phrase.
  - internal/notes/generate.go:150-173, internal/engine/regenerate_fresh.go:59 and cmd/mint/regenerate_all.go:117 — regenerate's fresh source and every `--all` version run through the same `generateFromDiffWithContext` → `generateChunked` → wave path.
- Notes: A halved part's slices partition the parent slice. Whole files are re-split by `splitFileDiffs`, and piece bodies are cut once each with the header repeated by design. Every file's diff is therefore carried exactly once. The earlier race on `slot.part` (fix round 1) is closed: `send` takes the part as copied under the lock (:181, :209). Nothing reads `slot.part` outside the mutex, and `slot.halvings` is immutable once the slot is made.

TESTS:
- Status: Adequate
- Coverage (by criterion):
  - AC1: internal/notes/halveparts_test.go:282 checks that the call names part 3 of 6, that each halved call is at most half the failed call, and that each opens with instructions plus the Change Map (with `context` set), ends with the reminder and carries part framing. It also checks that the slices, joined in part order, equal part 3's files byte for byte, which proves every file is carried exactly once.
  - AC2: :325 holds parts in flight while part 3 overflows. It checks that every other area is sent exactly once and that the merge body ships.
  - AC3: :361 checks that the transport-observed maximum in flight is 4. A halving that bypassed the queue would push the held calls to 5 or more and fail the test.
  - AC4: :383 checks smaller pieces, each opening with the header, each at most half the failed piece. It also checks the bodies rejoin the failed piece byte for byte and the pieces are renumbered to the grown total. The race regression is at :437 (two pieces of one file classified SIZE).
  - AC5: :482 checks a second-round halving at half the halved part's own size, every further-round call sent after that part's classify answer (event order), and the slices rejoining the halved part.
  - AC6: :529 (two subtests, one and two levels of halving) checks exactly one merge carrying every successful chunk call's notes, with parts 1, 2 and 4-6 in place and part 3's halves between them in order. It also checks section numbering and a byte-identical body that has trailing whitespace.
  - AC7: :592 (notes) checks that the events end `merge`, `classify` with no further call, the CauseText has no location, `errors.Is ErrMergeTooLarge` holds and the output is verbatim. internal/engine/notes_halveparts_test.go:111 adds the rendered StageFailure message and Output plus the AI call count.
  - AC8: internal/engine/notes_halveparts_test.go:148 (fresh source, which also asserts the version's own range diff) and :167 (`RegenerateAll` with the real `RegenerateFreshBody` producer for one version).
  - Forward release end-to-end: internal/engine/notes_halveparts_test.go:88 (tag annotation is the merge output).
- Notes: The tests are focused, with one or two per criterion and no redundant variants. Each would fail if its behaviour broke.

CODE QUALITY:
- Project conventions: Followed. External test packages, `t.Parallel()`, a fake transport, no real subprocesses; sentinels wrapped with `%w`; failure output carried verbatim through `ai.GenerationError`.
- SOLID principles: Good. `cutter` owns cutting, `wave` owns scheduling and outcomes, and `Generator` owns merge and classification.
- Complexity: Acceptable. `send`'s outcome switch is ordered so that an ended wave swallows later failures before halving is considered.
- Modern idioms: Yes (`slices.Replace`, `slices.Index`, `slices.Clone`, `%w: %w` double wrap, `min`).
- Readability: Good. The WHY-comments on `wave`, `run`, `send`, `halve` and `waveSlot.halvings` hold true against the code.
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
