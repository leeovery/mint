package notes

import "errors"

// ErrDiffTooLarge is the sentinel for an input over its size limit, matched with
// errors.Is.
var ErrDiffTooLarge = errors.New("diff too large")
