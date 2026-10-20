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
// in chunk order, and returns failure naming the saved file. partNotes holds parts'
// notes, "" for a part with none; when any part has none, the saved notes open with
// the line naming the files left without notes. A cancel, a run with no finished part
// and a failed save keep nothing and return failure as it is.
func (g *Generator) keepChunkNotes(failure error, parts []chunkPart, partNotes []string) error {
	finished := slices.DeleteFunc(slices.Clone(partNotes), func(notes string) bool { return notes == "" })
	if g.saveChunkNotes == nil || len(finished) == 0 || errors.Is(failure, context.Canceled) {
		return failure
	}
	saved := joinChunkNotes(finished)
	if missing := notesMissingLine(parts, partNotes); missing != "" {
		saved = missing + "\n\n" + saved
	}
	path, err := g.saveChunkNotes(saved)
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

// notesMissingLine names, in chunk order, every file a part with no notes carries, or
// is "" when every part has notes. A file cut into pieces, only some of which have
// notes, is marked "(partly)".
func notesMissingLine(parts []chunkPart, partNotes []string) string {
	var order []string
	written, missing := map[string]bool{}, map[string]bool{}
	for i, part := range parts {
		for _, path := range part.paths() {
			if !written[path] && !missing[path] {
				order = append(order, path)
			}
			if partNotes[i] == "" {
				missing[path] = true
			} else {
				written[path] = true
			}
		}
	}

	var listed []string
	for _, path := range order {
		if !missing[path] {
			continue
		}
		entry := "`" + path + "`"
		if written[path] {
			entry += " (partly)"
		}
		listed = append(listed, entry)
	}
	if len(listed) == 0 {
		return ""
	}
	return "**Notes missing:** no notes were written for " + strings.Join(listed, ", ") + " — add them before releasing."
}

// paths is the path of every file p carries a slice of.
func (p chunkPart) paths() []string {
	if p.file != "" {
		return []string{p.file}
	}
	fileDiffs := splitFileDiffs(p.slice)
	paths := make([]string, len(fileDiffs))
	for i, fileDiff := range fileDiffs {
		paths[i] = headerPath(fileDiff)
	}
	return paths
}
