## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/notes/chunk.go:271-277 (packPieceBodies). When a piece's body room is smaller than the first UTF-8 character left in a line, characterBoundary returns 0. The `_, end = utf8.DecodeRuneInString(segment)` fallback then emits a body longer than room, so mint sends a piece prompt up to 3 bytes over max_prompt_bytes. That breaks §4.1 ("No call mint makes is ever bigger than the cap") and the task's Outcome. It also makes two comments this diff added false: chunk.go:21-23 ("No call over maxPromptBytes is sent") and chunk.go:167-168 ("a run either fits every call under maxPromptBytes or sends none").
  Reproduced on a scratch copy of the repo (since deleted): a file of `+é\n` lines, cutParts with DefaultPrompt and a cap of pieceFixed + len(header) + 1. It returned 164 parts with a nil error, and 22 of those piece prompts were over the 3452-byte cap.
  This is the same class of failure as the header-fills-room branch the executor converted at chunk.go:232-234, and it sits in the cutting step this task made the gate for "send no call". It is reachable only when a piece's body room is 1 to 3 bytes and the file has non-ASCII text, so it is narrow. Nothing outside the code notices it: no error, no warning, just an over-cap call.
  FIX: Have packPieceBodies return ([]string, error). In the `end == 0` branch, return `nil, ErrNoRoomForDiff` when the segment starts with a valid character longer than room: `r, size := utf8.DecodeRuneInString(segment); if r != utf8.RuneError || size > 1 { return nil, ErrNoRoomForDiff }`. For invalid bytes, keep the existing 1-byte step (DecodeRune returns RuneError, 1). This keeps progress on invalid UTF-8 and never exceeds room. Propagate the error through cutPieces, which already returns an error. Add an internal test to /Users/leeovery/Code/mint/internal/notes/chunk_internal_test.go that passes cutParts the reproduction above (`+é\n` lines, one byte of piece body room) and asserts ErrNoRoomForDiff.
  ALTERNATIVE: In cutPieces, change the guard to `if bodyRoom < utf8.UTFMax { return nil, ErrNoRoomForDiff }`. It is a one-line change and the test does not depend on content. The cost is that it also rejects ASCII-only files whose piece room is 1 to 3 bytes, which could be cut under the cap today, though that would mean thousands of calls per file. I recommend the content-exact fix, because it changes nothing for any input that can actually be cut under the cap.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/size.go:10-12 — the cause it names is incomplete. cutPieces also returns this sentinel when a piece's repeated file header fills the room left by the instructions, Change Map, framing and reminder, and in that case those four do leave room.
  OLD: 	// ErrNoRoomForDiff reports a chunked run sending no call at all: a chunk call's
	// instructions, Change Map, framing and closing reminder leave no room for diff
	// under max_prompt_bytes.
  NEW: 	// ErrNoRoomForDiff reports a chunked run sending no call at all, because one of
	// its calls has no room under max_prompt_bytes for the diff it must carry.

NOTES:
- Fallback as built ships the commit-subject list with no warning. The fallback tests assert only the shipped body, which matches AC5's "as on a failed single-pass call". §12.1's cause-phrase warning belongs to a later phase.
- The `--all` test pins the as-built generic skip reason "(notes generation failed)". That matches the task context, which leaves naming the real cause phrase to Phase 5.
- ErrMergeTooLarge is returned without the partNotes. Phase 4's saved chunk-notes file (§11.4) will need generateChunked to keep them on this branch. That is expected follow-on work, not a problem now.
- For background I read the 2-7 fix-tracking file. It flagged the header-fills-room over-cap case, which this task fixes, and did not cover the case smaller than one character. I did not treat it as review input. The dispatch carried nothing beyond its enumerated inputs.
