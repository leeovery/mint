package engine

import (
	"fmt"

	"mint/internal/notes"
	"mint/internal/presenter"
)

// notesStage narrates the blocking notes stage. As the notes.Progress of the
// generation it runs around, it relays a chunked run's advance to the presenter, and
// names the part count on the completion line only once a merge has written the
// body; any other body completes as "Generated release notes".
type notesStage struct {
	p         presenter.Presenter
	succeeded func(detail, sentence string)
	parts     int
}

var _ notes.Progress = (*notesStage)(nil)

func startNotesStage(p presenter.Presenter) *notesStage {
	return &notesStage{p: p, succeeded: startBlockingStage(p, "notes", "generating release notes…")}
}

func (s *notesStage) PartsDone(promptBytes, parts, done int) {
	s.p.NotesProgress(presenter.NotesProgress{PromptBytes: promptBytes, Parts: parts, Done: done})
}

func (s *notesStage) Merging(parts int) {
	s.p.NotesMerging(presenter.NotesMerge{Parts: parts})
}

func (s *notesStage) Merged(parts int) {
	s.parts = parts
}

// done narrates the stage's success. Call it only once the generation has returned.
func (s *notesStage) done() {
	if s.parts == 0 {
		s.succeeded("generated", "Generated release notes")
		return
	}
	s.succeeded(fmt.Sprintf("generated in %d parts, merged", s.parts), fmt.Sprintf("Notes written (%d parts, merged)", s.parts))
}
