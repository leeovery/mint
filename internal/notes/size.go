package notes

import "errors"

// ErrDiffTooLarge is the sentinel for an input over its size limit, matched with
// errors.Is.
var ErrDiffTooLarge = errors.New("diff too large")

var (
	// ErrNoRoomForDiff reports a chunked run sending no call at all, because one of
	// its calls has no room under max_prompt_bytes for the diff it must carry.
	ErrNoRoomForDiff = errors.New("no room for the diff under max_prompt_bytes")
	// ErrMergeTooLarge reports a merge prompt over max_prompt_bytes, left unsent.
	ErrMergeTooLarge = errors.New("part notes too large to merge")
)
