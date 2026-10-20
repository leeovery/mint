package notes

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// SaveChunkNotes keeps a failed chunked run's finished chunk notes and returns the
// path of the file it kept them in.
type SaveChunkNotes func(notes string) (path string, err error)

// WithChunkNotes returns a copy of g that saves a failed chunked run's finished chunk
// notes through save; g itself is unchanged. A nil save keeps nothing.
func (g *Generator) WithChunkNotes(save SaveChunkNotes) *Generator {
	c := *g
	c.saveChunkNotes = save
	return &c
}

// SavedChunkNotesPath is the path a failed chunked run's finished chunk notes were
// saved to, or "" when failure names no saved file.
func SavedChunkNotesPath(failure error) string {
	var saved *chunkNotesSavedError
	if errors.As(failure, &saved) {
		return saved.path
	}
	return ""
}

// chunkNotesSavedError is a chunked run's failure whose finished chunk notes were
// saved to path. It reads as the failure itself.
type chunkNotesSavedError struct {
	path string
	err  error
}

func (e *chunkNotesSavedError) Error() string { return e.err.Error() }

func (e *chunkNotesSavedError) Unwrap() error { return e.err }

// keepChunkNotes saves the finished parts' notes of a run that failed with failure,
// in chunk order, and returns failure naming the saved file. A cancel, a run with no
// finished part and a failed save keep nothing and return failure as it is.
func (g *Generator) keepChunkNotes(failure error, partNotes []string) error {
	finished := slices.DeleteFunc(slices.Clone(partNotes), func(notes string) bool { return notes == "" })
	if g.saveChunkNotes == nil || len(finished) == 0 || errors.Is(failure, context.Canceled) {
		return failure
	}
	path, err := g.saveChunkNotes(joinChunkNotes(finished))
	if err != nil {
		return failure
	}
	return &chunkNotesSavedError{path: path, err: failure}
}

// joinChunkNotes keeps each part's notes byte for byte, with one blank line between
// one part's notes and the next.
func joinChunkNotes(partNotes []string) string {
	var b strings.Builder
	for i, notes := range partNotes {
		if i > 0 {
			if !strings.HasSuffix(partNotes[i-1], "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		b.WriteString(notes)
	}
	return b.String()
}
