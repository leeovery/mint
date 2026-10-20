package notes

import "errors"

// ErrDiffTooLarge is the sentinel for an input over its size limit, matched with
// errors.Is.
var ErrDiffTooLarge = errors.New("diff too large")

var (
	// ErrNoRoomForDiff reports a chunked run's call left unsent, because it has no
	// room under its bound for the diff it must carry.
	ErrNoRoomForDiff = errors.New("no room for the diff under max_prompt_bytes")
	// ErrMergeTooLarge reports a merge prompt over max_prompt_bytes, left unsent, or a
	// merge call whose failure is classified SIZE.
	ErrMergeTooLarge = errors.New("part notes too large to merge")
)
