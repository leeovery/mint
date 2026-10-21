TASK: Calls that cannot be made (token-aware-diff-sizing-2-9, tick-dc97ae)

ACCEPTANCE CRITERIA:
- Given a release whose single-pass prompt is over the cap, with a `[release].context` or `[release].prompt` override file large enough that a chunk call's fixed part leaves no room for diff, no chunk, merge or single-pass call is sent. The fixed part is the instructions, the condensed Change Map, the framing and the closing reminder (§8.3, §8.7)
- Given that release under `on_notes_failure = abort`, the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§8.7, §12.4)
- Given neither `[release].prompt` nor `[release].context` set and a `max_prompt_bytes` below the fixed part of mint's own instructions, no chunk, merge or single-pass call is sent. Under `on_notes_failure = abort` the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§4.1, §8.7, §12.4, Corrigendum 2026-10-19 on §8.7)
- Given chunk calls that succeed with notes whose merge prompt would exceed the cap, no merge call is sent. The merge prompt is the instructions, the Change Map, every chunk's notes, the framing and the closing reminder. Under `abort` the run stops before the tag with the ✗ line `part notes too large to merge` and no AI output beneath it (§8.7, §11.3, §12.4)
- Given either failure under `on_notes_failure = fallback`, the fallback body ships, as on a failed single-pass call (§12.1)
- Given either failure on `mint release regenerate <version>` from a fresh source, the regenerate aborts with the same ✗ line. Under `regenerate --all`, that version is skipped and the batch carries on (§12.3)

STATUS: complete

SPEC CONTEXT: §4.1 makes "no call mint makes is ever bigger than the cap" absolute, and any positive cap loads. §8.7 (with its 2026-10-19 corrigendum) names three no-room causes — outsized operator text, a cap below mint's own fixed part with neither key set, a halving round below the fixed part (Phase 4) — plus a merge prompt over the cap; mint sends no call and never trims operator text. §7.2 sends a single-pass prompt at or under the cap whatever its instructions' size, so no-room only arises once chunked. §11.3 counts an over-cap merge as a failed merge (chunk notes saved). §12.4 fixes the two phrases, and its suffix corrigendum makes both bare (no location). §12.1/§12.3 route them like any notes failure: abort/fallback on release, abort on single regenerate, skip-and-continue on --all.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/notes/size.go:12 (ErrNoRoomForDiff, phrase "no room for the diff under max_prompt_bytes"), internal/notes/size.go:15 (ErrMergeTooLarge, phrase "part notes too large to merge")
  - internal/notes/generate.go:227-230 — over-cap single-pass goes to generateChunked; no single-pass call is made on that branch
  - internal/notes/chunk.go:28-35 — cutRun runs before any call or progress report; a cut error returns immediately, so no chunk/merge call is sent
  - internal/notes/chunk.go:369-371 (fixedPart = instructions + Change Map + framing numbered at the bound + closing reminder, no slice), :377-381 (chunk room <= 0 → ErrNoRoomForDiff), :439-444 (piece room after header <= 0 → ErrNoRoomForDiff)
  - internal/notes/chunk.go:68-72 — merge prompt measured before g.progress.Merging and before sending; over the cap → ErrMergeTooLarge; writeParts (:57-66) routes it through keepChunkNotes so finished chunk notes are kept (§11.3)
  - internal/notes/chunk.go:562-568 — merge prompt composition: instructions, Change Map, framing, every chunk's notes, closing reminder
  - internal/notes/resolve.go:142-145 (ClassifyFailure: both sentinels checked first), :171-174 (CauseText returns both phrases bare, no location suffix, per the §12.4 corrigendum)
  - internal/engine/release.go:2045-2055 (failureMessage via notes.CauseText) and :2009-2015 (notesFailureOutput returns "" with no *ai.GenerationError in the chain — so no output beneath the ✗ line)
  - internal/engine/regenerate_batch.go:386-392 (notesSkipCause names the cause phrase for --all skips)
- Notes: The Change Map passed into the chunked run is the one BuildChangeMapForRange condensed against cfg.MaxPromptBytes (generate.go:215), so the fixed part is measured with the condensed map as AC1 requires. Halving's no-room case is correctly kept distinct: wave.send and generateHalved wrap ErrTooLargeForModel rather than surfacing ErrNoRoomForDiff (chunk.go:41-45, :236-237), so ClassifyFailure's ordering cannot misattribute it. No drift from the plan or spec.

TESTS:
- Status: Adequate
- Coverage:
  - internal/notes/chunkcannot_test.go:34-69 — context file and prompt override each leave no room; fixture guard asserts both single-pass prompt and fixed part are over the cap; asserts ErrNoRoomForDiff and zero transport calls (AC1)
  - internal/notes/chunkcannot_test.go:71-86 — cap at half of DefaultPrompt with neither key set: ErrNoRoomForDiff, zero calls (AC3)
  - internal/notes/chunkcannot_test.go:88-104 and :181-201 — no progress reported for an unsent run, no merging reported for an unsent merge
  - internal/notes/chunkcannot_test.go:130-146 — both chunk calls sent, zero merge calls, ErrMergeTooLarge (AC4); :148-179 — boundary: a merge prompt exactly at the cap is sent
  - internal/notes/causetext_test.go:29-30 and :181-208 — both phrases from CauseText, including bare (no location) from real chunked-run errors
  - internal/notes/classifyfailure_test.go:21-22, :54-55 — classification of both sentinels
  - internal/engine/notes_cannotcall_test.go:89-117 — release under abort, four cases (context, prompt override, cap below instructions, merge over cap): AbortError, no tag/push/release mutation, exact notes-call count, ✗ Message equals the phrase, Output empty (AC2, AC3, AC4)
  - internal/engine/notes_cannotcall_test.go:119-146 — same four cases under fallback ship the commit-subject list as the tag body (AC5)
  - internal/engine/notes_cannotcall_test.go:203-240 — fresh single-version regenerate aborts with the phrase, no output, no provider write; :242-278 — --all skips v2 naming the phrase, writes v1 and v3 (AC6)
- Notes: Each test would fail if its behaviour broke (a sent merge raises the call count to 3; a trimmed or truncated path would send calls; a location suffix would break the exact-Message match). The regenerate cases omit the prompt-override variant, but it shares the context case's code path through ResolveInstructions/cutParts; no gap worth reporting. Not over-tested: the release abort/fallback and regenerate single/all tables each pin a distinct routing.

CODE QUALITY:
- Project conventions: Followed — sentinel errors matched with errors.Is, lowercase messages, external test packages, t.Parallel throughout, FakeRunner/RecordingPresenter doubles, exact ✗ Message assertions, behaviour-level call-count proofs
- SOLID principles: Good — the no-room decision lives in the cutter, the merge-size decision in merge, phrase mapping in CauseText; the engine only renders
- Complexity: Low
- Modern idioms: Yes
- Readability: Good — doc comments on generateChunked, cutParts, cutPieces, the sentinels and CauseText hold true against the code
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
